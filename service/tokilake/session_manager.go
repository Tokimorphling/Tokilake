package tokilake

import (
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"one-api/model"

	"github.com/xtaci/smux"
)

var errNamespaceAlreadyConnected = errors.New("namespace already connected")

type InFlightRequest struct {
	RequestID string
	SessionID uint64
	Namespace string
	ChannelID int
	CreatedAt time.Time
	Cancel    context.CancelFunc
}

type GatewaySession struct {
	ID          uint64
	Token       *model.Token
	TokenKey    string
	RemoteAddr  string
	ConnectedAt time.Time
	WorkerID    int
	ChannelID   int
	Namespace   string
	Group       string
	BackendType string
	Models      []string
	Status      int
	SMux        *smux.Session
	Control     io.ReadWriteCloser

	controlCodec *controlCodec
	closeOnce    sync.Once
}

func (s *GatewaySession) Close() error {
	var err error
	s.closeOnce.Do(func() {
		if s.SMux != nil {
			err = s.SMux.Close()
		}
	})
	if errors.Is(err, io.ErrClosedPipe) {
		return nil
	}
	return err
}

type SessionManager struct {
	mu          sync.RWMutex
	nextID      atomic.Uint64
	byNamespace map[string]*GatewaySession
	byChannelID map[int]*GatewaySession

	requestMu sync.Mutex
	requests  map[string]*InFlightRequest
}

func NewSessionManager() *SessionManager {
	return &SessionManager{
		byNamespace: make(map[string]*GatewaySession),
		byChannelID: make(map[int]*GatewaySession),
		requests:    make(map[string]*InFlightRequest),
	}
}

func (m *SessionManager) NewGatewaySession(token *model.Token, tokenKey string, remoteAddr string, smuxSession *smux.Session) *GatewaySession {
	return &GatewaySession{
		ID:          m.nextID.Add(1),
		Token:       token,
		TokenKey:    tokenKey,
		RemoteAddr:  remoteAddr,
		ConnectedAt: time.Now(),
		SMux:        smuxSession,
	}
}

func (m *SessionManager) ClaimNamespace(session *GatewaySession, namespace string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if existing, ok := m.byNamespace[namespace]; ok && existing != session {
		return errNamespaceAlreadyConnected
	}

	session.Namespace = namespace
	m.byNamespace[namespace] = session
	return nil
}

func (m *SessionManager) BindChannel(session *GatewaySession, workerID int, channelID int, group string, models []string, backendType string, status int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session.WorkerID = workerID
	if session.ChannelID != 0 && session.ChannelID != channelID {
		delete(m.byChannelID, session.ChannelID)
	}
	session.ChannelID = channelID
	session.Group = group
	session.Models = append([]string(nil), models...)
	session.BackendType = backendType
	session.Status = status
	m.byChannelID[channelID] = session
}

func (m *SessionManager) Release(session *GatewaySession) {
	m.mu.Lock()
	if session.Namespace != "" {
		if current, ok := m.byNamespace[session.Namespace]; ok && current == session {
			delete(m.byNamespace, session.Namespace)
		}
	}
	if session.ChannelID != 0 {
		if current, ok := m.byChannelID[session.ChannelID]; ok && current == session {
			delete(m.byChannelID, session.ChannelID)
		}
	}
	m.mu.Unlock()

	m.cancelRequestsForSession(session.ID)
}

func (m *SessionManager) GetSessionByNamespace(namespace string) (*GatewaySession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	session, ok := m.byNamespace[namespace]
	return session, ok
}

func (m *SessionManager) GetSessionByChannelID(channelID int) (*GatewaySession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	session, ok := m.byChannelID[channelID]
	return session, ok
}

func (m *SessionManager) TrackRequest(request *InFlightRequest) {
	if request == nil || request.RequestID == "" {
		return
	}
	m.requestMu.Lock()
	defer m.requestMu.Unlock()
	m.requests[request.RequestID] = request
}

func (m *SessionManager) RemoveRequest(requestID string) {
	if requestID == "" {
		return
	}
	m.requestMu.Lock()
	defer m.requestMu.Unlock()
	delete(m.requests, requestID)
}

func (m *SessionManager) GetRequest(requestID string) (*InFlightRequest, bool) {
	if requestID == "" {
		return nil, false
	}
	m.requestMu.Lock()
	defer m.requestMu.Unlock()
	request, ok := m.requests[requestID]
	return request, ok
}

func (m *SessionManager) cancelRequestsForSession(sessionID uint64) {
	m.requestMu.Lock()
	defer m.requestMu.Unlock()

	for requestID, request := range m.requests {
		if request.SessionID != sessionID {
			continue
		}
		if request.Cancel != nil {
			request.Cancel()
		}
		delete(m.requests, requestID)
	}
}

var defaultSessionManager = NewSessionManager()

func GetSessionManager() *SessionManager {
	return defaultSessionManager
}
