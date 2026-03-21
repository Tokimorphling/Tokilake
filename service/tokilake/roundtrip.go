package tokilake

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"one-api/types"

	"github.com/google/uuid"
)

type tunnelStreamError struct {
	openAIError *types.OpenAIErrorWithStatusCode
}

func (e *tunnelStreamError) Error() string {
	if e == nil || e.openAIError == nil {
		return "tokiame stream error"
	}
	return e.openAIError.Message
}

func (e *tunnelStreamError) GetOpenAIError() *types.OpenAIErrorWithStatusCode {
	if e == nil {
		return nil
	}
	return e.openAIError
}

func DoTunnelRequest(ctx context.Context, channelID int, request *TunnelRequest) (*http.Response, string, error) {
	manager := GetSessionManager()
	session, ok := manager.GetSessionByChannelID(channelID)
	if !ok || session == nil || session.Tunnel == nil {
		return nil, "", fmt.Errorf("tokiame session is offline for channel %d", channelID)
	}
	return doTunnelRequestWithSession(ctx, manager, session, channelID, request)
}

func DoTunnelRequestByNamespace(ctx context.Context, namespace string, request *TunnelRequest) (*http.Response, string, error) {
	manager := GetSessionManager()
	session, ok := manager.GetSessionByNamespace(strings.TrimSpace(namespace))
	if !ok || session == nil || session.Tunnel == nil {
		return nil, "", fmt.Errorf("tokiame session is offline for namespace %s", namespace)
	}
	return doTunnelRequestWithSession(ctx, manager, session, session.ChannelID, request)
}

func doTunnelRequestWithSession(ctx context.Context, manager *SessionManager, session *GatewaySession, channelID int, request *TunnelRequest) (*http.Response, string, error) {
	if request == nil {
		return nil, "", fmt.Errorf("tunnel request is nil")
	}

	stream, err := session.Tunnel.OpenStream(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("open tokiame stream: %w", err)
	}

	requestID := strings.TrimSpace(request.RequestID)
	if requestID == "" {
		requestID = buildTunnelRequestID(session.Namespace)
		request.RequestID = requestID
	}

	requestCtx, cancel := context.WithCancel(ctx)
	manager.TrackRequest(&InFlightRequest{
		RequestID: requestID,
		SessionID: session.ID,
		Namespace: session.Namespace,
		ChannelID: channelID,
		CreatedAt: time.Now(),
		Cancel:    cancel,
	})

	codec := NewTunnelStreamCodec(stream)
	if err = codec.WriteRequest(request); err != nil {
		_ = stream.Close()
		manager.RemoveRequest(requestID)
		cancel()
		return nil, requestID, err
	}

	completed := make(chan struct{})
	var cleanupOnce sync.Once
	cleanup := func() {
		cleanupOnce.Do(func() {
			close(completed)
			cancel()
			manager.RemoveRequest(requestID)
		})
	}

	go watchTunnelContext(requestCtx, session, stream, requestID, completed)

	firstFrame, err := codec.ReadResponse()
	if err != nil {
		_ = stream.Close()
		cleanup()
		return nil, requestID, fmt.Errorf("read tokiame response header: %w", err)
	}
	if firstFrame.Error != nil {
		_ = stream.Close()
		cleanup()
		return nil, requestID, fmt.Errorf("tokiame request failed: %s", firstFrame.Error.Message)
	}
	if firstFrame.StatusCode == 0 {
		firstFrame.StatusCode = http.StatusBadGateway
	}

	pipeReader, pipeWriter := io.Pipe()
	response := &http.Response{
		StatusCode:    firstFrame.StatusCode,
		Status:        fmt.Sprintf("%d %s", firstFrame.StatusCode, http.StatusText(firstFrame.StatusCode)),
		Header:        expandHeaders(firstFrame.Headers),
		Body:          pipeReader,
		ContentLength: -1,
	}
	go pumpTunnelBody(stream, codec, pipeWriter, firstFrame, cleanup)

	return response, requestID, nil
}

func watchTunnelContext(ctx context.Context, session *GatewaySession, stream io.Closer, requestID string, completed chan struct{}) {
	select {
	case <-completed:
		return
	case <-ctx.Done():
		select {
		case <-completed:
			return
		default:
		}
		_ = SendCancelRequest(session, requestID, "client_disconnected")
		if stream != nil {
			_ = stream.Close()
		}
	}
}

func pumpTunnelBody(stream io.ReadWriteCloser, codec *TunnelStreamCodec, pipeWriter *io.PipeWriter, firstFrame *TunnelResponse, cleanup func()) {
	defer stream.Close()
	defer cleanup()

	if firstFrame != nil && len(firstFrame.BodyChunk) > 0 {
		if _, err := pipeWriter.Write(firstFrame.BodyChunk); err != nil {
			_ = pipeWriter.CloseWithError(err)
			return
		}
	}
	if firstFrame != nil && firstFrame.EOF {
		_ = pipeWriter.Close()
		return
	}

	for {
		frame, err := codec.ReadResponse()
		if err != nil {
			_ = pipeWriter.CloseWithError(err)
			return
		}
		if frame.Error != nil {
			_ = pipeWriter.CloseWithError(newTunnelStreamError(frame.Error))
			return
		}
		if len(frame.BodyChunk) > 0 {
			if _, err = pipeWriter.Write(frame.BodyChunk); err != nil {
				_ = pipeWriter.CloseWithError(err)
				return
			}
		}
		if frame.EOF {
			_ = pipeWriter.Close()
			return
		}
	}
}

func newTunnelStreamError(errMsg *ErrorMessage) error {
	if errMsg == nil {
		return &tunnelStreamError{
			openAIError: &types.OpenAIErrorWithStatusCode{
				OpenAIError: types.OpenAIError{
					Message: "tokiame stream error",
					Type:    "upstream_error",
					Code:    "tokiame_stream_error",
				},
				StatusCode: http.StatusBadGateway,
			},
		}
	}

	code := strings.TrimSpace(errMsg.Code)
	if code == "" {
		code = "tokiame_stream_error"
	}
	message := strings.TrimSpace(errMsg.Message)
	if message == "" {
		message = "tokiame stream error"
	}

	return &tunnelStreamError{
		openAIError: &types.OpenAIErrorWithStatusCode{
			OpenAIError: types.OpenAIError{
				Message: message,
				Type:    "upstream_error",
				Code:    code,
			},
			StatusCode: http.StatusBadGateway,
		},
	}
}

func buildTunnelRequestID(namespace string) string {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		namespace = "tokiame"
	}
	return fmt.Sprintf("%s:relay:%s", namespace, uuid.NewString())
}

func expandHeaders(headers map[string]string) http.Header {
	result := make(http.Header)
	for key, value := range headers {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		result.Set(key, value)
	}
	return result
}
