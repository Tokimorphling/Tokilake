use super::chat_completion::ResEvent;
use crate::error::Result;
use crate::handlers::chat_completion::convert_body_to_data;
use crate::requests::ChatCompletionRequest;
use crate::tools::create_done_frame;
use crate::tools::create_text_frame;
use crate::tools::generate_completion_id;
use crate::tools::generate_task_id;
use async_stream::stream;
use chrono::Utc;
use common::proxy::GrpcOriginalPayload;
use faststr::FastStr;
use futures_util::Stream;
use futures_util::StreamExt;
use futures_util::pin_mut;
use inference_server::InferenceServer;
use inference_server::InferenceService;
use inference_server::map_to_sse_stream;
use reqwest::StatusCode;
use std::sync::Arc;
use tracing::info;
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

async fn chat_completion_handler(
    Extension(server): Extension<InferenceServer>,
    Json(req): Json<ChatCompletionRequest>,
) -> Response {
    if !req.stream {
        return (
            StatusCode::BAD_REQUEST,
            "non-stream not supported currently",
        )
            .into_response();
    }

    let modelname_with_prefix = req.model.clone();
    if let Some((namespace, model_name)) = modelname_with_prefix.split_once(':') {
        let namespace = FastStr::from(namespace.to_owned());
        let model_name = FastStr::from(model_name.to_owned());
        info!(namespace=%namespace, model_name=%model_name, "recv inference quest");
        let data = convert_body_to_data(req, &model_name, generate_task_id(&namespace));
        let arc_server = Arc::new(server);
        let s = arc_server.chat_completion(namespace, data).await;
        let t = map_to_sse_stream(s).await;
        create_sse_response(t, model_name).await.into_response()
    } else {
        (
            StatusCode::BAD_REQUEST,
            "invalid format of model you provided",
        )
            .into_response()
    }
}

pub fn private_chat_completion_router() -> Router {
    Router::new().route("/v1/chat/completions", post(chat_completion_handler))
}

async fn create_sse_response<S, T>(
    input: S,
    model_name: FastStr,
) -> Sse<impl Stream<Item = Result<Event>>>
where
    S: Stream<Item = T>,
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
            let res_event = res_event.into();
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
                    break;
                }
                _ => {}
            }
        }
    };

    Sse::new(stream)
}

impl From<GrpcOriginalPayload> for ResEvent {
    fn from(val: GrpcOriginalPayload) -> Self {
        match val {
            GrpcOriginalPayload::StreamInferenceChunkResponse(chunk) => ResEvent::Text(chunk),
            GrpcOriginalPayload::StreamInferenceChunkEnd => ResEvent::Done,
            _ => ResEvent::Done,
        }
    }
}
