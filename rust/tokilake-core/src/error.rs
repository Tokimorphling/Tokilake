use thiserror::Error;

/// Tunnel error types.
#[derive(Debug, Error)]
pub enum TunnelError {
    #[error("stream closed")]
    StreamClosed,

    #[error("connection timeout")]
    Timeout,

    #[error("authentication failed: {message}")]
    AuthFailed { message: String },

    #[error("protocol error: {message}")]
    Protocol { message: String },

    #[error("transport error: {0}")]
    Transport(#[from] std::io::Error),

    #[error("serialization error: {0}")]
    Serialization(#[from] serde_json::Error),

    #[error("session offline")]
    SessionOffline,

    #[error("{0}")]
    Other(#[from] anyhow::Error),
}

impl TunnelError {
    pub fn auth_failed(message: impl Into<String>) -> Self {
        Self::AuthFailed {
            message: message.into(),
        }
    }

    pub fn protocol(message: impl Into<String>) -> Self {
        Self::Protocol {
            message: message.into(),
        }
    }
}

/// Error message for control protocol.
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct ErrorMessage {
    #[serde(skip_serializing_if = "String::is_empty", default)]
    pub code:    String,
    pub message: String,
}

impl ErrorMessage {
    pub fn new(code: impl Into<String>, message: impl Into<String>) -> Self {
        Self {
            code:    code.into(),
            message: message.into(),
        }
    }
}

impl std::fmt::Display for ErrorMessage {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        if self.code.is_empty() {
            write!(f, "{}", self.message)
        } else {
            write!(f, "[{}] {}", self.code, self.message)
        }
    }
}

/// Helper trait to map errors to TunnelError.
pub trait MapTunnelErr<T> {
    /// Maps any error to TunnelError::StreamClosed.
    fn map_stream_closed(self) -> Result<T, TunnelError>;
}

impl<T, E> MapTunnelErr<T> for Result<T, E> {
    fn map_stream_closed(self) -> Result<T, TunnelError> {
        self.map_err(|_| TunnelError::StreamClosed)
    }
}
