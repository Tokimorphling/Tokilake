package tokilake

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"one-api/types"

	"github.com/stretchr/testify/require"
)

type blockingStream struct {
	closeOnce sync.Once
	closeCh   chan struct{}
}

func newBlockingStream() *blockingStream {
	return &blockingStream{
		closeCh: make(chan struct{}),
	}
}

func (s *blockingStream) Read(_ []byte) (int, error) {
	<-s.closeCh
	return 0, io.ErrClosedPipe
}

func (s *blockingStream) Write(p []byte) (int, error) {
	return len(p), nil
}

func (s *blockingStream) Close() error {
	s.closeOnce.Do(func() {
		close(s.closeCh)
	})
	return nil
}

type captureControlStream struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *captureControlStream) Read(_ []byte) (int, error) {
	return 0, io.EOF
}

func (s *captureControlStream) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *captureControlStream) Close() error {
	return nil
}

func (s *captureControlStream) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

type bufferReadWriteCloser struct {
	*bytes.Buffer
}

func (b *bufferReadWriteCloser) Close() error {
	return nil
}

func TestWatchTunnelContextCancelsPendingRead(t *testing.T) {
	stream := newBlockingStream()
	controlStream := &captureControlStream{}
	session := &GatewaySession{
		Namespace:    "watch-namespace",
		controlCodec: newControlCodec(controlStream),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	codec := NewTunnelStreamCodec(stream)
	readErrCh := make(chan error, 1)

	go func() {
		_, err := codec.ReadResponse()
		readErrCh <- err
	}()

	completed := make(chan struct{})
	go watchTunnelContext(ctx, session, stream, "req-123", completed)

	cancel()

	select {
	case err := <-readErrCh:
		require.Error(t, err)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for pending read to be interrupted")
	}

	require.Eventually(t, func() bool {
		payload := controlStream.String()
		return strings.Contains(payload, "\"type\":\"cancel_request\"") &&
			strings.Contains(payload, "\"target_request_id\":\"req-123\"")
	}, time.Second, 10*time.Millisecond)
}

func TestPumpTunnelBodyWrapsMidStreamError(t *testing.T) {
	payload := &bytes.Buffer{}
	writerCodec := NewTunnelStreamCodec(payload)
	require.NoError(t, writerCodec.WriteResponse(&TunnelResponse{
		RequestID: "req-1",
		Error: &ErrorMessage{
			Code:    "worker_failure",
			Message: "worker exploded",
		},
	}))

	stream := &bufferReadWriteCloser{Buffer: payload}
	codec := NewTunnelStreamCodec(stream)
	pipeReader, pipeWriter := io.Pipe()
	done := make(chan struct{})

	go pumpTunnelBody(stream, codec, pipeWriter, &TunnelResponse{
		RequestID:  "req-1",
		StatusCode: http.StatusOK,
	}, func() {
		close(done)
	})

	_, err := io.ReadAll(pipeReader)
	require.Error(t, err)

	var carrier interface {
		GetOpenAIError() *types.OpenAIErrorWithStatusCode
	}
	require.True(t, errors.As(err, &carrier))

	openAIErr := carrier.GetOpenAIError()
	require.NotNil(t, openAIErr)
	require.Equal(t, http.StatusBadGateway, openAIErr.StatusCode)
	require.Equal(t, "worker_failure", openAIErr.Code)
	require.Equal(t, "upstream_error", openAIErr.Type)
	require.Equal(t, "worker exploded", openAIErr.Message)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for pump cleanup")
	}
}
