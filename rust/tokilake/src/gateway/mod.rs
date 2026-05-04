//! Tokilake Gateway — Service layer definitions.
//!
//! Each service implements `service_async::Service` and wraps an inner service,
//! following the monolake pattern of composable service stacks.
//!
//! The gateway pipeline is assembled via `FactoryStack`:
//!
//! ```ignore
//! FactoryStack::new(config)
//!     .push(UpstreamService::layer())      // bottom: forward to upstream LLM provider
//!     .push(RouteService::layer())         // match model → channel
//!     .push(AuthService::layer())          // validate Bearer token
//! ```

pub mod auth;
pub mod route;
pub mod upstream;

use http::Request;
use hyper::body::Incoming;

// ---------- Shared types flowing through the pipeline ----------

/// Information about a resolved upstream channel.
#[derive(Debug, Clone)]
pub struct ChannelInfo {
    pub name:     String,
    pub provider: String,
    pub base_url: Option<String>,
    pub api_key:  Option<String>,
    pub models:   String,
    pub weight:   i32,
}

/// A request that has passed authentication.
pub struct AuthedRequest {
    pub inner:      Request<Incoming>,
    pub token_name: String,
}

/// A request that has been routed to a specific channel.
pub struct GatewayRequest {
    pub inner:      Request<Incoming>,
    pub token_name: String,
    pub model:      String,
    pub channel:    ChannelInfo,
}

// ---------- Gateway configuration ----------

/// Config struct used by the `FactoryStack`.
#[derive(Clone)]
pub struct GatewayConfig {
    // Future: DB handle, rate-limit settings, etc.
}

/// Build the complete gateway service stack.
///
/// Returns a `MakeService` whose `.make()` produces the final composed Service.
pub fn build_gateway_stack(
    config: GatewayConfig,
) -> impl service_async::MakeService<
    Service = impl service_async::Service<
        Request<Incoming>,
        Response = http::Response<http_body_util::Full<bytes::Bytes>>,
        Error = anyhow::Error,
    >,
    Error = std::convert::Infallible,
> {
    use service_async::stack::FactoryStack;

    let stack = FactoryStack::new(config)
        .push(upstream::UpstreamService::layer())
        .push(route::RouteService::layer())
        .push(auth::AuthService::layer());

    stack.into_inner()
}
