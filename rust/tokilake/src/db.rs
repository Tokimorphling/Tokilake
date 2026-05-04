use anyhow::Result;
use toasty::Db;

pub async fn init_db() -> Result<Db> {
    // Build a Db handle, registering all models in this crate
    let db = toasty::Db::builder()
        .models(toasty::models!(crate::model::Channel, crate::model::Token))
        .connect("sqlite::memory:")
        .await?;

    // Create tables based on registered models
    db.push_schema().await?;

    Ok(db)
}
