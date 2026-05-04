//! Auth service — validates the Bearer token.
//!
//! Topmost layer of the gateway stack. Extracts `Authorization: Bearer <key>`
//! from the incoming request, validates it (against Toasty DB in the future),
//! and passes an `AuthedRequest` to the RouteService below.


use bytes::Bytes;
use http::{Request, Response, StatusCode};
use http_body_util::Full;
use hyper::body::Incoming;
use service_async::{
    MakeService, Service,
    layer::{FactoryLayer, layer_fn},
};

use super::AuthedRequest;

/// Authentication service: validates Bearer tokens.
pub struct AuthService<T> {
    pub inner: T,
}

impl<T> Service<Request<Incoming>> for AuthService<T>
where
    T: Service<AuthedRequest, Response = Response<Full<Bytes>>, Error = anyhow::Error>,
{
    type Response = Response<Full<Bytes>>;
    type Error = anyhow::Error;

    async fn call(&self, req: Request<Incoming>) -> Result<Self::Response, Self::Error> {
        // Extract Bearer token
        let auth_header = req.headers().get("authorization")
            .and_then(|v| v.to_str().ok())
            .and_then(|v| v.strip_prefix("Bearer "))
            .map(|s| s.to_string());

        let Some(token_key) = auth_header else {
            let body = serde_json::json!({
                "error": {
                    "message": "Missing or invalid Authorization header. Expected: Bearer <token>",
                    "type": "invalid_request_error",
                    "code": "invalid_api_key",
                }
            });
            let resp = Response::builder()
                .status(StatusCode::UNAUTHORIZED)
                .header("content-type", "application/json")
                .body(Full::new(Bytes::from(serde_json::to_vec(&body)?)))
                .unwrap();
            return Ok(resp);
        };

        // TODO: Validate token_key against Toasty Token table.
        // For now, accept any non-empty token.
        let token_name = format!("token:{}", &token_key[..token_key.len().min(8)]);

        let authed = AuthedRequest {
            inner: req,
            token_name,
        };

        self.inner.call(authed).await
    }
}

// -- Factory / Layer ----------------------------------------------------------

pub struct AuthServiceFactory<T> {
    inner: T,
}

impl<T: MakeService> MakeService for AuthServiceFactory<T> {
    type Service = AuthService<T::Service>;
    type Error = T::Error;

    fn make_via_ref(&self, old: Option<&Self::Service>) -> Result<Self::Service, Self::Error> {
        Ok(AuthService {
            inner: self.inner.make_via_ref(old.map(|o| &o.inner))?,
        })
    }
}

impl<T> AuthService<T> {
    pub fn layer<C>() -> impl FactoryLayer<C, T, Factory = AuthServiceFactory<T>> {
        layer_fn(|_c: &C, inner| AuthServiceFactory { inner })
    }
}
