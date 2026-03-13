package relay

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"one-api/common/config"
	"one-api/common/logger"
	"one-api/types"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

type stubStringStream struct {
	dataChan chan string
	errChan  chan error
}

func (s *stubStringStream) Recv() (<-chan string, <-chan error) {
	return s.dataChan, s.errChan
}

func (s *stubStringStream) Close() {}

type fakeOpenAIStreamError struct {
	err *types.OpenAIErrorWithStatusCode
}

func (e *fakeOpenAIStreamError) Error() string {
	return e.err.Message
}

func (e *fakeOpenAIStreamError) GetOpenAIError() *types.OpenAIErrorWithStatusCode {
	return e.err
}

func TestResponseStreamClientWritesStructuredOpenAIError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	viper.Reset()
	config.InitConf()
	logger.SetupLogger()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil)
	c.Set(logger.RequestIdKey, "req-stream-1")

	stream := &stubStringStream{
		dataChan: make(chan string),
		errChan:  make(chan error, 1),
	}
	stream.errChan <- &fakeOpenAIStreamError{
		err: &types.OpenAIErrorWithStatusCode{
			OpenAIError: types.OpenAIError{
				Message: "worker exploded",
				Type:    "upstream_error",
				Code:    "worker_failure",
			},
			StatusCode: http.StatusBadGateway,
		},
	}

	_, err := responseStreamClient(c, stream, nil)
	require.Nil(t, err)

	body := recorder.Body.String()
	require.Contains(t, body, `data: {"error":`)
	require.Contains(t, body, `"code":"worker_failure"`)
	require.Contains(t, body, `"type":"upstream_error"`)
	require.Contains(t, body, `worker exploded (request id: req-stream-1)`)
	require.NotContains(t, body, `data: [DONE]`)
}
