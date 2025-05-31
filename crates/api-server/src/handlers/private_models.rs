use crate::tools::generate_task_id;
use faststr::FastStr;
use inference_server::{InferenceServer, InferenceService, map_to_http_response};
use tracing::info;
use volo_http::{
    response::Response,
    server::{IntoResponse, Router, param::PathParams, route::get},
    utils::Extension,
};

async fn model_handler(
    Extension(server): Extension<InferenceServer>,
    PathParams(namespace): PathParams<FastStr>,
) -> Response {
    info!(namesapce=%namespace, "finding models belong to namespace");
    let task_id = generate_task_id(&namespace);
    map_to_http_response(server.models(task_id, namespace).await).into_response()
}

pub fn private_model_router() -> Router {
    Router::new().route("/v1/models/{:namespace}", get(model_handler))
}
