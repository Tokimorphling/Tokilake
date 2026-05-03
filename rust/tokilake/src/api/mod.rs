use axum::{Router, routing::get};

pub fn router() -> Router {
    Router::new()
        .route("/api/channel", get(list_channels))
        .route("/api/token", get(list_tokens))
}

async fn list_channels() -> &'static str {
    // TODO: fetch from Toasty
    "[]"
}

async fn list_tokens() -> &'static str {
    // TODO: fetch from Toasty
    "[]"
}
