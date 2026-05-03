use crate::{
    error::TunnelError,
    protocol::{RegisterResult, Token},
};
use std::future::Future;

/// Authenticator trait - authenticates tunnel worker tokens.
pub trait Authenticator: Send + Sync + 'static {
    fn authenticate_token_key(
        &self,
        token_key: &str,
    ) -> impl Future<Output = Result<(String, Token), TunnelError>> + Send;
}

/// Worker registry trait - manages worker registration.
pub trait WorkerRegistry: Send + Sync + 'static {
    fn register_worker(
        &self,
        session_id: u64,
        namespace: &str,
        node_name: &str,
        group: &str,
        models: &[String],
        backend_type: &str,
    ) -> impl Future<Output = Result<RegisterResult, TunnelError>> + Send;

    fn update_heartbeat(
        &self,
        worker_id: i32,
        status: i32,
        node_name: &str,
        current_models: &[String],
    ) -> impl Future<Output = Result<(), TunnelError>> + Send;

    fn sync_models(
        &self,
        worker_id: i32,
        group: &str,
        models: &[String],
        backend_type: &str,
    ) -> impl Future<Output = Result<(), TunnelError>> + Send;

    fn cleanup_worker(
        &self,
        worker_id: i32,
    ) -> impl Future<Output = Result<(), TunnelError>> + Send;
}
