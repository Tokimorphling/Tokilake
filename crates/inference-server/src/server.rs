use crate::pb::control_command::CommandType;
use crate::pb::{
    Acknowledgement, TokiameMessage, TokilakeMessage, tokiame_message::Payload as TokiamePayload,
    tokilake_message::Payload as TokilakePayload,
};
use crate::pb::{ControlCommand, TokilakeCoordinatorService, TokilakeCoordinatorServiceServer};
use crate::{InferenceService, MakeMessage};
use common::data::ChatCompletionsData;
use common::proxy::GrpcOriginalPayload;
use dashmap::DashMap;
use futures_util::{Stream, StreamExt};
use serde_json::json;
use std::net::SocketAddr;
use std::result::Result;
use std::sync::Arc;
use std::time::Duration;
use storage::{ClientCache, Storage};
use tokio::sync::mpsc; // mpsc::Sender and mpsc::UnboundedSender are here
use tokio::time::timeout;
use tokio_stream::wrappers::ReceiverStream; // ReceiverStream is also here if needed
use tracing::{debug, error, info, warn};
use volo::FastStr;
use volo_grpc::server::{Server, ServiceBuilder};
use volo_grpc::{BoxStream, RecvStream, Request, Response, Status};

const TOKAME_SEND_TIMEOUT: Duration = Duration::from_millis(2000);

// --- Type Aliases ---
type ClientId = FastStr;
type ToClientSender = mpsc::Sender<Result<TokilakeMessage, Status>>;
type RequestId = FastStr;
type ResponseSender = mpsc::Sender<Result<TokiameMessage, Status>>;
type ResponseReceiver = mpsc::Receiver<Result<TokiameMessage, Status>>;

type ClientResponseDispatcherMap = Arc<DashMap<RequestId, ResponseSender>>;
type GlobalActiveClientsMap = Arc<DashMap<ClientId, ClientComms>>;

// --- Dummy ClientComms for context ---
#[derive(Debug, Clone)]

pub struct ClientComms {
    to_client_tx:        ToClientSender,
    response_dispatcher: ClientResponseDispatcherMap,
}

async fn handle_client_messages(
    mut incoming_messages_from_client: RecvStream<TokiameMessage>,
    client_namespace: ClientId,
    to_client_tx: ToClientSender,
    client_response_dispatcher: ClientResponseDispatcherMap,
    global_active_clients: GlobalActiveClientsMap,
) {
    info!(client = %client_namespace, "Starting message handling loop for client.");

    while let Some(message_result) = incoming_messages_from_client.next().await {
        match message_result {
            Ok(tokiame_msg) => {
                debug!(client = %client_namespace, id = %tokiame_msg.tokiame_id, "received message payload: {:?}", &tokiame_msg.payload);

                let task_id = &tokiame_msg.tokiame_id;

                match &tokiame_msg.payload {
                    Some(TokiamePayload::Heartbeat(hb)) => {
                        info!(
                            client = %client_namespace,
                            timestamp = ?hb.timestamp,
                            status = %hb.current_status.to_string(),
                            "Received heartbeat"
                        );
                        let ack = Acknowledgement {
                            message_id_acknowledged: tokiame_msg.tokiame_id,
                            success: true,
                            ..Default::default()
                        };
                        let ack_message = TokilakeMessage::make_message(
                            "heartbeat_ack".into(),
                            Some(TokilakePayload::Ack(ack)),
                        );
                        if to_client_tx.send(Ok(ack_message)).await.is_err() {
                            error!(client = %client_namespace, "Failed to send heartbeat ack. Client might be disconnected.");
                            break;
                        }
                    }
                    Some(TokiamePayload::Chunk(stream_chunk)) => {
                        let chunk_request_id = stream_chunk.request_id.clone();
                        info!(
                            client = %client_namespace,
                            request_id = %chunk_request_id,
                            "Received chunk for request."
                        );
                        if let Some(entry) = client_response_dispatcher.get(task_id) {
                            let to_actor_tx = entry.value().clone();

                            if let Err(e) = to_actor_tx.send(Ok(tokiame_msg.clone())).await {
                                warn!(client = %client_namespace, task_id = %task_id, "Failed to send message to actor mailbox: {}.", e);
                            }
                        } else {
                            warn!(client = %client_namespace, request_id = %task_id, "No actor mailbox found for message's task_id. Notifying Tokiame to shutdown.");
                            cancel_task(&to_client_tx, task_id, &client_namespace).await;
                            continue;
                        }
                    }
                    Some(TokiamePayload::Registration(_)) => {
                        warn!(client = %client_namespace, "Received unexpected Registration message after initial registration. Ignoring.");
                    }
                    Some(TokiamePayload::Models(models)) => {
                        debug!(mdoels= ?models);
                        let task_id = &tokiame_msg.tokiame_id.clone();
                        if let Some(sender) = client_response_dispatcher
                            .get(task_id)
                            .map(|entry| entry.value().clone())
                        {
                            if let Err(e) = sender.send(Ok(tokiame_msg)).await {
                                error!(error=%e, "error sending message");
                                client_response_dispatcher.remove(task_id);
                            }
                            debug!("sucessfully sending model infoamtion");
                        } else {
                            warn!(task_id=%task_id, "task id not found");
                        }
                    }

                    Some(other_payload_type) => {
                        debug!(client = %client_namespace, id = %tokiame_msg.tokiame_id, "Received unhandled payload type: {:?}", other_payload_type);
                    }
                    None => {
                        warn!(client = %client_namespace, id = %tokiame_msg.tokiame_id, "Received message with no payload. Ignoring.");
                    }
                }
            }
            Err(e) => {
                error!(client = %client_namespace, "Error receiving message from client: {:?}. Disconnecting.", e);
                break;
            }
        }
    }

    // --- Cleanup Phase ---
    info!(client = %client_namespace, "Client stream ended or error occurred. Initiating cleanup.");

    if global_active_clients.remove(&client_namespace).is_some() {
        info!(client = %client_namespace, "Successfully removed from global active clients map.");
    } else {
        warn!(client = %client_namespace, "Attempted to remove client from global map, but it was not found (might have been removed concurrently or due to registration issue).");
    }

    let keys: Vec<_> = client_response_dispatcher
        .iter()
        .map(|e| e.key().clone())
        .collect();
    let senders_to_notify: Vec<_> = keys
        .into_iter()
        .filter_map(|k| {
            client_response_dispatcher
                .get(&k)
                .map(|e| (k, e.value().clone()))
        })
        .collect();

    if !senders_to_notify.is_empty() {
        info!(client = %client_namespace, count = senders_to_notify.len(), "Notifying pending requests of client disconnection.");
        for (request_id, sender) in senders_to_notify {
            let error_status = Status::unavailable(format!(
                "Client '{client_namespace}' disconnected while request '{request_id}' was \
                 pending."
            ));
            if sender.send(Err(error_status)).await.is_err() {
                warn!(client = %client_namespace, request_id = %request_id, "Failed to send disconnect notification to a pending request; receiver likely already dropped.");
            }
        }
        client_response_dispatcher.clear(); // Explicitly clear the map.
        info!(client = %client_namespace, "Finished notifying pending requests. Dispatcher is now clear.");
    } else {
        info!(client = %client_namespace, "Response dispatcher was empty or already cleared. No pending requests to notify.");
    }

    info!(client = %client_namespace, "Cleanup complete for client.");
}

impl InferenceService for InferenceServer {
    async fn chat_completion(
        self: Arc<Self>,
        namespace: FastStr,
        request: ChatCompletionsData,
    ) -> impl Stream<Item = Result<TokiameMessage, Status>> {
        let task_id = request.task_id.clone();
        info!(namespace = %namespace, task_id = %task_id, model=%request.model_name, "Received chat completion request.");

        let client_comms = match self.active_clients.get(&namespace) {
            Some(entry) => entry.value().clone(),
            None => {
                let msg =
                    format!("Client namespace '{namespace}' not found for task_id '{task_id}'");
                error!("{}", msg);
                let (tx, rx) = mpsc::channel(1);
                tx.send(Err(Status::internal(msg))).await.ok();
                return ReceiverStream::new(rx);
            }
        };

        if client_comms.response_dispatcher.contains_key(&task_id) {
            let msg = format!("Task ID '{task_id}' already exists for client '{namespace}'");
            error!("{}", msg);
            let (tx, rx) = mpsc::channel(1);
            tx.send(Err(Status::already_exists(msg))).await.ok();
            return ReceiverStream::new(rx);
        }

        const USER_FINAL_RESPONSE_CHANNEL_SIZE: usize = 1024;
        const ACTOR_CHANNEL_SIZE: usize = 256;
        let (user_final_tx, user_final_rx) = mpsc::channel(USER_FINAL_RESPONSE_CHANNEL_SIZE);
        let (actor_tx, actor_rx) = mpsc::channel(ACTOR_CHANNEL_SIZE);

        client_comms
            .response_dispatcher
            .insert(task_id.clone(), actor_tx.clone());

        let actor_params = (
            task_id.clone(),
            namespace.clone(),
            actor_rx,
            user_final_tx.clone(),
            client_comms.response_dispatcher.clone(),
            client_comms.to_client_tx.clone(),
        );
        tokio::spawn(async move {
            per_task_actor(
                actor_params.0,
                actor_params.1,
                actor_params.2,
                actor_params.3,
                actor_params.4,
                actor_params.5,
            )
            .await
        });

        let wrapped_payload = GrpcOriginalPayload::ChatCompletionsRequest(request);
        let tokilake_request_payload: TokilakePayload = wrapped_payload.into();
        debug!(payload=?tokilake_request_payload);
        let message_to_send =
            TokilakeMessage::make_message(task_id.clone(), Some(tokilake_request_payload));

        if let Err(e) =
            Self::send_to_client(&client_comms, &namespace, &task_id, message_to_send).await
        {
            user_final_tx.send(Err(e)).await.ok();
            client_comms.response_dispatcher.remove(&task_id);
        }

        ReceiverStream::new(user_final_rx)
    }

    async fn models(&self, task_id: FastStr, namespace: FastStr) -> Result<TokiameMessage, Status> {
        info!(namespace = %namespace, task_id = %task_id, "Received models request.");
        let (client_comms, response_tx, mut response_rx) =
            match self.prepare_client_comms(&namespace, &task_id, 1).await {
                Ok((c, tx, rx)) => (c, tx, rx),
                Err(e) => {
                    error!("{e}");
                    return Err(e);
                }
            };

        let payload = TokilakePayload::Command(ControlCommand {
            command_type: CommandType::MODELS,
            request_id: task_id.clone(),
            ..Default::default()
        });
        let message = TokilakeMessage::make_message(task_id.clone(), Some(payload));

        if let Err(e) = Self::send_to_client(&client_comms, &namespace, &task_id, message).await {
            let _ = response_tx.send(Err(e)).await;
        }

        let result = match response_rx.recv().await {
            Some(Ok(msg)) => {
                info!(%namespace, %task_id, "Successfully received models response.");
                Ok(msg)
            }
            Some(Err(status)) => {
                warn!(%namespace, %task_id, "Received error status for models request: {}", status);
                Err(status)
            }
            None => {
                warn!(%namespace, %task_id, "Models response channel closed prematurely (Tokiame likely disconnected or task canceled).");
                Err(Status::unavailable(format!(
                    "Models response channel closed for task_id '{task_id}', client '{namespace}'"
                )))
            }
        };

        client_comms.response_dispatcher.remove(&task_id);
        info!(%namespace, %task_id, "Removed response channel for models request from dispatcher.");
        result
    }
}

async fn per_task_actor(
    task_id: FastStr,
    client_namespace: ClientId,
    mut actor_mailbox_rx: ResponseReceiver, // 改为 ResponseReceiver (拼写)
    user_response_tx: ResponseSender,
    response_dispatcher_map: ClientResponseDispatcherMap,
    tokiame_command_tx: ToClientSender,
) {
    const PER_TASK_ACTOR_INACTIVITY_TIMEOUT: Duration = Duration::from_secs(2 * 60);
    info!(taskid=%task_id, client_namespace=%client_namespace, "Per-task actor started. Inactivity timeout: {:?}", PER_TASK_ACTOR_INACTIVITY_TIMEOUT);
    loop {
        tokio::select! {
            biased;
            maybe_message = actor_mailbox_rx.recv() => {
                match maybe_message {
                    Some(Ok(message)) => {
                        match message.payload {
                            Some(TokiamePayload::Chunk(_)) => {
                                if user_response_tx.send(Ok(message)).await.is_err() {
                                    error!(%task_id, %client_namespace, "User channel dropped. Actor stopping & notifying Tokiame.");
                                    cancel_task(&tokiame_command_tx, &task_id, &client_namespace).await;

                                    break;
                                }
                            }
                            Some(_) => {
                                warn!(%task_id, %client_namespace, "Per-task actor received unexpected payload type. Ignoring.");
                            }
                            None => {
                                warn!(%task_id, %client_namespace, "Per-task actor received message with no payload. Ignoring.");
                            }
                        }
                    }
                    Some(Err(status)) => {
                        error!(%task_id, %client_namespace, "Actor received explicit error status: {:?}. Notifying user and stopping.", status);
                        if user_response_tx.send(Err(status)).await.is_err() {
                             warn!(%task_id, %client_namespace, "Failed to send error status to user; user channel might already be closed.");
                        }
                        break;
                    }
                    None => {
                        info!(%task_id, %client_namespace, "Actor mailbox channel closed. Stream finished from sender side.");

                        break;
                    }
                }
            }

            _ = tokio::time::sleep(PER_TASK_ACTOR_INACTIVITY_TIMEOUT) => {
                warn!(%task_id, %client_namespace, "Actor timed out due to inactivity after {:?}.", PER_TASK_ACTOR_INACTIVITY_TIMEOUT);


                let timeout_status = Status::deadline_exceeded(format!(
                    "Task '{task_id}' timed out due to inactivity from the inference backend."
                ));
                if user_response_tx.send(Err(timeout_status)).await.is_err() {
                    warn!(%task_id, %client_namespace, "Failed to send timeout status to user; user channel might already be closed.");
                }


                info!(%task_id, %client_namespace, "Notifying Tokiame to cancel task due to actor inactivity timeout.");
                cancel_task(&tokiame_command_tx, &task_id, &client_namespace).await;


                break;
            }
        }
    }

    if response_dispatcher_map.remove(&task_id).is_some() {
        info!(%task_id, %client_namespace, "Per-task actor stopped and removed itself from dispatcher.");
    } else {
        warn!(%task_id, %client_namespace, "Per-task actor stopped but was not found in dispatcher (possibly already removed by client disconnect or other cleanup).");
    }

    info!(taskid=%task_id, client_namespace=%client_namespace, "Per-task actor finished.");
}

#[derive(Clone)]
pub struct InferenceServer {
    pub active_clients: GlobalActiveClientsMap,
    pub db:             Arc<Storage<ClientCache>>,
}

impl InferenceServer {
    pub fn new(db: Arc<Storage<ClientCache>>) -> Self {
        InferenceServer {
            active_clients: Arc::new(DashMap::with_capacity(100)),
            db,
        }
    }
    async fn prepare_client_comms(
        &self,
        namespace: &str,
        task_id: &str,
        channel_size: usize,
    ) -> Result<(ClientComms, ResponseSender, ResponseReceiver), Status> {
        let (response_tx, response_rx) = mpsc::channel(channel_size);
        let client_comms = match self.active_clients.get(namespace) {
            Some(entry) => entry.value().clone(),
            None => {
                let message = format!(
                    "Client with namespace [{namespace}] not found for task_id [{task_id}]."
                );
                if response_tx
                    .send(Err(Status::not_found(message.clone())))
                    .await
                    .is_err()
                {
                    error!(
                        "Failed to send error to response channel for task_id [{}]",
                        task_id
                    );
                }
                return Err(Status::not_found(message));
            }
        };
        if client_comms.response_dispatcher.contains_key(task_id) {
            let message = format!(
                "Task ID [{task_id}] already exists in dispatcher for client [{namespace}]."
            );
            if response_tx
                .send(Err(Status::already_exists(message.clone())))
                .await
                .is_err()
            {
                error!(
                    "Failed to send error to response channel for task_id [{}]",
                    task_id
                );
            }
            return Err(Status::already_exists(message));
        }

        client_comms
            .response_dispatcher
            .insert(FastStr::new(task_id), response_tx.clone());

        info!(namespace = %namespace, task_id = %task_id, "Response channel registered in client's dispatcher.");

        Ok((client_comms, response_tx, response_rx))
    }

    async fn send_to_client(
        client_comms: &ClientComms,
        namespace: &str,
        task_id: &str,
        message: TokilakeMessage,
    ) -> Result<(), Status> {
        match timeout(
            TOKAME_SEND_TIMEOUT,
            client_comms.to_client_tx.send(Ok(message)),
        )
        .await
        {
            Ok(Ok(_)) => {
                info!(namespace = %namespace, task_id = %task_id, "Request successfully sent to client.");
                Ok(())
            }
            Ok(Err(e)) => {
                let error_message = format!(
                    "Failed to send request to client '{namespace}' for task_id [{task_id}]: \
                     channel closed: {e:?}"
                );
                error!("{}", error_message);
                client_comms.response_dispatcher.remove(task_id);
                info!(namespace = %namespace, task_id = %task_id, "Removed response channel due to client channel closed.");
                Err(Status::unavailable(error_message))
            }
            Err(_) => {
                let error_message = format!(
                    "Timeout sending request to client '{namespace}' for task_id [{task_id}]. \
                     Client might be overwhelmed."
                );
                error!("{}", error_message);
                client_comms.response_dispatcher.remove(task_id);
                info!(namespace = %namespace, task_id = %task_id, "Removed response channel due to timeout sending to client.");
                Err(Status::deadline_exceeded(error_message))
            }
        }
    }
}

impl TokilakeCoordinatorService for InferenceServer {
    async fn establish_tokiame_link(
        &self,
        req: Request<RecvStream<TokiameMessage>>,
    ) -> Result<Response<BoxStream<'static, Result<TokilakeMessage, Status>>>, Status> {
        info!("Client connection attempt: waiting for registration message.");
        let mut client_stream = req.into_inner();

        let registration_msg = client_stream
            .next()
            .await
            .ok_or_else(|| {
                Status::invalid_argument("Client stream ended before registration message.")
            })?
            .map_err(|e| Status::internal(format!("Failed to read from client stream: {e}")))?;

        let namespace = registration_msg
            .payload
            .and_then(|p| match p {
                TokiamePayload::Registration(reg) => Some(reg.tokiame_namespace),
                _ => None,
            })
            .ok_or_else(|| {
                Status::invalid_argument("First message was not a valid Registration message.")
            })?;

        let (to_client_tx, to_client_rx) = mpsc::channel(10240);

        let client_comms = ClientComms {
            to_client_tx:        to_client_tx.clone(),
            response_dispatcher: Arc::new(DashMap::with_capacity(100)),
        };

        if self.active_clients.contains_key(&namespace) {
            warn!(
                "Client with namespace '{}' already exists. Registration rejected.",
                namespace
            );
            return Err(Status::already_exists(format!(
                "Client with namespace '{namespace}' already exists."
            )));
        }
        self.active_clients
            .insert(namespace.clone(), client_comms.clone());
        info!("Client '{}' successfully registered.", namespace);

        let global_active_clients_for_task = self.active_clients.clone();
        let client_specific_dispatcher_for_task = client_comms.response_dispatcher.clone();

        tokio::spawn(async move {
            handle_client_messages(
                client_stream,
                namespace,    // namespace is moved here
                to_client_tx, // to_client_tx is moved
                client_specific_dispatcher_for_task,
                global_active_clients_for_task,
            )
            .await;
        });

        Ok(Response::new(Box::pin(ReceiverStream::new(to_client_rx))))
    }
}

pub async fn run_inference_server(addr: SocketAddr, db: Storage<ClientCache>) -> InferenceServer {
    info!("Inference Server preparing to run on: {}", addr);
    let addr = volo::net::Address::from(addr);
    let server_instance = InferenceServer::new(Arc::new(db));

    // Clone server_instance for the spawned gRPC server task (cheap clone)
    let server_for_task = server_instance.clone();
    tokio::spawn(async move {
        info!("Starting gRPC server on {}...", addr);
        Server::new()
            .add_service(
                ServiceBuilder::new(TokilakeCoordinatorServiceServer::new(server_for_task)).build(),
            )
            .run(addr)
            .await
            .unwrap_or_else(|e| panic!("gRPC Server failed to run: {e:?}")); // Consider more graceful error handling than panic
    });
    server_instance // Return the original instance for potential further use
}

pub fn map_to_http_response(message: Result<TokiameMessage, Status>) -> FastStr {
    let value = match message {
        Ok(message) => match &message.payload {
            Some(TokiamePayload::Models(models_payload)) => {
                let supported_models = &models_payload.supported_models;
                let models: Vec<serde_json::Value> = supported_models
                    .iter()
                    .map(|model| {
                        json!({
                            "model": model.id,
                            "type": model.r#type,
                            "backend_engine": model.backend_engine
                        })
                    })
                    .collect();
                json!({
                    "models": models
                })
            }
            _ => {
                json!({
                    "message": "Unsupported payload type or no payload"
                })
            }
        },
        Err(e) => {
            json!({"error": e.message()})
        }
    };
    serde_json::to_string(&value).unwrap_or_default().into()
}

pub async fn map_to_sse_stream<S>(input: S) -> impl Stream<Item = GrpcOriginalPayload>
where
    S: Stream<Item = Result<TokiameMessage, Status>> + Send,
{
    input.then(
        async move |result_tokiame_message| match result_tokiame_message {
            Ok(tokiame_message) => {
                if let Some(payload) = tokiame_message.payload {
                    match payload {
                        TokiamePayload::Chunk(_) => GrpcOriginalPayload::from(payload),
                        other_payload => {
                            debug!(
                                "map_to_sse_stream_no_filter: Unsupported payload type: {:?}, \
                                 mapping to Empty",
                                other_payload
                            );
                            GrpcOriginalPayload::Empty
                        }
                    }
                } else {
                    debug!(
                        "map_to_sse_stream_no_filter: TokiameMessage with no payload, mapping to \
                         Empty"
                    );
                    GrpcOriginalPayload::Empty
                }
            }
            Err(status) => {
                error!(
                    "map_to_sse_stream_no_filter: Error in input stream: {:?}, mapping to Empty",
                    status
                );
                GrpcOriginalPayload::Empty
            }
        },
    )
}

async fn cancel_task(
    tokiame_command_tx: &ToClientSender,
    task_id: &RequestId,
    client_namespace: &ClientId,
) {
    let command_payload = TokilakePayload::Command(ControlCommand {
        command_type: CommandType::SHUTDOWN_GRACEFULLY,
        request_id: task_id.clone(),
        ..Default::default()
    });
    let command_message = TokilakeMessage::make_message(task_id.clone(), Some(command_payload));
    if let Err(e) = tokiame_command_tx.send(Ok(command_message)).await {
        error!(task_id=%task_id, client=%client_namespace, "Failed to send SHUTDOWN_GRACEFULLY to Tokiame: {:?}", e);
    }
}
