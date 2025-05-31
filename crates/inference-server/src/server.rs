use crate::pb::control_command::CommandType; // Ensuring this is used as requested
use crate::pb::{
    Acknowledgement, TokiameMessage, TokilakeMessage, tokiame_message::Payload as TokiamePayload,
    tokilake_message::Payload as TokilakePayload,
};
use crate::pb::{
    ControlCommand, StreamedInferenceChunk, TokilakeCoordinatorService,
    TokilakeCoordinatorServiceServer,
};
use crate::{InferenceService, MakeMessage};
use common::data::ChatCompletionsData;
use common::proxy::GrpcOriginalPayload;
use dashmap::DashMap;
use futures_util::{Stream, StreamExt};
// use serde::Serialize;
use serde_json::json;
use std::net::SocketAddr;
use std::result::Result;
use std::sync::Arc;
use std::sync::atomic::{AtomicI32, Ordering};
use storage::{ClientCache, Storage};
use tokio::sync::mpsc; // mpsc::Sender and mpsc::UnboundedSender are here
use tokio_stream::wrappers::ReceiverStream; // ReceiverStream is also here if needed
use tracing::{debug, error, info, warn};
use volo::FastStr;
use volo_grpc::server::{Server, ServiceBuilder};
use volo_grpc::{BoxStream, RecvStream, Request, Response, Status};

// --- Type Aliases ---
type ClientId = FastStr;
type ToClientSender = mpsc::Sender<Result<TokilakeMessage, Status>>;
type RequestId = FastStr;
type ResponseSender = mpsc::Sender<Result<TokiameMessage, Status>>;
type ClientResponseDispatcherMap = Arc<DashMap<RequestId, ResponseSender>>;
type GlobalActiveClientsMap = Arc<DashMap<ClientId, ClientComms>>;

// --- Dummy ClientComms for context ---
#[derive(Debug, Clone)]
pub struct ClientComms {
    to_client_tx:         ToClientSender,
    response_dispatcher:  ClientResponseDispatcherMap,
    tokilake_message_cnt: Arc<AtomicI32>,
    tokiame_message_cnt:  Arc<AtomicI32>,
    tokens_sent:          Arc<AtomicI32>,
    tokens_recv:          Arc<AtomicI32>,
}

enum ProcessChunkOutcome {
    SentSuccessfully,
    SenderUnavailable,
    SendFailed,
}

async fn process_chunk_for_client(
    stream_chunk: &StreamedInferenceChunk,
    maybe_response_tx: Option<ResponseSender>,
) -> ProcessChunkOutcome {
    let request_id = stream_chunk.request_id.clone();

    if let Some(response_tx) = maybe_response_tx {
        let message_payload = TokiamePayload::Chunk(stream_chunk.clone());
        let tokiame_message =
            TokiameMessage::make_message(request_id.clone(), Some(message_payload));

        if response_tx.send(Ok(tokiame_message)).await.is_err() {
            error!(request_id = %request_id, "Error sending chunk via provided sender. Receiver likely dropped.");
            ProcessChunkOutcome::SendFailed
        } else {
            debug!(request_id = %request_id, "Successfully forwarded chunk via provided sender.");
            ProcessChunkOutcome::SentSuccessfully
        }
    } else {
        warn!(request_id = %request_id, "No sender provided for chunk. Request ID likely not found in dispatcher by caller.");
        ProcessChunkOutcome::SenderUnavailable
    }
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
                debug!(client = %client_namespace, id = %tokiame_msg.tokiame_id, "received message payload: {:?}", tokiame_msg.payload);
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

                        let maybe_sender_for_chunk: Option<ResponseSender> =
                            client_response_dispatcher
                                .get(&chunk_request_id)
                                .map(|entry| entry.value().clone());

                        let outcome =
                            process_chunk_for_client(stream_chunk, maybe_sender_for_chunk).await;

                        match outcome {
                            ProcessChunkOutcome::SentSuccessfully => {}
                            ProcessChunkOutcome::SendFailed
                            | ProcessChunkOutcome::SenderUnavailable => {
                                client_response_dispatcher.remove(&chunk_request_id);

                                let command_payload = TokilakePayload::Command(ControlCommand {
                                    command_type: CommandType::SHUTDOWN_GRACEFULLY,
                                    request_id: chunk_request_id.clone(),
                                    ..Default::default()
                                });
                                let command_message = TokilakeMessage::make_message(
                                    chunk_request_id.clone(),
                                    Some(command_payload),
                                );

                                if to_client_tx.send(Ok(command_message)).await.is_err() {
                                    error!(client = %client_namespace, request_id = %chunk_request_id, "Error sending SHUTDOWN_GRACEFULLY. Client connection likely lost.");
                                    break; // Exit loop, proceed to cleanup
                                }

                                if matches!(outcome, ProcessChunkOutcome::SendFailed) {
                                    info!(client = %client_namespace, request_id = %chunk_request_id, "Removed sender from dispatcher due to send failure (receiver dropped) and sent SHUTDOWN_GRACEFULLY.");
                                } else {
                                    warn!(client = %client_namespace, request_id = %chunk_request_id, "Sender unavailable for chunk (request ID not in dispatcher). Sent SHUTDOWN_GRACEFULLY.");
                                }
                            }
                        }
                    }
                    Some(TokiamePayload::Registration(_)) => {
                        warn!(client = %client_namespace, "Received unexpected Registration message after initial registration. Ignoring.");
                    }
                    Some(TokiamePayload::Models(models)) => {
                        debug!(mdoels= ?models);
                        let task_id = &tokiame_msg.tokiame_id;
                        if let Some(sender) = client_response_dispatcher
                            .get(task_id)
                            .map(|entry| entry.value().clone())
                        {
                            let message = tokiame_msg;
                            if let Err(e) = sender.send(Ok(message)).await {
                                error!(error=%e, "error sending message");
                                // client_response_dispatcher.remove(task_id);
                            }
                            debug!("sucessfully sending model infoamtion");
                        } else {
                            warn!(task_id=%task_id, "task id not found");
                        }
                    }

                    Some(other_payload_type) => {
                        // It's good practice to log unhandled known payload types, or handle them.
                        debug!(client = %client_namespace, id = %tokiame_msg.tokiame_id, "Received unhandled payload type: {:?}", other_payload_type);
                    }
                    None => {
                        warn!(client = %client_namespace, id = %tokiame_msg.tokiame_id, "Received message with no payload. Ignoring.");
                    }
                }
            }
            Err(e) => {
                error!(client = %client_namespace, "Error receiving message from client: {:?}. Disconnecting.", e);
                break; // Exit message handling loop
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

    let senders_to_notify: Vec<(_, _)> = client_response_dispatcher
        .iter()
        .map(|entry| (entry.key().clone(), entry.value().clone())) // Clones FastStr and UnboundedSender (cheap)
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
        let active_clients_clone = &self.active_clients;

        // let namespace = namespace_str;

        let task_id = request.task_id.clone();

        info!(namespace = %namespace, task_id = %task_id, "Received chat completion request.");

        let (response_tx, response_rx) = mpsc::channel(256);

        let client_comms = match active_clients_clone.get(&namespace) {
            // 使用 owned namespace
            Some(entry) => entry.value().clone(),
            None => {
                let message = format!(
                    "Client with namespace [{namespace}] not found for task_id [{task_id}]."
                );
                error!("{}", message);
                if response_tx
                    .send(Err(Status::not_found(message)))
                    .await
                    .is_err()
                {
                    error!(
                        "Failed to send error to response channel for task_id [{}]",
                        task_id
                    );
                }
                return ReceiverStream::new(response_rx);
            }
        };

        if client_comms.response_dispatcher.contains_key(&task_id) {
            let message = format!(
                "Task ID [{task_id}] already exists in dispatcher for client [{namespace}]."
            );
            error!("{}", message);
            if response_tx
                .send(Err(Status::already_exists(message)))
                .await
                .is_err()
            {
                error!(
                    "Failed to send error to response channel for task_id [{}]",
                    task_id
                );
            }
            return ReceiverStream::new(response_rx);
        }

        client_comms
            .response_dispatcher
            .insert(task_id.clone(), response_tx.clone());
        info!(namespace = %namespace, task_id = %task_id, "Response channel registered in client's dispatcher.");

        let wrapped_payload = GrpcOriginalPayload::ChatCompletionsRequest(request);
        let tokilake_request_payload: TokilakePayload = wrapped_payload.into();
        let message_to_send =
            TokilakeMessage::make_message(task_id.clone(), Some(tokilake_request_payload));

        if let Err(e) = client_comms.to_client_tx.send(Ok(message_to_send)).await {
            let error_message = format!(
                "Failed to send request to client '{namespace}' for task_id [{task_id}]: \
                 {e:?}"
            );
            error!("{}", error_message);
            client_comms.response_dispatcher.remove(&task_id);
            info!(namespace = %namespace, task_id = %task_id, "Removed response channel from dispatcher due to client send failure.");
            if response_tx
                .send(Err(Status::internal(error_message)))
                .await
                .is_err()
            {
                error!(
                    "Failed to send error to response channel for task_id [{}]",
                    task_id
                );
            }
        } else {
            client_comms
                .tokilake_message_cnt
                .fetch_add(1, Ordering::Relaxed);
            info!(namespace = %namespace, task_id = %task_id, "Chat completion request successfully sent to client.");
        }

        ReceiverStream::new(response_rx)
    }

    async fn models(&self, task_id: FastStr, namespace: FastStr) -> Result<TokiameMessage, Status> {
        let payload = TokilakePayload::Command(ControlCommand {
            command_type: CommandType::MODELS,
            request_id: task_id.clone(),
            ..Default::default()
        });

        let message = TokilakeMessage::make_message(task_id.clone(), Some(payload));

        let (response_tx, mut response_rx) = mpsc::channel(1);
        let client_comms = match self.active_clients.get(&namespace) {
            Some(entry) => entry.value().clone(),
            None => {
                let message = format!(
                    "Client with namespace [{namespace}] not found for task_id [{task_id}]."
                );
                error!("{}", message);
                // If send fails, the stream will simply end after this error.
                let _ = response_tx.send(Err(Status::not_found(message))).await;
                return response_rx
                    .recv()
                    .await
                    .unwrap_or(Err(Status::internal("internal error")));
            }
        };

        client_comms
            .response_dispatcher
            .insert(task_id.clone(), response_tx.clone());
        info!(namespace = %namespace, task_id = %task_id, "Response channel registered in client's dispatcher.");

        if let Err(e) = client_comms.to_client_tx.send(Ok(message)).await {
            let error_message = format!(
                "Failed to send request to client '{namespace}' for task_id [{task_id}]: {e:?}"
            );
            error!("{}", error_message);
        } else {
            info!(namespace = %namespace, task_id = %task_id, "Model request successfully sent to client.");
        }
        response_rx
            .recv()
            .await
            .unwrap_or(Err(Status::internal("internal error")))
    }
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
}

impl TokilakeCoordinatorService for InferenceServer {
    async fn establish_tokiame_link(
        &self,
        req: Request<RecvStream<TokiameMessage>>,
    ) -> Result<Response<BoxStream<'static, Result<TokilakeMessage, Status>>>, Status> {
        info!("Client connection attempt: waiting for registration message.");
        let mut client_stream = req.into_inner();

        // Gracefully handle stream ending before registration or read errors
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

        let (to_client_tx, to_client_rx) = mpsc::channel(1024);

        let client_comms = ClientComms {
            to_client_tx:         to_client_tx.clone(), // Cheap UnboundedSender clone
            response_dispatcher:  Arc::new(DashMap::with_capacity(100)),
            tokiame_message_cnt:  Arc::new(AtomicI32::new(0)),
            tokilake_message_cnt: Arc::new(AtomicI32::new(0)),
            tokens_sent:          Arc::new(AtomicI32::new(0)),
            tokens_recv:          Arc::new(AtomicI32::new(0)),
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
