package rpc

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	pb "tokiame/internal/pb"
	"tokiame/pkg/config"
	"tokiame/pkg/log"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	_ "google.golang.org/grpc/resolver/dns"
)

const (
	defaultSendChannelBuffer = 10240
	defaultHeartbeatInterval = 120 * time.Second
)

type TokiameClient struct {
	Namespace      string
	conn           *grpc.ClientConn
	stream         pb.TokilakeCoordinatorService_EstablishTokiameLinkClient
	mainCtx        context.Context
	serverAddr     string
	sendChan       chan *pb.TokiameMessage
	wg             sync.WaitGroup
	cancelStream   context.CancelFunc
	tasksMu        sync.Mutex
	tasks          map[string]context.CancelFunc
	conf           *config.Manager
	retryPolicy    *RetryPolicy
	isShuttingDown atomic.Bool
}

func NewTokiameClient(ctx context.Context, address string, namespace string, conf *config.Manager) *TokiameClient {
	tc := &TokiameClient{
		Namespace:   namespace,
		mainCtx:     ctx,
		serverAddr:  address,
		conf:        conf,
		sendChan:    make(chan *pb.TokiameMessage, defaultSendChannelBuffer),
		tasks:       make(map[string]context.CancelFunc),
		retryPolicy: NewRetryPolicy(1*time.Second, 30*time.Second, 2.0), // Example policy
	}
	tc.isShuttingDown.Store(false)
	return tc
}

// Run starts the client, including connection management and automatic reconnection.
func (tc *TokiameClient) Run() error {
	log.Infof("[%s] TokiameClient Run loop starting.", tc.Namespace)
	tc.StartHeartbeat(defaultHeartbeatInterval) // Heartbeat is a client-lifetime goroutine

	for {
		if tc.isShuttingDown.Load() {
			log.Infof("[%s] Client is marked as shutting down, Run loop will not attempt new connections.", tc.Namespace)
			break
		}

		log.Infof("[%s] Attempting to establish and maintain stream connection...", tc.Namespace)

		streamErr := tc.establishAndMaintainStream()

		if streamErr == nil {
			log.Infof("[%s] establishAndMaintainStream exited gracefully (likely due to main context cancellation). Exiting Run loop.", tc.Namespace)
			break
		}

		log.Warnf("[%s] Stream session ended with error: %v. Will attempt to reconnect.", tc.Namespace, streamErr)

		if tc.isShuttingDown.Load() {
			log.Infof("[%s] Client is marked as shutting down during retry sequence. Aborting reconnection.", tc.Namespace)
			break
		}

		retryInterval := tc.retryPolicy.NextInterval()
		log.Infof("[%s] Waiting %v before next reconnection attempt.", tc.Namespace, retryInterval)

		select {
		case <-time.After(retryInterval):
			// Continue to the next iteration of the loop to retry connection.
		case <-tc.mainCtx.Done():
			log.Infof("[%s] Main context cancelled while waiting for retry. Exiting Run loop.", tc.Namespace)
			tc.isShuttingDown.Store(true) // Ensure state is consistent
			// The loop condition 'tc.isShuttingDown.Load()' will handle exiting the for-loop.
			// Or we can 'break' here directly.
			return nil // Exit Run function
		}
	}

	log.Infof("[%s] TokiameClient Run loop finished.", tc.Namespace)
	return nil
}

// establishAndMaintainStream handles a single stream session: connect, start sender/receiver, and wait.
// Returns nil if tc.mainCtx is done (graceful shutdown of this stream attempt due to client shutdown).
// Returns an error if the stream session ends for other reasons (requiring Run loop to reconnect).
func (tc *TokiameClient) establishAndMaintainStream() error {
	// streamCtx is specific to this stream session, derived from the main client context.
	streamCtx, streamCancel := context.WithCancel(tc.mainCtx)
	// IMPORTANT: defer streamCancel AFTER tc.cancelStream is set, and handle potential nil if Connect fails early.
	// tc.cancelStream will be set IF Connect succeeds and before sender/receiver start.

	// Attempt to connect. This method will set tc.conn and tc.stream.
	if err := tc.Connect(streamCtx, tc.serverAddr); err != nil {
		streamCancel() // Cancel the context we created if connect failed.
		return fmt.Errorf("connection attempt failed: %w", err)
	}
	// If Connect succeeded, tc.stream is now active for this streamCtx.
	tc.cancelStream = streamCancel // Store the cancel func for THIS specific stream session.
	defer func() {
		// This defer ensures that if establishAndMaintainStream exits, the streamCtx is cancelled.
		// This is crucial for stopping the sender and receiver of this session.
		log.Infof("[%s] establishAndMaintainStream exiting, ensuring streamCtx is cancelled.", tc.Namespace)
		if tc.cancelStream != nil { // tc.cancelStream is this function's streamCancel
			tc.cancelStream()
		}
	}()

	log.Infof("[%s] Successfully connected and established new stream.", tc.Namespace)
	tc.retryPolicy.Reset() // Reset retry backoff on successful connection

	// streamSessionWg is for the sender and receiver of THIS specific stream session.
	var streamSessionWg sync.WaitGroup
	streamSessionWg.Add(2)

	go func() {
		defer streamSessionWg.Done()
		tc.sender(streamCtx) // Pass streamCtx
	}()
	go func() {
		defer streamSessionWg.Done()
		tc.receiver(streamCtx) // Pass streamCtx
	}()

	tc.SendRegistration()

	var errToReturn error

	select {
	case <-streamCtx.Done():
		errToReturn = fmt.Errorf("stream context finished: %w", streamCtx.Err())
		log.Infof("[%s] %s", tc.Namespace, errToReturn.Error())
	case <-tc.mainCtx.Done():
		log.Infof("[%s] Main context done during active stream session. This stream session will now end.", tc.Namespace)

		errToReturn = nil
	}

	log.Infof("[%s] Waiting for current stream's sender and receiver to finish...", tc.Namespace)
	streamSessionWg.Wait()
	log.Infof("[%s] Current stream's sender and receiver finished.", tc.Namespace)

	tc.cleanupCurrentStream() // Clean up tc.stream, tc.cancelStream for this ended session.
	return errToReturn
}

func (tc *TokiameClient) Connect(ctx context.Context, serverAddress string) error {

	if tc.conn != nil {
		log.Warnf("[%s] Connect called while a connection already exists. Closing old one.", tc.Namespace)
		tc.conn.Close()
		tc.conn = nil
	}

	dialTarget, credOpt, isTLS := tc.prepareDialConfig(serverAddress)
	var dialOpts []grpc.DialOption
	if credOpt != nil {
		dialOpts = append(dialOpts, credOpt)
	}
	conn, err := grpc.NewClient(dialTarget, dialOpts...)

	// conn, err := grpc.NewClient(serverAddress, grpc.WithTransportCredentials(creds))
	if err != nil {
		tc.conn = nil
		connectionType := "insecure"
		if isTLS {
			connectionType = "TLS"
		}
		return fmt.Errorf("grpc.NewClient to target '%s' (%s) failed: %w", dialTarget, connectionType, err)
	}

	tc.conn = conn
	log.Infof("[%s] gRPC client connected to %s", tc.Namespace, serverAddress)

	clientService := pb.NewTokilakeCoordinatorServiceClient(tc.conn)
	stream, err := clientService.EstablishTokiameLink(ctx)
	if err != nil {
		if tc.conn != nil {
			tc.conn.Close()
			tc.conn = nil
		}
		return fmt.Errorf("EstablishTokiameLink failed: %w", err)
	}

	tc.stream = stream

	connectionType := "insecure"
	if isTLS {
		connectionType = "TLS"
	}
	log.Infof("[%s] Successfully connected to target '%s' and established bi-directional stream (%s)",
		tc.Namespace, dialTarget, connectionType)

	return nil
}

func (tc *TokiameClient) Close() {
	log.Infof("[%s] Initiating TokiameClient FULL shutdown...", tc.Namespace)

	tc.isShuttingDown.Store(true)

	if tc.cancelStream != nil {
		log.Infof("[%s] Close: Cancelling current stream context.", tc.Namespace)
		tc.cancelStream()
	}

	tc.tasksMu.Lock()
	activeTaskCount := len(tc.tasks)
	if activeTaskCount > 0 {
		log.Infof("[%s] Close: Cancelling %d active task(s)...", tc.Namespace, activeTaskCount)
		for taskId, cancel := range tc.tasks {
			log.Debugf("[%s] Close: Cancelling task %s.", tc.Namespace, taskId)
			cancel()

		}
	}
	tc.tasksMu.Unlock()

	log.Infof("[%s] Close: Waiting for long-lived goroutines (e.g., heartbeat) to finish...", tc.Namespace)
	tc.wg.Wait()
	log.Infof("[%s] Close: All long-lived goroutines finished.", tc.Namespace)

	if tc.sendChan != nil {

		close(tc.sendChan)
		log.Infof("[%s] Close: sendChan closed.", tc.Namespace)
		tc.sendChan = nil // Avoid reuse
	}

	// 6. Clean up the gRPC connection and stream resources.
	tc.cleanupFullConnection()
	log.Infof("[%s] TokiameClient FULL shutdown complete.", tc.Namespace)
}

// cleanupCurrentStream resets resources related to the just-ended stream session.
// It does NOT close tc.conn.
func (tc *TokiameClient) cleanupCurrentStream() {
	log.Debugf("[%s] Cleaning up resources for the just-ended stream session.", tc.Namespace)
	if tc.stream != nil {
		// tc.stream.CloseSend()
		tc.stream = nil
	}
	tc.cancelStream = nil
}

func (tc *TokiameClient) cleanupFullConnection() {
	log.Debugf("[%s] Cleaning up full gRPC connection and stream state.", tc.Namespace)
	tc.cleanupCurrentStream()
	if tc.conn != nil {
		log.Infof("[%s] Closing main gRPC connection to %s.", tc.Namespace, tc.serverAddr)
		if err := tc.conn.Close(); err != nil {
			log.Errorf("[%s] Error closing main gRPC connection: %v", tc.Namespace, err)
		}
		tc.conn = nil
	}
}

func (tc *TokiameClient) prepareDialConfig(serverAddress string) (dialTarget string, credOption grpc.DialOption, isTLS bool) {
	dialTarget = serverAddress
	isTLS = false // Default to insecure

	if strings.HasPrefix(serverAddress, "grpcs://") {
		dialTarget = strings.TrimPrefix(serverAddress, "grpcs://")
		log.Infof("[%s] TLS scheme detected for address '%s'. Dial Target will be: '%s'", tc.Namespace, serverAddress, dialTarget)

		tlsConfig := &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
		credOption = grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig))
		isTLS = true
	} else {
		if strings.HasPrefix(serverAddress, "grpc://") {
			dialTarget = strings.TrimPrefix(serverAddress, "grpc://")
			log.Infof("[%s] Insecure 'grpc://' scheme detected for address '%s'. Dial Target will be: '%s'", tc.Namespace, serverAddress, dialTarget)
		} else {
			// No "grpcs://" or "grpc://" scheme, assume the address is the target for an insecure connection.
			log.Infof("[%s] No 'grpcs://' scheme in address '%s'. Assuming insecure connection. Dial Target: '%s'", tc.Namespace, serverAddress, dialTarget)
		}
		credOption = grpc.WithTransportCredentials(insecure.NewCredentials())
		// isTLS remains false
	}
	return dialTarget, credOption, isTLS
}
