package tokilake

import (
	"context"
)

type Token struct {
	UserId int
}

type RegisterResult struct {
	WorkerID    int
	ChannelID   int
	Namespace   string
	Group       string
	Models      []string
	BackendType string
	Status      int
}

type Authenticator interface {
	AuthenticateTokenKey(ctx context.Context, tokenKey string) (string, *Token, error)
}

type WorkerRegistry interface {
	RegisterWorker(ctx context.Context, session *GatewaySession, register *RegisterMessage) (*RegisterResult, error)
	UpdateHeartbeat(ctx context.Context, session *GatewaySession, heartbeat *HeartbeatMessage) error
	SyncModels(ctx context.Context, session *GatewaySession, modelsSync *ModelsSyncMessage) error
	CleanupWorker(ctx context.Context, session *GatewaySession) error
}

type RouteResolver interface {
	ResolveRoute(ctx context.Context, namespace string) error
}

type Logger interface {
	SysLog(msg string)
	SysError(msg string)
	FatalLog(msg string)
}

var DefaultLogger Logger = &nopLogger{}

type nopLogger struct{}
func (l *nopLogger) SysLog(msg string) {}
func (l *nopLogger) SysError(msg string) {}
func (l *nopLogger) FatalLog(msg string) {}
