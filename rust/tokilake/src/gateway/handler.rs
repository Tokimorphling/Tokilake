use std::future::Future;
use std::pin::Pin;
use std::task::{Context, Poll};
use hyper::{Request, Response};
use hyper::body::Incoming;
use http_body_util::Full;
use bytes::Bytes;
use tower_service::Service;

use tokilake_core::session::SessionManager;
use tokilake_core::tunnel::smux::SmuxSession;
use tokilake_core::tunnel::quic::QuicSession;
use std::sync::Arc;

#[derive(Clone)]
pub struct GatewayProxy {
    pub smux_manager: Arc<SessionManager<SmuxSession>>,
    pub quic_manager: Arc<SessionManager<QuicSession>>,
}

impl GatewayProxy {
    pub fn new(
        smux_manager: Arc<SessionManager<SmuxSession>>,
        quic_manager: Arc<SessionManager<QuicSession>>,
    ) -> Self {
        Self {
            smux_manager,
            quic_manager,
        }
    }
}

impl Service<Request<Incoming>> for GatewayProxy {
    type Response = Response<Full<Bytes>>;
    type Error = anyhow::Error;
    type Future = Pin<Box<dyn Future<Output = Result<Self::Response, Self::Error>> + Send>>;

    fn poll_ready(&mut self, _cx: &mut Context<'_>) -> Poll<Result<(), Self::Error>> {
        Poll::Ready(Ok(()))
    }

    fn call(&mut self, req: Request<Incoming>) -> Self::Future {
        let clone = self.clone();
        Box::pin(async move {
            // Log incoming request
            println!("Gateway received request: {} {}", req.method(), req.uri());
            
            // TODO: Extract API key, find Channel/Token from DB, routing.
            // For now, return a mock response.
            let response = Response::builder()
                .status(200)
                .body(Full::new(Bytes::from("Hello from Tokilake Gateway Proxy!")))
                .unwrap();
                
            Ok(response)
        })
    }
}
