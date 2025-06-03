use faststr::FastStr;
use thiserror::Error;

#[derive(Debug, Error)]
pub enum Error {
    #[error("Io Error: {0}")]
    Io(#[from] std::io::Error),

    #[error("Invalid Requst Body: {0}")]
    InvalidRequestBody(#[from] serde_json::Error),

    #[error("invalid model: {0}")]
    InvalidModel(FastStr),

    #[error("model not found: {0}")]
    ModelNotFound(FastStr),

    #[error("invalid namespace")]
    InvalidNamespace(FastStr),

    #[error("faild to build client")]
    FailedToBuildClient,

    #[error("failed to create sse response: {0}")]
    CreateSseResponse(FastStr),

    #[error("unresolved resevent")]
    UnresolvedResEvent,

    #[error("{0}")]
    MsgError(&'static str),
}

pub type Result<T> = std::result::Result<T, Error>;
