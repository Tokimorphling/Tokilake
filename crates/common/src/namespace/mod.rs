use crate::{data::ChatCompletionsData, stream::sse::SseEvent};
use futures_util::Stream;

pub struct Config<'a> {
    pub name:     &'a str,
    pub model:    &'a str,
    pub api_base: Option<&'a str>,
    pub api_key:  Option<&'a str>,
    pub org_id:   Option<&'a str>,
}

pub trait Proxy: Send + Sync {
    fn stream_chat(
        &self,
        req: ChatCompletionsData,
        config: Config<'_>,
    ) -> impl Future<Output = impl Stream<Item = SseEvent>> + Send;
}
