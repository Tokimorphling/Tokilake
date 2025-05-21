pub use common::logging_stdout;
use inference_server::InferenceServer;
use std::net::SocketAddr;
use volo_http::Address;
use volo_http::server::{Router, Server};
use volo_http::utils::Extension;

pub mod error;
pub mod handlers;
pub mod models;
pub mod tools;

pub async fn run_api_server(addr: SocketAddr, server: InferenceServer) {
    let app = Router::new()
        .merge(handlers::private_chat_completion_router())
        .merge(handlers::chat_completion_router())
        .merge(handlers::public_models_router())
        // .layer(Extension(db))
        .layer(Extension(server));
    let addr = Address::from(addr);
    Server::new(app).run(addr).await.unwrap();
}
