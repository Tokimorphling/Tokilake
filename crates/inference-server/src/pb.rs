include!(concat!(env!("OUT_DIR"), concat!("/", "volo_gen.rs")));
use common::{data::ChatCompletionsData, messages::Message, proxy::GrpcOriginalPayload};
use tokiame_message::Payload as TokiamePayload;
use tokilake_message::Payload as TokilakePayload;
pub use volo_gen::tokilake::inference::v1::*;

use self::control_command::CommandType;

impl From<GrpcOriginalPayload> for TokilakePayload {
    fn from(val: GrpcOriginalPayload) -> Self {
        match val {
            GrpcOriginalPayload::ChatCompletionsRequest(data) => {
                let req = build_chat_completion_request(data);
                TokilakePayload::ChatcompletionRequest(req)
            }
            _ => TokilakePayload::Command(ControlCommand {
                command_type: CommandType::COMMAND_TYPE_UNSPECIFIED,
                ..Default::default()
            }),
        }
    }
}

impl From<TokiamePayload> for GrpcOriginalPayload {
    fn from(value: TokiamePayload) -> Self {
        match value {
            TokiamePayload::Chunk(chunk_payload) => {
                chunk_payload
                    .chunk
                    .as_ref()
                    .and_then(|detail| detail.choices.first())
                    .and_then(|choice| choice.delta.as_ref())
                    .and_then(|delta| delta.content.as_ref())
                    .map(|content_str| Self::StreamInferenceChunkResponse(content_str.clone()))
                    .unwrap_or(Self::StreamInferenceChunkEnd) // If any None, default to End
            }
            _ => Self::Empty,
        }
    }
}

fn build_chat_completion_request(data: ChatCompletionsData) -> ChatCompletionRequest {
    let ChatCompletionsData {
        task_id: _,
        model_name: model,
        messages,
        temperature,
        max_tokens,
        top_p,
        stream,
        frequency_penalty,
        enable_thinking: _,
        thinking_budget: _,
    } = data;

    ChatCompletionRequestBuilder::new(&model, messages)
        .max_tokens(max_tokens)
        .temperature(temperature)
        .top_p(top_p)
        .stream(stream)
        .frequency_penalty(frequency_penalty)
        .build()
}

#[derive(Default)]
pub struct ChatCompletionRequestBuilder<'a> {
    model:             &'a str,
    messages:          Vec<Message>,
    temperature:       Option<f32>,
    top_p:             Option<f32>,
    n:                 Option<i32>,
    stream:            Option<bool>,
    stop:              Vec<&'a str>,
    max_tokens:        Option<i32>,
    presence_penalty:  Option<f32>,
    frequency_penalty: Option<f32>,
    user:              Option<&'a str>,
    tools:             Vec<Tool>,
    // tool_choice_oneof: Option<chat_completion_request::ToolChoiceOneof>,
    response_format:   Option<ResponseFormat>,
    seed:              Option<i64>,
    system_message:    Option<&'a str>,
    // metadata: Option<ProtobufStruct>,
    // suffix: Option<::pilota::FastStr>,
    // min_tokens: Option<i32>,
    // logprobs: Option<bool>,
    // tool_config: Option<ProtobufStruct>,
    // top_logprobs: Option<i32>,
    // provider_specific_config: Option<ProtobufStruct>,
    // stream_usage: Option<bool>,
    // safety_settings: ::std::vec::Vec<SafetySetting>,
    // generation_config: Option<GenerationConfig>,
}

impl<'a> ChatCompletionRequestBuilder<'a> {
    /// Creates a new builder with essential parameters.
    /// Model and messages are typically always required.
    pub fn new(model: &'a str, messages: Vec<Message>) -> Self {
        ChatCompletionRequestBuilder {
            model,
            messages,
            // Defaults for "usually used" optional parameters (can be overridden)
            temperature: Some(0.7), // A common default temperature
            stream: Some(false),
            max_tokens: Some(2048), // A common default

            // Defaults for "unusually used" parameters (mostly None or empty)
            top_p: None,
            n: None, // Typically 1, so None implies the server default
            stop: Vec::new(),
            presence_penalty: None,
            frequency_penalty: None,
            user: None,
            tools: Vec::new(),
            // tool_choice_oneof: None,
            response_format: None,
            seed: None,
            system_message: None,
            // ..Default::default()
        }
    }

    // --- Setters for "usually used" parameters ---
    pub fn temperature(mut self, temp: Option<f32>) -> Self {
        self.temperature = temp;
        self
    }

    pub fn stream(mut self, stream: bool) -> Self {
        self.stream = Some(stream);
        self
    }

    pub fn max_tokens(mut self, tokens: Option<i32>) -> Self {
        self.max_tokens = tokens;
        self
    }


    // --- Setters for "unusually used" parameters (allowing them to be set if needed) ---
    pub fn top_p(mut self, top_p: Option<f32>) -> Self {
        self.top_p = top_p;
        self
    }

    pub fn n(mut self, n: i32) -> Self {
        self.n = Some(n);
        self
    }

    pub fn add_stop_sequence(mut self, sequence: &'a str) -> Self {
        self.stop.push(sequence);
        self
    }

    // pub fn stop_sequences(mut self, sequences: Vec<impl Into<pilota::FastStr>>) -> Self {
    //     self.stop = sequences.into_iter().map(|s| s.into()).collect();
    //     self
    // }

    pub fn presence_penalty(mut self, penalty: Option<f32>) -> Self {
        self.presence_penalty = penalty;
        self
    }

    pub fn frequency_penalty(mut self, penalty: Option<f32>) -> Self {
        self.frequency_penalty = penalty;
        self
    }

    // pub fn add_logit_bias(mut self, token: impl Into<pilota::FastStr>, bias: f32) -> Self {
    //     self.logit_bias.insert(token.into(), bias);
    //     self
    // }

    // pub fn logit_bias_map(mut self, bias_map: ::pilota::AHashMap<::pilota::FastStr, f32>) -> Self {
    //     self.logit_bias = bias_map;
    //     self
    // }

    pub fn user(mut self, user: &'a str) -> Self {
        self.user = Some(user);
        self
    }

    pub fn add_tool(mut self, tool: Tool) -> Self {
        self.tools.push(tool);
        self
    }

    pub fn tools_vec(mut self, tools: Vec<Tool>) -> Self {
        self.tools = tools;
        self
    }

    // pub fn tool_choice(mut self, choice: chat_completion_request::ToolChoiceOneof) -> Self {
    //     self.tool_choice_oneof = Some(choice);
    //     self
    // }

    pub fn response_format(mut self, format: ResponseFormat) -> Self {
        self.response_format = Some(format);
        self
    }

    pub fn seed(mut self, seed: i64) -> Self {
        self.seed = Some(seed);
        self
    }

    pub fn system_message(mut self, message: &'a str) -> Self {
        self.system_message = Some(message);
        self
    }

    /// Builds the `ChatCompletionRequest`.
    pub fn build(self) -> ChatCompletionRequest {
        // Basic validation: model and messages are essential.
        if self.model.is_empty() {
            panic!("Model cannot be empty"); // Or return Result<_, _>
        }
        if self.messages.is_empty() {
            panic!("Messages cannot be empty"); // Or return Result<_, _>
        }

        let stop: Vec<_> = self.stop.into_iter().map(|x| x.to_owned().into()).collect();

        ChatCompletionRequest {
            model: self.model.to_owned().into(),
            messages: self
                .messages
                .into_iter()
                .map(converts::msg_to_msg)
                .collect(),
            temperature: self.temperature,
            top_p: self.top_p,
            n: self.n,
            stream: self.stream,
            stop,
            max_tokens: self.max_tokens,
            presence_penalty: self.presence_penalty,
            frequency_penalty: self.frequency_penalty,

            user: self.user.map(volo::FastStr::new),
            tools: self.tools,
            response_format: self.response_format,
            seed: self.seed,
            system_message: self.system_message.map(volo::FastStr::new),
            ..Default::default()
        }
    }
}

mod converts {
    use super::chat_message::{ContentType, Role};
    use super::{ChatMessage, ContentPart, ContentParts};
    use crate::pb::ImageData;
    use crate::pb::content_part::PartType;
    use common::messages::{Message, MessageContent, MessageContentPart, MessageRole};
    pub fn part_to_part(part: MessageContentPart) -> ContentPart {
        match part {
            MessageContentPart::Text { text } => ContentPart {
                part_type: Some(PartType::Text(text)),
            },
            MessageContentPart::ImageUrl { image_url } => ContentPart {
                part_type: Some(PartType::ImageData(ImageData {
                    uri: Some(image_url.url),
                    ..Default::default()
                })),
            },
        }
    }
    #[inline]
    pub fn msg_to_msg(message: Message) -> ChatMessage {
        let role = match message.role {
            MessageRole::User => Role::ROLE_USER,
            MessageRole::Assistant => Role::ROLE_ASSISTANT,
            MessageRole::System => Role::ROLE_SYSTEM,
            MessageRole::Tool => Role::ROLE_TOOL,
        };

        let content_type = match message.content {
            MessageContent::Text(text) => ContentType::TextContent(text),
            MessageContent::Array(parts) => ContentType::MultiContent(ContentParts {
                parts: parts.into_iter().map(part_to_part).collect(),
            }),
        };
        ChatMessage {
            role,
            content_type: Some(content_type),
            ..Default::default()
        }
    }

    // pub fn ToTokilake()

    // pub fn
}

#[cfg(test)]
mod tests {
    use crate::pb::build_chat_completion_request;

    use common::data::ChatCompletionsData;
    use pilota::prost::Message;
    #[test]
    fn test_text_message_convert() {
        let data: ChatCompletionsData = serde_json::from_str(TEXT_REQUET).unwrap();
        let grpc_request = build_chat_completion_request(data);
        println!(
            "grpc len: {}, json: {}",
            grpc_request.encoded_len(),
            TEXT_REQUET.len()
        );
        // assert_eq!(grpc_request.messages, )
    }
    #[test]
    fn test_image_message_convert() {
        let data: ChatCompletionsData = serde_json::from_str(IMAMGE_REQUEST).unwrap();
        let grpc_request = build_chat_completion_request(data);

        println!(
            "grpc len: {}, json: {}",
            grpc_request.encoded_len(),
            IMAMGE_REQUEST.len()
        );
    }

    static TEXT_REQUET: &str = r#"
    
   {
   "model":"moonshot-v1-8k",
   "messages":[
      {
         "role":"system",
         "content":"Current model: kimi:moonshot-v1-8k\nCurrent date: 2025-05-17\nI want you to act as a social media influencer. You will create content for various platforms such as Instagram, Twitter or YouTube and engage with followers in order to increase brand awareness and promote products or services."
      },
      {
         "role":"user",
         "content":"Tweet out to let everyone know: The latest version of Chatbox has been released"
      },
      {
         "role":"assistant",
         "content":"🚀💬 Exciting news, everyone! We've just launched the latest version of #Chatbox! 🎉 Experience a whole new level of conversation with enhanced features and improved user interface. �  Don't miss out on the upgrade – your digital interactions just got smarter and more engaging! 🔗 [Insert Download/Update Link] #ChatboxUpdate #StayConnected #TechInnovations\n\nRemember to tap that update button and let the world know what you think! 💬👍 #ChatboxFamily #UpgradeNow\n\n\n\n🚀 **Exciting news!** The latest version of **Chatbox** is now live! 🚀  \n\n✨ **What’s new?**  \n✅ Faster response times  \n✅ Improved accuracy & smarter conversations  \n✅ New tools to boost productivity  \n✅ Bug fixes & smoother performance  \n\n💡 **Ready to level up your chats?** Tap the link in bio to download the update and experience the difference!  \n\n💬 What are you most excited about? Drop your thoughts below! 👇  \n#ChatboxUpdate #AIRevolution #TechNews #Innovation  \n\n*(Link to download: [chatbox.com/latest](http://chatbox.com/latest))*  \n\nLet me know if you'd like a version tailored for Instagram Stories or YouTube! 📱🎥"
      }
   ],
   "temperature":0.7,
   "top_p":null,
   "max_tokens":null,
   "stream":true,
   "frequency_penalty":null
}
    
    "#;

    static IMAMGE_REQUEST: &str = r#"
    
    {
   "model":"glm-4v-flash",
   "messages":[
      {
         "role":"system",
         "content":"Current model: zhipu:glm-4v-flash\nCurrent date: 2025-05-17\nYou are a helpful assistant. You can help me by answering my questions. You can also ask me questions."
      },
      {
         "role":"user",
         "content":[
            {
               "type":"text",
               "text":"这是什么？"
            },
            {
               "type":"image_url",
               "image_url":{
                  "url":"data:image/png;base64,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"
               }
            }
         ]
      }
   ],
   "temperature":0.7,
   "top_p":null,
   "max_tokens":null,
   "stream":true,
   "frequency_penalty":null
}
    
    "#;
}
