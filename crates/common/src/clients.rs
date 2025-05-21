use faststr::FastStr;
use serde::{Deserialize, Serialize};
use std::collections::HashSet;

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct Models(pub HashSet<FastStr>);

impl From<Vec<FastStr>> for Models {
    fn from(value: Vec<FastStr>) -> Self {
        Self(value.into_iter().collect())
    }
}

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct ForwardClient {
    pub id:          i32,
    pub namespace:   FastStr,
    #[serde(rename = "type")]
    pub ty:          FastStr,
    pub api_base:    FastStr,
    pub api_keys:    Vec<FastStr>,
    pub model_names: Models,
    pub public:      bool,
}

#[cfg(test)]
mod tests {

    // #[tokio::test]
    // async fn test_clients() {
    //     let pool = setup_db().await;
    //     let storage = Storage { pool };
    //     let clients = storage.get_clients(None).await.unwrap();
    //     assert_eq!(clients.len(), 2);
    //     assert_eq!(clients[0].name, "client1");
    //     assert_eq!(clients[1].name, "client2");
    // }
}
