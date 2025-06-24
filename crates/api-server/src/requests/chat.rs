use common::messages::Message;
use faststr::FastStr;
use serde::{Deserialize, Serialize};
use serde_json::Value;

#[derive(Default, Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ChatCompletionRequest {
    pub model:       FastStr,
    #[serde(default)]
    pub stream:      bool,
    // #[serde(rename = "max_tokens")]
    pub max_tokens:  Option<i32>,
    pub messages:    Vec<Message>,
    pub temperature: Option<f32>,
    // #[serde(rename = "top_p")]
    pub top_p:       Option<f32>,

    #[serde(flatten)]
    extra_values: Value,
    /*
    Extra values:
        #[serde(default)]
        pub tools:       Option<Vec<Value>>,
        #[serde(rename = "enable_thinking", default = "default_enable_thinking")]
        pub enable_thinking: bool,
        #[serde(rename = "thinking_budget", default = "default_thinking_budget")]
        pub thinking_budget: u32,
        #[serde(rename = "min_p", default = "default_min_p")]
        pub min_p: f64,
        #[serde(rename = "top_k")]
        pub top_k: u32,
        #[serde(rename = "frequency_penalty")]
        pub frequency_penalty: f64,
        pub n: u32,
        #[serde(default)]
        pub stop: Vec<FastStr>,
    */
}

#[cfg(test)]
mod tests {

    use super::*;
    #[test]
    fn test_chat_request() {
        let vlm_req: Result<ChatCompletionRequest, _> = serde_json::from_str(VLM_CHAT_REQ);
        assert!(vlm_req.is_ok(), "Failed to parse VLM request: {vlm_req:?}");

        let llm_req: Result<ChatCompletionRequest, _> = serde_json::from_str(LLM_CHAT_REQ);
        assert!(llm_req.is_ok(), "Failed to parse LLM request: {llm_req:?}");
    }

    const LLM_CHAT_REQ: &str = r#"
        {
  "model": "p:Qwen/Qwen2.5-VL-32B-Instruct",
  "stream": false,
  "max_tokens": 512,
  "enable_thinking": true,
  "thinking_budget": 512,
  "min_p": 0.05,
  "temperature": 0.7,
  "top_p": 0.7,
  "top_k": 50,
  "frequency_penalty": 0.5,
  "n": 1,
  "stop": [],
  "messages": [
    {
      "role": "system",
      "content": "aaa"
    },
    {
      "role": "user",
      "content": "What opportunities and challenges will the Chinese large model industry face in 2025?"
    }
  ]
  
}
    "#;
    const VLM_CHAT_REQ: &str = r#"
    {
  "model": "p:Qwen/Qwen2.5-VL-32B-Instruct",
  "stream": false,
  "max_tokens": 512,
  "enable_thinking": true,
  "thinking_budget": 512,
  "min_p": 0.05,
  "temperature": 0.7,
  "top_p": 0.7,
  "top_k": 50,
  "frequency_penalty": 0.5,
  "n": 1,
  "stop": [],

  "messages": [
    {
      "role": "system",
      "content": [
        {
          "image_url": {
            "detail": "low",
            "url": "utl11"
          },
          "type": "image_url"
        },
        {
          "text": "Describe this picture.",
          "type": "text"
        }
      ]
    },
    {
      "role": "user",
      "content": [
        {
          "text": "You are an asstant",
          "type": "text"
        }
      ]
    }
  ]
}
    "#;
}

/*
data: {"id":"0196978eee8a9b59c9e36649b3801f23","object":"chat.completion.chunk","created":1746299448,"model":"Qwen/Qwen3-8B","choices":[{"index":0,"delta":{"content":null,"reasoning_content":"2","role":"assistant"},"finish_reason":null}],"system_fingerprint":"","usage":{"prompt_tokens":141,"completion_tokens":24,"total_tokens":165,"completion_tokens_details":{"reasoning_tokens":24}}}
data: {"id":"0196978fefd3b788b1e4a96b2eb7980b","object":"chat.completion.chunk","created":1746299515,"model":"Qwen/Qwen3-8B","choices":[{"index":0,"delta":{"content":null,"reasoning_content":null,"tool_calls":[{"index":0,"id":null,"type":null,"function":{"arguments":" face"}}]},"finish_reason":null}],"system_fingerprint":"","usage":{"prompt_tokens":145,"completion_tokens":16,"total_tokens":161}}
data: {"id":"chatcmpl-178239000","object":"chat.completion.chunk","created":1746299682,"model":"siliconflow:Qwen/Qwen2.5-7B-Instruct","choices":[{"index":0,"delta":{"content":" it"},"finish_reason":null}]}
*/
