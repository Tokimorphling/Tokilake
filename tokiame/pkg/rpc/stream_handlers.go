package rpc

import (
	"context"
	"errors"
	"io"
	"tokiame/pkg/log"
	// ...
)

func (tc *TokiameClient) sender(streamCtx context.Context) {
	log.Infof("[%s] Sender goroutine started for current stream", tc.Namespace)
	defer log.Infof("[%s] Sender goroutine stopped for current stream", tc.Namespace)

	for {
		select {
		case <-streamCtx.Done():
			log.Infof("[%s] Sender: Stream context cancelled (reason: %v), stopping.", tc.Namespace, streamCtx.Err())
			return
		case msg, ok := <-tc.sendChan:
			if !ok {
				log.Warnf("[%s] Sender: sendChan closed. This typically means client is shutting down. Stopping sender for current stream.", tc.Namespace)
				// If sendChan is closed, the client is shutting down.
				// Signal this stream session to end if it hasn't already from mainCtx.
				if tc.cancelStream != nil { // tc.cancelStream is the cancel func for streamCtx
					tc.cancelStream()
				}
				return
			}

			// This check should ideally not be hit if Connect and establishAndMaintainStream manage tc.stream correctly.
			if tc.stream == nil {
				log.Errorf("[%s] Sender: Stream is nil but sender is active. This is unexpected. Cancelling stream session.", tc.Namespace)
				if tc.cancelStream != nil {
					tc.cancelStream()
				}
				return // Should not proceed if stream is nil.
			}

			if err := tc.stream.Send(msg); err != nil {
				log.Errorf("[%s] Sender: Error sending message via stream: %v. (For TokiameId: %s)", tc.Namespace, err, msg.TokiameId)
				// This is a critical stream error. Cancel the current stream's context
				// to signal establishAndMaintainStream that this session is over and needs cleanup/reconnect.
				if tc.cancelStream != nil {
					tc.cancelStream()
				}
				return // Exit sender for this broken stream.
			}

			// Minimal logging for sent messages to reduce noise; specific logging in message_builders or task_processor
			if msg.GetRegistration() != nil {
				log.Infof("[%s] Sent registration details via stream.", tc.Namespace)
			} else if msg.GetHeartbeat() != nil {
				log.Debugf("[%s] Sent heartbeat via stream.", tc.Namespace)
			} else {
				log.Debugf("[%s] Sent a message (ID: %s) via stream.", tc.Namespace, msg.TokiameId)
			}
		}
	}
}

// receiver goroutine for receiving messages of a single stream session.
// It uses streamCtx for its lifecycle.
func (tc *TokiameClient) receiver(streamCtx context.Context) {
	log.Infof("[%s] Receiver goroutine started for current stream", tc.Namespace)
	defer log.Infof("[%s] Receiver goroutine stopped for current stream", tc.Namespace)

	for {
		// This check should ideally not be hit if tc.stream is managed correctly.
		if tc.stream == nil {
			log.Warnf("[%s] Receiver: Stream is nil but receiver is active. Checking stream context (done: %v).", tc.Namespace, streamCtx.Err() != nil)
			select {
			case <-streamCtx.Done(): // Primary exit condition if stream became nil due to cancellation.
				return
			default:
				// If stream is nil and context not yet done, it's an inconsistent state.
				// Force cancellation of this stream session.
				log.Errorf("[%s] Receiver: Stream is nil unexpectedly. Cancelling stream session.", tc.Namespace)
				if tc.cancelStream != nil {
					tc.cancelStream()
				}
				return // Exit, as stream is unusable.
			}
		}

		// Before calling Recv, check if the context has been cancelled.
		// This avoids blocking on Recv if cancellation happened.
		select {
		case <-streamCtx.Done():
			log.Infof("[%s] Receiver: Stream context cancelled prior to Recv (reason: %v), stopping.", tc.Namespace, streamCtx.Err())
			return
		default:
			// Continue to Recv
		}

		in, err := tc.stream.Recv()
		if err != nil {
			// Check if the error is due to the streamCtx being cancelled (expected during shutdown or error).
			select {
			case <-streamCtx.Done():
				log.Infof("[%s] Receiver: Stream context was already done or just done (Recv err: %v, ctx err: %v). Stopping.", tc.Namespace, err, streamCtx.Err())
			default:
				// Context not (yet) cancelled, so 'err' is a genuine Recv error (EOF, network issue).
				if errors.Is(err, io.EOF) {
					log.Infof("[%s] Receiver: Stream closed by Tokilake (EOF).", tc.Namespace)
				} else {
					log.Errorf("[%s] Receiver: Error receiving from stream: %v", tc.Namespace, err)
				}
				// Any error from Recv (including EOF) means this stream session is over.
				// Signal establishAndMaintainStream by cancelling this stream's context.
				if tc.cancelStream != nil {
					tc.cancelStream()
				}
			}
			return // Exit receiver loop for this stream session.
		}
		// If Recv was successful, process the incoming message.
		tc.handleIncomingMessage(in) // Delegate to message_handlers.go
	}
}
