//! Request roundtrip forwarding through the tunnel.
//!
//! This module handles forwarding API requests through the tunnel to workers
//! and streaming responses back to the client. It supports:
//! - Request routing by channel ID or namespace
//! - Streaming response bodies
//! - Request cancellation
//! - Error propagation

use crate::{
    error::{ErrorMessage, TunnelError},
    protocol::{TunnelRequest, TunnelResponse},
    service::Service,
    session::{InFlightRequest, SessionManager},
    tunnel::{TunnelSession, TunnelStream},
};
use std::{collections::HashMap, sync::Arc, time::Instant};
use tokio::sync::{mpsc, oneshot};
use uuid::Uuid;

/// Error returned when a tunnel stream request fails.
#[derive(Debug, Clone)]
pub struct TunnelStreamError {
    /// HTTP status code to return to the client.
    pub status_code: u16,

    /// Error code identifier.
    pub code: String,

    /// Human-readable error message.
    pub message: String,

    /// Error type category.
    pub error_type: String,
}

impl std::fmt::Display for TunnelStreamError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "{}", self.message)
    }
}

impl std::error::Error for TunnelStreamError {}

impl TunnelStreamError {
    /// Create a new tunnel stream error.
    pub fn new(status_code: u16, code: impl Into<String>, message: impl Into<String>) -> Self {
        Self {
            status_code,
            code: code.into(),
            message: message.into(),
            error_type: "upstream_error".to_string(),
        }
    }

    /// Create from an error message.
    pub fn from_error_message(err: &ErrorMessage) -> Self {
        let code = if err.code.is_empty() {
            "tokiame_stream_error"
        } else {
            &err.code
        };
        let message = if err.message.is_empty() {
            "tokiame stream error"
        } else {
            &err.message
        };

        Self {
            status_code: 502,
            code:        code.to_string(),
            message:     message.to_string(),
            error_type:  "upstream_error".to_string(),
        }
    }
}

/// Response from a tunnel request.
pub struct TunnelRoundtripResponse {
    /// HTTP status code.
    pub status_code: u16,

    /// Response headers.
    pub headers: HashMap<String, String>,

    /// Channel for receiving streaming body chunks.
    pub body_rx: mpsc::Receiver<Result<Vec<u8>, TunnelStreamError>>,

    /// Handle to cancel the request.
    pub cancel_tx: Option<oneshot::Sender<String>>,
}

/// Roundtrip handler for forwarding requests through the tunnel.
///
/// Manages the lifecycle of forwarded requests including stream setup,
/// response pumping, and cancellation.
#[derive(Clone)]
pub struct Roundtrip<T: TunnelSession> {
    session_manager: Arc<SessionManager<T>>,
}

impl<T: TunnelSession> Roundtrip<T> {
    /// Create a new roundtrip handler.
    pub fn new(session_manager: Arc<SessionManager<T>>) -> Self {
        Self { session_manager }
    }
}

pub enum RoundtripRequest {
    ByChannel {
        channel_id: i32,
        request:    TunnelRequest,
    },
    ByNamespace {
        namespace: Arc<str>,
        request:   TunnelRequest,
    },
}

impl<T: TunnelSession> Service<RoundtripRequest> for Roundtrip<T> {
    type Response = TunnelRoundtripResponse;
    type Error = TunnelError;

    async fn call(&self, req: RoundtripRequest) -> Result<Self::Response, Self::Error> {
        let (session, mut request) = match req {
            RoundtripRequest::ByChannel {
                channel_id,
                request,
            } => {
                let session = self
                    .session_manager
                    .get_by_channel_id(channel_id)
                    .ok_or_else(|| {
                        TunnelError::protocol(format!(
                            "tokiame session is offline for channel {}",
                            channel_id
                        ))
                    })?;
                if !session.read().await.is_alive() {
                    return Err(TunnelError::protocol(format!(
                        "tokiame session is offline for channel {}",
                        channel_id
                    )));
                }
                (session, request)
            }
            RoundtripRequest::ByNamespace { namespace, request } => {
                let session = self
                    .session_manager
                    .get_by_namespace(&namespace)
                    .ok_or_else(|| {
                        TunnelError::protocol(format!(
                            "tokiame session is offline for namespace {}",
                            namespace
                        ))
                    })?;
                if !session.read().await.is_alive() {
                    return Err(TunnelError::protocol(format!(
                        "tokiame session is offline for namespace {}",
                        namespace
                    )));
                }
                (session, request)
            }
        };

        let session_guard = session.read().await;
        let (namespace, channel_id) = if let Some(ref info) = session_guard.worker_info {
            (info.namespace.clone(), info.channel_id)
        } else {
            return Err(TunnelError::protocol("session is not fully registered"));
        };

        // Ensure request has an ID
        if request.request_id.trim().is_empty() {
            request.request_id = build_request_id(&namespace);
        }

        // Open a stream for this request.
        let tunnel_session = session_guard
            .tunnel_session
            .clone()
            .ok_or(TunnelError::StreamClosed)?;
        let mut stream = {
            let mut session_lock = tunnel_session.lock().await;
            session_lock.open_stream().await?
        };

        // Track the in-flight request
        let (cancel_tx, cancel_rx) = oneshot::channel();
        let request_id: Arc<str> = request.request_id.as_str().into();

        self.session_manager.track_request(InFlightRequest {
            request_id: request_id.clone(),
            session_id: session_guard.id,
            namespace: namespace.as_str().into(),
            channel_id,
            created_at: Instant::now(),
        });

        // Write the request
        let request_json = serde_json::to_vec(&request)?;
        stream.write(&request_json).await?;
        stream.flush().await?;

        // Read the first frame for headers
        let mut buf = vec![0u8; 8192];
        let mut response_buffer = Vec::new();
        let first_response = loop {
            let n = stream.read(&mut buf).await?;
            if n == 0 {
                return Err(TunnelError::protocol(
                    "stream closed before receiving response",
                ));
            }

            response_buffer.extend_from_slice(&buf[..n]);

            // Limit max frame size to prevent OOM (16 MB)
            if response_buffer.len() > 16 * 1024 * 1024 {
                return Err(TunnelError::protocol(
                    "response frame too large (potential backpressure/OOM protection)",
                ));
            }

            if let Some(newline_pos) = response_buffer.iter().position(|&b| b == b'\n') {
                let line = response_buffer[..newline_pos].to_vec();
                response_buffer.drain(..=newline_pos);

                if line.is_empty() {
                    continue;
                }

                match serde_json::from_slice::<TunnelResponse>(&line) {
                    Ok(resp) => break resp,
                    Err(e) => return Err(TunnelError::Serialization(e)),
                }
            }
        };

        if let Some(err) = &first_response.error {
            return Err(TunnelError::protocol(err.message.clone()));
        }

        let status_code = first_response.status_code;
        let headers = first_response.headers.clone();

        // Set up response channel
        let (body_tx, body_rx) = mpsc::channel(32);

        // If the first response has a body chunk, send it
        if !first_response.body_chunk.is_empty() {
            let _ = body_tx.send(Ok(first_response.body_chunk.0)).await;
        }

        // Spawn response pump
        let session_manager = self.session_manager.clone();

        // Only spawn pump if not EOF
        if !first_response.eof {
            tokio::spawn(async move {
                let result = pump_response(
                    stream,
                    body_tx,
                    cancel_rx,
                    request_id.clone(),
                    response_buffer,
                )
                .await;

                session_manager.remove_request(&request_id);

                if let Err(e) = result {
                    tracing::error!("tunnel response pump error: {}", e);
                }
            });
        } else {
            self.session_manager.remove_request(&request_id);
        }

        Ok(TunnelRoundtripResponse {
            status_code: if status_code == 0 { 200 } else { status_code },
            headers,
            body_rx,
            cancel_tx: Some(cancel_tx),
        })
    }
}

impl<T: TunnelSession> Roundtrip<T> {
    /// Cancel an in-flight request.
    pub fn cancel_request(&self, request_id: &str, _reason: &str) -> Result<(), TunnelError> {
        if let Some(_entry) = self.session_manager.get_request(request_id) {
            // In a real implementation, send cancel through the control stream
            self.session_manager.remove_request(request_id);
        }
        Ok(())
    }
}

/// Build a unique request ID for a tunnel request.
fn build_request_id(namespace: &str) -> String {
    let namespace = namespace.trim();
    let namespace = if namespace.is_empty() {
        "tokiame"
    } else {
        namespace
    };
    format!("{}:relay:{}", namespace, Uuid::new_v4())
}

/// Pump response frames from the tunnel stream to the response channel.
async fn pump_response<S: TunnelStream>(
    mut stream: S,
    body_tx: mpsc::Sender<Result<Vec<u8>, TunnelStreamError>>,
    cancel_rx: oneshot::Receiver<String>,
    request_id: Arc<str>,
    mut response_buffer: Vec<u8>,
) -> Result<(), TunnelError> {
    let mut buf = vec![0u8; 8192];

    tokio::select! {
        _ = async {
            loop {
                let n = stream.read(&mut buf).await?;
                if n == 0 {
                    break;
                }

                response_buffer.extend_from_slice(&buf[..n]);

                // Limit max frame size to prevent OOM (16 MB)
                if response_buffer.len() > 16 * 1024 * 1024 {
                    return Err(TunnelError::protocol("response frame too large (potential backpressure/OOM protection)"));
                }

                // Try to parse complete JSON lines
                while let Some(newline_pos) = response_buffer.iter().position(|&b| b == b'\n') {
                    let line = response_buffer[..newline_pos].to_vec();
                    response_buffer.drain(..=newline_pos);

                    if line.is_empty() {
                        continue;
                    }

                    match serde_json::from_slice::<TunnelResponse>(&line) {
                        Ok(response) => {
                            if let Some(err) = &response.error {
                                let _ = body_tx.send(Err(TunnelStreamError::from_error_message(err))).await;
                                return Ok(());
                            }

                            if !response.body_chunk.is_empty()
                                && body_tx.send(Ok(response.body_chunk.0)).await.is_err()
                            {
                                return Ok(());
                            }

                            if response.eof {
                                return Ok(());
                            }
                        }
                        Err(e) => {
                            return Err(TunnelError::Serialization(e));
                        }
                    }
                }
            }
            Ok::<(), TunnelError>(())
        } => {}

        reason = cancel_rx => {
            let reason = reason.unwrap_or_else(|_| "client_disconnected".to_string());
            tracing::info!("request {} cancelled: {}", request_id, reason);
            let _ = stream.close().await;
        }
    }

    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_build_request_id() {
        let id = build_request_id("my-namespace");
        assert!(id.starts_with("my-namespace:relay:"));
        assert!(id.len() > 20);
    }

    #[test]
    fn test_build_request_id_empty() {
        let id = build_request_id("");
        assert!(id.starts_with("tokiame:relay:"));
    }

    #[test]
    fn test_tunnel_stream_error_display() {
        let err = TunnelStreamError::new(502, "test_error", "test message");
        assert_eq!(err.to_string(), "test message");
        assert_eq!(err.status_code, 502);
    }
}
