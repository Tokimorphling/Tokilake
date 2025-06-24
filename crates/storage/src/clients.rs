use crate::cache::KVCache;
use crate::{Storage, error::Result};
use common::clients::ForwardClient;
use faststr::FastStr;
use sqlx::prelude::FromRow;
use tracing::info;

#[derive(Debug, FromRow)]
struct Client {
    id:          i32,
    // "type" is a reserved keyword in Rust, so we must rename it.
    // The `#[sqlx(rename = "type")]` attribute tells sqlx to map
    // the database column "type" to this field.
    #[sqlx(rename = "type")]
    client_type: FastStr,
    namespace:   FastStr,
    // Nullable columns in SQL map to Option<T> in Rust.
    api_base:    Option<FastStr>,
    api_key:     Vec<FastStr>,
    public:      bool,
}

impl<C: KVCache<ForwardClient>> Storage<C> {
    pub async fn is_public(&self, _namespace: impl AsRef<str>) -> bool {
        true
    }

    pub async fn get_public_clients(&self) -> Result<Vec<ForwardClient>> {
        let records = sqlx::query_as!(
            ForwardClient,
            r#"
        SELECT 
            c.id, 
            c.type as "ty!: FastStr", 
            c.namespace as "namespace!: FastStr", 
            c.api_base as "api_base!: FastStr", 
            c.api_key as "api_keys!: Vec<FastStr>",
            array_remove(array_agg(m.name) FILTER (WHERE m.name IS NOT NULL), NULL) as "model_names!: Vec<FastStr>",
            c.public as "public!: bool"
        FROM clients c
        LEFT JOIN models m ON c.id = m.client_id
        WHERE c.public = true
        GROUP BY c.id, c.type, c.namespace, c.api_base, c.api_key
        ORDER BY c.id
        "#
        ).fetch_all(&self.pool).await?;

        Ok(records)
    }

    pub async fn warm_up(&self) -> Result<()> {
        let records = self.get_public_clients().await?;
        for client in &records {
            self.cache.set(&client.namespace, client.clone()).await;
        }
        info!(
            "warm-up complete: {} public clients with models loaded.",
            records.len()
        );
        Ok(())
    }

    pub async fn get_client_by_namespace(
        &self,
        namespace: impl AsRef<str>,
    ) -> Result<Option<ForwardClient>> {
        if let Some(client) = self.cache.get(namespace.as_ref()).await {
            return Ok(Some(client));
        }

        let client = sqlx::query_as!(
        ForwardClient,
        r#"
        SELECT 
            c.id, 
            c.type as "ty!: FastStr", 
            c.namespace as "namespace!: FastStr", 
            c.api_base as "api_base!: FastStr", 
            c.api_key as "api_keys!: Vec<FastStr>",
            array_remove(array_agg(m.name) FILTER (WHERE m.name IS NOT NULL), NULL) as "model_names!: Vec<FastStr>",
            c.public as "public!: bool"
        FROM clients c
        LEFT JOIN models m ON c.id = m.client_id
        WHERE ($1::text IS NULL OR c.namespace = $1)
        GROUP BY c.id, c.type, c.namespace, c.api_base, c.api_key
        ORDER BY c.id
        "#,
        namespace.as_ref()
        ).fetch_optional(&self.pool).await?;

        if let Some(ref c) = client {
            self.cache.set(&c.namespace, c.clone()).await;
        }
        Ok(client)
    }

    pub async fn get_clients(&self, client_name: Option<&str>) -> Result<Vec<ForwardClient>> {
        Ok(sqlx::query_as!(
        ForwardClient,
        r#"
        SELECT 
            c.id, 
            c.type as "ty!: FastStr", 
            c.namespace as "namespace!: FastStr", 
            c.api_base as "api_base!: FastStr", 
            c.api_key as "api_keys!: Vec<FastStr>",
            array_remove(array_agg(m.name) FILTER (WHERE m.name IS NOT NULL), NULL) as "model_names!: Vec<FastStr>",
            c.public as "public!: bool"
        FROM clients c
        LEFT JOIN models m ON c.id = m.client_id
        WHERE ($1::text IS NULL OR c.namespace = $1)
        GROUP BY c.id, c.type, c.namespace, c.api_base, c.api_key
        ORDER BY c.id
        "#,
        client_name
    )
    .fetch_all(&self.pool)
    .await?)
    }

    pub async fn get_client_by_name(&self, name: &str) -> Result<Option<ForwardClient>> {
        let clients = self.get_clients(Some(name)).await?;
        Ok(clients.into_iter().next())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::{cache::lru::ClientCache, tests::setup_db};
    #[tokio::test]
    async fn test_clients() {
        let pool = setup_db().await;
        let storage = Storage {
            pool,
            cache: ClientCache::init(),
        };
        let clients = storage.get_clients(None).await.unwrap();
        println!("{clients:?}");
    }
}
