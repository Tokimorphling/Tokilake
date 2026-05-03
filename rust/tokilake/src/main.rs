use tokilake::db::init_db;
use anyhow::Result;
use axum::{Router, middleware};
use std::net::SocketAddr;
use tokio::net::TcpListener;
use tokilake::api;
use tokilake::relay;

#[tokio::main]
async fn main() -> Result<()> {
    // Initialize Toasty ORM
    let _db = init_db().await?;
    
    println!("Tokilake starting...");
    
    // Gateway Server and API Server will be started here.
    let app = Router::new()
        // API routes
        .nest("/", api::router())
        // Relay middleware for OpenAI passthrough
        .layer(middleware::from_fn(relay::middleware::auth_middleware));

    let addr = SocketAddr::from(([0, 0, 0, 0], 3000));
    println!("Listening on http://{}", addr);
    
    let listener = TcpListener::bind(addr).await?;
    axum::serve(listener, app).await?;

    Ok(())
}
