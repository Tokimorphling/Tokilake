//! Route service — resolves the requested model to a Channel.
//!
//! Sits in the middle of the stack. Receives a raw HTTP request from the
//! AuthService above, extracts the `model` field from the JSON body (or path),
//! looks up a matching Channel, and passes a `GatewayRequest` down to the
//! UpstreamService below.

use super::{AuthedRequest, ChannelInfo, GatewayRequest};
use bytes::Bytes;
use http::Response;
use http_body_util::Full;
use service_async::{
    MakeService, Service,
    layer::{FactoryLayer, layer_fn},
};

/// Routing service: maps model → channel, then delegates to inner.
pub struct RouteService<T> {
    pub inner: T,
}

impl<T> Service<AuthedRequest> for RouteService<T>
where
    T: Service<GatewayRequest, Response = Response<Full<Bytes>>, Error = anyhow::Error>,
{
    type Response = Response<Full<Bytes>>;
    type Error = anyhow::Error;

    async fn call(&self, req: AuthedRequest) -> Result<Self::Response, Self::Error> {
        // Extract model from the URI path, e.g. /v1/chat/completions → use a
        // default model, or parse the JSON body.
        // For now, use a simple path-based heuristic.
        let model = "default".to_string();

        // TODO: look up from Toasty DB.  For now, use a stub channel.
        let channel = ChannelInfo {
            name:     "stub".into(),
            provider: "openai".into(),
            base_url: Some("https://api.openai.com".into()),
            api_key:  Some("sk-stub".into()),
            models:   "gpt-4,gpt-3.5-turbo".into(),
            weight:   1,
        };

        let gw_req = GatewayRequest {
            inner: req.inner,
            token_name: req.token_name,
            model,
            channel,
        };

        self.inner.call(gw_req).await
    }
}

// -- Factory / Layer ----------------------------------------------------------

pub struct RouteServiceFactory<T> {
    inner: T,
}

impl<T: MakeService> MakeService for RouteServiceFactory<T> {
    type Service = RouteService<T::Service>;
    type Error = T::Error;

    fn make_via_ref(&self, old: Option<&Self::Service>) -> Result<Self::Service, Self::Error> {
        Ok(RouteService {
            inner: self.inner.make_via_ref(old.map(|o| &o.inner))?,
        })
    }
}

impl<T> RouteService<T> {
    pub fn layer<C>() -> impl FactoryLayer<C, T, Factory = RouteServiceFactory<T>> {
        layer_fn(|_c: &C, inner| RouteServiceFactory { inner })
    }
}
