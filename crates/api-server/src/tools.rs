use crate::error::{Error, Result};
use chrono::Timelike;
use common::clients::ForwardClient;
use faststr::FastStr;
use reqwest::Client as ReqwestClient;
use serde_json::{Value, json};
use std::time::Duration;
use storage::{ClientCache, Storage};

#[inline]
pub fn build_http_client() -> Result<ReqwestClient> {
    let builder = ReqwestClient::builder();
    let timeout = 10;
    let client = builder
        // .no_proxy()
        .connect_timeout(Duration::from_secs(timeout))
        .build()
        .map_err(|_| Error::FailedToBuildClient)?;
    Ok(client)
}

#[inline]
pub fn split_model(model: &str) -> Result<(&str, &str)> {
    let (namespace, name) = model
        .split_once(':')
        .ok_or(Error::InvalidModel(model.to_owned().into()))?;
    Ok((namespace, name))
}

pub async fn find_forward_clients(
    db: &Storage<ClientCache>,
    namespace: &str,
    model_name: &str,
) -> Result<(ForwardClient, FastStr)> {
    let client = match db.get_client_by_namespace(namespace).await {
        Ok(Some(client)) => client,
        Ok(None) => return Err(Error::InvalidNamespace(namespace.to_owned().into())),
        Err(_e) => return Err(Error::MsgError("db error")),
    };
    if client.model_names.0.contains(model_name) {
        return Ok((client.clone(), model_name.to_owned().into()));
    }

    Err(Error::ModelNotFound(model_name.to_owned().into()))
}

#[inline]
pub fn generate_completion_id() -> FastStr {
    let random_id = chrono::Utc::now().nanosecond();
    format!("chatcmpl-{random_id}").into()
}

#[inline]
pub fn create_text_frame(id: &str, model: &str, created: i64, content: &str) -> FastStr {
    let delta = if content.is_empty() {
        json!({ "role": "assistant", "content": content })
    } else {
        json!({ "content": content })
    };
    let choice = json!({
        "index": 0,
        "delta": delta,
        "finish_reason": null,
    });
    let value = build_chat_completion_chunk_json(id, model, created, &choice);
    format!("{value}").into()
}
#[inline]
pub fn create_done_frame(id: &str, model: &str, created: i64) -> (FastStr, FastStr) {
    let finish_reason = "stop";
    let choice = json!({
        "index": 0,
        "delta": {},
        "finish_reasion": finish_reason
    });
    let v = build_chat_completion_chunk_json(id, model, created, &choice);
    (format!("{v}").into(), "[DONE]".into())
}

#[inline]
fn build_chat_completion_chunk_json(id: &str, model: &str, created: i64, choice: &Value) -> Value {
    json!({
        "id": id,
        "object": "chat.completion.chunk",
        "created": created,
        "model": model,
        "choices": [choice],
    })
}

#[inline]
pub fn generate_task_id(namespace: &str) -> FastStr {
    let random_id = chrono::Utc::now().nanosecond();
    format!("{namespace}-{random_id}").into()
}
