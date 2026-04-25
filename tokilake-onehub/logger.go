package tokilake_onehub

import (
	"one-api/common/logger"
)

type HubLogger struct{}

func NewHubLogger() *HubLogger {
	return &HubLogger{}
}

func (l *HubLogger) SysLog(msg string) {
	logger.SysLog(msg)
}

func (l *HubLogger) SysError(msg string) {
	logger.SysError(msg)
}

func (l *HubLogger) FatalLog(msg string) {
	logger.FatalLog(msg)
}
