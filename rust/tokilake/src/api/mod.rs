use axum::{
    Json, Router,
    extract::State,
    response::IntoResponse,
    routing::{get, post},
};
use std::sync::Arc;

/// Shared application state available to all handlers.
#[derive(Clone)]
pub struct AppState {
    pub start_time: std::time::Instant,
}

pub fn router(state: AppState) -> Router {
    Router::new()
        // Health / status
        .route("/health", get(health))
        // Management API
        .route("/api/channel", get(list_channels))
        .route("/api/token", get(list_tokens))
        // OpenAI-compatible relay (gateway service pipeline)
        .route("/v1/chat/completions", post(chat_completions))
        .with_state(Arc::new(state))
}

async fn health(State(state): State<Arc<AppState>>) -> impl IntoResponse {
    let uptime = state.start_time.elapsed().as_secs();
    Json(serde_json::json!({
        "status": "ok",
        "uptime": uptime,
        "sessions": 0,
    }))
}

async fn list_channels() -> &'static str {
    // TODO: fetch from Toasty
    "[]"
}

async fn list_tokens() -> &'static str {
    // TODO: fetch from Toasty
    "[]"
}

async fn chat_completions(Json(body): Json<serde_json::Value>) -> impl IntoResponse {
    // TODO: Wire through the gateway service stack.
    // For now, return a stub OpenAI-compatible response.
    let model = body
        .get("model")
        .and_then(|v| v.as_str())
        .unwrap_or("unknown");

    let is_stream = body
        .get("stream")
        .and_then(|v| v.as_bool())
        .unwrap_or(false);

    if is_stream {
        // SSE stub
        let sse_body = format!(
            "data: {{\"id\":\"chatcmpl-stub\",\"object\":\"chat.completion.chunk\",\"model\":\"\
             {model}\",\"choices\":[{{\"index\":0,\"delta\":{{\"role\":\"assistant\",\"content\":\\
             "Hello from Tokilake!\"}},\"finish_reason\":null}}]}}\n\ndata: \
             {{\"id\":\"chatcmpl-stub\",\"object\":\"chat.completion.chunk\",\"model\":\"{model}\"\
             ,\"choices\":[{{\"index\":0,\"delta\":{{}},\"finish_reason\":\"stop\"}}]}}\n\ndata: \
             [DONE]\n\n"
        );
        axum::response::Response::builder()
            .status(200)
            .header("content-type", "text/event-stream")
            .body(axum::body::Body::from(sse_body))
            .unwrap()
            .into_response()
    } else {
        Json(serde_json::json!({
            "id": "chatcmpl-stub",
            "object": "chat.completion",
            "model": model,
            "choices": [{
                "index": 0,
                "message": {
                    "role": "assistant",
                    "content": "Hello from Tokilake!"
                },
                "finish_reason": "stop"
            }],
            "usage": {
                "prompt_tokens": 10,
                "completion_tokens": 5,
                "total_tokens": 15
            }
        }))
        .into_response()
    }
}
