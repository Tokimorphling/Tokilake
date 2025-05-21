mod pb;

mod server;
use std::sync::Arc;

use common::data::ChatCompletionsData;
use futures_util::Stream;
use pb::{TokiameMessage, TokilakeMessage, tokiame_message, tokilake_message};
use volo::FastStr;

pub use server::{InferenceServer, map_to_sse_stream, run_inference_server};

pub use volo_grpc::Status;

pub(crate) trait MakeMessage {
    type Payload;
    type Message;
    fn make_message(id: FastStr, payload: Option<Self::Payload>) -> Self::Message;
}

impl MakeMessage for TokilakeMessage {
    type Message = TokilakeMessage;
    type Payload = tokilake_message::Payload;
    fn make_message(task_id: FastStr, payload: Option<Self::Payload>) -> Self::Message {
        TokilakeMessage { task_id, payload }
    }
}

impl MakeMessage for TokiameMessage {
    type Message = TokiameMessage;
    type Payload = tokiame_message::Payload;
    fn make_message(id: FastStr, payload: Option<Self::Payload>) -> Self::Message {
        TokiameMessage {
            tokiame_id: id,
            payload,
        }
    }
}

pub trait InferenceService {
    fn chat_completion(
        self: Arc<Self>,
        namespace: FastStr,
        request: ChatCompletionsData,
    ) -> impl Future<Output = impl Stream<Item = Result<TokiameMessage, Status>>> + Send;
}
