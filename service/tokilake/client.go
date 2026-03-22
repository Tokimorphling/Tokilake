package tokilake

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"one-api/common"
	applog "one-api/pkg/log"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/quic-go/quic-go"
	"github.com/xtaci/smux"
)

const (
	defaultHeartbeatInterval = 15 * time.Second
	defaultReconnectDelay    = 5 * time.Second
	defaultQUICDialTimeout   = 3 * time.Second
	controlAckTimeout        = 15 * time.Second
	defaultAPIKeyHeader      = "Authorization"
	defaultAPIKeyPrefix      = "Bearer "
	TransportModeAuto        = "auto"
	TransportModeQUIC        = "quic"
	TransportModeWebSocket   = "websocket"
)

type ModelTargetConfig struct {
	URL          string            `json:"url"`
	MappedName   string            `json:"mapped_name,omitempty"`
	BackendType  string            `json:"backend_type,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"`
	APIKeys      []string          `json:"api_keys,omitempty"`
	APIKeyHeader string            `json:"api_key_header,omitempty"`
	APIKeyPrefix string            `json:"api_key_prefix,omitempty"`
}

type ResolvedTarget struct {
	ModelName     string
	UpstreamModel string
	URL           string
	BackendType   string
	Headers       map[string]string
}

type ClientConfig struct {
	GatewayURL               string                       `json:"gateway_url"`
	QuicEndpoint             string                       `json:"quic_endpoint,omitempty"`
	TransportMode            string                       `json:"transport_mode,omitempty"`
	Token                    string                       `json:"token"`
	Namespace                string                       `json:"namespace"`
	NodeName                 string                       `json:"node_name,omitempty"`
	Group                    string                       `json:"group,omitempty"`
	BackendType              string                       `json:"backend_type,omitempty"`
	ModelTargets             map[string]ModelTargetConfig `json:"model_targets,omitempty"`
	HeartbeatIntervalSeconds int                          `json:"heartbeat_interval_seconds,omitempty"`
	ReconnectDelaySeconds    int                          `json:"reconnect_delay_seconds,omitempty"`
	InsecureSkipVerify       bool                         `json:"insecure_skip_verify,omitempty"`
}

type Client struct {
	config *ClientConfig
	dialer *websocket.Dialer
	logger applog.Logger

	requestMu      sync.Mutex
	requestCancels map[string]context.CancelFunc
	targetMu       sync.Mutex
	targetKeyNext  map[string]int
}

type clientTunnel struct {
	transport     string
	session       TunnelSession
	controlStream TunnelStream
}

func (t *clientTunnel) Close() error {
	if t == nil {
		return nil
	}
	if t.controlStream != nil {
		_ = t.controlStream.Close()
	}
	if t.session != nil {
		return t.session.Close()
	}
	return nil
}

func LoadClientConfigFromEnv() (*ClientConfig, error) {
	config := &ClientConfig{}

	if configPath := strings.TrimSpace(os.Getenv("TOKIAME_CONFIG")); configPath != "" {
		file, err := os.Open(configPath)
		if err != nil {
			return nil, fmt.Errorf("open TOKIAME_CONFIG: %w", err)
		}
		defer file.Close()
		if err = common.DecodeJson(file, config); err != nil {
			return nil, fmt.Errorf("decode TOKIAME_CONFIG: %w", err)
		}
	}

	overrideStringEnv(&config.GatewayURL, "TOKIAME_GATEWAY_URL")
	overrideStringEnv(&config.QuicEndpoint, "TOKIAME_QUIC_ENDPOINT")
	overrideStringEnv(&config.TransportMode, "TOKIAME_TRANSPORT_MODE")
	overrideStringEnv(&config.Token, "TOKIAME_TOKEN")
	overrideStringEnv(&config.Namespace, "TOKIAME_NAMESPACE")
	overrideStringEnv(&config.NodeName, "TOKIAME_NODE_NAME")
	overrideStringEnv(&config.Group, "TOKIAME_GROUP")
	overrideStringEnv(&config.BackendType, "TOKIAME_BACKEND_TYPE")

	if interval, ok := parsePositiveEnvInt("TOKIAME_HEARTBEAT_INTERVAL_SECONDS"); ok {
		config.HeartbeatIntervalSeconds = interval
	}
	if reconnect, ok := parsePositiveEnvInt("TOKIAME_RECONNECT_DELAY_SECONDS"); ok {
		config.ReconnectDelaySeconds = reconnect
	}

	modelTargetsRaw := strings.TrimSpace(os.Getenv("TOKIAME_MODEL_TARGETS"))
	if modelTargetsRaw != "" {
		modelTargets := make(map[string]ModelTargetConfig)
		if err := common.UnmarshalJsonStr(modelTargetsRaw, &modelTargets); err != nil {
			return nil, fmt.Errorf("decode TOKIAME_MODEL_TARGETS: %w", err)
		}
		config.ModelTargets = modelTargets
	}

	if err := config.Validate(); err != nil {
		return nil, err
	}
	return config, nil
}

func (c *ClientConfig) Validate() error {
	if strings.TrimSpace(c.GatewayURL) == "" {
		return errors.New("TOKIAME_GATEWAY_URL is required")
	}
	if strings.TrimSpace(c.Token) == "" {
		return errors.New("TOKIAME_TOKEN is required")
	}
	if strings.TrimSpace(c.Namespace) == "" {
		return errors.New("TOKIAME_NAMESPACE is required")
	}

	c.ModelTargets = normalizeModelTargets(c.ModelTargets)
	if len(c.ModelTargets) == 0 {
		return errors.New("TOKIAME_MODEL_TARGETS must contain at least one model mapping")
	}

	if c.HeartbeatIntervalSeconds <= 0 {
		c.HeartbeatIntervalSeconds = int(defaultHeartbeatInterval / time.Second)
	}
	if c.ReconnectDelaySeconds <= 0 {
		c.ReconnectDelaySeconds = int(defaultReconnectDelay / time.Second)
	}
	c.QuicEndpoint = normalizeQUICEndpoint(c.QuicEndpoint)
	c.TransportMode = normalizeTransportMode(c.TransportMode)
	if c.TransportMode == "" {
		return errors.New("TOKIAME_TRANSPORT_MODE must be one of auto, quic, websocket")
	}
	c.NodeName = strings.TrimSpace(c.NodeName)
	c.Group = strings.TrimSpace(c.Group)
	c.BackendType = strings.TrimSpace(c.BackendType)
	c.Namespace = strings.TrimSpace(c.Namespace)
	c.GatewayURL = strings.TrimSpace(c.GatewayURL)
	c.Token = strings.TrimSpace(c.Token)
	if c.TransportMode == TransportModeQUIC {
		if _, err := c.ResolveQUICEndpoint(); err != nil {
			return err
		}
	}
	return nil
}

func (c *ClientConfig) ModelNames() []string {
	models := make([]string, 0, len(c.ModelTargets))
	for modelName := range c.ModelTargets {
		models = append(models, modelName)
	}
	slices.Sort(models)
	return models
}

func (c *ClientConfig) HeartbeatInterval() time.Duration {
	return time.Duration(c.HeartbeatIntervalSeconds) * time.Second
}

func (c *ClientConfig) ReconnectDelay() time.Duration {
	return time.Duration(c.ReconnectDelaySeconds) * time.Second
}

func (c *ClientConfig) ControlPlaneBackendType() string {
	backendTypes := make(map[string]struct{})
	for _, target := range c.ModelTargets {
		backendType := effectiveBackendType(target.BackendType, c.BackendType)
		if backendType == "" {
			continue
		}
		backendTypes[backendType] = struct{}{}
	}
	switch len(backendTypes) {
	case 0:
		return normalizeClientBackendType(c.BackendType)
	case 1:
		for backendType := range backendTypes {
			return backendType
		}
	}
	return "mixed"
}

func NewClient(config *ClientConfig) *Client {
	return &Client{
		config: config,
		dialer: &websocket.Dialer{
			Subprotocols: []string{"tokilake.v1"},
		},
		logger: applog.New(
			"component", "tokiame",
			"namespace", config.Namespace,
			"node_name", firstNonEmptyString(config.NodeName, config.Namespace),
		),
		requestCancels: make(map[string]context.CancelFunc),
		targetKeyNext:  make(map[string]int),
	}
}

func (c *Client) Run(ctx context.Context) error {
	c.info("client run loop started gateway_url=%s quic_endpoint=%s transport_mode=%s models=%v group=%s heartbeat_interval=%s reconnect_delay=%s backend_type=%s",
		c.config.GatewayURL,
		c.config.QuicEndpoint,
		c.config.TransportMode,
		c.config.ModelNames(),
		c.config.Group,
		c.config.HeartbeatInterval(),
		c.config.ReconnectDelay(),
		c.config.ControlPlaneBackendType(),
	)
	for {
		err := c.runOnce(ctx)
		if ctx.Err() != nil {
			c.info("client run loop stopped reason=%v", ctx.Err())
			return nil
		}
		if err != nil {
			c.warn("connection closed, retrying retry_delay=%s err=%v", c.config.ReconnectDelay(), err)
		}
		select {
		case <-time.After(c.config.ReconnectDelay()):
		case <-ctx.Done():
			c.info("client run loop stopped reason=%v", ctx.Err())
			return nil
		}
	}
}

func (c *Client) runOnce(ctx context.Context) error {
	tunnel, err := c.dialGateway(ctx)
	if err != nil {
		return err
	}
	defer tunnel.Close()

	codec := newControlCodec(tunnel.controlStream)
	if tunnel.transport == TunnelTransportQUIC {
		if err = c.authenticate(codec, tunnel.controlStream); err != nil {
			return err
		}
	}
	if err = c.register(codec, tunnel.controlStream); err != nil {
		return err
	}
	if err = c.syncModels(codec, tunnel.controlStream); err != nil {
		return err
	}

	c.info("worker connected transport=%s group=%s models=%v backend_type=%s",
		tunnel.transport,
		c.config.Group,
		c.config.ModelNames(),
		c.config.ControlPlaneBackendType(),
	)

	errCh := make(chan error, 2)
	go c.acceptDataStreams(ctx, tunnel.session, errCh)
	go c.readControlLoop(ctx, codec, errCh)
	go c.heartbeatLoop(ctx, codec, errCh)

	select {
	case <-ctx.Done():
		return nil
	case err = <-errCh:
		return err
	}
}

func (c *Client) dialGateway(ctx context.Context) (*clientTunnel, error) {
	switch c.config.TransportMode {
	case TransportModeWebSocket:
		return c.dialWebSocketTunnel(ctx)
	case TransportModeQUIC:
		return c.dialQUICTunnel(ctx)
	default:
		if !c.config.ShouldAttemptQUIC() {
			return c.dialWebSocketTunnel(ctx)
		}
		tunnel, err := c.dialQUICTunnel(ctx)
		if err == nil {
			return tunnel, nil
		}
		c.warn("quic dial failed, falling back transport=%s gateway_url=%s quic_endpoint=%s err=%v",
			TunnelTransportWebSocket, c.config.GatewayURL, c.config.QuicEndpoint, err)
		return c.dialWebSocketTunnel(ctx)
	}
}

func (c *Client) dialWebSocketTunnel(ctx context.Context) (*clientTunnel, error) {
	websocketURL, err := normalizeWebSocketGatewayURL(c.config.GatewayURL)
	if err != nil {
		return nil, err
	}

	headers := make(http.Header)
	headers.Set("Authorization", "Bearer "+c.config.Token)

	c.info("dialing gateway transport=%s gateway_url=%s", TunnelTransportWebSocket, websocketURL)

	wsConn, response, err := c.dialer.DialContext(ctx, websocketURL, headers)
	if err != nil {
		if response != nil {
			return nil, fmt.Errorf("dial websocket gateway failed: %w (status=%s)", err, response.Status)
		}
		return nil, fmt.Errorf("dial websocket gateway failed: %w", err)
	}

	smuxConfig := smux.DefaultConfig()
	smuxConfig.KeepAliveDisabled = true
	smuxSession, err := smux.Client(newWebsocketStreamConn(wsConn), smuxConfig)
	if err != nil {
		_ = wsConn.Close()
		return nil, fmt.Errorf("create smux client: %w", err)
	}

	tunnelSession := newSMuxTunnelSession(smuxSession)
	controlStream, err := tunnelSession.OpenStream(ctx)
	if err != nil {
		_ = tunnelSession.Close()
		_ = wsConn.Close()
		return nil, fmt.Errorf("open websocket control stream: %w", err)
	}

	return &clientTunnel{
		transport:     TunnelTransportWebSocket,
		session:       tunnelSession,
		controlStream: controlStream,
	}, nil
}

func (c *Client) dialQUICTunnel(ctx context.Context) (*clientTunnel, error) {
	quicEndpoint, err := c.config.ResolveQUICEndpoint()
	if err != nil {
		return nil, err
	}

	tlsConfig, err := c.newQUICClientTLSConfig(quicEndpoint)
	if err != nil {
		return nil, err
	}

	dialCtx, cancel := context.WithTimeout(ctx, defaultQUICDialTimeout)
	defer cancel()

	c.info("dialing gateway transport=%s gateway_url=%s quic_endpoint=%s", TunnelTransportQUIC, c.config.GatewayURL, quicEndpoint)
	conn, err := quic.DialAddr(dialCtx, quicEndpoint, tlsConfig, &quic.Config{
		KeepAlivePeriod: c.config.HeartbeatInterval(),
	})
	if err != nil {
		return nil, fmt.Errorf("dial quic gateway failed: %w", err)
	}

	tunnelSession := newQUICTunnelSession(conn)
	controlStream, err := tunnelSession.OpenStream(ctx)
	if err != nil {
		_ = tunnelSession.Close()
		return nil, fmt.Errorf("open quic control stream: %w", err)
	}

	return &clientTunnel{
		transport:     TunnelTransportQUIC,
		session:       tunnelSession,
		controlStream: controlStream,
	}, nil
}

func (c *Client) authenticate(codec *controlCodec, controlStream readDeadlineSetter) error {
	requestID := c.nextRequestID("auth")
	authMsg := &ControlMessage{
		Type:      ControlMessageTypeAuth,
		RequestID: requestID,
		Auth: &AuthMessage{
			Token: c.config.Token,
		},
	}
	c.debug(">>> sending auth request_id=%s namespace=%s", requestID, c.config.Namespace)
	if err := codec.WriteMessage(authMsg); err != nil {
		c.warn("<<< send auth failed request_id=%s err=%v", requestID, err)
		return fmt.Errorf("send auth: %w", err)
	}
	return c.awaitAck(codec, controlStream, requestID, "auth")
}

func (c *Client) register(codec *controlCodec, controlStream readDeadlineSetter) error {
	requestID := c.nextRequestID("register")
	registerMsg := &ControlMessage{
		Type:      ControlMessageTypeRegister,
		RequestID: requestID,
		Register: &RegisterMessage{
			Namespace:    c.config.Namespace,
			NodeName:     c.config.NodeName,
			Group:        c.config.Group,
			Models:       c.config.ModelNames(),
			HardwareInfo: collectHardwareInfo(c.config),
			BackendType:  c.config.ControlPlaneBackendType(),
		},
	}
	c.debug(">>> sending register request_id=%s namespace=%s node_name=%s group=%s models=%v backend_type=%s",
		requestID, c.config.Namespace, c.config.NodeName, c.config.Group, c.config.ModelNames(), c.config.ControlPlaneBackendType())
	if err := codec.WriteMessage(registerMsg); err != nil {
		c.warn("<<< send register failed request_id=%s err=%v", requestID, err)
		return fmt.Errorf("send register: %w", err)
	}
	c.debug(">>> register sent successfully, waiting for ack...")
	return c.awaitAck(codec, controlStream, requestID, "register")
}

func (c *Client) syncModels(codec *controlCodec, controlStream readDeadlineSetter) error {
	requestID := c.nextRequestID("models")
	modelsSyncMsg := &ControlMessage{
		Type:      ControlMessageTypeModelsSync,
		RequestID: requestID,
		ModelsSync: &ModelsSyncMessage{
			Group:        c.config.Group,
			Models:       c.config.ModelNames(),
			HardwareInfo: collectHardwareInfo(c.config),
			BackendType:  c.config.ControlPlaneBackendType(),
		},
	}
	c.debug(">>> sending models_sync request_id=%s namespace=%s models=%v",
		requestID, c.config.Namespace, c.config.ModelNames())
	if err := codec.WriteMessage(modelsSyncMsg); err != nil {
		c.warn("<<< send models_sync failed request_id=%s err=%v", requestID, err)
		return fmt.Errorf("send models_sync: %w", err)
	}
	return c.awaitAck(codec, controlStream, requestID, "models_sync")
}

func (c *Client) awaitAck(codec *controlCodec, controlStream readDeadlineSetter, requestID string, action string) error {
	_ = controlStream.SetReadDeadline(time.Now().Add(controlAckTimeout))
	defer controlStream.SetReadDeadline(time.Time{})

	c.debug("waiting for %s ack request_id=%s", action, requestID)
	for {
		msg, err := codec.ReadMessage()
		if err != nil {
			c.warn("<<< read %s ack failed request_id=%s err=%v", action, requestID, err)
			return fmt.Errorf("read %s ack: %w", action, err)
		}
		switch msg.Type {
		case ControlMessageTypeAck:
			if msg.RequestID != requestID {
				c.debug("received ack for different request_id=%s (expected %s)", msg.RequestID, requestID)
				continue
			}
			c.debug("<<< received %s ack request_id=%s message=%s namespace=%s worker_id=%d channel_id=%d",
				action, msg.RequestID, firstNonEmptyString(msg.Ack.Message, "ok"), msg.Ack.Namespace, msg.Ack.WorkerID, msg.Ack.ChannelID)
			return nil
		case ControlMessageTypeError:
			if msg.RequestID != requestID {
				c.debug("received error for different request_id=%s (expected %s)", msg.RequestID, requestID)
				continue
			}
			if msg.Error != nil {
				c.warn("<<< %s rejected request_id=%s code=%s message=%s", action, requestID, msg.Error.Code, msg.Error.Message)
				return fmt.Errorf("%s rejected: %s", action, msg.Error.Message)
			}
			c.warn("<<< %s rejected request_id=%s", action, requestID)
			return fmt.Errorf("%s rejected", action)
		default:
			c.debug("received unexpected message type=%s while waiting for ack", msg.Type)
		}
	}
}

func (c *Client) readControlLoop(ctx context.Context, codec *controlCodec, errCh chan<- error) {
	for {
		msg, err := codec.ReadMessage()
		if err != nil {
			if ctx.Err() != nil {
				c.debug("control loop read stopped due to context cancellation")
				return
			}
			c.warn("<<< read control message failed err=%v", err)
			select {
			case errCh <- err:
			default:
			}
			return
		}

		switch msg.Type {
		case ControlMessageTypeAck:
			c.debug("<<< received ack request_id=%s message=%s", msg.RequestID, firstNonEmptyString(msg.Ack.Message, "unknown"))
			continue
		case ControlMessageTypeError:
			if msg.Error == nil {
				continue
			}
			c.warn("<<< gateway error request_id=%s code=%s message=%s", msg.RequestID, msg.Error.Code, msg.Error.Message)
			select {
			case errCh <- fmt.Errorf("gateway error: code=%s message=%s", msg.Error.Code, msg.Error.Message):
			default:
			}
			return
		case ControlMessageTypeCancelRequest:
			if msg.CancelRequest == nil {
				continue
			}
			c.info("<<< received cancel_request request_id=%s target_request_id=%s reason=%s",
				msg.RequestID, msg.CancelRequest.TargetRequestID, firstNonEmptyString(msg.CancelRequest.Reason, "unknown"))
			cancelled := c.cancelLocalRequest(msg.CancelRequest.TargetRequestID)
			ackMessage := "cancel_noop"
			if cancelled {
				ackMessage = "cancel_ok"
			}
			_ = codec.WriteMessage(&ControlMessage{
				Type:      ControlMessageTypeAck,
				RequestID: msg.RequestID,
				Ack: &AckMessage{
					Message:   ackMessage,
					Namespace: c.config.Namespace,
				},
			})
			c.debug(">>> sent cancel ack request_id=%s cancelled=%v", msg.RequestID, cancelled)
		default:
			c.debug("ignoring unsupported control message type=%s request_id=%s", msg.Type, msg.RequestID)
		}
	}
}

func (c *Client) heartbeatLoop(ctx context.Context, codec *controlCodec, errCh chan<- error) {
	ticker := time.NewTicker(c.config.HeartbeatInterval())
	defer ticker.Stop()

	c.info("heartbeat loop started interval=%s", c.config.HeartbeatInterval())
	for {
		select {
		case <-ctx.Done():
			c.debug("heartbeat loop stopped due to context cancellation")
			return
		case <-ticker.C:
			heartbeatMsg := &ControlMessage{
				Type:      ControlMessageTypeHeartbeat,
				RequestID: c.nextRequestID("heartbeat"),
				Heartbeat: &HeartbeatMessage{
					NodeName:      c.config.NodeName,
					HardwareInfo:  collectHardwareInfo(c.config),
					CurrentModels: c.config.ModelNames(),
				},
			}
			c.debug(">>> sending heartbeat request_id=%s node_name=%s models=%v",
				heartbeatMsg.RequestID, c.config.NodeName, c.config.ModelNames())
			err := codec.WriteMessage(heartbeatMsg)
			if err != nil {
				c.warn("<<< send heartbeat failed request_id=%s err=%v", heartbeatMsg.RequestID, err)
				select {
				case errCh <- fmt.Errorf("send heartbeat: %w", err):
				default:
				}
				return
			}
		}
	}
}

func (c *Client) nextRequestID(prefix string) string {
	return fmt.Sprintf("%s:%s:%s", c.config.Namespace, prefix, uuid.NewString())
}

func (c *ClientConfig) ShouldAttemptQUIC() bool {
	if strings.TrimSpace(c.QuicEndpoint) != "" {
		return true
	}

	gatewayURL, err := url.Parse(strings.TrimSpace(c.GatewayURL))
	if err != nil {
		return false
	}

	switch strings.ToLower(strings.TrimSpace(gatewayURL.Scheme)) {
	case "wss", "https":
		return true
	default:
		return false
	}
}

func (c *ClientConfig) ResolveQUICEndpoint() (string, error) {
	if endpoint := normalizeQUICEndpoint(c.QuicEndpoint); endpoint != "" {
		return endpoint, nil
	}

	gatewayURL, err := url.Parse(strings.TrimSpace(c.GatewayURL))
	if err != nil {
		return "", fmt.Errorf("parse TOKIAME_GATEWAY_URL: %w", err)
	}

	switch strings.ToLower(strings.TrimSpace(gatewayURL.Scheme)) {
	case "wss", "https":
	default:
		return "", errors.New("QUIC requires a secure TOKIAME_GATEWAY_URL or TOKIAME_QUIC_ENDPOINT")
	}

	host := strings.TrimSpace(gatewayURL.Hostname())
	if host == "" {
		return "", errors.New("TOKIAME_GATEWAY_URL host is required")
	}

	port := strings.TrimSpace(gatewayURL.Port())
	if port == "" {
		port = "443"
	}

	return net.JoinHostPort(host, port), nil
}

func (c *Client) resolveModelTarget(modelName string) (*ResolvedTarget, error) {
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return nil, errors.New("model is required")
	}
	target, ok := c.config.ModelTargets[modelName]
	if !ok {
		return nil, fmt.Errorf("no local target configured for model %s", modelName)
	}
	return c.buildResolvedTarget(modelName, target), nil
}

func (c *Client) buildResolvedTarget(modelName string, target ModelTargetConfig) *ResolvedTarget {
	headers := cloneStringMap(target.Headers)
	if apiKey := c.selectTargetAPIKey(modelName, target.APIKeys); apiKey != "" {
		headerName := strings.TrimSpace(target.APIKeyHeader)
		if headerName == "" {
			headerName = defaultAPIKeyHeader
		}
		headerValue := apiKey
		if strings.EqualFold(headerName, defaultAPIKeyHeader) {
			prefix := target.APIKeyPrefix
			if prefix == "" {
				prefix = defaultAPIKeyPrefix
			}
			headerValue = prefix + apiKey
		} else if target.APIKeyPrefix != "" {
			headerValue = target.APIKeyPrefix + apiKey
		}
		headers[headerName] = headerValue
	}
	return &ResolvedTarget{
		ModelName:     modelName,
		UpstreamModel: firstNonEmptyString(target.MappedName, modelName),
		URL:           target.URL,
		BackendType:   effectiveBackendType(target.BackendType, c.config.BackendType),
		Headers:       headers,
	}
}

func (c *Client) selectTargetAPIKey(modelName string, apiKeys []string) string {
	if len(apiKeys) == 0 {
		return ""
	}
	if len(apiKeys) == 1 {
		return apiKeys[0]
	}
	c.targetMu.Lock()
	defer c.targetMu.Unlock()
	index := c.targetKeyNext[modelName]
	selected := apiKeys[index%len(apiKeys)]
	c.targetKeyNext[modelName] = (index + 1) % len(apiKeys)
	return selected
}

func (c *Client) trackLocalRequest(requestID string, cancel context.CancelFunc) {
	if requestID == "" || cancel == nil {
		return
	}
	c.requestMu.Lock()
	defer c.requestMu.Unlock()
	c.requestCancels[requestID] = cancel
}

func (c *Client) removeLocalRequest(requestID string) {
	if requestID == "" {
		return
	}
	c.requestMu.Lock()
	defer c.requestMu.Unlock()
	delete(c.requestCancels, requestID)
}

func (c *Client) cancelLocalRequest(requestID string) bool {
	if requestID == "" {
		return false
	}
	c.requestMu.Lock()
	cancel, ok := c.requestCancels[requestID]
	c.requestMu.Unlock()
	if !ok || cancel == nil {
		return false
	}
	cancel()
	return true
}

func collectHardwareInfo(config *ClientConfig) map[string]any {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = ""
	}
	return map[string]any{
		"hostname":              hostname,
		"goos":                  runtime.GOOS,
		"goarch":                runtime.GOARCH,
		"num_cpu":               runtime.NumCPU(),
		"default_backend_type":  normalizeClientBackendType(config.BackendType),
		"control_plane_backend": config.ControlPlaneBackendType(),
		"model_target_summaries": sanitizeModelTargets(
			config.ModelTargets,
			config.BackendType,
		),
	}
}

func sanitizeModelTargets(modelTargets map[string]ModelTargetConfig, defaultBackendType string) map[string]any {
	if len(modelTargets) == 0 {
		return nil
	}
	summary := make(map[string]any, len(modelTargets))
	for modelName, target := range modelTargets {
		summary[modelName] = map[string]any{
			"url":          sanitizeTargetURL(target.URL),
			"mapped_name":  target.MappedName,
			"backend_type": effectiveBackendType(target.BackendType, defaultBackendType),
			"has_api_keys": len(target.APIKeys) > 0,
			"header_count": len(target.Headers),
		}
	}
	return summary
}

func sanitizeTargetURL(raw string) string {
	return strings.TrimSpace(raw)
}

func normalizeModelTargets(modelTargets map[string]ModelTargetConfig) map[string]ModelTargetConfig {
	normalized := make(map[string]ModelTargetConfig, len(modelTargets))
	for modelName, target := range modelTargets {
		modelName = strings.TrimSpace(modelName)
		target = normalizeModelTargetConfig(target)
		if modelName == "" || target.URL == "" {
			continue
		}
		normalized[modelName] = target
	}
	return normalized
}

func normalizeModelTargetConfig(target ModelTargetConfig) ModelTargetConfig {
	target.URL = strings.TrimSpace(target.URL)
	target.MappedName = strings.TrimSpace(target.MappedName)
	target.BackendType = strings.TrimSpace(target.BackendType)
	target.APIKeyHeader = strings.TrimSpace(target.APIKeyHeader)
	target.Headers = normalizeHeaderMap(target.Headers)
	target.APIKeys = normalizeAPIKeys(target.APIKeys)
	return target
}

func normalizeHeaderMap(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	normalized := make(map[string]string, len(headers))
	for key, value := range headers {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		normalized[key] = value
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func normalizeAPIKeys(apiKeys []string) []string {
	if len(apiKeys) == 0 {
		return nil
	}
	normalized := make([]string, 0, len(apiKeys))
	for _, apiKey := range apiKeys {
		apiKey = strings.TrimSpace(apiKey)
		if apiKey == "" {
			continue
		}
		normalized = append(normalized, apiKey)
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func effectiveBackendType(targetBackendType string, defaultBackendType string) string {
	return normalizeClientBackendType(firstNonEmptyString(targetBackendType, defaultBackendType))
}

func normalizeClientBackendType(backendType string) string {
	backendType = strings.ToLower(strings.TrimSpace(backendType))
	switch backendType {
	case "", "openai", "sglang":
		if backendType == "" {
			return "openai"
		}
		return backendType
	default:
		return backendType
	}
}

func normalizeTransportMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", TransportModeAuto:
		return TransportModeAuto
	case TransportModeQUIC:
		return TransportModeQUIC
	case TransportModeWebSocket:
		return TransportModeWebSocket
	default:
		return ""
	}
}

func normalizeQUICEndpoint(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	if strings.Contains(raw, "://") {
		parsed, err := url.Parse(raw)
		if err != nil {
			return raw
		}
		host := strings.TrimSpace(parsed.Hostname())
		port := strings.TrimSpace(parsed.Port())
		if host == "" || port == "" {
			return raw
		}
		return net.JoinHostPort(host, port)
	}

	host, port, err := net.SplitHostPort(raw)
	if err != nil || strings.TrimSpace(host) == "" || strings.TrimSpace(port) == "" {
		return raw
	}
	return net.JoinHostPort(host, port)
}

func normalizeWebSocketGatewayURL(raw string) (string, error) {
	gatewayURL, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", fmt.Errorf("parse TOKIAME_GATEWAY_URL: %w", err)
	}

	switch strings.ToLower(strings.TrimSpace(gatewayURL.Scheme)) {
	case "ws", "wss":
	case "http":
		gatewayURL.Scheme = "ws"
	case "https":
		gatewayURL.Scheme = "wss"
	default:
		return "", fmt.Errorf("unsupported TOKIAME_GATEWAY_URL scheme: %s", gatewayURL.Scheme)
	}

	return gatewayURL.String(), nil
}

func (c *Client) newQUICClientTLSConfig(endpoint string) (*tls.Config, error) {
	host, _, err := net.SplitHostPort(endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid QUIC endpoint %q: %w", endpoint, err)
	}
	host = strings.Trim(strings.TrimSpace(host), "[]")
	if host == "" {
		return nil, fmt.Errorf("invalid QUIC endpoint %q: host is required", endpoint)
	}

	return &tls.Config{
		MinVersion:         tls.VersionTLS13,
		NextProtos:         []string{"tokilake.v1"},
		ServerName:         host,
		InsecureSkipVerify: c.config.InsecureSkipVerify,
	}, nil
}

func overrideStringEnv(target *string, envName string) {
	if target == nil {
		return
	}
	if value := strings.TrimSpace(os.Getenv(envName)); value != "" {
		*target = value
	}
}

func parsePositiveEnvInt(envName string) (int, bool) {
	value := strings.TrimSpace(os.Getenv(envName))
	if value == "" {
		return 0, false
	}
	var parsed int
	if _, err := fmt.Sscanf(value, "%d", &parsed); err != nil || parsed <= 0 {
		return 0, false
	}
	return parsed, true
}

func cloneStringMap(source map[string]string) map[string]string {
	if len(source) == 0 {
		return map[string]string{}
	}
	cloned := make(map[string]string, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func (c *Client) info(format string, args ...any) {
	c.logger.Info(fmt.Sprintf(format, args...))
}

func (c *Client) warn(format string, args ...any) {
	c.logger.Warn(fmt.Sprintf(format, args...))
}

func (c *Client) debug(format string, args ...any) {
	c.logger.Debug(fmt.Sprintf(format, args...))
}

type readDeadlineSetter interface {
	SetReadDeadline(t time.Time) error
}
