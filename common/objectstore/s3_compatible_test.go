package objectstore

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestS3CompatibleConfigFromViper(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	viper.Set("storage.object.provider", "alioss")
	viper.Set("storage.object.endpoint", "https://oss-cn-beijing.aliyuncs.com")
	viper.Set("storage.object.region", "cn-beijing")
	viper.Set("storage.object.bucketName", "video-bucket")
	viper.Set("storage.object.accessKeyId", "access-key")
	viper.Set("storage.object.accessKeySecret", "secret-key")
	viper.Set("storage.object.sessionToken", "session-token")
	viper.Set("storage.object.cdnurl", "https://cdn.example.com")
	viper.Set("storage.object.forcePathStyle", false)
	viper.Set("storage.object.presign_ttl_seconds", 120)

	config, ok := s3CompatibleConfigFromViper()
	require.True(t, ok)
	require.Equal(t, "alioss", config.Provider)
	require.Equal(t, "https://oss-cn-beijing.aliyuncs.com", config.Endpoint)
	require.Equal(t, "cn-beijing", config.Region)
	require.Equal(t, "video-bucket", config.BucketName)
	require.Equal(t, "access-key", config.AccessKeyID)
	require.Equal(t, "secret-key", config.AccessKeySecret)
	require.Equal(t, "session-token", config.SessionToken)
	require.Equal(t, "https://cdn.example.com", config.PublicBaseURL)
	require.False(t, config.ForcePathStyle)
	require.Equal(t, 120*time.Second, config.PresignTTL)
}

func TestS3CompatibleStorePublicURL(t *testing.T) {
	store := NewS3CompatibleStore(S3CompatibleConfig{
		Provider:        "r2",
		Endpoint:        "https://example.r2.cloudflarestorage.com",
		Region:          "auto",
		BucketName:      "video-bucket",
		AccessKeyID:     "access-key",
		AccessKeySecret: "secret-key",
		PublicBaseURL:   "https://cdn.example.com/media/",
		ForcePathStyle:  true,
	})

	url, err := store.GetObjectURL(context.Background(), "/videos/task.mp4", time.Minute)
	require.NoError(t, err)
	require.Equal(t, "https://cdn.example.com/media/videos/task.mp4", url)
}

func TestS3CompatibleStorePresignPutObject(t *testing.T) {
	store := NewS3CompatibleStore(S3CompatibleConfig{
		Provider:        "minio",
		Endpoint:        "https://storage.example.com",
		Region:          "us-east-1",
		BucketName:      "video-bucket",
		AccessKeyID:     "access-key",
		AccessKeySecret: "secret-key",
		ForcePathStyle:  true,
		PresignTTL:      time.Minute,
	})

	request, err := store.PresignPutObject(context.Background(), "videos/task.mp4", "video/mp4", time.Minute)
	require.NoError(t, err)
	require.Equal(t, "minio", request.Provider)
	require.Equal(t, "video-bucket", request.Bucket)
	require.Equal(t, "videos/task.mp4", request.Key)
	require.Equal(t, http.MethodPut, request.Method)
	require.True(t, strings.HasPrefix(request.URL, "https://storage.example.com/video-bucket/videos/task.mp4?"))
	require.Equal(t, "video/mp4", request.Headers["Content-Type"])
	require.Greater(t, request.ExpiresAt, time.Now().Unix())
}
