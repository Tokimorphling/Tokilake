use crate::error::{Error, Result};
use faststr::FastStr;
use futures_util::StreamExt;
use reqwest::RequestBuilder;
use reqwest_eventsource::{Error as EventSourceError, Event, RequestBuilderExt};
use serde_json::Value;
use std::fmt::Display;
use tokio::sync::mpsc::UnboundedSender;
use tracing::{debug, warn};

#[derive(Debug, Clone)]
pub enum SseEvent {
    Text(FastStr),
    Done,
    Error,
}

impl Display for SseEvent {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::Text(t) => write!(f, "{t}"),
            Self::Done => write!(f, "Done"),
            Self::Error => write!(f, "Error"),
        }
    }
}

#[derive(Debug)]
pub struct SseMmessage {
    #[allow(unused)]
    pub event: FastStr,
    pub data:  FastStr,
}

pub struct SseHandler {
    sender: UnboundedSender<SseEvent>,
}

impl SseHandler {
    pub fn new(sender: UnboundedSender<SseEvent>) -> Self {
        Self { sender }
    }

    pub fn text(&mut self, text: &str) -> Result<()> {
        debug!("SseEvnet: {text}");
        if text.is_empty() {
            return Ok(());
        }

        let message = SseEvent::Text(text.to_owned().into());
        if self.sender.send(message.clone()).is_err() {
            return Err(Error::SendSseEventError(message));
        }
        Ok(())
    }

    pub fn done(&mut self) {
        debug!("SseEvnet: Done");
        let message = SseEvent::Done;
        if self.sender.send(message.clone()).is_err() {
            warn!("failed to send SseEvent:Done, tx maybe closed")
        }
    }
}

pub async fn sse_stream<F>(builder: RequestBuilder, mut handle: F) -> Result<()>
where
    F: FnMut(SseMmessage) -> Result<bool>,
{
    let mut es = builder.eventsource()?;
    while let Some(event) = es.next().await {
        match event {
            Ok(Event::Open) => {}
            Ok(Event::Message(message)) => {
                let message = SseMmessage {
                    event: message.event.into(),
                    data:  message.data.into(),
                };
                if handle(message)? {
                    break;
                }
            }
            Err(err) => {
                match err {
                    EventSourceError::StreamEnded => {
                        debug!("Stream End");
                    }
                    EventSourceError::InvalidStatusCode(status, res) => {
                        let text = res.text().await?;
                        let _data: Value = match text.parse() {
                            Ok(data) => data,
                            Err(_) => {
                                return Err(Error::InvalidResponseData(
                                    text.into(),
                                    status.as_u16(),
                                ));
                            }
                        };
                    }
                    EventSourceError::InvalidContentType(header_value, res) => {
                        let text = res.text().await?;
                        return Err(Error::InvalidResponseEventStream(
                            header_value.to_str().unwrap_or_default().to_owned().into(),
                            text.into(),
                        ));
                    }
                    _ => {
                        return Err(Error::ReqwestEventsourceError(err.into()));
                    }
                }
                es.close();
            }
        }
    }

    Ok(())
}
