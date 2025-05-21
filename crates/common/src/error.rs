use faststr::FastStr;
use reqwest_eventsource::CannotCloneRequestError;
use thiserror::Error;

use crate::stream::sse::SseEvent;

#[derive(Debug, Error)]
pub enum Error {
    #[error("{0}")]
    MsgError(FastStr),
    #[error("{0}")]
    CannotCloneRequestError(#[from] CannotCloneRequestError),
    #[error("Reqwest error: {0}")]
    ReqwestError(#[from] reqwest::Error),
    #[error("Invalid response data: {0} status: {1}")]
    InvalidResponseData(FastStr, u16),
    #[error("Invalid response event-stream: content-type: {0}, data: {1}")]
    InvalidResponseEventStream(FastStr, FastStr),
    #[error("Reqwest eventsource error: {0}")]
    ReqwestEventsourceError(#[from] Box<reqwest_eventsource::Error>),
    #[error("serde error: {0}")]
    SerdeError(#[from] serde_json::Error),
    #[error("Failed to send SseEvent: {0}")]
    SendSseEventError(SseEvent),
}

pub type Result<T> = std::result::Result<T, Error>;
