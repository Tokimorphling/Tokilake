package rpc

import (
	"context"
	pb "tokiame/internal/pb"
	"tokiame/pkg/log"
)

func (tc *TokiameClient) SendRegistration() {
	regDetails := &pb.RegistrationDetails{
		TokiameNamespace: tc.Namespace,
		// SupportedModels: tc.buildPbModelDetailsList(), // Optional: include models in registration
	}
	msg := &pb.TokiameMessage{
		TokiameId: tc.Namespace, // Or a unique client ID for this instance
		Payload: &pb.TokiameMessage_Registration{
			Registration: regDetails,
		},
	}

	// Check if client is shutting down before attempting to send.
	select {
	case <-tc.mainCtx.Done():
		log.Warnf("[%s] Client main context is done. Cannot send registration message.", tc.Namespace)
		return
	default:
		// Proceed to send
	}

	select {
	case tc.sendChan <- msg:
		log.Infof("[%s] Registration message queued for sending.", tc.Namespace)
	case <-tc.mainCtx.Done(): // Check again in case of race.
		log.Warnf("[%s] Client main context done while trying to queue registration message.", tc.Namespace)
		// No default here to allow blocking if sendChan is full, unless non-blocking send is a strict requirement for registration.
		// For critical messages like registration, blocking until space is available (or client shuts down) is often preferred over dropping.
		// If tc.sendChan can get full for long periods, that indicates a deeper issue with the sender or stream.
	}
}

// buildPbModelDetailsList is a helper to convert config models to protobuf models.
func (tc *TokiameClient) buildPbModelDetailsList() []*pb.ModelDetails {
	confModels := tc.conf.Get().SupportedModels
	pbModels := make([]*pb.ModelDetails, 0, len(confModels))
	for _, model := range confModels {
		pbModels = append(pbModels, &pb.ModelDetails{
			Id:            model.Id,
			Description:   model.Description,
			Type:          model.Type,
			Capabilities:  model.Capabilities,
			BackendEngine: model.BackendEngine,
			Status:        model.Status,
		})
	}
	return pbModels
}

// SendModelsList sends the list of supported models to Tokilake.
// Takes a context (e.g., tc.mainCtx or a shorter-lived one if appropriate for this specific command).
func (tc *TokiameClient) SendModelsList(ctx context.Context, taskId string) {
	log.Debugf("[%s] Preparing models list for task_id_ref: %s", tc.Namespace, taskId)

	payload := &pb.TokiameMessage_Models{
		Models: &pb.Models{
			SupportedModels: tc.buildPbModelDetailsList(),
		},
	}
	message := &pb.TokiameMessage{
		TokiameId: taskId, // Use the provided taskId for correlation with the command.
		Payload:   payload,
	}

	select {
	case <-ctx.Done(): // Check the context passed to this function first.
		log.Warnf("[%s] Context for SendModelsList (task_id_ref: %s) is done: %v. Cannot send.", tc.Namespace, taskId, ctx.Err())
		return
	case <-tc.mainCtx.Done(): // Then check the main client context.
		log.Warnf("[%s] Client main context is done. Cannot send models list for task_id_ref: %s.", tc.Namespace, taskId)
		return
	default:
		// Proceed to send
	}

	select {
	case tc.sendChan <- message:
		log.Infof("[%s] Models list queued for sending (task_id_ref: %s).", tc.Namespace, taskId)
	case <-ctx.Done():
		log.Warnf("[%s] Context for SendModelsList (task_id_ref: %s) done while trying to queue message: %v.", tc.Namespace, taskId, ctx.Err())
	case <-tc.mainCtx.Done():
		log.Warnf("[%s] Client main context done while trying to queue models list (task_id_ref: %s).", tc.Namespace, taskId)
	}
}

// mapContentChunkToPayload creates a message for a content chunk.
func (tc *TokiameClient) mapContentChunkToPayload(content string, taskId string, oaiStreamId string, oaiFinishReason string) *pb.TokiameMessage {
	choice := pb.ChunkChoice{Delta: &pb.ChatMessageDelta{Content: &content}}
	// If OpenAI provided a finish reason for this chunk, include it.
	if oaiFinishReason != "" && oaiFinishReason != "null" {
		fr := oaiFinishReason // Create a new string variable for the pointer
		choice.FinishReason = &fr
	}

	return &pb.TokiameMessage{
		TokiameId: taskId, // Corresponds to the Tokilake TaskId
		Payload: &pb.TokiameMessage_Chunk{
			Chunk: &pb.StreamedInferenceChunk{
				RequestId: taskId,
				// OpenAIStreamId: oaiStreamId, // Optional: include OpenAI's stream/request ID if your proto supports it
				Chunk: &pb.ChatCompletionChunk{
					Choices: []*pb.ChunkChoice{&choice},
					// Id: oaiStreamId, // Can populate OpenAI stream ID here if proto has it
				},
			},
		},
	}
}

// createFinalChunk creates a message indicating the end of a stream with a specific reason.
func (tc *TokiameClient) createFinalChunk(taskId string, reason string) *pb.TokiameMessage {
	finalChoice := pb.ChunkChoice{FinishReason: &reason}
	return &pb.TokiameMessage{
		TokiameId: taskId,
		Payload: &pb.TokiameMessage_Chunk{
			Chunk: &pb.StreamedInferenceChunk{
				RequestId: taskId,
				Chunk: &pb.ChatCompletionChunk{
					Choices: []*pb.ChunkChoice{&finalChoice},
				},
			},
		},
	}
}

// createErrorChunk creates a message for reporting an error for a task back to Tokilake.
func (tc *TokiameClient) createErrorChunk(taskId string, errorDetail string) *pb.TokiameMessage {
	// Use a specific finish_reason to indicate an error.
	// Or, if your protobuf has a dedicated error field in the chunk/choice, use that.
	reason := "ERROR: " + errorDetail // Prefix to distinguish from normal finish reasons.
	errorChoice := pb.ChunkChoice{
		FinishReason: &reason,
		// Delta: &pb.ChatMessageDelta{Content: &errorDetail}, // Optionally, put error in content if client expects it.
	}
	return &pb.TokiameMessage{
		TokiameId: taskId,
		Payload: &pb.TokiameMessage_Chunk{
			Chunk: &pb.StreamedInferenceChunk{
				RequestId: taskId,
				Chunk: &pb.ChatCompletionChunk{
					Choices: []*pb.ChunkChoice{&errorChoice},
				},
			},
		},
	}
}
