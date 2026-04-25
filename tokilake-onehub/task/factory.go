package task

import (
	"one-api/model"
	"one-api/relay/task"
	"one-api/relay/task/base"

	"github.com/gin-gonic/gin"
)

func TokiameVideoTaskFactory(c *gin.Context, platform string) base.TaskInterface {
	return &TokiameVideoTask{
		TaskBase: task.GetTaskBase(c, model.TaskPlatformTokiameVideo),
	}
}
