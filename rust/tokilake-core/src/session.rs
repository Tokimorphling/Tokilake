use crate::{error::TunnelError, protocol::Token};
use dashmap::DashMap;
use std::{
    sync::{
        atomic::{AtomicU64, Ordering},
        Arc,
    },
    time::Instant,
};
use tokio::sync::RwLock;

/// Gateway session - represents a connected tunnel worker.
pub struct GatewaySession {
    pub id:            u64,
    pub token:         Option<Token>,
    pub token_key:     String,
    pub remote_addr:   String,
    pub connected_at:  Instant,
    pub worker_id:     i32,
    pub channel_id:    i32,
    pub namespace:     String,
    pub group:         String,
    pub backend_type:  String,
    pub models:        Vec<String>,
    pub status:        i32,
    pub transport:     String,
    pub authenticated: bool,
    // Control stream write half (for sending messages to worker)
    pub control_tx:    Option<tokio::sync::mpsc::Sender<Vec<u8>>>,
    // Smux session for opening data streams
    pub smux_session:  Option<Arc<tokio::sync::Mutex<tokilake_smux::Session>>>,
}

pub struct TunnelStreamRequest {
    pub request_id:  String,
    pub response_tx: tokio::sync::oneshot::Sender<TunnelStreamResponse>,
}

pub struct TunnelStreamResponse {
    pub reader: Box<dyn tokio::io::AsyncRead + Unpin + Send>,
    pub writer: Box<dyn tokio::io::AsyncWrite + Unpin + Send>,
}

impl GatewaySession {
    pub fn new(
        id: u64,
        token: Option<Token>,
        token_key: String,
        remote_addr: String,
        transport: String,
    ) -> Self {
        let authenticated = token.is_some();
        Self {
            id,
            token,
            token_key,
            remote_addr,
            connected_at: Instant::now(),
            worker_id: 0,
            channel_id: 0,
            namespace: String::new(),
            group: String::new(),
            backend_type: String::new(),
            models: Vec::new(),
            status: 0,
            transport,
            authenticated,
            control_tx: None,
            smux_session: None,
        }
    }
}

/// Thread-safe session manager.
pub struct SessionManager {
    next_id:       AtomicU64,
    by_namespace:  DashMap<String, Arc<RwLock<GatewaySession>>>,
    by_channel_id: DashMap<i32, Arc<RwLock<GatewaySession>>>,
    requests:      DashMap<String, InFlightRequest>,
}

#[derive(Debug, Clone)]
pub struct InFlightRequest {
    pub request_id: String,
    pub session_id: u64,
    pub namespace:  String,
    pub channel_id: i32,
    pub created_at: Instant,
}

impl SessionManager {
    pub fn new() -> Self {
        Self {
            next_id:       AtomicU64::new(1),
            by_namespace:  DashMap::new(),
            by_channel_id: DashMap::new(),
            requests:      DashMap::new(),
        }
    }

    pub fn new_session(
        &self,
        token: Option<Token>,
        token_key: String,
        remote_addr: String,
        transport: String,
    ) -> Arc<RwLock<GatewaySession>> {
        let id = self.next_id.fetch_add(1, Ordering::Relaxed);
        Arc::new(RwLock::new(GatewaySession::new(
            id,
            token,
            token_key,
            remote_addr,
            transport,
        )))
    }

    pub async fn claim_namespace(
        &self,
        session: &Arc<RwLock<GatewaySession>>,
        namespace: &str,
    ) -> Result<(), TunnelError> {
        if let Some(existing) = self.by_namespace.get(namespace) {
            let existing_id = existing.read().await.id;
            let session_id = session.read().await.id;
            if existing_id != session_id {
                return Err(TunnelError::protocol("namespace already connected"));
            }
        }
        self.by_namespace
            .insert(namespace.to_string(), session.clone());
        Ok(())
    }

    pub async fn bind_channel(
        &self,
        session: &Arc<RwLock<GatewaySession>>,
        worker_id: i32,
        channel_id: i32,
        group: String,
        models: Vec<String>,
        backend_type: String,
        status: i32,
    ) {
        let mut s = session.write().await;
        if s.channel_id != 0 && s.channel_id != channel_id {
            self.by_channel_id.remove(&s.channel_id);
        }
        s.worker_id = worker_id;
        s.channel_id = channel_id;
        s.group = group;
        s.models = models;
        s.backend_type = backend_type;
        s.status = status;
        self.by_channel_id.insert(channel_id, session.clone());
    }

    pub async fn release(&self, session: &GatewaySession) {
        if !session.namespace.is_empty() {
            if let Some(entry) = self.by_namespace.get(&session.namespace) {
                if entry.read().await.id == session.id {
                    self.by_namespace.remove(&session.namespace);
                }
            }
        }
        if session.channel_id != 0 {
            if let Some(entry) = self.by_channel_id.get(&session.channel_id) {
                if entry.read().await.id == session.id {
                    self.by_channel_id.remove(&session.channel_id);
                }
            }
        }
    }

    pub fn get_by_namespace(&self, namespace: &str) -> Option<Arc<RwLock<GatewaySession>>> {
        self.by_namespace.get(namespace).map(|r| r.value().clone())
    }

    pub fn get_by_channel_id(&self, channel_id: i32) -> Option<Arc<RwLock<GatewaySession>>> {
        self.by_channel_id
            .get(&channel_id)
            .map(|r| r.value().clone())
    }

    pub fn track_request(&self, request: InFlightRequest) {
        self.requests.insert(request.request_id.clone(), request);
    }

    pub fn remove_request(&self, request_id: &str) {
        self.requests.remove(request_id);
    }

    pub fn get_request(&self, request_id: &str) -> Option<InFlightRequest> {
        self.requests.get(request_id).map(|r| r.value().clone())
    }

    pub fn session_count(&self) -> usize {
        self.by_namespace.len()
    }
}

impl Default for SessionManager {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn test_session_manager_basic() {
        let manager = SessionManager::new();
        let session = manager.new_session(
            None,
            "test-key".to_string(),
            "127.0.0.1:12345".to_string(),
            "websocket".to_string(),
        );
        assert_eq!(session.read().await.id, 1);
        assert_eq!(manager.session_count(), 0);
    }

    #[tokio::test]
    async fn test_namespace_claim() {
        let manager = SessionManager::new();
        let session = manager.new_session(
            None,
            "test-key".to_string(),
            "127.0.0.1:12345".to_string(),
            "websocket".to_string(),
        );
        manager.claim_namespace(&session, "test-ns").await.unwrap();
        assert_eq!(manager.session_count(), 1);
        let found = manager.get_by_namespace("test-ns").unwrap();
        assert_eq!(found.read().await.id, session.read().await.id);
    }

    #[tokio::test]
    async fn test_request_tracking() {
        let manager = SessionManager::new();
        let req = InFlightRequest {
            request_id: "req-123".to_string(),
            session_id: 1,
            namespace:  "test".to_string(),
            channel_id: 42,
            created_at: Instant::now(),
        };
        manager.track_request(req);
        assert!(manager.get_request("req-123").is_some());
        manager.remove_request("req-123");
        assert!(manager.get_request("req-123").is_none());
    }
}
