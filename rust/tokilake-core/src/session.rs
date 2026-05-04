use crate::{error::TunnelError, protocol::Token, tunnel::TunnelSession};
use dashmap::DashMap;
use std::{
    sync::{
        Arc,
        atomic::{AtomicU64, Ordering},
    },
    time::Instant,
};
use tokio::sync::RwLock;

#[derive(Debug, Clone)]
pub struct WorkerInfo {
    pub worker_id:    i32,
    pub channel_id:   i32,
    pub namespace:    String,
    pub group:        String,
    pub backend_type: String,
    pub models:       Vec<String>,
    pub status:       i32,
}

/// Gateway session - represents a connected tunnel worker.
pub struct GatewaySession<T: TunnelSession> {
    pub id:             u64,
    pub token:          Option<Token>,
    pub token_key:      String,
    pub remote_addr:    String,
    pub connected_at:   Instant,
    pub worker_info:    Option<WorkerInfo>,
    pub transport:      String,
    pub authenticated:  bool,
    // Control stream write half (for sending messages to worker)
    pub control_tx:     Option<tokio::sync::mpsc::Sender<Vec<u8>>>,
    // Tunnel session for opening data streams
    pub tunnel_session: Option<Arc<tokio::sync::Mutex<T>>>,
}

// pub struct TunnelStreamRequest {
//     pub request_id:  String,
//     pub response_tx: tokio::sync::oneshot::Sender<TunnelStreamResponse>,
// }

// pub struct TunnelStreamResponse {
//     pub reader: Box<dyn tokio::io::AsyncRead + Unpin + Send>,
//     pub writer: Box<dyn tokio::io::AsyncWrite + Unpin + Send>,
// }

impl<T: TunnelSession> GatewaySession<T> {
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
            worker_info: None,
            transport,
            authenticated,
            control_tx: None,
            tunnel_session: None,
        }
    }

    pub fn is_alive(&self) -> bool {
        self.worker_info
            .as_ref()
            .is_some_and(|info| info.status == 1)
            && self.tunnel_session.is_some()
    }
}

/// Thread-safe session manager.
pub struct SessionManager<T: TunnelSession> {
    next_id:       AtomicU64,
    by_namespace:  DashMap<Arc<str>, Arc<RwLock<GatewaySession<T>>>>,
    by_channel_id: DashMap<i32, Arc<RwLock<GatewaySession<T>>>>,
    requests:      DashMap<Arc<str>, InFlightRequest>,
}

#[derive(Debug, Clone)]
pub struct InFlightRequest {
    pub request_id: Arc<str>,
    pub session_id: u64,
    pub namespace:  Arc<str>,
    pub channel_id: i32,
    pub created_at: Instant,
}

/// Parameters for binding a channel to a session.
pub struct ChannelBindParams {
    pub worker_id:    i32,
    pub channel_id:   i32,
    pub namespace:    String,
    pub group:        String,
    pub models:       Vec<String>,
    pub backend_type: String,
    pub status:       i32,
}

impl<T: TunnelSession> SessionManager<T> {
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
    ) -> Arc<RwLock<GatewaySession<T>>> {
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
        session: &Arc<RwLock<GatewaySession<T>>>,
        namespace: &str,
    ) -> Result<(), TunnelError> {
        use dashmap::mapref::entry::Entry;
        match self.by_namespace.entry(namespace.into()) {
            Entry::Occupied(e) => {
                if !Arc::ptr_eq(e.get(), session) {
                    return Err(TunnelError::protocol("namespace already connected"));
                }
            }
            Entry::Vacant(e) => {
                e.insert(session.clone());
            }
        }
        Ok(())
    }

    pub async fn bind_channel(
        &self,
        session: &Arc<RwLock<GatewaySession<T>>>,
        params: ChannelBindParams,
    ) {
        let mut s = session.write().await;
        if let Some(ref mut info) = s.worker_info {
            if info.channel_id != 0 && info.channel_id != params.channel_id {
                self.by_channel_id.remove(&info.channel_id);
            }
        }

        let new_info = WorkerInfo {
            worker_id:    params.worker_id,
            channel_id:   params.channel_id,
            namespace:    params.namespace,
            group:        params.group,
            models:       params.models,
            backend_type: params.backend_type,
            status:       params.status,
        };

        s.worker_info = Some(new_info);
        self.by_channel_id
            .insert(params.channel_id, session.clone());
    }

    pub async fn release(&self, session: &GatewaySession<T>) {
        if let Some(ref info) = session.worker_info {
            if !info.namespace.is_empty() {
                if let Some(entry) = self.by_namespace.get(info.namespace.as_str()) {
                    if entry.read().await.id == session.id {
                        self.by_namespace.remove(info.namespace.as_str());
                    }
                }
            }
            if info.channel_id != 0 {
                if let Some(entry) = self.by_channel_id.get(&info.channel_id) {
                    if entry.read().await.id == session.id {
                        self.by_channel_id.remove(&info.channel_id);
                    }
                }
            }
        }
    }

    pub fn get_by_namespace(&self, namespace: &str) -> Option<Arc<RwLock<GatewaySession<T>>>> {
        self.by_namespace.get(namespace).map(|r| r.value().clone())
    }

    pub fn get_by_channel_id(&self, channel_id: i32) -> Option<Arc<RwLock<GatewaySession<T>>>> {
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

impl<T: TunnelSession> Default for SessionManager<T> {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn test_session_manager_basic() {
        struct DummySession;
        impl crate::tunnel::TunnelSession for DummySession {
            type Stream = crate::tunnel::memory::MemoryStream;
            fn accept_stream(
                &mut self,
            ) -> impl std::future::Future<Output = Result<Option<Self::Stream>, TunnelError>> + Send + '_
            {
                std::future::ready(Ok(None))
            }
            fn open_stream(
                &mut self,
            ) -> impl std::future::Future<Output = Result<Self::Stream, TunnelError>> + Send + '_
            {
                std::future::ready(Err(TunnelError::StreamClosed))
            }
            fn close(
                &self,
            ) -> impl std::future::Future<Output = Result<(), TunnelError>> + Send + '_
            {
                std::future::ready(Ok(()))
            }
            fn is_alive(&self) -> bool {
                true
            }
        }

        let manager = SessionManager::<DummySession>::new();
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
        struct DummySession;
        impl crate::tunnel::TunnelSession for DummySession {
            type Stream = crate::tunnel::memory::MemoryStream;
            async fn accept_stream(&mut self) -> Result<Option<Self::Stream>, TunnelError> {
                Ok(None)
            }
            async fn open_stream(&mut self) -> Result<Self::Stream, TunnelError> {
                Err(TunnelError::StreamClosed)
            }
            async fn close(&self) -> Result<(), TunnelError> {
                Ok(())
            }
            fn is_alive(&self) -> bool {
                true
            }
        }

        let manager = SessionManager::<DummySession>::new();
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
        struct DummySession;
        impl crate::tunnel::TunnelSession for DummySession {
            type Stream = crate::tunnel::memory::MemoryStream;
            async fn accept_stream(&mut self) -> Result<Option<Self::Stream>, TunnelError> {
                Ok(None)
            }
            async fn open_stream(&mut self) -> Result<Self::Stream, TunnelError> {
                Err(TunnelError::StreamClosed)
            }
            async fn close(&self) -> Result<(), TunnelError> {
                Ok(())
            }
            fn is_alive(&self) -> bool {
                true
            }
        }

        let manager = SessionManager::<DummySession>::new();
        let req = InFlightRequest {
            request_id: "req-123".into(),
            session_id: 1,
            namespace:  "test".into(),
            channel_id: 42,
            created_at: Instant::now(),
        };
        manager.track_request(req);
        assert!(manager.get_request("req-123").is_some());
        manager.remove_request("req-123");
        assert!(manager.get_request("req-123").is_none());
    }
}
