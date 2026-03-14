package tokiamevideo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"one-api/common"
	"one-api/common/config"
	"one-api/common/logger"
	"one-api/model"
	"one-api/providers"
	providersbase "one-api/providers/base"
	"one-api/relay/task/base"
	"one-api/types"

	"github.com/gin-gonic/gin"
)

type TokiameVideoTask struct {
	base.TaskBase
	Request  *types.VideoRequest
	Provider providersbase.VideoInterface
}

func (t *TokiameVideoTask) HandleError(err *base.TaskError) {
	if err == nil {
		return
	}
	writeStringOpenAIError(t.C, err.StatusCode, err.Code, err.Message)
}

func (t *TokiameVideoTask) Init() *base.TaskError {
	if err := common.UnmarshalBodyReusable(t.C, &t.Request); err != nil {
		return base.StringTaskError(http.StatusBadRequest, "invalid_request", err.Error(), true)
	}
	if t.Request == nil {
		return base.StringTaskError(http.StatusBadRequest, "invalid_request", "request is required", true)
	}

	t.Request.Model = strings.TrimSpace(t.Request.Model)
	t.Request.Mode = normalizeVideoMode(t.Request.Mode)
	if t.Request.Mode == "" {
		return base.StringTaskError(http.StatusBadRequest, "invalid_request", "mode must be text2video or image2video", true)
	}
	t.Request.Prompt = strings.TrimSpace(t.Request.Prompt)
	t.Request.ImageURL = strings.TrimSpace(t.Request.ImageURL)
	t.Request.ImageB64JSON = strings.TrimSpace(t.Request.ImageB64JSON)
	t.Request.Size = strings.TrimSpace(t.Request.Size)
	if t.Request.Model == "" {
		return base.StringTaskError(http.StatusBadRequest, "invalid_request", "model is required", true)
	}

	if t.Request.N == nil {
		defaultN := 1
		t.Request.N = &defaultN
	}
	if *t.Request.N != 1 {
		return base.StringTaskError(http.StatusBadRequest, "invalid_request", "n must be 1", true)
	}

	switch t.Request.Mode {
	case types.VideoModeTextToVideo:
		if t.Request.Prompt == "" {
			return base.StringTaskError(http.StatusBadRequest, "invalid_request", "prompt is required for text2video", true)
		}
		if t.Request.ImageURL != "" || t.Request.ImageB64JSON != "" {
			return base.StringTaskError(http.StatusBadRequest, "invalid_request", "text2video does not support image inputs", true)
		}
	case types.VideoModeImageToVideo:
		hasImageURL := t.Request.ImageURL != ""
		hasImageB64 := t.Request.ImageB64JSON != ""
		if hasImageURL == hasImageB64 {
			return base.StringTaskError(http.StatusBadRequest, "invalid_request", "image2video requires exactly one of image_url or image_b64_json", true)
		}
	}

	t.OriginalModel = t.Request.Model
	return nil
}

func (t *TokiameVideoTask) SetProvider() *base.TaskError {
	t.C.Set("allow_channel_type", []int{config.ChannelTypeTokiame})

	provider, err := t.GetProviderByModel()
	if err != nil {
		return base.StringTaskError(http.StatusServiceUnavailable, "provider_not_found", err.Error(), true)
	}

	videoProvider, ok := provider.(providersbase.VideoInterface)
	if !ok {
		return base.StringTaskError(http.StatusServiceUnavailable, "provider_not_found", "provider not found", true)
	}

	t.Provider = videoProvider
	t.BaseProvider = provider
	return nil
}

func (t *TokiameVideoTask) Relay() *base.TaskError {
	response, errWithCode := t.Provider.CreateVideo(t.Request)
	if errWithCode != nil {
		return base.OpenAIErrToTaskErr(errWithCode)
	}
	if response == nil || strings.TrimSpace(response.ID) == "" {
		return base.StringTaskError(http.StatusBadGateway, "invalid_video_response", "video id is required", false)
	}

	t.InitTask()
	t.Task.TaskID = strings.TrimSpace(response.ID)
	t.Task.ChannelId = t.Provider.GetChannel().Id
	t.Task.Action = taskActionFromMode(t.Request.Mode)
	t.Task.Properties = marshalTaskJSON(propertiesFromRequest(t.Request))

	merged := mergeVideoTaskObject(t.Task, response)
	t.Task.Properties = marshalTaskJSON(propertiesFromRequest(t.Request))
	applyVideoTaskState(t.Task, merged)
	t.Response = merged
	return nil
}

func (t *TokiameVideoTask) ShouldRetry(_ *gin.Context, _ *base.TaskError) bool {
	return false
}

func (t *TokiameVideoTask) UpdateTaskStatus(ctx context.Context, taskChannelM map[int][]string, taskM map[string]*model.Task) error {
	for channelID, taskIDs := range taskChannelM {
		if err := updateTokiameVideoTasks(ctx, channelID, taskIDs, taskM); err != nil {
			logger.LogError(ctx, fmt.Sprintf("渠道 #%d 更新视频任务失败: %s", channelID, err.Error()))
		}
	}
	return nil
}

func updateTokiameVideoTasks(ctx context.Context, channelID int, taskIDs []string, taskM map[string]*model.Task) error {
	if len(taskIDs) == 0 {
		return nil
	}

	channel := model.ChannelGroup.GetChannel(channelID)
	if channel == nil {
		err := model.TaskBulkUpdate(taskIDs, map[string]any{
			"fail_reason": "获取渠道信息失败，请联系管理员",
			"status":      model.TaskStatusFailure,
			"progress":    100,
		})
		if err != nil {
			logger.SysError(fmt.Sprintf("UpdateTask error: %v", err))
		}
		return fmt.Errorf("channel not found")
	}

	provider := providers.GetProvider(channel, nil)
	videoProvider, ok := provider.(providersbase.VideoInterface)
	if !ok {
		err := model.TaskBulkUpdate(taskIDs, map[string]any{
			"fail_reason": "获取视频供应商失败，请联系管理员",
			"status":      model.TaskStatusFailure,
			"progress":    100,
		})
		if err != nil {
			logger.SysError(fmt.Sprintf("UpdateTask error: %v", err))
		}
		return fmt.Errorf("video provider not found")
	}

	for _, taskID := range taskIDs {
		task := taskM[taskID]
		if task == nil {
			continue
		}

		properties := propertiesFromTask(task)
		videoProvider.SetOriginalModel(properties.Model)
		response, errWithCode := videoProvider.GetVideo(taskID)
		if errWithCode != nil {
			logger.LogError(ctx, fmt.Sprintf("Get video task %s error: %s", taskID, errWithCode.Message))
			continue
		}

		merged := mergeVideoTaskObject(task, response)
		if !videoTaskNeedUpdate(task, merged) {
			continue
		}

		wasTerminal := task.Status == model.TaskStatusFailure || task.Status == model.TaskStatusSuccess
		applyVideoTaskState(task, merged)
		if task.Status == model.TaskStatusFailure && !wasTerminal {
			logger.LogError(ctx, task.TaskID+" 构建失败，"+task.FailReason)
			quota := task.Quota
			if quota > 0 {
				if err := model.IncreaseUserQuota(task.UserId, quota); err != nil {
					logger.LogError(ctx, "fail to increase user quota: "+err.Error())
				}
				logContent := fmt.Sprintf("异步任务执行失败 %s，补偿 %s", task.TaskID, common.LogQuota(quota))
				model.RecordLog(task.UserId, model.LogTypeSystem, logContent)
			}
		}

		if err := task.Update(); err != nil {
			logger.SysError("UpdateTask task error: " + err.Error())
		}
	}
	return nil
}

func videoTaskNeedUpdate(task *model.Task, video *types.VideoTaskObject) bool {
	if task == nil || video == nil {
		return false
	}
	if task.Status != videoStatusToTaskStatus(video.Status) {
		return true
	}
	if task.Progress != videoStatusToProgress(video.Status) {
		return true
	}
	failReason := ""
	if video.Error != nil {
		failReason = strings.TrimSpace(video.Error.Message)
	}
	if task.FailReason != failReason {
		return true
	}
	newData, _ := json.Marshal(video)
	return string(task.Data) != string(newData)
}
