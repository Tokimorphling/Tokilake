package task

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"one-api/common"
	"one-api/model"
	"one-api/types"

	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"
)

func writeOpenAIError(c *gin.Context, errWithCode *types.OpenAIErrorWithStatusCode) {
	if c == nil || errWithCode == nil {
		return
	}
	c.JSON(errWithCode.StatusCode, types.OpenAIErrorResponse{
		Error: errWithCode.OpenAIError,
	})
}

func writeStringOpenAIError(c *gin.Context, statusCode int, code string, message string) {
	writeOpenAIError(c, common.StringErrorWrapperLocal(message, code, statusCode))
}

func normalizeVideoMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", types.VideoModeTextToVideo:
		return types.VideoModeTextToVideo
	case types.VideoModeImageToVideo:
		return types.VideoModeImageToVideo
	default:
		return ""
	}
}

func normalizeVideoStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case types.VideoStatusSubmitted:
		return types.VideoStatusSubmitted
	case "queued", "pending":
		return types.VideoStatusQueued
	case "processing", "running", "in_progress":
		return types.VideoStatusProcessing
	case "completed", "success", "succeeded":
		return types.VideoStatusCompleted
	case "failed", "failure", "error", "cancelled", "canceled":
		return types.VideoStatusFailed
	default:
		return ""
	}
}

func taskStatusToVideoStatus(status model.TaskStatus) string {
	switch status {
	case model.TaskStatusSubmitted:
		return types.VideoStatusSubmitted
	case model.TaskStatusQueued:
		return types.VideoStatusQueued
	case model.TaskStatusInProgress:
		return types.VideoStatusProcessing
	case model.TaskStatusSuccess:
		return types.VideoStatusCompleted
	case model.TaskStatusFailure:
		return types.VideoStatusFailed
	default:
		return types.VideoStatusSubmitted
	}
}

func videoStatusToTaskStatus(status string) model.TaskStatus {
	switch normalizeVideoStatus(status) {
	case types.VideoStatusSubmitted:
		return model.TaskStatusSubmitted
	case types.VideoStatusQueued:
		return model.TaskStatusQueued
	case types.VideoStatusProcessing:
		return model.TaskStatusInProgress
	case types.VideoStatusCompleted:
		return model.TaskStatusSuccess
	case types.VideoStatusFailed:
		return model.TaskStatusFailure
	default:
		return model.TaskStatusUnknown
	}
}

func videoStatusToProgress(status string) int {
	switch normalizeVideoStatus(status) {
	case types.VideoStatusSubmitted, types.VideoStatusQueued:
		return 10
	case types.VideoStatusProcessing:
		return 60
	case types.VideoStatusCompleted, types.VideoStatusFailed:
		return 100
	default:
		return 0
	}
}

func taskActionFromMode(mode string) string {
	if normalizeVideoMode(mode) == types.VideoModeImageToVideo {
		return "IMAGE2VIDEO"
	}
	return "TEXT2VIDEO"
}

func taskModeFromAction(action string) string {
	if strings.EqualFold(strings.TrimSpace(action), "IMAGE2VIDEO") {
		return types.VideoModeImageToVideo
	}
	return types.VideoModeTextToVideo
}

func videoContentPath(taskID string) string {
	return fmt.Sprintf("/v1/videos/%s/content", strings.TrimSpace(taskID))
}

func propertiesFromRequest(request *types.VideoRequest) *types.VideoTaskProperties {
	if request == nil {
		return &types.VideoTaskProperties{}
	}
	properties := &types.VideoTaskProperties{
		Model:    strings.TrimSpace(request.Model),
		Mode:     normalizeVideoMode(request.Mode),
		Prompt:   strings.TrimSpace(request.Prompt),
		Size:     strings.TrimSpace(request.Size),
		Duration: request.Duration,
		FPS:      request.FPS,
		Seed:     request.Seed,
	}
	if request.ImageURL != "" {
		properties.ImageSource = "image_url"
	}
	if request.ImageB64JSON != "" {
		properties.ImageSource = "image_b64_json"
		properties.HasImageB64 = true
	}
	if request.ReferenceURL != "" {
		properties.ImageSource = "reference_url"
	}
	if request.HasInputReference {
		properties.ImageSource = "input_reference"
	}
	return properties
}

func propertiesFromTask(task *model.Task) *types.VideoTaskProperties {
	properties := &types.VideoTaskProperties{}
	if task == nil || len(task.Properties) == 0 {
		return properties
	}
	_ = json.Unmarshal(task.Properties, properties)
	if properties.Mode == "" {
		properties.Mode = taskModeFromAction(task.Action)
	}
	return properties
}

func mergeVideoTaskObject(task *model.Task, response *types.VideoTaskObject) *types.VideoTaskObject {
	properties := propertiesFromTask(task)
	merged := &types.VideoTaskObject{}
	if response != nil {
		*merged = *response
	}
	if task != nil && strings.TrimSpace(merged.ID) == "" {
		merged.ID = strings.TrimSpace(task.TaskID)
	}
	if merged.Object == "" {
		merged.Object = "video"
	}
	if merged.Created == 0 && task != nil {
		merged.Created = task.SubmitTime
	}
	if merged.Model == "" {
		merged.Model = properties.Model
	}
	merged.Mode = normalizeVideoMode(firstNonEmptyString(merged.Mode, properties.Mode))
	if merged.Mode == "" {
		merged.Mode = types.VideoModeTextToVideo
	}
	status := normalizeVideoStatus(merged.Status)
	if status == "" && task != nil {
		status = taskStatusToVideoStatus(task.Status)
	}
	if status == "" {
		status = types.VideoStatusSubmitted
	}
	merged.Status = status
	if merged.Prompt == "" {
		merged.Prompt = properties.Prompt
	}
	if merged.Size == "" {
		merged.Size = properties.Size
	}
	if merged.Duration == nil {
		merged.Duration = properties.Duration
	}
	if merged.FPS == nil {
		merged.FPS = properties.FPS
	}
	if merged.Seed == nil {
		merged.Seed = properties.Seed
	}
	if merged.ID != "" {
		merged.ContentURL = videoContentPath(merged.ID)
		merged.DownloadURL = merged.ContentURL
	}
	if merged.Status == types.VideoStatusFailed && merged.Error == nil && task != nil && task.FailReason != "" {
		merged.Error = &types.VideoTaskError{
			Message: task.FailReason,
			Type:    "upstream_error",
			Code:    "video_failed",
		}
	}
	if merged.Status != types.VideoStatusFailed {
		merged.Error = nil
	}
	return merged
}

func videoTaskFromTask(task *model.Task) *types.VideoTaskObject {
	if task == nil {
		return &types.VideoTaskObject{Object: "video"}
	}
	response := &types.VideoTaskObject{}
	if len(task.Data) > 0 {
		_ = json.Unmarshal(task.Data, response)
	}
	return mergeVideoTaskObject(task, response)
}

func marshalTaskJSON(payload any) datatypes.JSON {
	if payload == nil {
		return datatypes.JSON([]byte("{}"))
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return datatypes.JSON([]byte("{}"))
	}
	return datatypes.JSON(data)
}

func applyVideoTaskState(task *model.Task, video *types.VideoTaskObject) {
	if task == nil || video == nil {
		return
	}

	now := time.Now().Unix()
	nextStatus := videoStatusToTaskStatus(video.Status)
	task.Status = nextStatus
	task.Progress = videoStatusToProgress(video.Status)

	if video.Created > 0 {
		task.SubmitTime = video.Created
	} else if task.SubmitTime == 0 {
		task.SubmitTime = now
	}

	switch nextStatus {
	case model.TaskStatusInProgress:
		if task.StartTime == 0 {
			task.StartTime = now
		}
	case model.TaskStatusSuccess, model.TaskStatusFailure:
		if task.StartTime == 0 {
			task.StartTime = now
		}
		if task.FinishTime == 0 {
			task.FinishTime = now
		}
	}

	if video.Error != nil {
		task.FailReason = strings.TrimSpace(video.Error.Message)
	} else if nextStatus != model.TaskStatusFailure {
		task.FailReason = ""
	}

	task.Properties = marshalTaskJSON(propertiesFromTask(task))
	task.Data = marshalTaskJSON(video)
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
