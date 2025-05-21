package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	pb "tokiame/internal/pb"
	"tokiame/internal/utils"
	"tokiame/pkg/config"
	"tokiame/pkg/log"
	openaiclient "tokiame/pkg/openai_client"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	_ "google.golang.org/grpc/resolver/dns"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type TokiameClient struct {
	Namespace       string
	conn            *grpc.ClientConn
	stream          pb.TokilakeCoordinatorService_EstablishTokiameLinkClient
	supportedModels map[string]*config.ModelDetails
	sendChan        chan *pb.TokiameMessage
	wg              sync.WaitGroup
	cancelCtx       context.CancelFunc
	tasksMu         sync.Mutex
	tasks           map[string]context.CancelFunc
}

func NewTokiameClient(namespace string, models []*config.ModelDetails) *TokiameClient {
	modelsMp := utils.SliceToMap(models, func(model *config.ModelDetails) string {
		return model.Id
	})
	return &TokiameClient{
		Namespace:       namespace,
		supportedModels: modelsMp,
		sendChan:        make(chan *pb.TokiameMessage, 10240), // Buffered channel
		tasks:           make(map[string]context.CancelFunc),
	}
}

func (tc *TokiameClient) Connect(ctx context.Context, serverAddress string) error {
	creds := credentials.NewClientTLSFromCert(nil, "")
	var err error
	tc.conn, err = grpc.NewClient(serverAddress, grpc.WithTransportCredentials(creds))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	log.Infof("[%s] Connected to Tokilake server at %s", tc.Namespace, serverAddress)

	client := pb.NewTokilakeCoordinatorServiceClient(tc.conn)
	streamCtx, streamCancel := context.WithCancel(ctx) // Separate context for the stream, can be cancelled by main ctx
	tc.cancelCtx = streamCancel                        // Store cancel func to call on shutdown

	tc.stream, err = client.EstablishTokiameLink(streamCtx)
	if err != nil {
		tc.conn.Close()
		return fmt.Errorf("could not establish link: %v", err)
	}
	log.Infof("[%s] Established bi-directional stream with Tokilake", tc.Namespace)
	return nil
}

func (tc *TokiameClient) Close() {
	if tc.cancelCtx != nil {
		tc.cancelCtx() // Cancel stream context
	}
	close(tc.sendChan) // Signal sender goroutine to stop
	tc.wg.Wait()       // Wait for goroutines to finish
	if tc.stream != nil {
		// Closing the send direction of the stream. Recv will eventually error or return EOF.
		if err := tc.stream.CloseSend(); err != nil {
			log.Infof("[%s] Error closing send stream: %v", tc.Namespace, err)
		}
	}
	if tc.conn != nil {
		tc.conn.Close()
		log.Infof("[%s] Connection closed.", tc.Namespace)
	}
}

// // Goroutine for sending messages to Tokilake
func (tc *TokiameClient) sender() {
	defer tc.wg.Done()
	log.Infof("[%s] Sender goroutine started", tc.Namespace)
	for msg := range tc.sendChan {
		if tc.stream == nil {
			log.Infof("[%s] Sender: Stream is nil, cannot send message", tc.Namespace)
			continue
		}
		if err := tc.stream.Send(msg); err != nil {
			log.Infof("[%s] Error sending message via stream: %v", tc.Namespace, err)
			// If send fails, the stream might be broken. Consider breaking the loop or re-establishing.
			// For simplicity, we'll just log and continue, but this might lead to dropped messages.
			// A more robust implementation would signal the main loop to reconnect.
			return // Exit sender if stream is broken
		}
		if msg.GetRegistration() != nil {
			log.Infof("[%s] Sent registration details", tc.Namespace)
		} else if msg.GetHeartbeat() != nil {
			// log.Infof("[%s] Sent heartbeat", tc.Namespace) // Can be noisy
		} else {
			log.Infof("[%s] Sent a message to Tokilake", tc.Namespace)
		}
	}
	log.Infof("[%s] Sender goroutine stopped", tc.Namespace)
}

// Goroutine for receiving messages from Tokilake
func (tc *TokiameClient) receiver() {
	defer tc.wg.Done()
	log.Infof("[%s] Receiver goroutine started", tc.Namespace)
	for {
		if tc.stream == nil {
			log.Infof("[%s] Receiver: Stream is nil, cannot receive.", tc.Namespace)
			time.Sleep(1 * time.Second) // Avoid busy loop if stream is not yet established or broken
			continue
		}
		in, err := tc.stream.Recv()
		if err == io.EOF {
			log.Infof("[%s] Stream closed by Tokilake (EOF)", tc.Namespace)
			return // Stream ended
		}
		if err != nil {
			// Check if the error is due to context cancellation (expected on shutdown)
			select {
			case <-tc.stream.Context().Done():
				log.Infof("[%s] Stream context done, receiver stopping: %v", tc.Namespace, tc.stream.Context().Err())
			default:
				log.Infof("[%s] Error receiving from stream: %v", tc.Namespace, err)
			}
			return // Error receiving
		}

		if ack := in.GetAck(); ack != nil {
			log.Infof("[%s] Received Ack: Success=%t, Details='%s'", tc.Namespace, ack.Success, ack.Details)
		} else if req := in.GetChatcompletionRequest(); req != nil {
			log.Infof("[%s] Received TaskInstruction for request_id: %s, model: %s", in.TaskId, tc.Namespace, req.Model)
			// TODO: Process the task (e.g., call local inference engine)
			// For now, just acknowledge we got it. Simulate sending a dummy result after a delay.
			// go tc.simulateTaskProcessing(task)
			taskCtx, taskCancel := context.WithCancel(context.Background())
			tc.tasksMu.Lock()
			tc.tasks[in.TaskId] = taskCancel
			tc.tasksMu.Unlock()
			go tc.StreamTaskProcessing(taskCtx, req, in.TaskId)
		} else if cmd := in.GetCommand(); cmd != nil {
			log.Infof("[%s] Received Command: %s, Reason: %s", tc.Namespace, pb.ControlCommand_CommandType_name[int32(cmd.CommandType)], cmd.Reason)
			if cmd.CommandType == pb.ControlCommand_SHUTDOWN_GRACEFULLY {
				log.Infof("[%s] Received shutdown command, initiating stop task %s.", tc.Namespace, in.TaskId)
				tc.tasksMu.Lock()
				if cancel, ok := tc.tasks[in.TaskId]; ok {
					cancel()
					delete(tc.tasks, in.TaskId)
				} else {
					log.Debugf("[%s] Task %s not found in tasks map, already finished", tc.Namespace, in.TaskId)
				}
				tc.tasksMu.Unlock()
			}
		} else {
			log.Infof("[%s] Received unknown message type from Tokilake", tc.Namespace)
		}
	}
}

func (tc *TokiameClient) StreamTaskProcessing(ctx context.Context, req *pb.ChatCompletionRequest, taskId string) {
	log.Debugf("[%s] process stream task...", tc.Namespace)

	jsonReq, err := json.Marshal(req)
	if err != nil {
		log.Error("Serialization failed, maybe error format")
		return
	}
	log.Debugf("[%s] request: %s", tc.Namespace, string(jsonReq))

	if _, ok := tc.supportedModels[req.Model]; !ok {
		log.Errorf("[%s] not registerd", req.Model)
		return
	}

	innerModel := tc.supportedModels[req.Model]
	baseURL := innerModel.BackendBase
	messages := req.Messages

	var temp float32 = 0.7
	if req.Temperature != nil {
		temp = *req.Temperature
	}
	var topp float32 = 0.95
	if req.TopP != nil {
		topp = *req.TopP
	}

	config := openaiclient.NewOpenAIClientConfigBuilder().
		BaseURL(baseURL).
		Model(req.Model).
		Messages(messages).
		Tempratrue(temp).
		Topp(topp).
		Build()
	client, err := openaiclient.NewOpenAIClient(config)
	if err != nil {
		log.Errorf("Error creating clinet: %v", err)
	}
	stream, err := client.CreateChatCompletionStream(context.Background())

	if err != nil {
		log.Errorf("Error creating stream: %v", err)
	}

	numChunks := 0
	for {
		resp, streamErr := stream.Recv()
		if errors.Is(streamErr, io.EOF) {
			fmt.Println("\nStream finished.")
			break
		}
		if streamErr != nil {
			fmt.Printf("\nStream error: %v\n", streamErr)
			break
		}
		chunk := resp.Choices[0].Delta.Content
		log.Debugf("Recv chunk: %s", chunk)
		message := mapChunkToPayload(chunk, taskId)

		select {
		case <-ctx.Done():
			log.Infof("[%s] Context cancelled before sending stream chunk %d for %s", tc.Namespace, numChunks, taskId)
			return // Exit if context is cancelled
		case tc.sendChan <- message:
			numChunks += 1
			log.Infof("[%s] Sent stream chunk %d for task %s", tc.Namespace, numChunks, taskId)
		case <-tc.stream.Context().Done(): // Check if stream context is cancelled
			log.Infof("[%s] Stream context cancelled before sending stream chunk %d for %s", tc.Namespace, numChunks, taskId)
			return // Exit if stream is done
		default:
			log.Infof("[%s] sendChan is full or closed, could not send stream chunk %d for %s", tc.Namespace, numChunks, taskId)
			// Depending on requirements, you might want to return or implement a retry mechanism.
			return // Exit if cannot send to prevent deadlock or excessive logging
		}
	}
	fc := finalChunk(taskId)
	tc.sendChan <- fc
	log.Infof("[%s] Finished simulating STREAM processing for task %s", tc.Namespace, taskId)
}

func (tc *TokiameClient) SendRegistration() {
	supportedModels := make([]*pb.ModelDetails, 0)
	for _, model := range tc.supportedModels {
		detail := &pb.ModelDetails{
			Id:          model.Id,
			Description: model.Description,
			Type:        model.Type,
		}
		supportedModels = append(supportedModels, detail)
	}

	regDetails := &pb.RegistrationDetails{
		SupportedModels:  supportedModels,
		TokiameNamespace: tc.Namespace,
	}
	msg := &pb.TokiameMessage{
		TokiameId: tc.Namespace,
		Payload: &pb.TokiameMessage_Registration{
			Registration: regDetails,
		},
	}
	tc.sendChan <- msg
}

func (tc *TokiameClient) StartHeartbeat(ctx context.Context, interval time.Duration) {
	tc.wg.Add(1)
	go func() {
		defer tc.wg.Done()
		log.Infof("[%s] Heartbeat goroutine started (interval: %s)", tc.Namespace, interval)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				heartbeatPayload := &pb.Heartbeat{
					Timestamp:     timestamppb.Now(),
					CurrentStatus: pb.ServingStatus_SERVING_STATUS_SERVING, // Example status
				}
				msg := &pb.TokiameMessage{
					TokiameId: tc.Namespace,
					Payload: &pb.TokiameMessage_Heartbeat{
						Heartbeat: heartbeatPayload,
					},
				}
				// Non-blocking send for heartbeat, ok to drop if channel is full
				select {
				case tc.sendChan <- msg:
					log.Debugf("[%s] Heartbeat sent.", tc.Namespace)
				default:
					log.Infof("[%s] Heartbeat sendChan full, skipping heartbeat.", tc.Namespace)
				}
			case <-ctx.Done(): // Main context cancelled
				log.Infof("[%s] Heartbeat goroutine stopping due to context cancellation.", tc.Namespace)
				return
			case <-tc.stream.Context().Done(): // Stream specific context cancelled
				log.Infof("[%s] Heartbeat goroutine stopping due to stream context cancellation.", tc.Namespace)
				return
			}
		}
	}()
}

func (tc *TokiameClient) Run(mainCtx context.Context, address string) error {
	if err := tc.Connect(mainCtx, address); err != nil {
		return err
	}

	tc.wg.Add(2) // For sender and receiver goroutines
	go tc.sender()
	go tc.receiver()

	tc.SendRegistration()
	// tc.StartHeartbeat(mainCtx, 15*time.Second) // Send heartbeat every 15 seconds
	tc.StartHeartbeat(mainCtx, 20*time.Second) // Send heartbeat every 5 seconds
	// Wait for context cancellation (e.g. Ctrl+C)
	<-mainCtx.Done()
	log.Infof("[%s] Main context cancelled, shutting down client...", tc.Namespace)
	return nil
}

func mapChunkToPayload(chunk string, taskId string) *pb.TokiameMessage {
	choice := pb.ChunkChoice{Delta: &pb.ChatMessageDelta{Content: &chunk}}
	message := pb.TokiameMessage{TokiameId: taskId, Payload: &pb.TokiameMessage_Chunk{
		Chunk: &pb.StreamedInferenceChunk{
			RequestId: taskId,
			Chunk: &pb.ChatCompletionChunk{
				Choices: []*pb.ChunkChoice{&choice}}}},
	}
	return &message
}

func finalChunk(taskId string) *pb.TokiameMessage {
	stop := "stop"
	final := pb.ChunkChoice{FinishReason: &stop}
	message := pb.TokiameMessage{TokiameId: taskId, Payload: &pb.TokiameMessage_Chunk{
		Chunk: &pb.StreamedInferenceChunk{
			RequestId: taskId,
			Chunk: &pb.ChatCompletionChunk{
				Choices: []*pb.ChunkChoice{&final}}}},
	}
	return &message
}
