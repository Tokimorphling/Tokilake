// rpc/heartbeat.go
package rpc

import (
	"time"

	// ...
	pb "tokiame/internal/pb"
	"tokiame/pkg/log"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func (tc *TokiameClient) StartHeartbeat(interval time.Duration) {
	tc.wg.Add(1) // This wg is for client-lifetime goroutines
	go func() {
		defer tc.wg.Done()
		log.Infof("[%s] Heartbeat goroutine started (interval: %s)", tc.Namespace, interval)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		defer log.Infof("[%s] Heartbeat goroutine stopped.", tc.Namespace)

		for {
			select {
			case <-tc.mainCtx.Done(): // Main client context cancelled
				log.Infof("[%s] Heartbeat: Main context cancelled, stopping.", tc.Namespace)
				return
			// Optional: if heartbeat should stop if current stream is down, add:
			// case <-streamSpecificContext.Done(): // Would need to pass this context or get it
			//    log.Infof("[%s] Heartbeat: Stream context cancelled, pausing/stopping heartbeat.", tc.Namespace)
			//    // Decide: stop heartbeat or just pause sending? For now, it's client-lifetime.
			case <-ticker.C:
				// Check mainCtx again before trying to send, in case it was cancelled while ticker was waiting.
				if tc.mainCtx.Err() != nil {
					log.Infof("[%s] Heartbeat: Main context cancelled before sending heartbeat. Stopping.", tc.Namespace)
					return
				}

				heartbeatPayload := &pb.Heartbeat{
					Timestamp:     timestamppb.Now(),
					CurrentStatus: pb.ServingStatus_SERVING_STATUS_SERVING,
				}
				msg := &pb.TokiameMessage{
					TokiameId: tc.Namespace,
					Payload:   &pb.TokiameMessage_Heartbeat{Heartbeat: heartbeatPayload},
				}

				// Non-blocking send for heartbeat, but prioritize client shutdown.
				select {
				case <-tc.mainCtx.Done(): // Check again to avoid panic on closed channel if shutdown is racing.
					log.Infof("[%s] Heartbeat: Main context cancelled just before sending to sendChan. Stopping.", tc.Namespace)
					return
				case tc.sendChan <- msg:
					log.Debugf("[%s] Heartbeat sent.", tc.Namespace)
				default:
					// This default case means tc.sendChan is full.
					// Log a warning if the client isn't already shutting down.
					if tc.mainCtx.Err() == nil { // Check if client is NOT shutting down.
						log.Warnf("[%s] Heartbeat sendChan full, skipping heartbeat. (Chan capacity: %d, current length: %d)", tc.Namespace, cap(tc.sendChan), len(tc.sendChan))
					} else {
						log.Debugf("[%s] Heartbeat: sendChan full, but client is shutting down. Skipping send.", tc.Namespace)
					}
				}
			}
		}
	}()
}
