package rpc

import (
	"context"
	pb "tokiame/internal/pb"
	"tokiame/pkg/log"
)

func (tc *TokiameClient) handleIncomingMessage(msg *pb.TokilakeMessage) {
	if ack := msg.GetAck(); ack != nil {
		tc.handleAck(ack)
	} else if req := msg.GetChatcompletionRequest(); req != nil {
		tc.handleChatCompletionRequest(req, msg.TaskId)
	} else if cmd := msg.GetCommand(); cmd != nil {
		tc.handleCommand(cmd, msg.GetTaskId()) // Use msg.GetTaskId() for consistency
	} else {
		log.Warnf("[%s] Received unknown message type from Tokilake for TaskId: %s", tc.Namespace, msg.GetTaskId())
	}
}

func (tc *TokiameClient) handleAck(ack *pb.Acknowledgement) {
	log.Infof("[%s] Received Ack: Success=%t, Details='%s'", tc.Namespace, ack.Success, ack.Details)
}

func (tc *TokiameClient) handleChatCompletionRequest(req *pb.ChatCompletionRequest, taskId string) {
	log.Infof("[%s] Received TaskInstruction for request_id: %s, model: %s", tc.Namespace, taskId, req.Model)

	// Derive taskCtx from mainCtx so tasks are cancelled if the client is shutting down globally.
	taskCtx, taskCancel := context.WithCancel(tc.mainCtx)

	tc.tasksMu.Lock()
	// Check if a task with this ID already exists and cancel it if policy dictates.
	if oldCancel, exists := tc.tasks[taskId]; exists {
		log.Warnf("[%s] Task %s already exists. Cancelling previous instance.", tc.Namespace, taskId)
		oldCancel() // Cancel the old task's context.
		// The old task's goroutine defer should handle deleting itself from the map.
		// However, to be safe, we can delete here and re-add, or rely on the new task overwriting.
		// delete(tc.tasks, taskId) // Or let the new assignment overwrite.
	}
	tc.tasks[taskId] = taskCancel
	tc.tasksMu.Unlock()

	// Spawn a new goroutine for this task.
	go func(id string, currentTaskCancel context.CancelFunc) { // Pass taskId and its specific cancel func
		defer func() {
			// This defer runs when the task goroutine exits (completes, errors, or is cancelled).
			currentTaskCancel() // Ensure the task's context is cancelled.
			tc.tasksMu.Lock()
			// Only delete if the stored cancel func is the one for THIS instance of the task.
			// This avoids a new task with the same ID deleting the cancel func of an even newer one if there's a rapid replacement.
			// However, simpler is just to delete by ID, assuming the new task's cancel func is already in the map.
			if c, ok := tc.tasks[id]; ok && &c == &currentTaskCancel { // Compare pointers to ensure it's the same cancel func
				delete(tc.tasks, id)
			} else if ok {
				log.Debugf("[%s] Task %s was likely replaced; not deleting its entry from tasks map by this older instance.", tc.Namespace, id)
			}
			tc.tasksMu.Unlock()
			log.Infof("[%s] Goroutine for task %s finished and cleaned up from active tasks map.", tc.Namespace, id)
		}()
		// Pass the task-specific context to the processing function.
		tc.processChatCompletionStream(taskCtx, req, id)
	}(taskId, taskCancel) // Pass the current taskId and its cancel function to the goroutine.
}

func (tc *TokiameClient) handleCommand(cmd *pb.ControlCommand, taskId string) {
	log.Infof("[%s] Received Command: %s for TaskId (or command reference): %s", tc.Namespace, pb.ControlCommand_CommandType_name[int32(cmd.CommandType)], taskId)

	switch cmd.CommandType {
	case pb.ControlCommand_SHUTDOWN_GRACEFULLY:
		log.Infof("[%s] Received shutdown command for specific task %s.", tc.Namespace, taskId)
		tc.tasksMu.Lock()
		if cancel, ok := tc.tasks[taskId]; ok {
			cancel() // This will trigger the task's goroutine defer for cleanup.
		} else {
			log.Debugf("[%s] Task %s for shutdown not found in active tasks, may have already finished.", tc.Namespace, taskId)
		}
		tc.tasksMu.Unlock()

	case pb.ControlCommand_MODELS:
		log.Infof("[%s] Received 'models' command, task_id_ref: %s", tc.Namespace, taskId)
		// Use a short-lived context or mainCtx for this, as it's a direct request-response.
		// If it can be long, derive from mainCtx. If quick, context.Background() is okay.
		go tc.SendModelsList(context.TODO(), taskId) // TODO: Decide appropriate context for SendModelsList. mainCtx is safer.
	default:
		log.Warnf("[%s] Received unknown command type: %d for task %s", tc.Namespace, cmd.CommandType, taskId)
	}
}
