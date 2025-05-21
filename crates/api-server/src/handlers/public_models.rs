use faststr::FastStr;
use reqwest::StatusCode;
use serde::Serialize;
use std::collections::HashSet;
use storage::{ClientCache, Storage};
use tracing::error;
use volo_http::{
    response::Response,
    server::{IntoResponse, Router, route::get},
    utils::Extension,
};

async fn public_models_handler(Extension(db): Extension<Storage<ClientCache>>) -> Response {
    let records = match db.get_public_clients().await {
        Ok(records) => records,
        Err(e) => {
            error!("{e}");
            return StatusCode::BAD_REQUEST.into_response();
        }
    };

    #[derive(Debug, Serialize)]
    struct PublicModel {
        name:   FastStr,
        models: HashSet<FastStr>,
    }
    let res: Vec<_> = records
        .into_iter()
        .map(|r| PublicModel {
            name:   r.namespace,
            models: r.model_names.0,
        })
        .collect();
    let res = serde_json::to_string_pretty(&res).unwrap();
    (StatusCode::OK, res).into_response()
}

pub fn public_models_router() -> Router {
    Router::new().route("/v2/models", get(public_models_handler))
    // .route("/v1/chat/completions2", post(chat_completion_handler2))
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
