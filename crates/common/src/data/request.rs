use faststr::FastStr;
use serde_json::Value;
use std::collections::HashMap;
pub struct RequestData {
    pub url:     FastStr,
    pub headers: HashMap<FastStr, FastStr>,
    pub body:    Value,
}
impl RequestData {
    pub fn new<T>(url: T, body: Value) -> Self
    where
        T: Into<FastStr>,
    {
        Self {
            url: url.into(),
            headers: Default::default(),
            body,
        }
    }

    pub fn bearer_auth<T>(&mut self, auth: T)
    where
        T: Into<FastStr>,
    {
        self.headers.insert(
            "authorization".into(),
            format!("Bearer {}", auth.into()).into(),
        );
    }
}
