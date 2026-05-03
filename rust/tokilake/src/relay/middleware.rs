use axum::{
    extract::Request,
    middleware::Next,
    response::Response,
};

pub async fn auth_middleware(req: Request, next: Next) -> Result<Response, axum::http::StatusCode> {
    // TODO: Verify Bearer token against Toasty DB
    let auth_header = req.headers().get("Authorization");
    
    if let Some(_header) = auth_header {
        // Authenticate...
        Ok(next.run(req).await)
    } else {
        Err(axum::http::StatusCode::UNAUTHORIZED)
    }
}
