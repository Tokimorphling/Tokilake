use faststr::FastStr;

use crate::data::ChatCompletionsData;
#[derive(Debug)]
pub enum GrpcOriginalPayload {
    ChatCompletionsRequest(ChatCompletionsData),
    StreamInferenceChunkResponse(FastStr),
    StreamInferenceChunkEnd,
    Empty,
}

pub enum SseStatus {}

// pub type ClientOriginalTx = Sender<GrpcOriginalPayload>;
// pub type TokiameMap = Arc<RwLock<HashMap<FastStr, ClientOriginalTx>>>;
