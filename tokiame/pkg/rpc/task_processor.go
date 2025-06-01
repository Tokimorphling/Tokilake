package rpc

import (
	"context"
	"errors"
	"fmt"
	"io"
	pb "tokiame/internal/pb"
	"tokiame/internal/utils"
	"tokiame/pkg/config"
	"tokiame/pkg/log"

	// ...
	openaiclient "tokiame/pkg/openai_client"
)

const (
	finishReasonStop = "stop"
)

// processChatCompletionStream handles the interaction with the OpenAI (or compatible) backend.
// taskCtx is the context for this specific task.
func (tc *TokiameClient) processChatCompletionStream(taskCtx context.Context, req *pb.ChatCompletionRequest, taskId string) {
	log.Debugf("[%s] Starting stream processing for task %s, model %s", tc.Namespace, taskId, req.Model)
	defer log.Infof("[%s] Finished stream processing for task %s", tc.Namespace, taskId) // This log will appear when the func returns.

	// For detailed logging of the request (optional)
	// jsonReq, _ := json.Marshal(req)
	// log.Debugf("[%s] Task %s request details: %s", tc.Namespace, taskId, string(jsonReq))

	modelDetails, ok := (*tc.SupportedModels())[req.Model]
	if !ok {
		log.Errorf("[%s] Model %s not registered/supported for task %s.", tc.Namespace, req.Model, taskId)
		errMsg := tc.createErrorChunk(taskId, fmt.Sprintf("Model %s not supported by this Tokiame instance", req.Model))
		select {
		case tc.sendChan <- errMsg:
		case <-taskCtx.Done():
			log.Warnf("[%s] Task %s context done before sending 'model not supported' error: %v", tc.Namespace, taskId, taskCtx.Err())
		}
		return
	}

	var temp float32 = 0.75 // Use configured default
	if req.Temperature != nil {
		temp = *req.Temperature
	}
	var topp float32 = 0.98 // Use configured default
	if req.TopP != nil {
		topp = *req.TopP
	}

	// Ensure APIKey is sourced correctly, e.g., from modelDetails or tc.conf
	// apiKey := modelDetails.ApiKey // Assuming ModelDetails has ApiKey field populated from config
	// if apiKey == "" {
	// 	log.Errorf("[%s] API key for model %s (backend: %s) is missing for task %s.", tc.Namespace, req.Model, modelDetails.BackendBase, taskId)
	// 	errMsg := tc.createErrorChunk(taskId, "Internal configuration error: API key missing")
	// 	select {
	// 	case tc.sendChan <- errMsg:
	// 	case <-taskCtx.Done():
	// 		log.Warnf("[%s] Task %s context done before sending API key error: %v", tc.Namespace, taskId, taskCtx.Err())
	// 	}
	// 	return
	// }

	oaiConfig := openaiclient.NewOpenAIClientConfigBuilder().
		BaseURL(modelDetails.BackendBase).
		APIKey("testkey"). // Pass the API key
		Model(req.Model).  // Or modelDetails.ActualBackendModelName if different
		Messages(req.Messages).
		Tempratrue(temp).
		Topp(topp).
		// Stream(true).
		Build()

	client, err := openaiclient.NewOpenAIClient(oaiConfig)
	if err != nil {
		log.Errorf("[%s] Error creating OpenAI client for task %s: %v", tc.Namespace, taskId, err)
		errMsg := tc.createErrorChunk(taskId, fmt.Sprintf("Internal error creating backend client: %v", err))
		select {
		case tc.sendChan <- errMsg:
		case <-taskCtx.Done():
			log.Warnf("[%s] Task %s context done before sending OAI client creation error: %v", tc.Namespace, taskId, taskCtx.Err())
		}
		return
	}

	// Use taskCtx for the OpenAI stream request.
	oaiStream, err := client.CreateChatCompletionStream(taskCtx)
	if err != nil {
		log.Errorf("[%s] Error creating OpenAI completion stream for task %s: %v", tc.Namespace, taskId, err)
		errMsg := tc.createErrorChunk(taskId, fmt.Sprintf("Internal error creating backend stream: %v", err))
		select {
		case tc.sendChan <- errMsg:
		case <-taskCtx.Done():
			log.Warnf("[%s] Task %s context done before sending OAI stream creation error: %v", tc.Namespace, taskId, taskCtx.Err())
		}
		return
	}
	defer oaiStream.Close()

	numChunks := 0
	for {
		// Check for task context cancellation before trying to receive from OpenAI stream.
		select {
		case <-taskCtx.Done():
			log.Infof("[%s] Task %s context cancelled (reason: %v), stopping OpenAI stream recv.", tc.Namespace, taskId, taskCtx.Err())
			// Optionally send a "cancelled by client" final chunk if protocol supports/requires.
			// finalMsg := tc.createFinalChunk(taskId, "task_cancelled_by_client")
			// select { case tc.sendChan <- finalMsg: case <-time.After(time.Second): log.Warnf(...) }
			return
		default:
			// Proceed to receive from OpenAI stream.
		}

		resp, streamErr := oaiStream.Recv()

		if errors.Is(streamErr, io.EOF) {
			log.Infof("[%s] OpenAI stream finished successfully for task %s after %d chunks.", tc.Namespace, taskId, numChunks)
			finalMsg := tc.createFinalChunk(taskId, finishReasonStop) // Assuming finishReasonStop is defined
			select {
			case tc.sendChan <- finalMsg:
			case <-taskCtx.Done():
				log.Warnf("[%s] Task %s context done before sending final EOF chunk: %v", tc.Namespace, taskId, taskCtx.Err())
			}
			return // Successful completion of this task's stream.
		}
		if streamErr != nil {
			// Check if the error from oaiStream.Recv() is due to taskCtx cancellation.
			if errors.Is(streamErr, context.Canceled) || errors.Is(streamErr, taskCtx.Err()) {
				log.Infof("[%s] OpenAI stream for task %s cancelled via its context during Recv: %v", tc.Namespace, taskId, streamErr)
			} else {
				log.Errorf("[%s] OpenAI stream error for task %s during Recv: %v", tc.Namespace, taskId, streamErr)
				errMsg := tc.createErrorChunk(taskId, fmt.Sprintf("Backend stream error: %v", streamErr))
				select {
				case tc.sendChan <- errMsg:
				case <-taskCtx.Done():
					log.Warnf("[%s] Task %s context done before sending backend stream error chunk: %v", tc.Namespace, taskId, taskCtx.Err())
				}
			}
			return // Exit on any OpenAI stream error.
		}

		if len(resp.Choices) == 0 || resp.Choices[0].Delta.Content == "" {
			log.Debugf("[%s] Task %s: Received empty or non-content chunk from OpenAI, skipping.", tc.Namespace, taskId)
			continue
		}
		chunkContent := resp.Choices[0].Delta.Content
		// Determine finish reason from OpenAI response if present
		var oaiFinishReason string
		if resp.Choices[0].FinishReason != "" {
			oaiFinishReason = string(resp.Choices[0].FinishReason)
		}

		message := tc.mapContentChunkToPayload(chunkContent, taskId, resp.ID, oaiFinishReason)

		// Send the mapped chunk to Tokilake via sendChan.
		// This select block ensures that if the task is cancelled while trying to send, it exits.
		select {
		case <-taskCtx.Done():
			log.Infof("[%s] Task %s context cancelled (reason: %v) before sending stream chunk %d to Tokilake.", tc.Namespace, taskId, taskCtx.Err(), numChunks)
			return
		case tc.sendChan <- message:
			numChunks++
			log.Debugf("[%s] Sent stream chunk %d for task %s to Tokilake.", tc.Namespace, numChunks, taskId)
			// Note: No default case here. This means sending to tc.sendChan will block if the channel is full.
			// This is a form of backpressure. If the sender goroutine (reading from sendChan) is slow or stuck,
			// task processing will also slow down or pause here.
		}

		// If OpenAI itself signals the end of the stream with a finish reason.
		if oaiFinishReason != "" && oaiFinishReason != "null" { // OpenAI sometimes sends "null" as a string.
			log.Infof("[%s] OpenAI stream for task %s indicated finish reason: '%s'. Ending task processing.", tc.Namespace, taskId, oaiFinishReason)
			// The 'message' already sent might have included this finish_reason.
			// If not, or if an explicit final "stop" chunk is needed regardless, send it here.
			// For now, assume mapContentChunkToPayload handles embedding finish_reason if it's the last chunk.
			return
		}
	}
}

// SupportedModels helper can stay here or in client.go, depending on preference.
// It's used by processChatCompletionStream.
func (tc *TokiameClient) SupportedModels() *map[string]*config.ModelDetails {
	// Ensure thread-safety if config can be reloaded live.
	// If config is static after NewTokiameClient, current approach is fine.
	models := tc.conf.Get().SupportedModels // Assuming Get() returns a snapshot or is thread-safe.
	modelsMp := utils.SliceToMap(models, func(model *config.ModelDetails) string {
		return model.Id
	})
	return &modelsMp
}
