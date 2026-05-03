use crate::error::ErrorMessage;
use serde::{de, Deserialize, Deserializer, Serialize};
use std::collections::HashMap;

/// Body chunk that can be either a base64 string (from Go) or a byte array.
#[derive(Debug, Clone, Default)]
pub struct BodyChunk(pub Vec<u8>);

impl Serialize for BodyChunk {
    fn serialize<S: serde::Serializer>(&self, serializer: S) -> Result<S::Ok, S::Error> {
        serializer.serialize_bytes(&self.0)
    }
}

impl<'de> Deserialize<'de> for BodyChunk {
    fn deserialize<D: Deserializer<'de>>(deserializer: D) -> Result<Self, D::Error> {
        struct Visitor;
        impl<'de> serde::de::Visitor<'de> for Visitor {
            type Value = BodyChunk;
            fn expecting(&self, f: &mut std::fmt::Formatter) -> std::fmt::Result {
                f.write_str("base64 string or byte array")
            }
            fn visit_str<E: de::Error>(self, v: &str) -> Result<Self::Value, E> {
                use base64::Engine;
                base64::engine::general_purpose::STANDARD
                    .decode(v)
                    .map(BodyChunk)
                    .map_err(de::Error::custom)
            }
            fn visit_string<E: de::Error>(self, v: String) -> Result<Self::Value, E> {
                use base64::Engine;
                base64::engine::general_purpose::STANDARD
                    .decode(&v)
                    .map(BodyChunk)
                    .map_err(de::Error::custom)
            }
            fn visit_seq<A: de::SeqAccess<'de>>(self, mut seq: A) -> Result<Self::Value, A::Error> {
                let mut bytes = Vec::new();
                while let Some(b) = seq.next_element::<u8>()? {
                    bytes.push(b);
                }
                Ok(BodyChunk(bytes))
            }
        }
        deserializer.deserialize_any(Visitor)
    }
}

impl std::ops::Deref for BodyChunk {
    type Target = Vec<u8>;
    fn deref(&self) -> &Self::Target {
        &self.0
    }
}

pub mod control_type {
    pub const AUTH: &str = "auth";
    pub const REGISTER: &str = "register";
    pub const HEARTBEAT: &str = "heartbeat";
    pub const MODELS_SYNC: &str = "models_sync";
    pub const CANCEL_REQUEST: &str = "cancel_request";
    pub const ACK: &str = "ack";
    pub const ERROR: &str = "error";
}

pub mod route_kind {
    pub const CHAT_COMPLETIONS: &str = "chat_completions";
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ControlMessage {
    #[serde(rename = "type")]
    pub msg_type:       String,
    #[serde(skip_serializing_if = "Option::is_none", default)]
    pub request_id:     Option<String>,
    #[serde(skip_serializing_if = "Option::is_none", default)]
    pub auth:           Option<AuthMessage>,
    #[serde(skip_serializing_if = "Option::is_none", default)]
    pub register:       Option<RegisterMessage>,
    #[serde(skip_serializing_if = "Option::is_none", default)]
    pub heartbeat:      Option<HeartbeatMessage>,
    #[serde(skip_serializing_if = "Option::is_none", default)]
    pub models_sync:    Option<ModelsSyncMessage>,
    #[serde(skip_serializing_if = "Option::is_none", default)]
    pub cancel_request: Option<CancelRequestMessage>,
    #[serde(skip_serializing_if = "Option::is_none", default)]
    pub ack:            Option<AckMessage>,
    #[serde(skip_serializing_if = "Option::is_none", default)]
    pub error:          Option<ErrorMessage>,
}

impl ControlMessage {
    pub fn auth(token: impl Into<String>) -> Self {
        Self {
            msg_type:       control_type::AUTH.to_string(),
            request_id:     None,
            auth:           Some(AuthMessage {
                token: token.into(),
            }),
            register:       None,
            heartbeat:      None,
            models_sync:    None,
            cancel_request: None,
            ack:            None,
            error:          None,
        }
    }

    pub fn ack(request_id: impl Into<String>, ack: AckMessage) -> Self {
        Self {
            msg_type:       control_type::ACK.to_string(),
            request_id:     Some(request_id.into()),
            auth:           None,
            register:       None,
            heartbeat:      None,
            models_sync:    None,
            cancel_request: None,
            ack:            Some(ack),
            error:          None,
        }
    }

    pub fn error_msg(request_id: impl Into<String>, error: ErrorMessage) -> Self {
        Self {
            msg_type:       control_type::ERROR.to_string(),
            request_id:     Some(request_id.into()),
            auth:           None,
            register:       None,
            heartbeat:      None,
            models_sync:    None,
            cancel_request: None,
            ack:            None,
            error:          Some(error),
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AuthMessage {
    pub token: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RegisterMessage {
    pub namespace:     String,
    #[serde(default)]
    pub node_name:     String,
    #[serde(default)]
    pub group:         String,
    #[serde(default)]
    pub models:        Vec<String>,
    #[serde(default)]
    pub hardware_info: HashMap<String, serde_json::Value>,
    #[serde(default)]
    pub backend_type:  String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct HeartbeatMessage {
    #[serde(default)]
    pub status:         i32,
    #[serde(default)]
    pub node_name:      String,
    #[serde(default)]
    pub hardware_info:  HashMap<String, serde_json::Value>,
    #[serde(default)]
    pub current_models: Vec<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ModelsSyncMessage {
    #[serde(default)]
    pub group:         String,
    #[serde(default)]
    pub models:        Vec<String>,
    #[serde(default)]
    pub hardware_info: HashMap<String, serde_json::Value>,
    #[serde(default)]
    pub backend_type:  String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CancelRequestMessage {
    pub target_request_id: String,
    #[serde(default)]
    pub reason:            String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AckMessage {
    #[serde(default)]
    pub message:    String,
    #[serde(default)]
    pub namespace:  String,
    #[serde(default)]
    pub worker_id:  i32,
    #[serde(default)]
    pub channel_id: i32,
}

#[derive(Debug, Clone)]
pub struct RegisterResult {
    pub worker_id:    i32,
    pub channel_id:   i32,
    pub namespace:    String,
    pub group:        String,
    pub models:       Vec<String>,
    pub backend_type: String,
    pub status:       i32,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TunnelRequest {
    pub request_id: String,
    pub route_kind: String,
    pub method:     String,
    pub path:       String,
    #[serde(default)]
    pub model:      String,
    #[serde(default)]
    pub headers:    HashMap<String, String>,
    #[serde(default)]
    pub is_stream:  bool,
    #[serde(default)]
    #[serde(with = "serde_bytes")]
    pub body:       Vec<u8>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TunnelResponse {
    pub request_id:  String,
    #[serde(default)]
    pub status_code: u16,
    #[serde(default)]
    pub headers:     HashMap<String, String>,
    #[serde(default)]
    pub body_chunk:  BodyChunk,
    #[serde(default)]
    pub eof:         bool,
    #[serde(skip_serializing_if = "Option::is_none", default)]
    pub error:       Option<ErrorMessage>,
}

#[derive(Debug, Clone)]
pub struct Token {
    pub user_id: i64,
}
