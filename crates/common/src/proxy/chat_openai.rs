use crate::error::{Error, Result};
use crate::{
    RequestBuilder,
    data::{ChatCompletionsData, RequestData},
    stream::sse::{SseHandler, SseMmessage, sse_stream},
};

use serde::{Deserialize, Serialize};
use serde_json::Value;
#[derive(Debug, Serialize, Deserialize)]
pub struct OpenAIClinetConfig<'a> {
    pub name:     &'a str,
    pub api_key:  Option<&'a str>,
    pub api_base: &'a str,
    pub org_id:   &'a str,
    pub model:    &'a str,
}

impl<'a> OpenAIClinetConfig<'a> {
    pub fn new(
        name: &'a str,
        api_key: Option<&'a str>,
        api_base: &'a str,
        org_id: &'a str,
        model: &'a str,
    ) -> Self {
        Self {
            name,
            api_key,
            api_base,
            org_id,
            model,
        }
    }
}

pub struct OpenAIClient<'a> {
    pub config: OpenAIClinetConfig<'a>,
}

impl<'a> OpenAIClient<'a> {
    pub fn new(config: OpenAIClinetConfig<'a>) -> Self {
        Self { config }
    }
    pub async fn openai_chat_completions_streaming(
        &self,
        client: &reqwest::Client,
        handler: &mut SseHandler,
        data: ChatCompletionsData,
    ) -> Result<()> {
        let request_data = prepare_chat_completions(self, data)?;
        let builder = request_builder(request_data, client);
        openai_chat_completions_streaming(builder, handler).await?;
        Ok(())
    }
}

fn request_builder(request_data: RequestData, client: &reqwest::Client) -> RequestBuilder {
    let RequestData {
        url, body, headers, ..
    } = request_data;
    let mut builder = client.post(url.as_str());
    for (k, v) in headers {
        builder = builder.header(k.as_str(), v.as_str());
    }
    builder = builder.json(&body);
    builder
}

fn prepare_chat_completions(
    client: &OpenAIClient,
    data: ChatCompletionsData,
) -> Result<RequestData> {
    let url = client.config.api_base;
    let url = format!("{url}/chat/completions");
    let body = openai_build_chat_completions_body(data)?;

    let mut request_data = RequestData::new(url, body);
    if let Some(key) = client.config.api_key {
        request_data.bearer_auth(key.to_owned());
    }
    Ok(request_data)
}

fn openai_build_chat_completions_body(data: ChatCompletionsData) -> Result<Value> {
    Ok(serde_json::to_value(data)?)
}

pub async fn openai_chat_completions_streaming(
    builder: RequestBuilder,
    handler: &mut SseHandler,
) -> Result<()> {
    let mut reasoning_state = false;
    let handle = |message: SseMmessage| -> std::result::Result<bool, Error> {
        if message.data == "[DONE]" {
            Ok(true)
        } else {
            let data: Value = serde_json::from_str(&message.data)?;
            // debug!("stream-data: {data}");
            if let Some(text) = data["choices"][0]["delta"]["content"]
                .as_str()
                .filter(|v| !v.is_empty())
            {
                if reasoning_state {
                    handler.text("\n</think>\n\n")?;
                    reasoning_state = false;
                }
                handler.text(text)?;
            } else if let Some(text) = data["choices"][0]["delta"]["reasoning_content"]
                .as_str()
                .or_else(|| data["choices"][0]["delta"]["reasoning"].as_str())
                .filter(|v| !v.is_empty())
            {
                if !reasoning_state {
                    handler.text("<think>\n")?;
                    reasoning_state = true;
                }
                handler.text(text)?;
            }
            Ok(false)
        }
    };

    sse_stream(builder, handle).await
}
