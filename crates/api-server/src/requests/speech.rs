// use std::default;

use faststr::FastStr;
use serde::{Deserialize, Serialize};

#[derive(Default, Debug, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum ResponseFormat {
    Mp3,
    Wav,
    #[default]
    Opus,
}

#[derive(Serialize, Deserialize, Debug, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct ExtraBody {
    pub references: Vec<Reference>,
}

#[derive(Serialize, Deserialize, Debug, PartialEq, Eq)]
pub struct Reference {
    pub audio: FastStr,
    pub text:  FastStr,
}

#[derive(Debug, Serialize, Deserialize, PartialEq, Eq)]
// #[serde(rename_all = "camelCase")]
pub struct SpeechRequest {
    pub model:           FastStr,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub voice:           Option<FastStr>,
    pub input:           FastStr,
    #[serde(default)]
    pub response_format: ResponseFormat,
    #[serde(flatten, default = "Option::None")]
    pub extra:           Option<ExtraBody>,
}

#[cfg(test)]
mod tests {
    use crate::requests::speech::*;
    use serde_json::json;
    #[test]
    fn test_deserialize_minimal() {
        let json = json!({
            "model": "tts-1",
            "input": "Hello, world!"
        });
        let req: SpeechRequest = serde_json::from_value(json).unwrap();

        assert_eq!(req.model, "tts-1");
        assert_eq!(req.input, "Hello, world!");
        assert_eq!(req.response_format, ResponseFormat::Opus);
        assert!(req.voice.is_none());
        assert!(req.extra.is_none());
    }

    #[test]
    fn test_deserialize_full_fields() {
        let json = json!({
            "model": "tts-2",
            "voice": "alloy",
            "input": "Goodbye!",
            "responseFormat": "mp3",
            "references": [
                {"audio": "audio1.wav", "text": "Sample 1"},
                {"audio": "audio2.wav", "text": "Sample 2"}
            ]
        });
        let req: SpeechRequest = serde_json::from_value(json).unwrap();

        assert_eq!(req.model, "tts-2");
        assert_eq!(req.voice.as_deref(), Some("alloy"));
        assert_eq!(req.input, "Goodbye!");
        assert_eq!(req.response_format, ResponseFormat::Mp3);

        let extra = req.extra.as_ref().expect("Expected Some(ExtraBody)");
        let refs = &extra.references;
        assert_eq!(refs.len(), 2);
        assert_eq!(refs[0].audio, "audio1.wav");
        assert_eq!(refs[0].text, "Sample 1");
        assert_eq!(refs[1].audio, "audio2.wav");
        assert_eq!(refs[1].text, "Sample 2");
    }

    #[test]
    fn test_deserialize_with_defaults() {
        let json = json!({
            "model": "tts-3",
            "input": "Default Test",
            "responseFormat": "wav"
        });
        let req: SpeechRequest = serde_json::from_value(json).unwrap();

        assert_eq!(req.model, "tts-3");
        assert_eq!(req.input, "Default Test");
        assert_eq!(req.response_format, ResponseFormat::Wav);
        assert!(req.voice.is_none());
        assert!(req.extra.is_none());
    }

    #[test]
    fn test_deserialize_ignore_unknown_fields() {
        let json = json!({
            "model": "tts-4",
            "input": "Ignore Extra",
            "extraField": "should-be-ignored"
        });
        let req: SpeechRequest = serde_json::from_value(json).unwrap();

        assert_eq!(req.model, "tts-4");
        assert_eq!(req.input, "Ignore Extra");
        assert_eq!(req.response_format, ResponseFormat::Opus);
        assert!(req.voice.is_none());
        assert!(req.extra.is_none());
    }
}
