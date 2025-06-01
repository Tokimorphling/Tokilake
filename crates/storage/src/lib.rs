mod auth;
mod cache;
mod clients;

pub mod error;
use cache::KVCache;
pub use cache::lru::ClientCache;
use common::clients::ForwardClient;
use error::{Error, Result};
use sqlx::{PgPool, migrate::MigrateDatabase, postgres::PgPoolOptions};
use std::time::Duration;
use tokio::time::{self, Instant};
use tracing::{debug, info};

#[derive(Clone)]
pub struct Storage<C> {
    pool:  sqlx::PgPool,
    cache: C,
}

impl<C: KVCache<ForwardClient>> Storage<C> {
    pub async fn new(database_url: &str) -> Result<Self> {
        if !sqlx::Postgres::database_exists(database_url).await? {
            sqlx::Postgres::create_database(database_url).await?;
        }

        debug!("Connecting to database: {}", database_url);
        let pool = PgPoolOptions::new()
            .max_connections(5)
            .connect(database_url)
            .await?;

        sqlx::migrate!("../../migrations")
            .run(&pool)
            .await
            .map_err(|e| Error::MigrateError(e.to_string().into()))?;

        let pool_clone = pool.clone();

        let storage = Self {
            pool,
            cache: C::init(),
        };

        storage.warm_up().await?;
        tokio::spawn(async move { Self::start_check_db_health(pool_clone, 120, 5).await });
        Ok(storage)
    }
    pub async fn start_check_db_health(
        pool: PgPool,
        interval_sec: u64,
        query_timeout_sec: u64,
    ) -> Result<()> {
        let mut interval = time::interval(Duration::from_secs(interval_sec));
        let query_timeout = Duration::from_secs(query_timeout_sec);

        loop {
            interval.tick().await;

            let start = Instant::now();
            let result = time::timeout(query_timeout, async {
                let _ = sqlx::query("SELECT 1").execute(&pool).await?;
                Ok(())
            })
            .await;

            match result {
                Ok(Ok(())) => {
                    let elapsed = start.elapsed();
                    info!(elapsed=?elapsed, "database health check successful");
                }
                Ok(Err(e)) => {
                    return Err(Error::SqlxError(e));
                }
                Err(_) => {
                    return Err(Error::DatabaseTimeOut);
                }
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use sqlx::postgres::PgPoolOptions;
    use std::env;
    pub async fn setup_db() -> sqlx::PgPool {
        let database_url = env::var("DATABASE_URL")
            .unwrap_or("postgres://postgres:123456@127.0.0.1:5432/tokilake".into());

        PgPoolOptions::new()
            .max_connections(5)
            .connect(&database_url)
            .await
            .expect("Failed to create pool.")
    }
}
