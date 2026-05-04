//! Upstream forwarding service — the bottom of the service stack.
//!
//! Receives a fully routed request (with the upstream base_url and api_key already
//! resolved by the RouteService above it) and forwards it to the LLM provider via
//! an HTTP client, streaming the response body back.

use super::GatewayRequest;
use bytes::Bytes;
use http::{Response, StatusCode};
use http_body_util::Full;
use service_async::{
    MakeService, Service,
    layer::{FactoryLayer, layer_fn},
};
use std::convert::Infallible;

/// The leaf service: forwards the request to the resolved upstream.
#[derive(Clone)]
pub struct UpstreamService;

impl Service<GatewayRequest> for UpstreamService {
    type Response = Response<Full<Bytes>>;
    type Error = anyhow::Error;

    async fn call(&self, req: GatewayRequest) -> Result<Self::Response, Self::Error> {
        let channel = &req.channel;
        let base_url = channel
            .base_url
            .as_deref()
            .unwrap_or("https://api.openai.com");
        let _api_key = channel.api_key.as_deref().unwrap_or("");

        // Build upstream URI
        let path = req
            .inner
            .uri()
            .path_and_query()
            .map(|pq| pq.as_str())
            .unwrap_or("/");
        let upstream_uri = format!("{}{}", base_url.trim_end_matches('/'), path);

        // TODO: Use hyper client to forward the request body as a stream.
        // For now, return a placeholder that proves the pipeline works end-to-end.
        let body = serde_json::json!({
            "status": "ok",
            "upstream": upstream_uri,
            "provider": &channel.provider,
            "model": &req.model,
        });

        let response = Response::builder()
            .status(StatusCode::OK)
            .header("content-type", "application/json")
            .body(Full::new(Bytes::from(serde_json::to_vec(&body)?)))
            .unwrap();

        Ok(response)
    }
}

// -- Factory / Layer ----------------------------------------------------------

pub struct UpstreamServiceFactory;

impl MakeService for UpstreamServiceFactory {
    type Service = UpstreamService;
    type Error = Infallible;

    fn make_via_ref(&self, _old: Option<&Self::Service>) -> Result<Self::Service, Self::Error> {
        Ok(UpstreamService)
    }
}

impl UpstreamService {
    pub fn layer<C>() -> impl FactoryLayer<C, (), Factory = UpstreamServiceFactory> {
        layer_fn(|_c: &C, _inner: ()| UpstreamServiceFactory)
    }
}
