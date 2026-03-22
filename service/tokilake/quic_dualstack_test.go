package tokilake

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"one-api/controller"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type deadlineBufferStream struct {
	reader *bytes.Reader
	writes bytes.Buffer
}

func newDeadlineBufferStream(payload []byte) *deadlineBufferStream {
	return &deadlineBufferStream{
		reader: bytes.NewReader(payload),
	}
}

func (s *deadlineBufferStream) Read(p []byte) (int, error) {
	return s.reader.Read(p)
}

func (s *deadlineBufferStream) Write(p []byte) (int, error) {
	return s.writes.Write(p)
}

func (s *deadlineBufferStream) Close() error {
	return nil
}

func (s *deadlineBufferStream) SetReadDeadline(time.Time) error {
	return nil
}

func (s *deadlineBufferStream) Written() string {
	return s.writes.String()
}

func TestClientConfigQUICInference(t *testing.T) {
	config := &ClientConfig{
		GatewayURL:    "https://api.example.com/api/tokilake/connect",
		Token:         "token",
		Namespace:     "worker-a",
		TransportMode: TransportModeAuto,
		ModelTargets: map[string]ModelTargetConfig{
			"model-a": {URL: "http://127.0.0.1:8000/v1"},
		},
	}
	require.NoError(t, config.Validate())

	endpoint, err := config.ResolveQUICEndpoint()
	require.NoError(t, err)
	require.Equal(t, "api.example.com:443", endpoint)
	require.True(t, config.ShouldAttemptQUIC())
}

func TestClientConfigSkipsQUICForPlainWebSocket(t *testing.T) {
	config := &ClientConfig{
		GatewayURL:    "ws://127.0.0.1:3000/api/tokilake/connect",
		Token:         "token",
		Namespace:     "worker-a",
		TransportMode: TransportModeAuto,
		ModelTargets: map[string]ModelTargetConfig{
			"model-a": {URL: "http://127.0.0.1:8000/v1"},
		},
	}
	require.NoError(t, config.Validate())
	require.False(t, config.ShouldAttemptQUIC())
}

func TestServeControlStreamRequiresAuthBeforeRegister(t *testing.T) {
	stream := newDeadlineBufferStream([]byte(`{"type":"register","request_id":"req-1","register":{"namespace":"worker-a","models":["model-a"]}}` + "\n"))
	session := &GatewaySession{
		Transport: TunnelTransportQUIC,
	}

	err := serveControlStream(context.Background(), NewSessionManager(), session, stream, newControlCodec(stream))
	require.Error(t, err)
	require.Contains(t, stream.Written(), `"code":"not_authenticated"`)
	require.Contains(t, stream.Written(), `"request_id":"req-1"`)
}

func TestClientRunOnceQUIC(t *testing.T) {
	setupGatewayTestDB(t)

	backend := newIPv4HTTPServer(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/plain")
		_, _ = writer.Write([]byte("ok:" + request.URL.Path))
	}))
	defer backend.Close()

	user := createGatewayTestUser(t, "quic-group")
	token := createGatewayTestToken(t, user.Id)
	certFile, keyFile := createQUICCertificateFiles(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	listener, err := newQUICGatewayListener("127.0.0.1:0", certFile, keyFile)
	require.NoError(t, err)
	defer listener.Close()
	go serveQUICGatewayListener(ctx, listener)

	config := &ClientConfig{
		GatewayURL:         "https://127.0.0.1/api/tokilake/connect",
		QuicEndpoint:       listener.Addr().String(),
		TransportMode:      TransportModeQUIC,
		InsecureSkipVerify: true,
		Token:              token.Key,
		Namespace:          "quic-worker",
		Group:              "quic-group",
		ModelTargets: map[string]ModelTargetConfig{
			"model-a": {URL: backend.URL + "/v1"},
		},
	}
	require.NoError(t, config.Validate())

	client := NewClient(config)
	runErrCh := make(chan error, 1)
	go func() {
		runErrCh <- client.Run(ctx)
	}()

	session := waitForSessionByNamespace(t, "quic-worker")
	require.Equal(t, TunnelTransportQUIC, session.Transport)

	response, requestID, err := DoTunnelRequestByNamespace(context.Background(), "quic-worker", &TunnelRequest{
		RequestID: "relay-quic-1",
		RouteKind: TunnelRouteKindChatCompletions,
		Method:    http.MethodPost,
		Path:      "/v1/chat/completions",
		Model:     "model-a",
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: []byte(`{"model":"model-a"}`),
	})
	require.NoError(t, err)
	require.Equal(t, "relay-quic-1", requestID)
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Equal(t, "ok:/v1/chat/completions", string(body))

	cancel()
	require.NoError(t, <-runErrCh)
}

func TestClientRunOnceAutoFallsBackToWebSocket(t *testing.T) {
	setupGatewayTestDB(t)

	backend := newIPv4HTTPServer(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/plain")
		_, _ = writer.Write([]byte("fallback:" + request.URL.Path))
	}))
	defer backend.Close()

	user := createGatewayTestUser(t, "fallback-group")
	token := createGatewayTestToken(t, user.Id)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/tokilake/connect", controller.TokilakeConnect)
	server := newIPv4HTTPServer(t, router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/tokilake/connect"
	config := &ClientConfig{
		GatewayURL:         wsURL,
		QuicEndpoint:       "127.0.0.1:1",
		TransportMode:      TransportModeAuto,
		InsecureSkipVerify: true,
		Token:              token.Key,
		Namespace:          "fallback-worker",
		Group:              "fallback-group",
		ModelTargets: map[string]ModelTargetConfig{
			"model-a": {URL: backend.URL + "/v1"},
		},
	}
	require.NoError(t, config.Validate())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := NewClient(config)
	runErrCh := make(chan error, 1)
	go func() {
		runErrCh <- client.Run(ctx)
	}()

	session := waitForSessionByNamespace(t, "fallback-worker")
	require.Equal(t, TunnelTransportWebSocket, session.Transport)

	response, _, err := DoTunnelRequestByNamespace(context.Background(), "fallback-worker", &TunnelRequest{
		RequestID: "relay-ws-1",
		RouteKind: TunnelRouteKindChatCompletions,
		Method:    http.MethodPost,
		Path:      "/v1/chat/completions",
		Model:     "model-a",
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: []byte(`{"model":"model-a"}`),
	})
	require.NoError(t, err)
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Equal(t, "fallback:/v1/chat/completions", string(body))

	cancel()
	require.NoError(t, <-runErrCh)
}

func TestClientRunOnceDoesNotFallbackAfterQUICAuthFailure(t *testing.T) {
	setupGatewayTestDB(t)

	certFile, keyFile := createQUICCertificateFiles(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	listener, err := newQUICGatewayListener("127.0.0.1:0", certFile, keyFile)
	require.NoError(t, err)
	defer listener.Close()
	go serveQUICGatewayListener(ctx, listener)

	config := &ClientConfig{
		GatewayURL:         "ws://127.0.0.1:1/api/tokilake/connect",
		QuicEndpoint:       listener.Addr().String(),
		TransportMode:      TransportModeAuto,
		InsecureSkipVerify: true,
		Token:              "invalid-token",
		Namespace:          "auth-fail-worker",
		ModelTargets: map[string]ModelTargetConfig{
			"model-a": {URL: "http://127.0.0.1:8000/v1"},
		},
	}
	require.NoError(t, config.Validate())

	client := NewClient(config)
	err = client.runOnce(context.Background())
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "rejected") || strings.Contains(err.Error(), "Application error"), "error should be auth related failure")
	require.NotContains(t, err.Error(), "dial websocket gateway failed")
}

func waitForSessionByNamespace(t *testing.T, namespace string) *GatewaySession {
	t.Helper()

	manager := GetSessionManager()
	var session *GatewaySession
	require.Eventually(t, func() bool {
		var ok bool
		session, ok = manager.GetSessionByNamespace(namespace)
		return ok && session != nil && session.ChannelID != 0 && session.WorkerID != 0
	}, 5*time.Second, 20*time.Millisecond)
	return session
}

func createQUICCertificateFiles(t *testing.T) (string, string) {
	t.Helper()

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "127.0.0.1",
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:              []string{"localhost"},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	require.NoError(t, err)

	certFile := filepath.Join(t.TempDir(), "quic-cert.pem")
	keyFile := filepath.Join(t.TempDir(), "quic-key.pem")

	certPEM, err := os.Create(certFile)
	require.NoError(t, err)
	defer certPEM.Close()
	require.NoError(t, pem.Encode(certPEM, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}))

	keyBytes, err := x509.MarshalECPrivateKey(privateKey)
	require.NoError(t, err)
	keyPEM, err := os.Create(keyFile)
	require.NoError(t, err)
	defer keyPEM.Close()
	require.NoError(t, pem.Encode(keyPEM, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes}))

	return certFile, keyFile
}

func newIPv4HTTPServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()

	server := httptest.NewUnstartedServer(handler)
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	require.NoError(t, err)
	server.Listener = listener
	server.Start()
	return server
}
