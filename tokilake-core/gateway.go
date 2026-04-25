package tokilake

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/xtaci/smux"
)

const (
	registerTimeout   = 30 * time.Second
	heartbeatTimeout  = 45 * time.Second
)

type Gateway struct {
	Auth     Authenticator
	Registry WorkerRegistry
	Resolver RouteResolver
	Logger   Logger
	Manager  *SessionManager
}

func NewGateway(auth Authenticator, registry WorkerRegistry, resolver RouteResolver, logger Logger, manager *SessionManager) *Gateway {
	if logger == nil {
		logger = DefaultLogger
	}
	if manager == nil {
		manager = NewSessionManager()
	}
	return &Gateway{
		Auth:     auth,
		Registry: registry,
		Resolver: resolver,
		Logger:   logger,
		Manager:  manager,
	}
}

func (g *Gateway) ExtractConnectToken(r *http.Request) (string, error) {
	key := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(key), "bearer ") {
		key = strings.TrimSpace(key[7:])
	}
	if key == "" {
		key = strings.TrimSpace(r.URL.Query().Get("access_token"))
	}
	if key == "" {
		key = strings.TrimSpace(r.URL.Query().Get("token"))
	}
	if key == "" {
		return "", errors.New("missing authorization token")
	}
	key = strings.TrimSpace(strings.TrimPrefix(key, "sk-"))
	if key == "" {
		return "", errors.New("missing authorization token")
	}
	return key, nil
}

func (g *Gateway) AuthenticateConnectRequest(ctx context.Context, r *http.Request) (string, *Token, error) {
	tokenKey, err := g.ExtractConnectToken(r)
	if err != nil {
		return "", nil, err
	}
	return g.Auth.AuthenticateTokenKey(ctx, tokenKey)
}

func (g *Gateway) HandleGatewayConnection(ctx context.Context, wsConn *websocket.Conn, token *Token, tokenKey string, remoteAddr string) error {
	streamConn := NewWebsocketStreamConn(wsConn)
	smuxConfig := smux.DefaultConfig()
	smuxConfig.KeepAliveDisabled = true

	smuxSession, err := smux.Server(streamConn, smuxConfig)
	if err != nil {
		return fmt.Errorf("create smux server: %w", err)
	}

	session := g.Manager.NewGatewaySession(token, tokenKey, remoteAddr, TunnelTransportWebSocket, NewSMuxTunnelSession(smuxSession))
	defer func() {
		if cleanupErr := g.Registry.CleanupWorker(ctx, session); cleanupErr != nil {
			g.Logger.SysError(fmt.Sprintf("tokilake cleanup failed: session_id=%d err=%v", session.ID, cleanupErr))
		}
		g.Manager.Release(session)
		session.Close()
	}()

	return g.serveGatewaySession(ctx, session)
}

func (g *Gateway) serveGatewaySession(ctx context.Context, session *GatewaySession) error {
	if session == nil || session.Tunnel == nil {
		return errors.New("tunnel session is unavailable")
	}

	controlCtx, cancel := context.WithTimeout(ctx, registerTimeout)
	defer cancel()

	controlStream, err := session.Tunnel.AcceptStream(controlCtx)
	if err != nil {
		return fmt.Errorf("accept control stream: %w", err)
	}

	codec := NewControlCodec(controlStream)
	session.Control = controlStream
	session.ControlCodec = codec
	return g.serveControlStream(ctx, session, controlStream, codec)
}

func (g *Gateway) serveControlStream(ctx context.Context, session *GatewaySession, controlStream TunnelStream, codec *ControlCodec) error {
	for {
		if !session.Authenticated || session.WorkerID == 0 {
			_ = controlStream.SetReadDeadline(time.Now().Add(registerTimeout))
		} else {
			_ = controlStream.SetReadDeadline(time.Now().Add(heartbeatTimeout))
		}

		msg, err := codec.ReadMessage()
		if err != nil {
			if errors.Is(err, context.Canceled) || ctx.Err() != nil {
				return nil
			}
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				return fmt.Errorf("control stream timeout: %w", err)
			}
			return err
		}

		switch msg.Type {
		case ControlMessageTypeAuth:
			if session.Authenticated {
				if writeErr := g.writeControlError(codec, msg.RequestID, "auth_already_completed", "auth message already handled"); writeErr != nil {
					return writeErr
				}
				return errors.New("duplicate auth message")
			}
			if msg.Auth == nil {
				if writeErr := g.writeControlError(codec, msg.RequestID, "auth_payload_missing", "auth payload is required"); writeErr != nil {
					return writeErr
				}
				return errors.New("auth payload is required")
			}
			tokenKey, token, authErr := g.Auth.AuthenticateTokenKey(ctx, msg.Auth.Token)
			if authErr != nil {
				if writeErr := g.writeControlError(codec, msg.RequestID, "auth_failed", authErr.Error()); writeErr != nil {
					return writeErr
				}
				return authErr
			}
			session.Token = token
			session.TokenKey = tokenKey
			session.Authenticated = true
			if writeErr := codec.WriteMessage(&ControlMessage{
				Type:      ControlMessageTypeAck,
				RequestID: msg.RequestID,
				Ack: &AckMessage{
					Message: "auth_ok",
				},
			}); writeErr != nil {
				return writeErr
			}
		case ControlMessageTypeRegister:
			if !session.Authenticated {
				if writeErr := g.writeControlError(codec, msg.RequestID, "not_authenticated", "authentication is required"); writeErr != nil {
					return writeErr
				}
				return errors.New("register received before auth")
			}
			if session.WorkerID != 0 {
				if writeErr := g.writeControlError(codec, msg.RequestID, "register_already_completed", "register message already handled"); writeErr != nil {
					return writeErr
				}
				return errors.New("duplicate register message")
			}
			if msg.Register == nil {
				if writeErr := g.writeControlError(codec, msg.RequestID, "register_payload_missing", "register payload is required"); writeErr != nil {
					return writeErr
				}
				return errors.New("register payload is required")
			}
			
			result, registerErr := g.Registry.RegisterWorker(ctx, session, msg.Register)
			if registerErr != nil {
				if writeErr := g.writeControlError(codec, msg.RequestID, "register_failed", registerErr.Error()); writeErr != nil {
					return writeErr
				}
				return registerErr
			}
			if writeErr := codec.WriteMessage(&ControlMessage{
				Type:      ControlMessageTypeAck,
				RequestID: msg.RequestID,
				Ack: &AckMessage{
					Message:   "register_ok",
					Namespace: result.Namespace,
					WorkerID:  result.WorkerID,
					ChannelID: result.ChannelID,
				},
			}); writeErr != nil {
				return writeErr
			}
		case ControlMessageTypeHeartbeat:
			if !session.Authenticated {
				if writeErr := g.writeControlError(codec, msg.RequestID, "not_authenticated", "authentication is required"); writeErr != nil {
					return writeErr
				}
				return errors.New("heartbeat received before auth")
			}
			if session.WorkerID == 0 {
				if writeErr := g.writeControlError(codec, msg.RequestID, "not_registered", "register is required before heartbeat"); writeErr != nil {
					return writeErr
				}
				return errors.New("heartbeat received before register")
			}
			if msg.Heartbeat == nil {
				if writeErr := g.writeControlError(codec, msg.RequestID, "heartbeat_payload_missing", "heartbeat payload is required"); writeErr != nil {
					return writeErr
				}
				return errors.New("heartbeat payload is required")
			}
			if err = g.Registry.UpdateHeartbeat(ctx, session, msg.Heartbeat); err != nil {
				if writeErr := g.writeControlError(codec, msg.RequestID, "heartbeat_failed", err.Error()); writeErr != nil {
					return writeErr
				}
				return err
			}
			if writeErr := codec.WriteMessage(&ControlMessage{
				Type:      ControlMessageTypeAck,
				RequestID: msg.RequestID,
				Ack: &AckMessage{
					Message:   "heartbeat_ok",
					Namespace: session.Namespace,
					WorkerID:  session.WorkerID,
					ChannelID: session.ChannelID,
				},
			}); writeErr != nil {
				return writeErr
			}
		case ControlMessageTypeModelsSync:
			if !session.Authenticated {
				if writeErr := g.writeControlError(codec, msg.RequestID, "not_authenticated", "authentication is required"); writeErr != nil {
					return writeErr
				}
				return errors.New("models_sync received before auth")
			}
			if session.WorkerID == 0 {
				if writeErr := g.writeControlError(codec, msg.RequestID, "not_registered", "register is required before models_sync"); writeErr != nil {
					return writeErr
				}
				return errors.New("models_sync received before register")
			}
			if msg.ModelsSync == nil {
				if writeErr := g.writeControlError(codec, msg.RequestID, "models_payload_missing", "models_sync payload is required"); writeErr != nil {
					return writeErr
				}
				return errors.New("models_sync payload is required")
			}
			if err = g.Registry.SyncModels(ctx, session, msg.ModelsSync); err != nil {
				if writeErr := g.writeControlError(codec, msg.RequestID, "models_sync_failed", err.Error()); writeErr != nil {
					return writeErr
				}
				return err
			}
			if writeErr := codec.WriteMessage(&ControlMessage{
				Type:      ControlMessageTypeAck,
				RequestID: msg.RequestID,
				Ack: &AckMessage{
					Message:   "models_sync_ok",
					Namespace: session.Namespace,
					WorkerID:  session.WorkerID,
					ChannelID: session.ChannelID,
				},
			}); writeErr != nil {
				return writeErr
			}
		case ControlMessageTypeAck:
			continue
		case ControlMessageTypeError:
			if msg.Error != nil {
				g.Logger.SysLog(fmt.Sprintf("tokiame reported error: namespace=%s transport=%s code=%s message=%s", session.Namespace, session.Transport, msg.Error.Code, msg.Error.Message))
			}
		default:
			if !session.Authenticated {
				if writeErr := g.writeControlError(codec, msg.RequestID, "not_authenticated", "authentication is required"); writeErr != nil {
					return writeErr
				}
				return fmt.Errorf("unsupported unauthenticated control message type: %s", msg.Type)
			}
			if writeErr := g.writeControlError(codec, msg.RequestID, "unsupported_message_type", fmt.Sprintf("unsupported message type: %s", msg.Type)); writeErr != nil {
				return writeErr
			}
			return fmt.Errorf("unsupported control message type: %s", msg.Type)
		}
	}
}

func (g *Gateway) SendCancelRequest(session *GatewaySession, targetRequestID string, reason string) error {
	if session == nil {
		return errors.New("session is nil")
	}
	if strings.TrimSpace(targetRequestID) == "" {
		return errors.New("target request id is required")
	}
	if session.ControlCodec == nil {
		return errors.New("control stream is unavailable")
	}
	return session.ControlCodec.WriteMessage(&ControlMessage{
		Type:      ControlMessageTypeCancelRequest,
		RequestID: fmt.Sprintf("%s:cancel:%d", session.Namespace, time.Now().UnixNano()),
		CancelRequest: &CancelRequestMessage{
			TargetRequestID: targetRequestID,
			Reason:          reason,
		},
	})
}

func (g *Gateway) writeControlError(codec *ControlCodec, requestID string, code string, message string) error {
	return codec.WriteMessage(&ControlMessage{
		Type:      ControlMessageTypeError,
		RequestID: requestID,
		Error: &ErrorMessage{
			Code:    code,
			Message: message,
		},
	})
}
