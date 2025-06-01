use anyhow::Result;
use dotenvy::dotenv;
use std::net::SocketAddr;
use storage::{ClientCache, Storage};
#[tokio::main]
async fn main() -> Result<()> {
    let _guard = api_server::logging_stdout();
    dotenv().ok();

    let database_url = std::env::var("DATABASE_URL").expect("no database url");

    
    let storage = Storage::<ClientCache>::new(&database_url).await?;

    let inference_server_addr: SocketAddr = "0.0.0.0:19982".parse()?;

    let api_server_addr = "0.0.0.0:19981".parse()?;

    let s = inference_server::run_inference_server(inference_server_addr, storage).await;

    api_server::run_api_server(api_server_addr, s).await;

    Ok(())
}
