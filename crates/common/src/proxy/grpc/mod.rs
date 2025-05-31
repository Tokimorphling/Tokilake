use crate::data::ChatCompletionsData;
use faststr::FastStr;
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
