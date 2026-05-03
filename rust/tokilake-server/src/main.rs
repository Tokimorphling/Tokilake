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
    session::{ChannelBindParams, GatewaySession, InFlightRequest, SessionManager},
    tunnel::{quic::QuicSession, TunnelSession},
};
use tokio::sync::{mpsc, RwLock};
use tracing::{info, warn};

#[derive(Clone)]
struct AppState {
    token:                String,
    session_manager:      Arc<SessionManager<tokilake_smux::Session>>,
    quic_session_manager: Arc<SessionManager<QuicSession>>,
    registry:             Arc<MemoryWorkerRegistry>,
}

struct MemoryWorkerRegistry {
    workers: parking_lot::RwLock<HashMap<i32, WorkerEntry>>,
}

struct WorkerEntry {
    #[allow(dead_code)]
    worker_id: i32,
    #[allow(dead_code)]
    namespace: String,
    models:    Vec<String>,
    #[allow(dead_code)]
    group:     String,
    #[allow(dead_code)]
    status:    String,
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
            group: group.to_string(),
            status: backend_type.to_string(),
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
        self: std::pin::Pin<&mut Self>,
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

/// Generate a self-signed TLS certificate for the QUIC endpoint.
fn generate_self_signed_cert() -> (
    rustls::pki_types::CertificateDer<'static>,
    rustls::pki_types::PrivateKeyDer<'static>,
) {
    let cert = rcgen::generate_simple_self_signed(vec!["localhost".to_string()]).unwrap();
    let cert_der = rustls::pki_types::CertificateDer::from(cert.cert);
    let key_der = rustls::pki_types::PrivateKeyDer::from(
        rustls::pki_types::PrivatePkcs8KeyDer::from(cert.key_pair.serialize_der()),
    );
    (cert_der, key_der)
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

    let session_manager = Arc::new(SessionManager::<tokilake_smux::Session>::new());
    let quic_session_manager = Arc::new(SessionManager::<QuicSession>::new());
    let registry = Arc::new(MemoryWorkerRegistry::new());

    let state = AppState {
        token,
        session_manager,
        quic_session_manager,
        registry,
    };

    let app = Router::new()
        .route("/connect", get(ws_handler))
        .route("/api/tokilake/connect", get(ws_handler))
        .route("/health", get(health_handler))
        .route("/v1/chat/completions", post(chat_completions_handler))
        .with_state(state.clone());

    let bind_addr = addr.trim_start_matches(':');
    let bind_addr = format!("0.0.0.0:{}", bind_addr);

    info!("tokilake server listening on {}", addr);

    // Spawn the QUIC listener
    let quic_state = state.clone();
    let quic_bind = bind_addr.clone();
    tokio::spawn(async move {
        if let Err(e) = run_quic_listener(&quic_bind, quic_state).await {
            warn!("QUIC listener failed: {}", e);
        }
    });

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
        sessions: state.session_manager.session_count()
            + state.quic_session_manager.session_count(),
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
    let worker_id = session_guard
        .worker_info
        .as_ref()
        .map_or(0, |i| i.worker_id);
    let _ = state.registry.cleanup_worker(worker_id);
    state.session_manager.release(&session_guard).await;
    drop(session_guard);
}

async fn serve_session(
    state: &AppState,
    session: &Arc<RwLock<GatewaySession<tokilake_smux::Session>>>,
    socket: WebSocket,
) -> Result<(), TunnelError> {
    let (mut ws_sender, mut ws_receiver) = socket.split();

    // Create channels for WebSocket I/O
    let (ws_out_tx, mut ws_out_rx) = mpsc::channel::<Vec<u8>>(32);
    let (ws_in_tx, ws_in_rx) = mpsc::channel::<Vec<u8>>(32);

    // Spawn task to forward outgoing WebSocket messages
    tokio::spawn(async move {
        while let Some(data) = ws_out_rx.recv().await {
            if ws_sender.send(Message::Binary(data.into())).await.is_err() {
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
                    if ws_in_tx_clone.send(data.into()).await.is_err() {
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
    let smux_config = tokilake_smux::Config {
        version: 1,
        keep_alive_disabled: true,
        ..Default::default()
    };

    let smux_session = tokilake_smux::Session::server(ws_stream, smux_config);
    let smux_session = Arc::new(tokio::sync::Mutex::new(smux_session));

    // Store control channel and smux session in session
    {
        let mut s = session.write().await;
        s.control_tx = Some(ws_out_tx.clone());
        s.tunnel_session = Some(smux_session.clone());
        // Mark as authenticated since WebSocket auth is done at the HTTP level
        s.authenticated = true;
    }

    // Accept control stream (first stream)
    let control_stream = {
        let mut smux = smux_session.lock().await;
        match smux.accept().await {
            Some(stream) => stream,
            None => {
                return Err(TunnelError::StreamClosed);
            }
        }
    };

    let mut control_stream = control_stream;

    // WebSocket connections are already authenticated via the Authorization header
    let mut authenticated = true;
    let mut worker_registered = false;
    let mut worker_id = 0;

    // Main control message loop - read directly from stream
    let mut control_buf = Vec::new();
    loop {
        // Read data from stream
        let mut read_buf = vec![0u8; 4096];
        let n = match control_stream.read(&mut read_buf).await {
            Ok(0) => {
                info!("control stream closed");
                break;
            }
            Ok(n) => n,
            Err(e) => {
                warn!("control stream error: {}", e);
                break;
            }
        };
        control_buf.extend_from_slice(&read_buf[..n]);

        // Parse complete messages from buffer
        while let Some(pos) = control_buf.iter().position(|&b| b == b'\n') {
            let line = control_buf[..pos].to_vec();
            control_buf.drain(..=pos);

            let trimmed = String::from_utf8_lossy(&line);
            let trimmed = trimmed.trim();
            if trimmed.is_empty() {
                continue;
            }

            let msg = match serde_json::from_str::<ControlMessage>(trimmed) {
                Ok(m) => {
                    info!("received control message: type={}", m.msg_type);
                    m
                }
                Err(e) => {
                    warn!("control message parse error: {}", e);
                    continue;
                }
            };

            let response = handle_control_message(
                ControlMessageContext {
                    token: &state.token,
                    registry: &state.registry,
                    session_manager: &state.session_manager,
                    session,
                    authenticated: &mut authenticated,
                    worker_registered: &mut worker_registered,
                    worker_id: &mut worker_id,
                },
                &msg,
            )
            .await;

            if let Some(resp) = response {
                let resp_data = serde_json::to_vec(&resp).unwrap();
                let mut resp_with_newline = resp_data;
                resp_with_newline.push(b'\n');
                if control_stream.write_all(&resp_with_newline).await.is_err() {
                    break;
                }
            }
        }
    }

    Ok(())
}

struct ControlMessageContext<'a, T: TunnelSession> {
    pub token:             &'a str,
    pub registry:          &'a MemoryWorkerRegistry,
    pub session_manager:   &'a SessionManager<T>,
    pub session:           &'a Arc<RwLock<GatewaySession<T>>>,
    pub authenticated:     &'a mut bool,
    pub worker_registered: &'a mut bool,
    pub worker_id:         &'a mut i32,
}

async fn handle_control_message<T: TunnelSession>(
    ctx: ControlMessageContext<'_, T>,
    msg: &ControlMessage,
) -> Option<ControlMessage> {
    let request_id = msg.request_id.clone().unwrap_or_default();

    match msg.msg_type.as_str() {
        control_type::AUTH => {
            if *ctx.authenticated {
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

            let auth_token = auth.token.trim();
            let auth_token = auth_token.strip_prefix("sk-").unwrap_or(auth_token);
            let expected = ctx.token.strip_prefix("sk-").unwrap_or(ctx.token);

            if auth_token != expected {
                return Some(ControlMessage::error_msg(
                    request_id,
                    ErrorMessage::new("auth_failed", "invalid token"),
                ));
            }

            *ctx.authenticated = true;
            ctx.session.write().await.authenticated = true;

            Some(ControlMessage::ack(request_id, AckMessage {
                message:    "auth_ok".to_string(),
                namespace:  String::new(),
                worker_id:  0,
                channel_id: 0,
            }))
        }

        control_type::REGISTER => {
            if !*ctx.authenticated {
                return Some(ControlMessage::error_msg(
                    request_id,
                    ErrorMessage::new("not_authenticated", "authentication is required"),
                ));
            }

            if *ctx.worker_registered {
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

            match ctx.registry.register_worker(
                &register.namespace,
                &register.models,
                &register.group,
                &register.backend_type,
            ) {
                Ok(result) => {
                    *ctx.worker_registered = true;
                    *ctx.worker_id = result.worker_id;

                    ctx.session_manager
                        .bind_channel(ctx.session, ChannelBindParams {
                            worker_id:    result.worker_id,
                            channel_id:   result.channel_id,
                            group:        result.group.clone(),
                            models:       result.models.clone(),
                            backend_type: result.backend_type.clone(),
                            status:       result.status,
                            namespace:    result.namespace.clone(),
                        })
                        .await;

                    let _ = ctx
                        .session_manager
                        .claim_namespace(ctx.session, &result.namespace)
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
            if !*ctx.authenticated {
                return Some(ControlMessage::error_msg(
                    request_id,
                    ErrorMessage::new("not_authenticated", "authentication is required"),
                ));
            }

            if !*ctx.worker_registered {
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

            let _ = ctx
                .registry
                .update_heartbeat(*ctx.worker_id, &heartbeat.current_models);

            let s = ctx.session.read().await;
            Some(ControlMessage::ack(request_id, AckMessage {
                message:    "heartbeat_ok".to_string(),
                namespace:  s
                    .worker_info
                    .as_ref()
                    .map_or(String::new(), |i| i.namespace.clone()),
                worker_id:  s.worker_info.as_ref().map_or(0, |i| i.worker_id),
                channel_id: s.worker_info.as_ref().map_or(0, |i| i.channel_id),
            }))
        }

        control_type::MODELS_SYNC => {
            if !*ctx.authenticated {
                return Some(ControlMessage::error_msg(
                    request_id,
                    ErrorMessage::new("not_authenticated", "authentication is required"),
                ));
            }

            if !*ctx.worker_registered {
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

            let s = ctx.session.read().await;
            Some(ControlMessage::ack(request_id, AckMessage {
                message:    "models_sync_ok".to_string(),
                namespace:  s
                    .worker_info
                    .as_ref()
                    .map_or(String::new(), |i| i.namespace.clone()),
                worker_id:  s.worker_info.as_ref().map_or(0, |i| i.worker_id),
                channel_id: s.worker_info.as_ref().map_or(0, |i| i.channel_id),
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
            if !*ctx.authenticated {
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

    // Try SMUX session first, then QUIC
    enum ResolvedSession {
        Smux {
            tunnel:     Arc<tokio::sync::Mutex<tokilake_smux::Session>>,
            session_id: u64,
            channel_id: i32,
            mgr:        Arc<SessionManager<tokilake_smux::Session>>,
        },
        Quic {
            tunnel:     Arc<tokio::sync::Mutex<QuicSession>>,
            session_id: u64,
            channel_id: i32,
            mgr:        Arc<SessionManager<QuicSession>>,
        },
    }

    let resolved = if let Some(session) = state.session_manager.get_by_namespace(&namespace) {
        let g = session.read().await;
        match &g.tunnel_session {
            Some(s) => Some(ResolvedSession::Smux {
                tunnel:     s.clone(),
                session_id: g.id,
                channel_id: g.worker_info.as_ref().map_or(0, |i| i.channel_id),
                mgr:        state.session_manager.clone(),
            }),
            None => None,
        }
    } else if let Some(session) = state.quic_session_manager.get_by_namespace(&namespace) {
        let g = session.read().await;
        match &g.tunnel_session {
            Some(s) => Some(ResolvedSession::Quic {
                tunnel:     s.clone(),
                session_id: g.id,
                channel_id: g.worker_info.as_ref().map_or(0, |i| i.channel_id),
                mgr:        state.quic_session_manager.clone(),
            }),
            None => None,
        }
    } else {
        None
    };

    let resolved = match resolved {
        Some(r) => r,
        None => {
            return (
                StatusCode::BAD_GATEWAY,
                Json(serde_json::json!({"error": format!("namespace '{}' is offline", namespace)})),
            )
                .into_response();
        }
    };

    // Extract common fields and open data stream based on transport type
    let (session_id, channel_id) = match &resolved {
        ResolvedSession::Smux {
            session_id,
            channel_id,
            ..
        } => (*session_id, *channel_id),
        ResolvedSession::Quic {
            session_id,
            channel_id,
            ..
        } => (*session_id, *channel_id),
    };

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

    // Track the request — use the appropriate manager
    match &resolved {
        ResolvedSession::Smux { mgr, .. } => {
            mgr.track_request(InFlightRequest {
                request_id: request_id.as_str().into(),
                session_id,
                namespace: namespace.as_str().into(),
                channel_id,
                created_at: std::time::Instant::now(),
            });
        }
        ResolvedSession::Quic { mgr, .. } => {
            mgr.track_request(InFlightRequest {
                request_id: request_id.as_str().into(),
                session_id,
                namespace: namespace.as_str().into(),
                channel_id,
                created_at: std::time::Instant::now(),
            });
        }
    }

    // Serialize the tunnel request
    let req_data = serde_json::to_vec(&tunnel_req).unwrap();
    let mut req_with_newline = req_data;
    req_with_newline.push(b'\n');

    // Open a data stream and send the request, then read the response
    match resolved {
        ResolvedSession::Smux { tunnel, mgr, .. } => {
            let mut data_stream = {
                let mut smux = tunnel.lock().await;
                match smux.open().await {
                    Some(stream) => stream,
                    None => {
                        mgr.remove_request(&request_id);
                        return (
                            StatusCode::BAD_GATEWAY,
                            Json(serde_json::json!({"error": "failed to open data stream"})),
                        )
                            .into_response();
                    }
                }
            };

            if let Err(e) = data_stream.write_all(&req_with_newline).await {
                mgr.remove_request(&request_id);
                return (
                    StatusCode::BAD_GATEWAY,
                    Json(serde_json::json!({"error": format!("failed to send request: {}", e)})),
                )
                    .into_response();
            }

            relay_response(data_stream, tokio::io::sink(), &request_id, &*mgr).await
        }
        ResolvedSession::Quic { tunnel, mgr, .. } => {
            let (send, recv) = {
                let conn = {
                    let q = tunnel.lock().await;
                    // Access the underlying quinn::Connection via a helper
                    q.connection().clone()
                };
                match conn.open_bi().await {
                    Ok(pair) => pair,
                    Err(e) => {
                        mgr.remove_request(&request_id);
                        return (
                            StatusCode::BAD_GATEWAY,
                            Json(serde_json::json!({"error": format!("failed to open QUIC stream: {}", e)})),
                        )
                            .into_response();
                    }
                }
            };

            let mut send = send;

            if let Err(e) = send.write_all(&req_with_newline).await {
                mgr.remove_request(&request_id);
                return (
                    StatusCode::BAD_GATEWAY,
                    Json(
                        serde_json::json!({"error": format!("failed to send QUIC request: {}", e)}),
                    ),
                )
                    .into_response();
            }

            relay_response(recv, send, &request_id, &*mgr).await
        }
    }
}

/// Common response relay logic — reads the tunnel response from a reader and
/// builds the HTTP response. Works for both SMUX streams and QUIC streams.
async fn relay_response<R, W, T>(
    reader: R,
    writer: W,
    request_id: &str,
    session_manager: &SessionManager<T>,
) -> axum::response::Response
where
    R: tokio::io::AsyncRead + Unpin,
    W: tokio::io::AsyncWrite + Unpin,
    T: TunnelSession,
{
    let mut response_codec = tokilake_core::codec::TunnelCodec::new(reader, writer);

    // Read first response frame
    let first_frame =
        match tokio::time::timeout(Duration::from_secs(30), response_codec.read_response()).await {
            Ok(Ok(Some(resp))) => resp,
            Ok(Ok(None)) => {
                session_manager.remove_request(request_id);
                return (
                    StatusCode::BAD_GATEWAY,
                    Json(serde_json::json!({"error": "stream closed before response"})),
                )
                    .into_response();
            }
            Ok(Err(e)) => {
                session_manager.remove_request(request_id);
                return (
                    StatusCode::BAD_GATEWAY,
                    Json(serde_json::json!({"error": format!("failed to read response: {}", e)})),
                )
                    .into_response();
            }
            Err(_) => {
                session_manager.remove_request(request_id);
                return (
                    StatusCode::BAD_GATEWAY,
                    Json(serde_json::json!({"error": "request timeout"})),
                )
                    .into_response();
            }
        };

    // Check for errors
    if let Some(err) = &first_frame.error {
        session_manager.remove_request(request_id);
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
                        session_manager.remove_request(request_id);
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
                    session_manager.remove_request(request_id);
                    return (
                        StatusCode::BAD_GATEWAY,
                        Json(serde_json::json!({"error": format!("read response: {}", e)})),
                    )
                        .into_response();
                }
                Err(_) => {
                    session_manager.remove_request(request_id);
                    return (
                        StatusCode::BAD_GATEWAY,
                        Json(serde_json::json!({"error": "request timeout"})),
                    )
                        .into_response();
                }
            }
        }
    }

    session_manager.remove_request(request_id);

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

// --------------------------------------------------------------------------
// QUIC listener
// --------------------------------------------------------------------------

async fn run_quic_listener(bind_addr: &str, state: AppState) -> Result<(), anyhow::Error> {
    let (cert_der, key_der) = generate_self_signed_cert();

    let server_crypto = rustls::ServerConfig::builder()
        .with_no_client_auth()
        .with_single_cert(vec![cert_der], key_der)?;

    let server_config = quinn::ServerConfig::with_crypto(Arc::new(
        quinn::crypto::rustls::QuicServerConfig::try_from(server_crypto)?,
    ));

    let endpoint = quinn::Endpoint::server(server_config, bind_addr.parse()?)?;
    info!("QUIC listener started on {}", bind_addr);

    while let Some(incoming) = endpoint.accept().await {
        let state = state.clone();
        tokio::spawn(async move {
            match incoming.await {
                Ok(conn) => {
                    let remote = conn.remote_address();
                    info!("QUIC connection from {}", remote);
                    handle_quic_connection(conn, state, remote.to_string()).await;
                }
                Err(e) => {
                    warn!("QUIC accept error: {}", e);
                }
            }
        });
    }

    Ok(())
}

async fn handle_quic_connection(conn: quinn::Connection, state: AppState, remote_addr: String) {
    let quic_session = QuicSession::new(conn.clone());
    let quic_session = Arc::new(tokio::sync::Mutex::new(quic_session));

    let session = state.quic_session_manager.new_session(
        None,
        String::new(), // QUIC doesn't pass token via HTTP headers
        remote_addr.clone(),
        "quic".to_string(),
    );

    // Store the QUIC session
    {
        let mut s = session.write().await;
        s.tunnel_session = Some(quic_session.clone());
    }

    // Accept the first bidirectional stream as the control stream
    let (mut send, mut recv) = match conn.accept_bi().await {
        Ok(pair) => pair,
        Err(e) => {
            warn!("QUIC: failed to accept control stream: {}", e);
            let session_guard = session.read().await;
            state.quic_session_manager.release(&session_guard).await;
            return;
        }
    };

    let mut authenticated = false;
    let mut worker_registered = false;
    let mut worker_id = 0i32;

    // Control message loop
    let mut control_buf = Vec::new();
    loop {
        let mut read_buf = vec![0u8; 4096];
        let n = match recv.read(&mut read_buf).await {
            Ok(Some(n)) => n,
            Ok(None) => {
                info!("QUIC control stream closed");
                break;
            }
            Err(e) => {
                warn!("QUIC control stream error: {}", e);
                break;
            }
        };
        control_buf.extend_from_slice(&read_buf[..n]);

        while let Some(pos) = control_buf.iter().position(|&b| b == b'\n') {
            let line = control_buf[..pos].to_vec();
            control_buf.drain(..=pos);

            let trimmed = String::from_utf8_lossy(&line);
            let trimmed = trimmed.trim();
            if trimmed.is_empty() {
                continue;
            }

            let msg = match serde_json::from_str::<ControlMessage>(trimmed) {
                Ok(m) => {
                    info!("QUIC control message: type={}", m.msg_type);
                    m
                }
                Err(e) => {
                    warn!("QUIC control message parse error: {}", e);
                    continue;
                }
            };

            let response = handle_control_message(
                ControlMessageContext {
                    token:             &state.token,
                    registry:          &state.registry,
                    session_manager:   &state.quic_session_manager,
                    session:           &session,
                    authenticated:     &mut authenticated,
                    worker_registered: &mut worker_registered,
                    worker_id:         &mut worker_id,
                },
                &msg,
            )
            .await;

            if let Some(resp) = response {
                let resp_data = serde_json::to_vec(&resp).unwrap();
                let mut resp_with_newline = resp_data;
                resp_with_newline.push(b'\n');
                if send.write_all(&resp_with_newline).await.is_err() {
                    break;
                }
            }
        }
    }

    // Cleanup
    let session_guard = session.read().await;
    let wid = session_guard
        .worker_info
        .as_ref()
        .map_or(0, |i| i.worker_id);
    let _ = state.registry.cleanup_worker(wid);
    state.quic_session_manager.release(&session_guard).await;
    drop(session_guard);
    info!("QUIC worker disconnected: addr={}", remote_addr);
}
