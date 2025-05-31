use faststr::FastStr;
use inference_server::InferenceServer;
use reqwest::StatusCode;
use serde::Serialize;
use serde_json::json;
use std::collections::HashSet;
use tracing::error;
use volo_http::{
    response::Response,
    server::{IntoResponse, Router, route::get},
    utils::Extension,
};

#[derive(Debug, Serialize)]
struct PublicModel {
    name:   FastStr,
    models: HashSet<FastStr>,
}

async fn public_models_handler(Extension(server): Extension<InferenceServer>) -> Response {
    let db = server.db;
    let records = match db.get_public_clients().await {
        Ok(records) => records,
        Err(e) => {
            error!("{e}");
            return StatusCode::BAD_REQUEST.into_response();
        }
    };

    let res: Vec<_> = records
        .into_iter()
        .map(|r| PublicModel {
            name:   r.namespace,
            models: r.model_names.0,
        })
        .collect();
    (StatusCode::OK, public_model_list(res)).into_response()
}

pub fn public_models_router() -> Router {
    Router::new().route("/v2/models", get(public_models_handler))
}

fn public_model_list(models: Vec<PublicModel>) -> FastStr {
    json!({
    "object": "list",
    "data": models.into_iter().flat_map(|m| m.models.into_iter().map(move |model| json!({
    "id": model,
    "object": "model",
    "created": 0,
    "owned_by": m.name
    })))
    .collect::<Vec<_>>(),
    })
    .to_string()
    .into()
}

/*
todo:
{
  "object": "list",
  "data": [
    {
      "id": "model-id-0",
      "object": "model",
      "created": 1686935002,
      "owned_by": "organization-owner"
    },
    {
      "id": "model-id-1",
      "object": "model",
      "created": 1686935002,
      "owned_by": "organization-owner",
    },
    {
      "id": "model-id-2",
      "object": "model",
      "created": 1686935002,
      "owned_by": "openai"
    },
  ],
  "object": "list"
}

*/
