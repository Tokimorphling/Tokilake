use crate::messages::Message;
use faststr::FastStr;
use serde::{Deserialize, Serialize};

#[derive(Debug, Serialize, Deserialize)]
pub struct ChatCompletionsData {
    #[serde(skip)]
    pub task_id:         FastStr,
    #[serde(rename = "model")]
    pub model_name:      FastStr,
    pub messages:        Vec<Message>,
    pub temperature:     Option<f32>,
    pub top_p:           Option<f32>,
    pub max_tokens:      Option<i32>,
    pub stream:          bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub enable_thinking: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub thinking_budget: Option<i32>,

    pub frequency_penalty: Option<f32>,
}

#[derive(Debug, Clone, Default)]
pub struct ChatCompletionsOutput {
    pub text:          FastStr,
    // pub tool_calls: Vec<ToolCall>,
    pub id:            Option<String>,
    pub input_tokens:  Option<u64>,
    pub output_tokens: Option<u64>,
}

impl ChatCompletionsOutput {
    pub fn new(text: &str) -> Self {
        Self {
            text: text.to_owned().into(),
            ..Default::default()
        }
    }
}