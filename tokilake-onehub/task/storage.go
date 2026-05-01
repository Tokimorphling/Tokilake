package task

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"strings"

	"one-api/common/logger"
	"one-api/common/objectstore"
	"one-api/model"
	providersbase "one-api/providers/base"
	"one-api/types"

	"github.com/spf13/viper"
)

const defaultVideoStorageMaxSizeMB int64 = 1024

func videoObjectStorageEnabled() bool {
	return viper.GetBool("storage.video.enabled")
}

func videoStorageMaxBytes() int64 {
	maxSizeMB := viper.GetInt64("storage.video.max_size_mb")
	if maxSizeMB <= 0 {
		maxSizeMB = defaultVideoStorageMaxSizeMB
	}
	return maxSizeMB * 1024 * 1024
}

func videoStoragePrefix() string {
	prefix := strings.Trim(strings.TrimSpace(viper.GetString("storage.video.prefix")), "/")
	if prefix == "" {
		return "videos"
	}
	return prefix
}

func persistCompletedVideoToObjectStorage(ctx context.Context, provider providersbase.VideoInterface, task *model.Task, video *types.VideoTaskObject) {
	if !videoObjectStorageEnabled() || provider == nil || task == nil || video == nil {
		return
	}
	if normalizeVideoStatus(video.Status) != types.VideoStatusCompleted || videoStorageDownloadURL(video) != "" {
		return
	}

	resp, errWithCode := provider.GetVideoContent(task.TaskID)
	if errWithCode != nil {
		logger.LogError(ctx, fmt.Sprintf("Get video content %s for object storage failed: %s", task.TaskID, errWithCode.Message))
		return
	}
	defer resp.Body.Close()

	reader, contentType, extension, err := readVideoContentForStorage(resp)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("Read video content %s for object storage failed: %s", task.TaskID, err.Error()))
		return
	}

	objectKey := videoStorageObjectKey(task.TaskID, extension)
	result, err := objectstore.PutObject(ctx, objectKey, reader, contentType)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("Upload video content %s to object storage failed: %s", task.TaskID, err.Error()))
		return
	}

	video.Storage = &types.VideoStorage{
		Provider: result.Provider,
		Bucket:   result.Bucket,
		Key:      result.Key,
		URL:      result.URL,
	}
	video.DownloadURL = result.URL
}

func readVideoContentForStorage(resp *http.Response) (io.Reader, string, string, error) {
	if resp == nil || resp.Body == nil {
		return nil, "", "", fmt.Errorf("empty video content response")
	}
	maxBytes := videoStorageMaxBytes()
	if resp.ContentLength == 0 {
		return nil, "", "", fmt.Errorf("video content is empty")
	}
	if maxBytes > 0 && resp.ContentLength > maxBytes {
		return nil, "", "", fmt.Errorf("video content is too large: %d bytes exceeds %d bytes", resp.ContentLength, maxBytes)
	}

	firstByte := make([]byte, 1)
	n, err := resp.Body.Read(firstByte)
	if err != nil && err != io.EOF {
		return nil, "", "", err
	}
	if n == 0 {
		return nil, "", "", fmt.Errorf("video content is empty")
	}

	reader := io.MultiReader(bytes.NewReader(firstByte[:n]), resp.Body)
	if maxBytes > 0 {
		reader = &videoStorageLimitReader{
			reader:   reader,
			maxBytes: maxBytes,
		}
	}

	contentType := resp.Header.Get("Content-Type")
	return reader, contentType, videoFileExtension(contentType), nil
}

type videoStorageLimitReader struct {
	reader    io.Reader
	maxBytes  int64
	readBytes int64
}

func (r *videoStorageLimitReader) Read(p []byte) (int, error) {
	if r.maxBytes <= 0 {
		return r.reader.Read(p)
	}
	remaining := r.maxBytes - r.readBytes
	if remaining <= 0 {
		var extra [1]byte
		n, err := r.reader.Read(extra[:])
		if n > 0 {
			return 0, fmt.Errorf("video content is too large: exceeds %d bytes", r.maxBytes)
		}
		return 0, err
	}
	if int64(len(p)) > remaining {
		p = p[:remaining]
	}
	n, err := r.reader.Read(p)
	r.readBytes += int64(n)
	return n, err
}

func videoFileExtension(contentType string) string {
	contentType = strings.TrimSpace(strings.Split(contentType, ";")[0])
	switch strings.ToLower(contentType) {
	case "video/mp4":
		return ".mp4"
	case "video/webm":
		return ".webm"
	case "video/quicktime":
		return ".mov"
	}
	if contentType != "" {
		if extensions, err := mime.ExtensionsByType(contentType); err == nil && len(extensions) > 0 {
			return extensions[0]
		}
	}
	return ".mp4"
}

func videoStorageObjectKey(taskID string, extension string) string {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		taskID = "video"
	}
	extension = strings.TrimSpace(extension)
	if extension == "" {
		extension = ".mp4"
	}
	if !strings.HasPrefix(extension, ".") {
		extension = "." + extension
	}
	return path.Join(videoStoragePrefix(), taskID+extension)
}
