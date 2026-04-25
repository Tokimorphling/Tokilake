package task

import (
	"errors"
	"one-api/common/config"
	"one-api/model"
	"one-api/relay/task/base"
	"one-api/relay/task/kling"
	"one-api/relay/task/suno"

	"github.com/gin-gonic/gin"
)

type TaskAdaptorFactory func(c *gin.Context, platform string) base.TaskInterface

var taskAdaptorFactories = make(map[int]TaskAdaptorFactory)
var platformToRelayMode = make(map[string]int)
var relayModeToPlatform = make(map[int]string)

func RegisterTaskAdaptor(relayMode int, platform string, factory TaskAdaptorFactory) {
	taskAdaptorFactories[relayMode] = factory
	platformToRelayMode[platform] = relayMode
	relayModeToPlatform[relayMode] = platform
}

func GetTaskAdaptor(relayType int, c *gin.Context) (base.TaskInterface, error) {
	if factory, ok := taskAdaptorFactories[relayType]; ok {
		return factory(c, relayModeToPlatform[relayType]), nil
	}

	switch relayType {
	case config.RelayModeSuno:
		return &suno.SunoTask{
			TaskBase: GetTaskBase(c, model.TaskPlatformSuno),
		}, nil
	case config.RelayModeKling:
		return &kling.KlingTask{
			TaskBase: GetTaskBase(c, model.TaskPlatformKling),
		}, nil
	default:
		return nil, errors.New("adaptor not found")
	}
}

func GetTaskAdaptorByPlatform(platform string) (base.TaskInterface, error) {
	relayType := config.RelayModeUnknown

	if relayMode, ok := platformToRelayMode[platform]; ok {
		return GetTaskAdaptor(relayMode, nil)
	}

	switch platform {
	case model.TaskPlatformSuno:
		relayType = config.RelayModeSuno
	case model.TaskPlatformKling:
		relayType = config.RelayModeKling
	}

	return GetTaskAdaptor(relayType, nil)
}

func GetTaskBase(c *gin.Context, platform string) base.TaskBase {
	return base.TaskBase{
		Platform: platform,
		C:        c,
	}
}
