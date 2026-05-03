use axum::{
    extract::{
        ws::{Message, WebSocket},
        Query, State, WebSocketUpgrade,
    },
    http::StatusCode,
    response::IntoResponse,
    routing::{get, post},
    Json, Router,
};
use futures_util::{SinkExt, StreamExt};
use serde::{Deserialize, Serialize};
use std::{collections::HashMap, sync::Arc, time::Duration};
use tokilake_core::{
    error::{ErrorMessage, TunnelError},
    protocol::*,
    session::{GatewaySession, InFlightRequest, SessionManager},
};
use tokio::{
    io::{AsyncReadExt, AsyncWriteExt},
    sync::{mpsc, RwLock},
};
use tracing::{info, warn};

#[derive(Clone)]
struct AppState {
    token:           String,
    session_manager: Arc<SessionManager>,
    registry:        Arc<MemoryWorkerRegistry>,
}

struct MemoryWorkerRegistry {
    workers: parking_lot::RwLock<HashMap<i32, WorkerEntry>>,
}

struct WorkerEntry {
    worker_id: i32,
    namespace: String,
    models:    Vec<String>,
}

impl MemoryWorkerRegistry {
    fn new() -> Self {
        Self {
            workers: parking_lot::RwLock::new(HashMap::new()),
        }
    }

    fn register_worker(
        &self,
        namespace: &str,
        models: &[String],
        group: &str,
        backend_type: &str,
    ) -> Result<RegisterResult, TunnelError> {
        if namespace.trim().is_empty() {
            return Err(TunnelError::protocol("namespace is required"));
        }

        let mut workers = self.workers.write();
        let worker_id = (workers.len() as i32) + 1;
        workers.insert(worker_id, WorkerEntry {
            worker_id,
            namespace: namespace.to_string(),
            models: models.to_vec(),
        });

        info!(
            "worker registered: id={} namespace={} models={:?}",
            worker_id, namespace, models
        );

        Ok(RegisterResult {
            worker_id,
            channel_id: worker_id,
            namespace: namespace.to_string(),
            group: group.to_string(),
            models: models.to_vec(),
            backend_type: backend_type.to_string(),
            status: 1,
        })
    }

    fn update_heartbeat(
        &self,
        worker_id: i32,
        current_models: &[String],
    ) -> Result<(), TunnelError> {
        let mut workers = self.workers.write();
        if let Some(entry) = workers.get_mut(&worker_id) {
            if !current_models.is_empty() {
                entry.models = current_models.to_vec();
            }
        }
        Ok(())
    }

    fn cleanup_worker(&self, worker_id: i32) -> Result<(), TunnelError> {
        let mut workers = self.workers.write();
        workers.remove(&worker_id);
        info!("worker removed: id={}", worker_id);
        Ok(())
    }
}

/// WebSocket stream wrapper for smux compatibility.
struct WebSocketStream {
    rx:     mpsc::Receiver<Vec<u8>>,
    tx:     mpsc::Sender<Vec<u8>>,
    buffer: Vec<u8>,
}

impl WebSocketStream {
    fn new(rx: mpsc::Receiver<Vec<u8>>, tx: mpsc::Sender<Vec<u8>>) -> Self {
        Self {
            rx,
            tx,
            buffer: Vec::new(),
        }
    }
}

impl tokio::io::AsyncRead for WebSocketStream {
    fn poll_read(
        mut self: std::pin::Pin<&mut Self>,
        cx: &mut std::task::Context<'_>,
        buf: &mut tokio::io::ReadBuf<'_>,
    ) -> std::task::Poll<std::io::Result<()>> {
        if !self.buffer.is_empty() {
            let n = std::cmp::min(buf.remaining(), self.buffer.len());
            buf.put_slice(&self.buffer[..n]);
            self.buffer.drain(..n);
            return std::task::Poll::Ready(Ok(()));
        }

        match self.rx.poll_recv(cx) {
            std::task::Poll::Ready(Some(data)) => {
                if data.is_empty() {
                    // Empty data means EOF
                    return std::task::Poll::Ready(Ok(()));
                }
                let n = std::cmp::min(buf.remaining(), data.len());
                buf.put_slice(&data[..n]);
                if n < data.len() {
                    self.buffer.extend_from_slice(&data[n..]);
                }
                std::task::Poll::Ready(Ok(()))
            }
            std::task::Poll::Ready(None) => {
                // Channel closed - return 0 bytes to indicate EOF
                std::task::Poll::Ready(Ok(()))
            }
            std::task::Poll::Pending => std::task::Poll::Pending,
        }
    }
}

impl tokio::io::AsyncWrite for WebSocketStream {
    fn poll_write(
        mut self: std::pin::Pin<&mut Self>,
        _cx: &mut std::task::Context<'_>,
        buf: &[u8],
    ) -> std::task::Poll<std::io::Result<usize>> {
        match self.tx.try_send(buf.to_vec()) {
            Ok(()) => std::task::Poll::Ready(Ok(buf.len())),
            Err(mpsc::error::TrySendError::Full(_)) => {
                // Channel is full, we need to wait
                // For simplicity, we'll just drop the data in this case
                // In a real implementation, you'd want to properly handle backpressure
                std::task::Poll::Ready(Ok(buf.len()))
            }
            Err(_) => std::task::Poll::Ready(Err(std::io::Error::new(
                std::io::ErrorKind::BrokenPipe,
                "channel closed",
            ))),
        }
    }

    fn poll_flush(
        self: std::pin::Pin<&mut Self>,
        _cx: &mut std::task::Context<'_>,
    ) -> std::task::Poll<std::io::Result<()>> {
        std::task::Poll::Ready(Ok(()))
    }

    fn poll_shutdown(
        self: std::pin::Pin<&mut Self>,
        _cx: &mut std::task::Context<'_>,
    ) -> std::task::Poll<std::io::Result<()>> {
        std::task::Poll::Ready(Ok(()))
    }
}

#[tokio::main]
async fn main() {
    tracing_subscriber::fmt::init();

    let addr = std::env::args()
        .position(|a| a == "-addr")
        .and_then(|i| std::env::args().nth(i + 1))
        .unwrap_or_else(|| ":18080".to_string());

    let token = std::env::args()
        .position(|a| a == "-token")
        .and_then(|i| std::env::args().nth(i + 1))
        .unwrap_or_else(|| "sk-test-token".to_string());

    let session_manager = Arc::new(SessionManager::new());
    let registry = Arc::new(MemoryWorkerRegistry::new());

    let state = AppState {
        token,
        session_manager,
        registry,
    };

    let app = Router::new()
        .route("/connect", get(ws_handler))
        .route("/api/tokilake/connect", get(ws_handler))
        .route("/health", get(health_handler))
        .route("/v1/chat/completions", post(chat_completions_handler))
        .with_state(state);

    let bind_addr = addr.trim_start_matches(':');
    let bind_addr = format!("0.0.0.0:{}", bind_addr);

    info!("tokilake server listening on {}", addr);

    let listener = tokio::net::TcpListener::bind(&bind_addr).await.unwrap();
    axum::serve(
        listener,
        app.into_make_service_with_connect_info::<std::net::SocketAddr>(),
    )
    .await
    .unwrap();
}

#[derive(Serialize)]
struct HealthResponse {
    status:   String,
    sessions: usize,
}

async fn health_handler(State(state): State<AppState>) -> Json<HealthResponse> {
    Json(HealthResponse {
        status:   "ok".to_string(),
        sessions: state.session_manager.session_count(),
    })
}

#[derive(Deserialize)]
struct ConnectQuery {
    token:        Option<String>,
    access_token: Option<String>,
}

async fn ws_handler(
    ws: WebSocketUpgrade,
    State(state): State<AppState>,
    Query(query): Query<ConnectQuery>,
    axum::extract::ConnectInfo(addr): axum::extract::ConnectInfo<std::net::SocketAddr>,
    headers: axum::http::HeaderMap,
) -> impl IntoResponse {
    let token_key = match extract_token_from_request(&state.token, &query, &headers) {
        Ok(t) => t,
        Err(e) => {
            return (
                StatusCode::UNAUTHORIZED,
                Json(serde_json::json!({"error": e.to_string()})),
            )
                .into_response();
        }
    };

    ws.protocols(["tokilake.v1"])
        .on_upgrade(move |socket| handle_ws_connection(socket, state, token_key, addr.to_string()))
}

fn extract_token_from_request(
    expected: &str,
    query: &ConnectQuery,
    headers: &axum::http::HeaderMap,
) -> Result<String, TunnelError> {
    // Try Authorization header first
    if let Some(auth) = headers.get("authorization") {
        if let Ok(auth_str) = auth.to_str() {
            let auth_str = auth_str.trim();
            let token = if auth_str.to_lowercase().starts_with("bearer ") {
                auth_str[7..].trim()
            } else {
                auth_str
            };
            if !token.is_empty() {
                let token = token.strip_prefix("sk-").unwrap_or(token);
                let expected = expected.strip_prefix("sk-").unwrap_or(expected);
                if token == expected {
                    return Ok(token.to_string());
                }
            }
        }
    }

    // Try query parameters
    let token = query
        .token
        .as_deref()
        .or(query.access_token.as_deref())
        .unwrap_or("");

    let token = token.trim();
    let token = token.strip_prefix("sk-").unwrap_or(token);
    let expected = expected.strip_prefix("sk-").unwrap_or(expected);

    if token.is_empty() || token != expected {
        return Err(TunnelError::auth_failed("invalid token"));
    }

    Ok(token.to_string())
}

async fn handle_ws_connection(
    socket: WebSocket,
    state: AppState,
    token_key: String,
    remote_addr: String,
) {
    let session = state.session_manager.new_session(
        None,
        token_key.clone(),
        remote_addr.clone(),
        "websocket".to_string(),
    );

    let result = serve_session(&state, &session, socket).await;

    if let Err(e) = &result {
        warn!("session error: {}", e);
    }

    // Cleanup
    let session_guard = session.read().await;
    let worker_id = session_guard.worker_id;
    let _ = state.registry.cleanup_worker(worker_id);
    state.session_manager.release(&session_guard).await;
    drop(session_guard);
}

async fn serve_session(
    state: &AppState,
    session: &Arc<RwLock<GatewaySession>>,
    socket: WebSocket,
) -> Result<(), TunnelError> {
    let (mut ws_sender, mut ws_receiver) = socket.split();

    // Create channels for WebSocket I/O
    let (ws_out_tx, mut ws_out_rx) = mpsc::channel::<Vec<u8>>(32);
    let (ws_in_tx, ws_in_rx) = mpsc::channel::<Vec<u8>>(32);

    // Spawn task to forward outgoing WebSocket messages
    tokio::spawn(async move {
        while let Some(data) = ws_out_rx.recv().await {
            if ws_sender.send(Message::Binary(data)).await.is_err() {
                break;
            }
        }
    });

    // Spawn task to forward incoming WebSocket messages
    let ws_in_tx_clone = ws_in_tx.clone();
    tokio::spawn(async move {
        while let Some(msg) = ws_receiver.next().await {
            match msg {
                Ok(Message::Binary(data)) => {
                    if ws_in_tx_clone.send(data).await.is_err() {
                        break;
                    }
                }
                Ok(Message::Close(_)) => break,
                Err(_) => break,
                _ => {}
            }
        }
        // Drop the sender to signal EOF
        drop(ws_in_tx_clone);
    });

    // Create smux session over WebSocket stream
    let ws_stream = WebSocketStream::new(ws_in_rx, ws_out_tx.clone());
    let smux_config = smux::ConfigBuilder::new()
        .enable_keep_alive(false)
        .build()
        .unwrap_or_default();

    let smux_session = smux::Session::server(ws_stream, smux_config)
        .await
        .map_err(|e| TunnelError::Transport(std::io::Error::other(e)))?;
    let smux_session = Arc::new(smux_session);

    // Store control channel and smux session in session
    {
        let mut s = session.write().await;
        s.control_tx = Some(ws_out_tx.clone());
        s.smux_session = Some(smux_session.clone());
        // Mark as authenticated since WebSocket auth is done at the HTTP level
        s.authenticated = true;
    }

    // Accept control stream (first stream)
    let control_stream = match smux_session.accept_stream().await {
        Ok(stream) => stream,
        Err(e) => {
            return Err(TunnelError::Transport(std::io::Error::other(e)));
        }
    };

    let (mut control_reader, mut control_writer) = tokio::io::split(control_stream);

    // Create codec for control stream
    let mut control_codec =
        tokilake_core::codec::ControlCodec::new(control_reader, tokio::io::sink());

    // WebSocket connections are already authenticated via the Authorization header
    let mut authenticated = true;
    let mut worker_registered = false;
    let mut worker_id = 0;

    // Main control message loop
    loop {
        // Read next control message
        let msg = match control_codec.read_message().await {
            Ok(Some(ctrl)) => {
                info!("received control message: type={}", ctrl.msg_type);
                ctrl
            }
            Ok(None) => {
                info!("control stream closed");
                break;
            }
            Err(e) => {
                warn!("control stream error: {}", e);
                break;
            }
        };

        let response = handle_control_message(
            state,
            session,
            &msg,
            &mut authenticated,
            &mut worker_registered,
            &mut worker_id,
        )
        .await;

        if let Some(resp) = response {
            let resp_data = serde_json::to_vec(&resp).unwrap();
            let mut resp_with_newline = resp_data;
            resp_with_newline.push(b'\n');
            if control_writer.write_all(&resp_with_newline).await.is_err() {
                break;
            }
        }
    }

    Ok(())
}

async fn handle_control_message(
    state: &AppState,
    session: &Arc<RwLock<GatewaySession>>,
    msg: &ControlMessage,
    authenticated: &mut bool,
    worker_registered: &mut bool,
    worker_id: &mut i32,
) -> Option<ControlMessage> {
    let request_id = msg.request_id.clone().unwrap_or_default();

    match msg.msg_type.as_str() {
        control_type::AUTH => {
            if *authenticated {
                return Some(ControlMessage::error_msg(
                    request_id,
                    ErrorMessage::new("auth_already_completed", "auth message already handled"),
                ));
            }

            let auth = match &msg.auth {
                Some(a) => a,
                None => {
                    return Some(ControlMessage::error_msg(
                        request_id,
                        ErrorMessage::new("auth_payload_missing", "auth payload is required"),
                    ));
                }
            };

            let token = auth.token.trim();
            let token = token.strip_prefix("sk-").unwrap_or(token);
            let expected = state.token.strip_prefix("sk-").unwrap_or(&state.token);

            if token != expected {
                return Some(ControlMessage::error_msg(
                    request_id,
                    ErrorMessage::new("auth_failed", "invalid token"),
                ));
            }

            *authenticated = true;
            session.write().await.authenticated = true;

            Some(ControlMessage::ack(request_id, AckMessage {
                message:    "auth_ok".to_string(),
                namespace:  String::new(),
                worker_id:  0,
                channel_id: 0,
            }))
        }

        control_type::REGISTER => {
            if !*authenticated {
                return Some(ControlMessage::error_msg(
                    request_id,
                    ErrorMessage::new("not_authenticated", "authentication is required"),
                ));
            }

            if *worker_registered {
                return Some(ControlMessage::error_msg(
                    request_id,
                    ErrorMessage::new(
                        "register_already_completed",
                        "register message already handled",
                    ),
                ));
            }

            let register = match &msg.register {
                Some(r) => r,
                None => {
                    return Some(ControlMessage::error_msg(
                        request_id,
                        ErrorMessage::new(
                            "register_payload_missing",
                            "register payload is required",
                        ),
                    ));
                }
            };

            match state.registry.register_worker(
                &register.namespace,
                &register.models,
                &register.group,
                &register.backend_type,
            ) {
                Ok(result) => {
                    *worker_registered = true;
                    *worker_id = result.worker_id;

                    {
                        let mut s = session.write().await;
                        s.worker_id = result.worker_id;
                        s.channel_id = result.channel_id;
                        s.namespace = result.namespace.clone();
                        s.group = result.group.clone();
                        s.models = result.models.clone();
                        s.backend_type = result.backend_type.clone();
                        s.status = result.status;
                    }

                    let _ = state
                        .session_manager
                        .claim_namespace(session, &result.namespace)
                        .await;

                    info!(
                        "worker registered: id={} namespace={}",
                        result.worker_id, result.namespace
                    );

                    Some(ControlMessage::ack(request_id, AckMessage {
                        message:    "register_ok".to_string(),
                        namespace:  result.namespace,
                        worker_id:  result.worker_id,
                        channel_id: result.channel_id,
                    }))
                }
                Err(e) => Some(ControlMessage::error_msg(
                    request_id,
                    ErrorMessage::new("register_failed", e.to_string()),
                )),
            }
        }

        control_type::HEARTBEAT => {
            if !*authenticated {
                return Some(ControlMessage::error_msg(
                    request_id,
                    ErrorMessage::new("not_authenticated", "authentication is required"),
                ));
            }

            if !*worker_registered {
                return Some(ControlMessage::error_msg(
                    request_id,
                    ErrorMessage::new("not_registered", "register is required before heartbeat"),
                ));
            }

            let heartbeat = match &msg.heartbeat {
                Some(h) => h,
                None => {
                    return Some(ControlMessage::error_msg(
                        request_id,
                        ErrorMessage::new(
                            "heartbeat_payload_missing",
                            "heartbeat payload is required",
                        ),
                    ));
                }
            };

            let _ = state
                .registry
                .update_heartbeat(*worker_id, &heartbeat.current_models);

            let s = session.read().await;
            Some(ControlMessage::ack(request_id, AckMessage {
                message:    "heartbeat_ok".to_string(),
                namespace:  s.namespace.clone(),
                worker_id:  s.worker_id,
                channel_id: s.channel_id,
            }))
        }

        control_type::MODELS_SYNC => {
            if !*authenticated {
                return Some(ControlMessage::error_msg(
                    request_id,
                    ErrorMessage::new("not_authenticated", "authentication is required"),
                ));
            }

            if !*worker_registered {
                return Some(ControlMessage::error_msg(
                    request_id,
                    ErrorMessage::new("not_registered", "register is required before models_sync"),
                ));
            }

            let _sync = match &msg.models_sync {
                Some(s) => s,
                None => {
                    return Some(ControlMessage::error_msg(
                        request_id,
                        ErrorMessage::new(
                            "models_payload_missing",
                            "models_sync payload is required",
                        ),
                    ));
                }
            };

            let s = session.read().await;
            Some(ControlMessage::ack(request_id, AckMessage {
                message:    "models_sync_ok".to_string(),
                namespace:  s.namespace.clone(),
                worker_id:  s.worker_id,
                channel_id: s.channel_id,
            }))
        }

        control_type::ACK => None,

        control_type::ERROR => {
            if let Some(err) = &msg.error {
                warn!("tokiame error: code={} message={}", err.code, err.message);
            }
            None
        }

        _ => {
            if !*authenticated {
                return Some(ControlMessage::error_msg(
                    request_id,
                    ErrorMessage::new("not_authenticated", "authentication is required"),
                ));
            }
            Some(ControlMessage::error_msg(
                request_id,
                ErrorMessage::new(
                    "unsupported_message_type",
                    format!("unsupported message type: {}", msg.msg_type),
                ),
            ))
        }
    }
}

#[derive(Deserialize)]
struct ChatQuery {
    namespace: Option<String>,
}

async fn chat_completions_handler(
    State(state): State<AppState>,
    Query(query): Query<ChatQuery>,
    Json(body): Json<serde_json::Value>,
) -> impl IntoResponse {
    let namespace = query.namespace.unwrap_or_else(|| "test-worker".to_string());

    let session = match state.session_manager.get_by_namespace(&namespace) {
        Some(s) => s,
        None => {
            return (
                StatusCode::BAD_GATEWAY,
                Json(serde_json::json!({"error": format!("namespace '{}' is offline", namespace)})),
            )
                .into_response();
        }
    };

    let session_guard = session.read().await;
    let smux_session = match &session_guard.smux_session {
        Some(s) => s.clone(),
        None => {
            return (
                StatusCode::BAD_GATEWAY,
                Json(serde_json::json!({"error": "smux session unavailable"})),
            )
                .into_response();
        }
    };
    let session_id = session_guard.id;
    let channel_id = session_guard.channel_id;
    drop(session_guard);

    let model = body
        .get("model")
        .and_then(|v| v.as_str())
        .unwrap_or("gpt-3.5-turbo")
        .to_string();

    let is_stream = body
        .get("stream")
        .and_then(|v| v.as_bool())
        .unwrap_or(false);

    let request_id = format!("{}:relay:{}", namespace, uuid::Uuid::new_v4());

    let tunnel_req = TunnelRequest {
        request_id: request_id.clone(),
        route_kind: route_kind::CHAT_COMPLETIONS.to_string(),
        method: "POST".to_string(),
        path: "/v1/chat/completions".to_string(),
        model,
        headers: {
            let mut h = HashMap::new();
            h.insert("Content-Type".to_string(), "application/json".to_string());
            h
        },
        is_stream,
        body: serde_json::to_vec(&body).unwrap_or_default(),
    };

    state.session_manager.track_request(InFlightRequest {
        request_id: request_id.clone(),
        session_id,
        namespace: namespace.clone(),
        channel_id,
        created_at: std::time::Instant::now(),
    });

    // Open a data stream to the worker
    let data_stream = match smux_session.open_stream().await {
        Ok(stream) => stream,
        Err(e) => {
            state.session_manager.remove_request(&request_id);
            return (
                StatusCode::BAD_GATEWAY,
                Json(serde_json::json!({"error": format!("failed to open data stream: {}", e)})),
            )
                .into_response();
        }
    };

    let (mut stream_reader, mut stream_writer) = tokio::io::split(data_stream);

    // Send tunnel request on data stream
    let req_data = serde_json::to_vec(&tunnel_req).unwrap();
    let mut req_with_newline = req_data;
    req_with_newline.push(b'\n');
    if let Err(e) = stream_writer.write_all(&req_with_newline).await {
        state.session_manager.remove_request(&request_id);
        return (
            StatusCode::BAD_GATEWAY,
            Json(serde_json::json!({"error": format!("failed to send request: {}", e)})),
        )
            .into_response();
    }

    // Read response from data stream
    let mut response_codec =
        tokilake_core::codec::TunnelCodec::new(stream_reader, tokio::io::sink());

    // Read first response frame
    let first_frame =
        match tokio::time::timeout(Duration::from_secs(30), response_codec.read_response()).await {
            Ok(Ok(Some(resp))) => resp,
            Ok(Ok(None)) => {
                state.session_manager.remove_request(&request_id);
                return (
                    StatusCode::BAD_GATEWAY,
                    Json(serde_json::json!({"error": "stream closed before response"})),
                )
                    .into_response();
            }
            Ok(Err(e)) => {
                state.session_manager.remove_request(&request_id);
                return (
                    StatusCode::BAD_GATEWAY,
                    Json(serde_json::json!({"error": format!("failed to read response: {}", e)})),
                )
                    .into_response();
            }
            Err(_) => {
                state.session_manager.remove_request(&request_id);
                return (
                    StatusCode::BAD_GATEWAY,
                    Json(serde_json::json!({"error": "request timeout"})),
                )
                    .into_response();
            }
        };

    // Check for errors
    if let Some(err) = &first_frame.error {
        state.session_manager.remove_request(&request_id);
        return (
            StatusCode::BAD_GATEWAY,
            Json(serde_json::json!({"error": err.message})),
        )
            .into_response();
    }

    let status_code = if first_frame.status_code == 0 {
        StatusCode::OK
    } else {
        StatusCode::from_u16(first_frame.status_code).unwrap_or(StatusCode::OK)
    };

    // Collect body chunks until EOF
    let mut body = first_frame.body_chunk.0;
    if !first_frame.eof {
        loop {
            match tokio::time::timeout(Duration::from_secs(30), response_codec.read_response())
                .await
            {
                Ok(Ok(Some(frame))) => {
                    if let Some(err) = &frame.error {
                        state.session_manager.remove_request(&request_id);
                        return (
                            StatusCode::BAD_GATEWAY,
                            Json(serde_json::json!({"error": err.message})),
                        )
                            .into_response();
                    }
                    body.extend_from_slice(&frame.body_chunk.0);
                    if frame.eof {
                        break;
                    }
                }
                Ok(Ok(None)) => break,
                Ok(Err(e)) => {
                    state.session_manager.remove_request(&request_id);
                    return (
                        StatusCode::BAD_GATEWAY,
                        Json(serde_json::json!({"error": format!("read response: {}", e)})),
                    )
                        .into_response();
                }
                Err(_) => {
                    state.session_manager.remove_request(&request_id);
                    return (
                        StatusCode::BAD_GATEWAY,
                        Json(serde_json::json!({"error": "request timeout"})),
                    )
                        .into_response();
                }
            }
        }
    }

    state.session_manager.remove_request(&request_id);

    // Build response
    let mut headers = axum::http::HeaderMap::new();
    for (k, v) in &first_frame.headers {
        if let (Ok(name), Ok(val)) = (
            axum::http::HeaderName::from_bytes(k.as_bytes()),
            axum::http::HeaderValue::from_str(v),
        ) {
            headers.insert(name, val);
        }
    }

    let mut response = (headers, body).into_response();
    *response.status_mut() = status_code;
    response
}
