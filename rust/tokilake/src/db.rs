use toasty::Db;
use anyhow::Result;

pub async fn init_db() -> Result<Db> {
    // Build a Db handle, registering all models in this crate
    let mut db = toasty::Db::builder()
        .models(toasty::models!(crate::model::*))
        .connect("sqlite::memory:")
        .await?;
        
    // Create tables based on registered models
    db.push_schema().await?;
    
    Ok(db)
}
