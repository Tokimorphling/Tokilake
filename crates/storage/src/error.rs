use faststr::FastStr;
use thiserror::Error;

#[derive(Error, Debug)]
pub enum Error {
    #[error("sqlx error: {0}")]
    SqlxError(#[from] sqlx::Error),
    #[error("database connection timeout")]
    DatabaseTimeOut,

    #[error("Invalid api key: {0}")]
    InvaliApiKey(FastStr),

    #[error("error: {0}")]
    CommonError(#[from] Box<common::error::Error>),

    #[error("error: {0}")]
    MsgError(FastStr),
    // #[error("{}")]
}

pub type Result<T> = std::result::Result<T, Error>;
