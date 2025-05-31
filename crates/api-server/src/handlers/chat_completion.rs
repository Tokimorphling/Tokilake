// Code from aichat
use crate::{
    error::Result,
    tools::{create_done_frame, create_text_frame},
};
use crate::{
    models::ChatCompletionRequest,
    tools::{build_http_client, find_forward_clients, generate_completion_id, split_model},
};
use async_stream::stream;
use chrono::Utc;
use common::proxy::chat_openai::{OpenAIClient, OpenAIClinetConfig};
use common::{
    ToRandomIterator,
    data::ChatCompletionsData,
    stream::sse::{SseEvent, SseHandler},
};
use faststr::FastStr;
use futures_util::{Stream, pin_mut};
use inference_server::InferenceServer;
use reqwest::StatusCode;
use std::sync::{
    Arc,
    atomic::{AtomicBool, Ordering},
};
use tokio::sync::mpsc::{UnboundedReceiver, UnboundedSender, unbounded_channel};
use tokio_stream::{StreamExt, wrappers::UnboundedReceiverStream};
use tracing::{debug, info, warn};
use volo_http::{
    response::Response,
    server::{
        IntoResponse,
        extract::Json,
        response::sse::{Event, Sse},
        route::{Router, post},
    },
    utils::Extension,
};

#[derive(Debug)]
pub enum ResEvent {
    First,
    Text(FastStr),
    Done,
}

async fn chat_completion_handler(
    Extension(server): Extension<InferenceServer>,
    Json(req): Json<ChatCompletionRequest>,
) -> Response {
    debug!(req=?req);
    if !req.stream {
        return (
            StatusCode::BAD_REQUEST,
            "non-stream not supported currently",
        )
            .into_response();
    }

    let (namespace, model_name) = match split_model(&req.model) {
        Ok((namespace, model_name)) => (namespace, model_name),
        Err(err) => {
            return (StatusCode::BAD_REQUEST, err.to_string()).into_response();
        }
    };
    let db = &server.db;
    info!(namespace=%namespace, model_name=%model_name);
    let (fc, model_name) = match find_forward_clients(db, namespace, model_name).await {
        Ok((model, model_name)) => (model, model_name),
        Err(err) => {
            return (StatusCode::BAD_REQUEST, err.to_string()).into_response();
        }
    };

    let (tx, rx) = unbounded_channel();
    let name = model_name.clone();
    tokio::spawn(async move {
        let (sse_tx, sse_rx) = unbounded_channel();
        let http_client = build_http_client().unwrap();
        let mut handler = SseHandler::new(sse_tx);
        let data = convert_body_to_data(req, &model_name, "".into());
        debug!("data: {}", serde_json::to_string_pretty(&data).unwrap());
        let namespace = fc.namespace;
        let api_secret = fc.api_keys.random_iter().next().unwrap();
        let api_base = fc.api_base;
        let config =
            OpenAIClinetConfig::new(&namespace, Some(api_secret), &api_base, "", &model_name);
        let client = OpenAIClient::new(config);
        let is_first = Arc::new(AtomicBool::new(true));
        tokio::join!(
            map_event(sse_rx, &tx, is_first.clone()),
            chat_completions(client, &http_client, &mut handler, data, &tx, is_first)
        );
    });
    let rx = UnboundedReceiverStream::new(rx);
    create_sse_response(rx, name).await.into_response()
}

#[inline]
pub fn convert_body_to_data(
    body: ChatCompletionRequest,
    name: &str,
    task_id: FastStr,
) -> ChatCompletionsData {
    ChatCompletionsData {
        task_id,

        model_name: name.to_owned().into(),
        messages: body.messages,
        temperature: body.temperature,
        top_p: body.top_p,
        stream: body.stream,
        max_tokens: body.max_tokens,
        enable_thinking: None,
        thinking_budget: None,
        frequency_penalty: None,
    }
}

async fn map_event(
    mut sse_rx: UnboundedReceiver<SseEvent>,
    tx: &UnboundedSender<ResEvent>,
    is_first: Arc<AtomicBool>,
) {
    while let Some(reply_event) = sse_rx.recv().await {
        if is_first.load(Ordering::SeqCst) {
            let _ = tx.send(ResEvent::First);
            is_first.store(false, Ordering::SeqCst)
        }
        match reply_event {
            SseEvent::Text(text) => {
                if tx.send(ResEvent::Text(text)).is_err() {
                    warn!("send event error: channel may closed");
                    sse_rx.close();
                }
            }
            SseEvent::Done => {
                let _ = tx.send(ResEvent::Done);
                sse_rx.close();
            }
            SseEvent::Error => {
                warn!("an error occuarred");
                sse_rx.close();
            }
        }
    }
}

async fn chat_completions(
    client: OpenAIClient<'_>,
    http_client: &reqwest::Client,
    handler: &mut SseHandler,
    data: ChatCompletionsData,
    tx: &UnboundedSender<ResEvent>,
    is_first: Arc<AtomicBool>,
) {
    let _first = if let Err(e) = client
        .openai_chat_completions_streaming(http_client, handler, data)
        .await
    {
        Some(FastStr::from(format!("{e:?}")))
    } else {
        None
    };
    if is_first.load(Ordering::SeqCst) {
        let _ = tx.send(ResEvent::First);
        is_first.store(false, Ordering::SeqCst);
    }
    handler.done();
}

async fn create_sse_response<S, T>(
    input: S,
    model_name: FastStr,
) -> Sse<impl Stream<Item = Result<Event>>>
where
    S: Stream<Item = T> + 'static,
    T: Into<ResEvent>,
{
    let comp_id = generate_completion_id();
    let created_at = Utc::now().timestamp();
    let shared = Arc::new((comp_id, created_at, model_name));

    let stream = stream! {
        let shared = shared.clone();
        pin_mut!(input);
        while let Some(res_event) = input.next().await {
            let (comp_id, created_at, model_name) = shared.as_ref();
            let res_event: ResEvent = res_event.into();
            match res_event {
                ResEvent::Text(t) => {
                    let frame = create_text_frame(
                        comp_id,
                        model_name,
                        *created_at,
                        &t,
                    );
                    yield Ok(Event::new().data(frame));
                }
                ResEvent::Done => {
                    let (frame, done) = create_done_frame(
                        comp_id,
                        model_name,
                        *created_at,
                    );
                    yield Ok(Event::new().data(frame));
                    yield Ok(Event::new().data(done));
                }
                _ => {}
            }
        }
    };

    Sse::new(stream)
}

pub fn chat_completion_router() -> Router {
    Router::new().route("/v2/chat/completions", post(chat_completion_handler))
}
