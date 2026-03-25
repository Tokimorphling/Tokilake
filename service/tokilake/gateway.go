package tokilake

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"slices"
	"strings"
	"time"

	"one-api/common"
	"one-api/common/config"
	"one-api/common/logger"
	"one-api/model"

	"github.com/gorilla/websocket"
	"github.com/xtaci/smux"
	"gorm.io/gorm"
)

const (
	registerTimeout   = 30 * time.Second
	heartbeatTimeout  = 45 * time.Second
	tokiameNamePrefix = "Tokiame"
)

type RegisterResult struct {
	WorkerID    int
	ChannelID   int
	Namespace   string
	Group       string
	Models      []string
	BackendType string
	Status      int
}

type controlPlaneError struct {
	code    string
	message string
}

func (e *controlPlaneError) Error() string {
	return e.message
}

func ExtractConnectToken(r *http.Request) (string, error) {
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

func AuthenticateTokenKey(tokenKey string) (string, *model.Token, error) {
	tokenKey = strings.TrimSpace(tokenKey)
	if strings.HasPrefix(strings.ToLower(tokenKey), "bearer ") {
		tokenKey = strings.TrimSpace(tokenKey[7:])
	}
	tokenKey = strings.TrimSpace(strings.TrimPrefix(tokenKey, "sk-"))
	if tokenKey == "" {
		return "", nil, errors.New("missing authorization token")
	}
	token, err := model.ValidateUserToken(tokenKey)
	if err != nil {
		return "", nil, err
	}
	return tokenKey, token, nil
}

func AuthenticateConnectRequest(r *http.Request) (string, *model.Token, error) {
	tokenKey, err := ExtractConnectToken(r)
	if err != nil {
		return "", nil, err
	}
	return AuthenticateTokenKey(tokenKey)
}

func HandleGatewayConnection(ctx context.Context, wsConn *websocket.Conn, token *model.Token, tokenKey string, remoteAddr string) error {
	streamConn := newWebsocketStreamConn(wsConn)
	smuxConfig := smux.DefaultConfig()
	smuxConfig.KeepAliveDisabled = true

	smuxSession, err := smux.Server(streamConn, smuxConfig)
	if err != nil {
		return fmt.Errorf("create smux server: %w", err)
	}

	manager := GetSessionManager()
	session := manager.NewGatewaySession(token, tokenKey, remoteAddr, TunnelTransportWebSocket, newSMuxTunnelSession(smuxSession))
	defer func() {
		if cleanupErr := cleanupGatewaySession(session); cleanupErr != nil {
			logger.SysError(fmt.Sprintf("tokilake cleanup failed: session_id=%d err=%v", session.ID, cleanupErr))
		}
	}()

	return serveGatewaySession(ctx, manager, session)
}

func serveGatewaySession(ctx context.Context, manager *SessionManager, session *GatewaySession) error {
	if session == nil || session.Tunnel == nil {
		return errors.New("tunnel session is unavailable")
	}

	controlCtx, cancel := context.WithTimeout(ctx, registerTimeout)
	defer cancel()

	controlStream, err := session.Tunnel.AcceptStream(controlCtx)
	if err != nil {
		return fmt.Errorf("accept control stream: %w", err)
	}

	codec := newControlCodec(controlStream)
	session.Control = controlStream
	session.controlCodec = codec
	return serveControlStream(ctx, manager, session, controlStream, codec)
}

func serveControlStream(ctx context.Context, manager *SessionManager, session *GatewaySession, controlStream TunnelStream, codec *controlCodec) error {
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
				if writeErr := writeControlError(codec, msg.RequestID, "auth_already_completed", "auth message already handled"); writeErr != nil {
					return writeErr
				}
				return errors.New("duplicate auth message")
			}
			if msg.Auth == nil {
				if writeErr := writeControlError(codec, msg.RequestID, "auth_payload_missing", "auth payload is required"); writeErr != nil {
					return writeErr
				}
				return errors.New("auth payload is required")
			}
			tokenKey, token, authErr := AuthenticateTokenKey(msg.Auth.Token)
			if authErr != nil {
				if writeErr := writeControlError(codec, msg.RequestID, "auth_failed", authErr.Error()); writeErr != nil {
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
				if writeErr := writeControlError(codec, msg.RequestID, "not_authenticated", "authentication is required"); writeErr != nil {
					return writeErr
				}
				return errors.New("register received before auth")
			}
			if session.WorkerID != 0 {
				if writeErr := writeControlError(codec, msg.RequestID, "register_already_completed", "register message already handled"); writeErr != nil {
					return writeErr
				}
				return errors.New("duplicate register message")
			}
			if msg.Register == nil {
				if writeErr := writeControlError(codec, msg.RequestID, "register_payload_missing", "register payload is required"); writeErr != nil {
					return writeErr
				}
				return errors.New("register payload is required")
			}
			result, registerErr := registerWorkerSession(manager, session, msg.Register)
			if registerErr != nil {
				if writeErr := writeControlError(codec, msg.RequestID, controlPlaneErrorCode(registerErr, "register_failed"), registerErr.Error()); writeErr != nil {
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
				if writeErr := writeControlError(codec, msg.RequestID, "not_authenticated", "authentication is required"); writeErr != nil {
					return writeErr
				}
				return errors.New("heartbeat received before auth")
			}
			if session.WorkerID == 0 {
				if writeErr := writeControlError(codec, msg.RequestID, "not_registered", "register is required before heartbeat"); writeErr != nil {
					return writeErr
				}
				return errors.New("heartbeat received before register")
			}
			if msg.Heartbeat == nil {
				if writeErr := writeControlError(codec, msg.RequestID, "heartbeat_payload_missing", "heartbeat payload is required"); writeErr != nil {
					return writeErr
				}
				return errors.New("heartbeat payload is required")
			}
			if err = updateWorkerHeartbeat(session, msg.Heartbeat); err != nil {
				if writeErr := writeControlError(codec, msg.RequestID, "heartbeat_failed", err.Error()); writeErr != nil {
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
				if writeErr := writeControlError(codec, msg.RequestID, "not_authenticated", "authentication is required"); writeErr != nil {
					return writeErr
				}
				return errors.New("models_sync received before auth")
			}
			if session.WorkerID == 0 {
				if writeErr := writeControlError(codec, msg.RequestID, "not_registered", "register is required before models_sync"); writeErr != nil {
					return writeErr
				}
				return errors.New("models_sync received before register")
			}
			if msg.ModelsSync == nil {
				if writeErr := writeControlError(codec, msg.RequestID, "models_payload_missing", "models_sync payload is required"); writeErr != nil {
					return writeErr
				}
				return errors.New("models_sync payload is required")
			}
			if err = syncWorkerModels(session, msg.ModelsSync); err != nil {
				if writeErr := writeControlError(codec, msg.RequestID, controlPlaneErrorCode(err, "models_sync_failed"), err.Error()); writeErr != nil {
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
				logger.SysLog(fmt.Sprintf("tokiame reported error: namespace=%s transport=%s code=%s message=%s", session.Namespace, session.Transport, msg.Error.Code, msg.Error.Message))
			}
		default:
			if !session.Authenticated {
				if writeErr := writeControlError(codec, msg.RequestID, "not_authenticated", "authentication is required"); writeErr != nil {
					return writeErr
				}
				return fmt.Errorf("unsupported unauthenticated control message type: %s", msg.Type)
			}
			if writeErr := writeControlError(codec, msg.RequestID, "unsupported_message_type", fmt.Sprintf("unsupported message type: %s", msg.Type)); writeErr != nil {
				return writeErr
			}
			return fmt.Errorf("unsupported control message type: %s", msg.Type)
		}
	}
}

func SendCancelRequest(session *GatewaySession, targetRequestID string, reason string) error {
	if session == nil {
		return errors.New("session is nil")
	}
	if strings.TrimSpace(targetRequestID) == "" {
		return errors.New("target request id is required")
	}
	if session.controlCodec == nil {
		return errors.New("control stream is unavailable")
	}
	return session.controlCodec.WriteMessage(&ControlMessage{
		Type:      ControlMessageTypeCancelRequest,
		RequestID: fmt.Sprintf("%s:cancel:%d", session.Namespace, time.Now().UnixNano()),
		CancelRequest: &CancelRequestMessage{
			TargetRequestID: targetRequestID,
			Reason:          reason,
		},
	})
}

func registerWorkerSession(manager *SessionManager, session *GatewaySession, register *RegisterMessage) (*RegisterResult, error) {
	namespace := strings.TrimSpace(register.Namespace)
	if namespace == "" {
		return nil, errors.New("namespace is required")
	}

	if err := manager.ClaimNamespace(session, namespace); err != nil {
		return nil, err
	}

	result, err := upsertWorkerAndChannel(session, register)
	if err != nil {
		manager.Release(session)
		return nil, err
	}

	manager.BindChannel(session, result.WorkerID, result.ChannelID, result.Group, result.Models, result.BackendType, result.Status)
	model.ChannelGroup.Load()
	model.GlobalUserGroupRatio.Load()
	return result, nil
}

func updateWorkerHeartbeat(session *GatewaySession, heartbeat *HeartbeatMessage) error {
	status := normalizeWorkerStatus(heartbeat.Status)
	models := session.Models
	modelsChanged := false
	if len(heartbeat.CurrentModels) > 0 {
		models = normalizeModels(heartbeat.CurrentModels)
		modelsChanged = !stringSlicesEqual(models, session.Models)
	}

	now := time.Now().Unix()
	statusChanged := status != session.Status

	// Build node updates directly without SELECT
	nodeUpdates := map[string]any{
		"status":         status,
		"last_heartbeat": now,
		"updated_at":     now,
	}
	if heartbeat.NodeName != "" {
		nodeUpdates["node_name"] = strings.TrimSpace(heartbeat.NodeName)
	}
	if heartbeat.HardwareInfo != nil {
		if data, err := common.Marshal(heartbeat.HardwareInfo); err == nil {
			nodeUpdates["hardware_info"] = string(data)
		}
	}
	if modelsChanged {
		if data, err := common.Marshal(models); err == nil {
			nodeUpdates["models"] = string(data)
		}
	}

	// Build channel updates directly without SELECT
	channelUpdates := map[string]any{
		"status": channelStatusFromWorkerStatus(status),
	}
	if modelsChanged {
		channelUpdates["models"] = strings.Join(models, ",")
	}

	// Direct UPDATE queries — no SELECT needed
	if err := model.DB.Model(&model.TokilakeWorkerNode{}).Where("id = ?", session.WorkerID).Updates(nodeUpdates).Error; err != nil {
		return err
	}
	if err := model.DB.Model(&model.Channel{}).Where("id = ?", session.ChannelID).Updates(channelUpdates).Error; err != nil {
		return err
	}

	// Update in-memory session state
	session.Status = status
	if modelsChanged {
		session.Models = models
	}

	// Only reload channel group when something user-visible changed
	if statusChanged || modelsChanged {
		model.ChannelGroup.Load()
	}
	return nil
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func syncWorkerModels(session *GatewaySession, modelsSync *ModelsSyncMessage) error {
	models := normalizeModels(modelsSync.Models)
	if len(models) == 0 {
		return errors.New("at least one model is required")
	}
	group, err := resolveAuthorizedGroup(session.Token.UserId, modelsSync.Group, session.Group)
	if err != nil {
		return err
	}
	backendType := normalizeBackendType(modelsSync.BackendType, session.BackendType)

	err = model.DB.Transaction(func(tx *gorm.DB) error {
		channel := &model.Channel{}
		if err := tx.First(channel, "id = ?", session.ChannelID).Error; err != nil {
			return err
		}
		node := &model.TokilakeWorkerNode{}
		if err := tx.First(node, "id = ?", session.WorkerID).Error; err != nil {
			return err
		}
		node.SetModels(models)
		if modelsSync.HardwareInfo != nil {
			node.SetHardwareInfo(modelsSync.HardwareInfo)
		}
		now := time.Now().Unix()
		if err := tx.Model(node).Updates(map[string]any{
			"models":         node.Models,
			"hardware_info":  node.HardwareInfo,
			"last_heartbeat": now,
			"updated_at":     now,
		}).Error; err != nil {
			return err
		}

		if err := ensureUserGroups(tx, group); err != nil {
			return err
		}

		channel.Models = strings.Join(models, ",")
		channel.Group = group
		channel.Status = channelStatusFromWorkerStatus(session.Status)
		if err := tx.Model(channel).Updates(map[string]any{
			"models":   channel.Models,
			"group":    channel.Group,
			"status":   channel.Status,
			"base_url": tokiameChannelBaseURL(session.Namespace),
			"type":     config.ChannelTypeTokiame,
		}).Error; err != nil {
			return err
		}

		session.Models = models
		session.Group = group
		session.BackendType = backendType
		return nil
	})
	if err == nil {
		model.ChannelGroup.Load()
		model.GlobalUserGroupRatio.Load()
	}
	return err
}

func upsertWorkerAndChannel(session *GatewaySession, register *RegisterMessage) (*RegisterResult, error) {
	models := normalizeModels(register.Models)
	if len(models) == 0 {
		return nil, errors.New("at least one model is required")
	}

	group, err := resolveAuthorizedGroup(session.Token.UserId, register.Group, "")
	if err != nil {
		return nil, err
	}
	nodeName := normalizeNodeName(register.Namespace, register.NodeName)
	backendType := normalizeBackendType(register.BackendType, "")
	status := model.TokilakeWorkerNodeStatusOnline

	result := &RegisterResult{
		Namespace:   strings.TrimSpace(register.Namespace),
		Group:       group,
		Models:      models,
		BackendType: backendType,
		Status:      status,
	}

	err = model.DB.Transaction(func(tx *gorm.DB) error {
		now := time.Now().Unix()

		node := &model.TokilakeWorkerNode{}
		err := tx.Where("namespace = ?", result.Namespace).First(node).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			node = &model.TokilakeWorkerNode{}
		} else if node.ProviderId != 0 && node.ProviderId != session.Token.UserId {
			return &controlPlaneError{
				code:    "namespace_not_owned",
				message: fmt.Sprintf("namespace %s is already owned by another user", result.Namespace),
			}
		}

		channel, err := loadOrCreateTokiameChannel(tx, node.ChannelId)
		if err != nil {
			return err
		}

		if err := ensureUserGroups(tx, group); err != nil {
			return err
		}

		channelName := tokiameChannelName(result.Namespace, nodeName)
		baseURL := tokiameChannelBaseURL(result.Namespace)
		if channel.Id == 0 {
			channel.Type = config.ChannelTypeTokiame
			channel.Key = ""
			channel.CreatedTime = now
			channel.Weight = &config.DefaultChannelWeight
		}
		channel.Type = config.ChannelTypeTokiame
		channel.Name = channelName
		channel.BaseURL = &baseURL
		channel.Models = strings.Join(models, ",")
		channel.Group = group
		channel.Status = channelStatusFromWorkerStatus(status)
		if channel.Id == 0 {
			if err = tx.Create(channel).Error; err != nil {
				return err
			}
		} else {
			if err = tx.Model(channel).Updates(map[string]any{
				"type":     channel.Type,
				"name":     channel.Name,
				"base_url": channel.BaseURL,
				"models":   channel.Models,
				"group":    channel.Group,
				"status":   channel.Status,
			}).Error; err != nil {
				return err
			}
		}

		node.ProviderId = session.Token.UserId
		node.Namespace = result.Namespace
		node.NodeName = nodeName
		node.Status = status
		node.ChannelId = channel.Id
		node.LastHeartbeat = now
		node.UpdatedAt = now
		if node.Id == 0 {
			node.CreatedAt = now
		}
		node.SetModels(models)
		if register.HardwareInfo != nil {
			node.SetHardwareInfo(register.HardwareInfo)
		}

		if node.Id == 0 {
			if err = tx.Create(node).Error; err != nil {
				return err
			}
		} else {
			if err = tx.Model(node).Updates(map[string]any{
				"provider_id":    node.ProviderId,
				"node_name":      node.NodeName,
				"status":         node.Status,
				"models":         node.Models,
				"hardware_info":  node.HardwareInfo,
				"last_heartbeat": node.LastHeartbeat,
				"channel_id":     node.ChannelId,
				"updated_at":     node.UpdatedAt,
			}).Error; err != nil {
				return err
			}
		}

		result.WorkerID = node.Id
		result.ChannelID = channel.Id
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func cleanupGatewaySession(session *GatewaySession) error {
	var err error
	if session.WorkerID != 0 && session.ChannelID != 0 {
		err = model.DB.Transaction(func(tx *gorm.DB) error {
			node := &model.TokilakeWorkerNode{}
			if txErr := tx.First(node, "id = ?", session.WorkerID).Error; txErr != nil {
				if errors.Is(txErr, gorm.ErrRecordNotFound) {
					return nil
				}
				return txErr
			}
			channel := &model.Channel{}
			if txErr := tx.First(channel, "id = ?", session.ChannelID).Error; txErr != nil {
				if errors.Is(txErr, gorm.ErrRecordNotFound) {
					return nil
				}
				return txErr
			}

			now := time.Now().Unix()
			node.Status = model.TokilakeWorkerNodeStatusOffline
			node.LastHeartbeat = now
			node.UpdatedAt = now
			if txErr := tx.Model(node).Updates(map[string]any{
				"status":         node.Status,
				"last_heartbeat": node.LastHeartbeat,
				"updated_at":     node.UpdatedAt,
			}).Error; txErr != nil {
				return txErr
			}

			channel.Status = config.ChannelStatusAutoDisabled
			if txErr := tx.Model(channel).Update("status", channel.Status).Error; txErr != nil {
				return txErr
			}
			return nil
		})
		if err == nil {
			model.ChannelGroup.Load()
		}
	}

	manager := GetSessionManager()
	manager.Release(session)

	closeErr := session.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func loadOrCreateTokiameChannel(tx *gorm.DB, channelID int) (*model.Channel, error) {
	channel := &model.Channel{}
	if channelID == 0 {
		return channel, nil
	}
	if err := tx.First(channel, "id = ?", channelID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &model.Channel{}, nil
		}
		return nil, err
	}
	return channel, nil
}

func ensureUserGroups(tx *gorm.DB, groups string) error {
	for _, group := range strings.Split(strings.TrimSpace(groups), ",") {
		group = strings.TrimSpace(group)
		if group == "" {
			continue
		}
		existing := &model.UserGroup{}
		err := tx.Where("symbol = ?", group).First(existing).Error
		if err == nil {
			continue
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		enable := true
		userGroup := &model.UserGroup{
			Symbol: group,
			Name:   group,
			Ratio:  1,
			Public: false,
			Enable: &enable,
		}
		if err = tx.Create(userGroup).Error; err != nil {
			if retryErr := tx.Where("symbol = ?", group).First(existing).Error; retryErr == nil {
				continue
			}
			return err
		}
	}
	return nil
}

func writeControlError(codec *controlCodec, requestID string, code string, message string) error {
	return codec.WriteMessage(&ControlMessage{
		Type:      ControlMessageTypeError,
		RequestID: requestID,
		Error: &ErrorMessage{
			Code:    code,
			Message: message,
		},
	})
}

func controlPlaneErrorCode(err error, fallback string) string {
	var target *controlPlaneError
	if errors.As(err, &target) && strings.TrimSpace(target.code) != "" {
		return target.code
	}
	return fallback
}

func normalizeModels(models []string) []string {
	seen := make(map[string]struct{}, len(models))
	normalized := make([]string, 0, len(models))
	for _, modelName := range models {
		modelName = strings.TrimSpace(modelName)
		if modelName == "" {
			continue
		}
		if _, ok := seen[modelName]; ok {
			continue
		}
		seen[modelName] = struct{}{}
		normalized = append(normalized, modelName)
	}
	slices.Sort(normalized)
	return normalized
}

func normalizeGroup(group string, fallback string) string {
	source := group
	if strings.TrimSpace(source) == "" {
		source = fallback
	}
	if strings.TrimSpace(source) == "" {
		source = "default"
	}
	parts := strings.Split(source, ",")
	seen := make(map[string]struct{}, len(parts))
	groups := make([]string, 0, len(parts))
	for _, item := range parts {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		groups = append(groups, item)
	}
	if len(groups) == 0 {
		groups = []string{"default"}
	}
	slices.Sort(groups)
	return strings.Join(groups, ",")
}

func resolveAuthorizedGroup(userID int, requested string, fallback string) (string, error) {
	if userID <= 0 {
		return "", errors.New("invalid user id")
	}

	primaryGroup, err := model.CacheGetUserGroup(userID)
	if err != nil {
		return "", err
	}
	primaryGroup = normalizeGroup(primaryGroup, "default")

	allowedGroups := map[string]struct{}{
		primaryGroup: {},
	}

	grants, err := model.GetUserPrivateGroupGrantDetails(userID)
	if err != nil {
		return "", err
	}
	for _, grant := range grants {
		groupSlug := strings.TrimSpace(grant.GroupSlug)
		if groupSlug == "" {
			continue
		}
		allowedGroups[groupSlug] = struct{}{}
	}

	source := strings.TrimSpace(requested)
	if source == "" {
		source = strings.TrimSpace(fallback)
	}
	if source == "" {
		return primaryGroup, nil
	}

	group := normalizeGroup(source, primaryGroup)
	for _, item := range strings.Split(group, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := allowedGroups[item]; ok {
			continue
		}
		return "", &controlPlaneError{
			code:    "group_not_authorized",
			message: fmt.Sprintf("group %s is not authorized for current user", item),
		}
	}
	return group, nil
}

func normalizeNodeName(namespace string, nodeName string) string {
	nodeName = strings.TrimSpace(nodeName)
	if nodeName != "" {
		return nodeName
	}
	return strings.TrimSpace(namespace)
}

func normalizeBackendType(backendType string, fallback string) string {
	backendType = strings.TrimSpace(backendType)
	if backendType != "" {
		return backendType
	}
	return strings.TrimSpace(fallback)
}

func normalizeWorkerStatus(status int) int {
	switch status {
	case model.TokilakeWorkerNodeStatusBusy:
		return model.TokilakeWorkerNodeStatusBusy
	case model.TokilakeWorkerNodeStatusOffline:
		return model.TokilakeWorkerNodeStatusOffline
	default:
		return model.TokilakeWorkerNodeStatusOnline
	}
}

func channelStatusFromWorkerStatus(status int) int {
	if status == model.TokilakeWorkerNodeStatusOffline {
		return config.ChannelStatusAutoDisabled
	}
	return config.ChannelStatusEnabled
}

func tokiameChannelName(namespace string, nodeName string) string {
	nodeName = strings.TrimSpace(nodeName)
	if nodeName == "" || nodeName == strings.TrimSpace(namespace) {
		return fmt.Sprintf("%s/%s", tokiameNamePrefix, strings.TrimSpace(namespace))
	}
	return fmt.Sprintf("%s/%s (%s)", tokiameNamePrefix, strings.TrimSpace(namespace), nodeName)
}

func tokiameChannelBaseURL(namespace string) string {
	return fmt.Sprintf("tokiame://%s", strings.TrimSpace(namespace))
}
