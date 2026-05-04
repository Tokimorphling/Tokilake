use tokilake::db::init_db;
use tokilake::gateway::{self, GatewayConfig};
use anyhow::Result;
use axum::Router;
use std::net::SocketAddr;
use tokio::net::TcpListener;
use tokilake::api;
use service_async::MakeService;

#[tokio::main]
async fn main() -> Result<()> {
    // Initialize Toasty ORM
    let _db = init_db().await?;

    println!("Tokilake starting...");

    // Build the gateway service stack (monolake-style)
    let config = GatewayConfig {};
    let gateway_factory = gateway::build_gateway_stack(config);
    let _gateway_svc = gateway_factory.make().expect("failed to build gateway service");

    // Management API (Axum — for admin CRUD on channels/tokens)
    let api_app = Router::new()
        .merge(api::router());

    let addr = SocketAddr::from(([0, 0, 0, 0], 3000));
    println!("Management API listening on http://{}", addr);

    let listener = TcpListener::bind(addr).await?;
    axum::serve(listener, api_app).await?;

    Ok(())
}
