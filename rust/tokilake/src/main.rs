use anyhow::Result;
use service_async::MakeService;
use std::net::SocketAddr;
use tokilake::{
    api::{self, AppState},
    db::init_db,
    gateway::{self, GatewayConfig},
};
use tokio::net::TcpListener;

#[tokio::main]
async fn main() -> Result<()> {
    // Parse port from args: -addr :PORT or --port PORT
    let port = parse_port();

    // Initialize Toasty ORM
    let _db = init_db().await?;

    println!("Tokilake starting...");

    // Build the gateway service stack (monolake-style)
    let config = GatewayConfig {};
    let gateway_factory = gateway::build_gateway_stack(config);
    let _gateway_svc = gateway_factory
        .make()
        .expect("failed to build gateway service");

    // Unified server: management API + OpenAI-compatible relay
    let state = AppState {
        start_time: std::time::Instant::now(),
    };
    let app = api::router(state);

    let addr = SocketAddr::from(([0, 0, 0, 0], port));
    println!("Tokilake listening on http://{}", addr);

    let listener = TcpListener::bind(addr).await?;
    axum::serve(listener, app).await?;

    Ok(())
}

fn parse_port() -> u16 {
    let args: Vec<String> = std::env::args().collect();
    let mut i = 1;
    while i < args.len() {
        match args[i].as_str() {
            "-addr" | "--addr" => {
                if i + 1 < args.len() {
                    // Format: ":18080" or "0.0.0.0:18080"
                    let addr = &args[i + 1];
                    if let Some(port_str) = addr.strip_prefix(':') {
                        return port_str.parse().unwrap_or(3000);
                    }
                    if let Some(pos) = addr.rfind(':') {
                        return addr[pos + 1..].parse().unwrap_or(3000);
                    }
                }
                i += 2;
            }
            "-port" | "--port" => {
                if i + 1 < args.len() {
                    return args[i + 1].parse().unwrap_or(3000);
                }
                i += 2;
            }
            // Accept -token flag (used by test script) — silently consume it.
            "-token" | "--token" => {
                // TODO: store token for auth validation
                i += 2;
            }
            _ => {
                i += 1;
            }
        }
    }
    3000
}
