//! Request roundtrip forwarding through the tunnel.
//!
//! This module handles forwarding API requests through the tunnel to workers
//! and streaming responses back to the client. It supports:
//! - Request routing by channel ID or namespace
//! - Streaming response bodies
//! - Request cancellation
//! - Error propagation

use std::collections::HashMap;
use std::sync::Arc;
use std::time::Instant;

use tokio::sync::{mpsc, oneshot};
use uuid::Uuid;

use crate::error::{ErrorMessage, TunnelError};
use crate::protocol::{TunnelRequest, TunnelResponse};
use crate::session::{InFlightRequest, SessionManager};

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
            code: code.to_string(),
            message: message.to_string(),
            error_type: "upstream_error".to_string(),
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
pub struct Roundtrip {
    session_manager: Arc<SessionManager>,
}

impl Roundtrip {
    /// Create a new roundtrip handler.
    pub fn new(session_manager: Arc<SessionManager>) -> Self {
        Self { session_manager }
    }

    /// Forward a request to a worker by channel ID.
    ///
    /// Returns a streaming response receiver.
    pub async fn do_request_by_channel(
        &self,
        channel_id: i32,
        mut request: TunnelRequest,
    ) -> Result<TunnelRoundtripResponse, TunnelError> {
        let session = self
            .session_manager
            .get_by_channel_id(channel_id)
            .ok_or_else(|| {
                TunnelError::protocol(format!(
                    "tokiame session is offline for channel {}",
                    channel_id
                ))
            })?;

        if !session.is_alive() {
            return Err(TunnelError::protocol(format!(
                "tokiame session is offline for channel {}",
                channel_id
            )));
        }

        self.do_request_with_session(&session, &mut request).await
    }

    /// Forward a request to a worker by namespace.
    pub async fn do_request_by_namespace(
        &self,
        namespace: &str,
        mut request: TunnelRequest,
    ) -> Result<TunnelRoundtripResponse, TunnelError> {
        let session = self
            .session_manager
            .get_by_namespace(namespace)
            .ok_or_else(|| {
                TunnelError::protocol(format!(
                    "tokiame session is offline for namespace {}",
                    namespace
                ))
            })?;

        if !session.is_alive() {
            return Err(TunnelError::protocol(format!(
                "tokiame session is offline for namespace {}",
                namespace
            )));
        }

        self.do_request_with_session(&session, &mut request).await
    }

    /// Forward a request using an existing session.
    async fn do_request_with_session(
        &self,
        session: &crate::session::GatewaySession,
        request: &mut TunnelRequest,
    ) -> Result<TunnelRoundtripResponse, TunnelError> {
        // Ensure request has an ID
        if request.request_id.trim().is_empty() {
            request.request_id = build_request_id(&session.namespace);
        }

        // Open a stream for this request
        let mut stream = session.open_stream().await?;

        // Track the in-flight request
        let (cancel_tx, cancel_rx) = oneshot::channel();
        let request_id = request.request_id.clone();

        self.session_manager.track_request(InFlightRequest {
            request_id: request_id.clone(),
            session_id: session.id,
            namespace: session.namespace.clone(),
            channel_id: session.channel_id,
            created_at: Instant::now(),
        });

        // Write the request
        let request_json = serde_json::to_vec(request)?;
        stream.write(&request_json).await?;
        stream.flush().await?;

        // Set up response channel
        let (body_tx, body_rx) = mpsc::channel(32);

        // Spawn response pump
        let session_id = session.id;
        let request_id_clone = request_id.clone();
        let session_manager = self.session_manager.clone();

        tokio::spawn(async move {
            let result =
                pump_response(stream, body_tx, cancel_rx, session_id, &request_id_clone).await;

            session_manager.remove_request(&request_id_clone);

            if let Err(e) = result {
                tracing::error!("tunnel response pump error: {}", e);
            }
        });

        // Read the first frame for headers
        // This is simplified - in reality you'd read from the stream
        Ok(TunnelRoundtripResponse {
            status_code: 200,
            headers: HashMap::new(),
            body_rx,
            cancel_tx: Some(cancel_tx),
        })
    }

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
    let namespace = if namespace.is_empty() { "tokiame" } else { namespace };
    format!("{}:relay:{}", namespace, Uuid::new_v4())
}

/// Pump response frames from the tunnel stream to the response channel.
async fn pump_response(
    mut stream: Box<dyn crate::tunnel::TunnelStream>,
    body_tx: mpsc::Sender<Result<Vec<u8>, TunnelStreamError>>,
    cancel_rx: oneshot::Receiver<String>,
    _session_id: u64,
    request_id: &str,
) -> Result<(), TunnelError> {
    let mut buf = vec![0u8; 8192];
    let mut response_buffer = Vec::new();

    tokio::select! {
        _ = async {
            loop {
                let n = stream.read(&mut buf).await?;
                if n == 0 {
                    break;
                }

                response_buffer.extend_from_slice(&buf[..n]);

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
                                && body_tx.send(Ok(response.body_chunk)).await.is_err()
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
