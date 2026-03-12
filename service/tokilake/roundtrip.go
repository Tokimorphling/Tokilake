package tokilake

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

func DoTunnelRequest(ctx context.Context, channelID int, request *TunnelRequest) (*http.Response, string, error) {
	manager := GetSessionManager()
	session, ok := manager.GetSessionByChannelID(channelID)
	if !ok || session == nil || session.SMux == nil {
		return nil, "", fmt.Errorf("tokiame session is offline for channel %d", channelID)
	}
	return doTunnelRequestWithSession(ctx, manager, session, channelID, request)
}

func DoTunnelRequestByNamespace(ctx context.Context, namespace string, request *TunnelRequest) (*http.Response, string, error) {
	manager := GetSessionManager()
	session, ok := manager.GetSessionByNamespace(strings.TrimSpace(namespace))
	if !ok || session == nil || session.SMux == nil {
		return nil, "", fmt.Errorf("tokiame session is offline for namespace %s", namespace)
	}
	return doTunnelRequestWithSession(ctx, manager, session, session.ChannelID, request)
}

func doTunnelRequestWithSession(ctx context.Context, manager *SessionManager, session *GatewaySession, channelID int, request *TunnelRequest) (*http.Response, string, error) {
	if request == nil {
		return nil, "", fmt.Errorf("tunnel request is nil")
	}

	stream, err := session.SMux.OpenStream()
	if err != nil {
		return nil, "", fmt.Errorf("open tokiame stream: %w", err)
	}

	requestID := strings.TrimSpace(request.RequestID)
	if requestID == "" {
		requestID = buildTunnelRequestID(session.Namespace)
		request.RequestID = requestID
	}

	_, cancel := context.WithCancel(ctx)
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

	firstFrame, err := codec.ReadResponse()
	if err != nil {
		_ = stream.Close()
		manager.RemoveRequest(requestID)
		cancel()
		return nil, requestID, fmt.Errorf("read tokiame response header: %w", err)
	}
	if firstFrame.Error != nil {
		_ = stream.Close()
		manager.RemoveRequest(requestID)
		cancel()
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

	completed := make(chan struct{})
	var cleanupOnce sync.Once
	cleanup := func() {
		cleanupOnce.Do(func() {
			close(completed)
			cancel()
			manager.RemoveRequest(requestID)
		})
	}

	go watchTunnelContext(ctx, session, requestID, completed)
	go pumpTunnelBody(stream, codec, pipeWriter, firstFrame, cleanup)

	return response, requestID, nil
}

func watchTunnelContext(ctx context.Context, session *GatewaySession, requestID string, completed chan struct{}) {
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
			_ = pipeWriter.CloseWithError(fmt.Errorf("tokiame response error: %s", frame.Error.Message))
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
