use crate::cache::KVCache;
use common::clients::ForwardClient;
use faststr::FastStr;
use moka::future::Cache;
use std::{sync::Arc, time::Duration};

#[derive(Clone)]
pub struct ClientCache(Arc<Cache<FastStr, ForwardClient>>);

impl Default for ClientCache {
    fn default() -> Self {
        Self::new()
    }
}

impl ClientCache {
    pub fn new() -> Self {
        Self(Arc::new(
            Cache::builder()
                .max_capacity(100)
                .time_to_live(Duration::from_secs(7200))
                .build(),
        ))
    }
}

impl KVCache<ForwardClient> for ClientCache {
    fn init() -> Self {
        Self::new()
    }
    async fn get(&self, k: &str) -> Option<ForwardClient> {
        self.0.get(k).await
    }

    async fn invalidate(&self, k: &str) {
        self.0.invalidate(k).await;
    }

    async fn set(&self, key: &str, value: ForwardClient) {
        self.0.insert(key.to_owned().into(), value).await;
    }
}
