use std::future::Future;

/// Generic service trait for handling requests, leveraging Higher-Rank Trait Bounds (HRTB)
/// to avoid dynamic dispatch (`Box<dyn Future>`) overhead.
pub trait Service<Req> {
    type Response;
    type Error;

    /// Process the request and return the response.
    fn call(&self, req: Req) -> impl Future<Output = Result<Self::Response, Self::Error>> + Send;
}
