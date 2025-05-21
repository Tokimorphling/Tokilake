pub mod lru;

pub trait KVCache<V>: Clone + Send + Sync + 'static
where
    V: Clone + Send + Sync + 'static,
{
    fn init() -> Self;

    fn get(&self, key: &str) -> impl Future<Output = Option<V>> + Send;

    fn invalidate(&self, key: &str) -> impl Future<Output = ()> + Send;

    fn set(&self, key: &str, value: V) -> impl Future<Output = ()> + Send;
}
