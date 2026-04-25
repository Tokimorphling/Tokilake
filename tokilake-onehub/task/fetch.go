package task

import (
	"io"
	"net/http"
	"strconv"
	"strings"

	"one-api/common"
	"one-api/model"
	"one-api/providers"
	providersbase "one-api/providers/base"
	"one-api/types"

	"github.com/gin-gonic/gin"
)

func ListVideos(c *gin.Context) {
	userID := c.GetInt("id")
	limit := 20
	if rawLimit := strings.TrimSpace(c.Query("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed <= 0 {
			writeStringOpenAIError(c, http.StatusBadRequest, "invalid_limit", "limit must be a positive integer")
			return
		}
		if parsed > 100 {
			parsed = 100
		}
		limit = parsed
	}

	statusFilter, errWithCode := parseVideoStatusFilter(c.Query("status"))
	if errWithCode != nil {
		writeOpenAIError(c, errWithCode)
		return
	}

	tasks, err := model.ListUserTasksByPlatform(userID, model.TaskPlatformTokiameVideo, statusFilter, limit)
	if err != nil {
		writeStringOpenAIError(c, http.StatusInternalServerError, "get_videos_failed", err.Error())
		return
	}

	items := make([]types.VideoTaskObject, 0, len(tasks))
	for _, task := range tasks {
		items = append(items, *videoTaskFromTask(task))
	}

	c.JSON(http.StatusOK, types.VideoListResponse{
		Object: "list",
		Data:   items,
	})
}

func GetVideoByID(c *gin.Context) {
	taskID := strings.TrimSpace(c.Param("id"))
	userID := c.GetInt("id")

	task, err := model.GetTaskByTaskId(model.TaskPlatformTokiameVideo, userID, taskID)
	if err != nil {
		writeStringOpenAIError(c, http.StatusInternalServerError, "get_video_failed", err.Error())
		return
	}
	if task == nil {
		writeStringOpenAIError(c, http.StatusNotFound, "video_not_found", "video not found")
		return
	}

	c.JSON(http.StatusOK, videoTaskFromTask(task))
}

func GetVideoContent(c *gin.Context) {
	taskID := strings.TrimSpace(c.Param("id"))
	userID := c.GetInt("id")

	task, err := model.GetTaskByTaskId(model.TaskPlatformTokiameVideo, userID, taskID)
	if err != nil {
		writeStringOpenAIError(c, http.StatusInternalServerError, "get_video_failed", err.Error())
		return
	}
	if task == nil {
		writeStringOpenAIError(c, http.StatusNotFound, "video_not_found", "video not found")
		return
	}

	switch task.Status {
	case model.TaskStatusFailure:
		message := task.FailReason
		if strings.TrimSpace(message) == "" {
			message = "video generation failed"
		}
		writeStringOpenAIError(c, http.StatusBadGateway, "video_failed", message)
		return
	case model.TaskStatusSuccess:
	default:
		writeStringOpenAIError(c, http.StatusConflict, "video_not_ready", "video is not ready yet")
		return
	}

	channel, err := model.GetChannelById(task.ChannelId)
	if err != nil {
		writeStringOpenAIError(c, http.StatusServiceUnavailable, "channel_not_found", "channel not found")
		return
	}
	provider := providers.GetProvider(channel, c)
	videoProvider, ok := provider.(providersbase.VideoInterface)
	if !ok {
		writeStringOpenAIError(c, http.StatusServiceUnavailable, "provider_not_found", "video provider not found")
		return
	}

	videoProvider.SetOriginalModel(propertiesFromTask(task).Model)
	resp, errWithCode := videoProvider.GetVideoContent(task.TaskID)
	if errWithCode != nil {
		writeOpenAIError(c, errWithCode)
		return
	}
	defer resp.Body.Close()

	if contentType := resp.Header.Get("Content-Type"); contentType != "" {
		c.Writer.Header().Set("Content-Type", contentType)
	}
	if contentLength := resp.Header.Get("Content-Length"); contentLength != "" {
		c.Writer.Header().Set("Content-Length", contentLength)
	}
	c.Writer.WriteHeader(resp.StatusCode)
	if _, err = io.Copy(c.Writer, resp.Body); err != nil {
		common.AbortWithErr(c, http.StatusInternalServerError, err)
		return
	}
}

func parseVideoStatusFilter(raw string) (*model.TaskStatus, *types.OpenAIErrorWithStatusCode) {
	normalized := normalizeVideoStatus(raw)
	if strings.TrimSpace(raw) != "" && normalized == "" {
		return nil, common.StringErrorWrapperLocal("invalid video status", "invalid_status", http.StatusBadRequest)
	}
	if normalized == "" {
		return nil, nil
	}

	status := videoStatusToTaskStatus(normalized)
	return &status, nil
}
