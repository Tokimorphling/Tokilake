
pub mod sse;

pub trait ChatCompletion {
    type RequestData;

    fn map_request_data(&mut self);
}

pub struct PublicChatCompletion {}
