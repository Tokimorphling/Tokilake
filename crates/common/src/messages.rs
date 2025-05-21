use faststr::FastStr;
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Copy, PartialEq, Eq, Deserialize, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum MessageRole {
    System,
    Assistant,
    User,
    Tool,
}

#[allow(dead_code)]
impl MessageRole {
    pub fn is_system(&self) -> bool {
        matches!(self, MessageRole::System)
    }

    pub fn is_user(&self) -> bool {
        matches!(self, MessageRole::User)
    }

    pub fn is_assistant(&self) -> bool {
        matches!(self, MessageRole::Assistant)
    }
}

#[derive(Debug, Clone, Deserialize, Serialize)]
#[serde(untagged)]
pub enum MessageContent {
    Text(FastStr),
    Array(Vec<MessageContentPart>),
    // Note: Thi  s type is primarily for convenience and does not exist in OpenAI's API.
    // ToolCalls(MessageContentToolCalls),
}

#[derive(Debug, Clone, Deserialize, Serialize)]
#[serde(tag = "type", rename_all = "snake_case")]
pub enum MessageContentPart {
    Text { text: FastStr },
    ImageUrl { image_url: ImageUrl },
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct ImageUrl {
    pub url: FastStr,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct Message {
    pub role:    MessageRole,
    pub content: MessageContent,
}

impl Default for Message {
    fn default() -> Self {
        Self {
            role:    MessageRole::User,
            content: MessageContent::Text("".into()),
        }
    }
}

impl Message {
    pub fn new(role: MessageRole, content: MessageContent) -> Self {
        Self { role, content }
    }

    // pub fn merge_system(&mut self, system: MessageContent) {
    //     match (&mut self.content, system) {
    //         (MessageContent::Text(text), MessageContent::Text(system_text)) => {
    //             self.content = MessageContent::Array(vec![
    //                 MessageContentPart::Text(system_text),
    //                 MessageContentPart::Text(text.clone()),
    //             ])
    //         }
    //         (MessageContent::Array(list), MessageContent::Text(system_text)) => {
    //             list.insert(0, MessageContentPart::Text(system_text))
    //         }
    //         (MessageContent::Text(text), MessageContent::Array(mut system_list)) => {
    //             system_list.push(MessageContentPart::Text(text.clone()));
    //             self.content = MessageContent::Array(system_list);
    //         }
    //         (MessageContent::Array(list), MessageContent::Array(mut system_list)) => {
    //             system_list.append(list);
    //             self.content = MessageContent::Array(system_list);
    //         } // _ => {}
    //     }
    // }
}

impl MessageContent {
    // pub fn render_input(
    //     &self,
    //     resolve_url_fn: impl Fn(&str) -> FastStr,
    //     agent_info: &Option<(String, Vec<String>)>,
    // ) -> FastStr {
    //     match self {
    //         MessageContent::Text(text) => multiline_text(text.as_str()),
    //         MessageContent::Array(list) => {
    //             let (mut concated_text, mut files) = (String::new(), vec![]);
    //             for item in list {
    //                 match item {
    //                     MessageContentPart::Text(text) => {
    //                         concated_text = format!("{concated_text} {text}")
    //                     }
    //                     MessageContentPart::ImageUrl(image_url) => {
    //                         files.push(resolve_url_fn(&image_url.0))
    //                     }
    //                 }
    //             }
    //             if !concated_text.is_empty() {
    //                 concated_text = format!(" -- {}", multiline_text(&concated_text))
    //             }
    //             format!(".file {}{}", files.join(" "), concated_text).into()
    //         } // MessageContent::ToolCalls(MessageContentToolCalls {
    //           //     tool_results, text, ..
    //           // }) => {
    //           //     let mut lines = vec![];
    //           //     if !text.is_empty() {
    //           //         lines.push(text.clone())
    //           //     }
    //           //     for tool_result in tool_results {
    //           //         let mut parts = vec!["Call".to_string()];
    //           //         if let Some((agent_name, functions)) = agent_info {
    //           //             if functions.contains(&tool_result.call.name) {
    //           //                 parts.push(agent_name.clone())
    //           //             }
    //           //         }
    //           //         parts.push(tool_result.call.name.clone());
    //           //         parts.push(tool_result.call.arguments.to_string());
    //           //         lines.push(dimmed_text(&parts.join(" ")));
    //           //     }
    //           //     lines.join("\n")
    //           // }
    //     }
    // }

    // pub fn merge_prompt(&mut self, replace_fn: impl Fn(&str) -> FastStr) {
    //     match self {
    //         MessageContent::Text(text) => *text = replace_fn(text),
    //         MessageContent::Array(list) => {
    //             if list.is_empty() {
    //                 list.push(MessageContentPart::Text(replace_fn("")))
    //             } else if let Some(MessageContentPart::Text(text)) = list.get_mut(0) {
    //                 *text = replace_fn(text)
    //             }
    //         } // MessageContent::ToolCalls(_) => {}
    //     }
    // }

    // pub fn to_text(&self) -> String {
    //     match self {
    //         MessageContent::Text(text) => text.to_string(),
    //         MessageContent::Array(list) => {
    //             let mut parts = vec![];
    //             for item in list {
    //                 if let MessageContentPart::Text(text) = item {
    //                     parts.push(text.clone())
    //                 }
    //             }
    //             parts.join("\n\n")
    //         }
    //         // MessageContent::ToolCalls(_) => String::new(),
    //     }
    // }
}
