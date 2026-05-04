#[derive(Debug, toasty::Model)]
pub struct Channel {
    #[key]
    #[auto]
    pub id:       u64,
    pub name:     String,
    pub provider: String,
    pub models:   String,
    pub base_url: Option<String>,
    pub api_key:  Option<String>,
    pub status:   i32,
    pub weight:   i32,
}

#[derive(Debug, toasty::Model)]
pub struct Token {
    #[key]
    #[auto]
    pub id:     u64,
    pub name:   String,
    #[unique]
    pub key:    String,
    pub status: i32,
}
