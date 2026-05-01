package objectstore

import (
	"strings"
	"time"

	"github.com/spf13/viper"
)

type S3CompatibleConfig struct {
	Provider        string
	Endpoint        string
	Region          string
	BucketName      string
	AccessKeyID     string
	AccessKeySecret string
	SessionToken    string
	PublicBaseURL   string
	ForcePathStyle  bool
	PresignTTL      time.Duration
}

func newStoreFromConfig() Store {
	config, ok := s3CompatibleConfigFromViper()
	if !ok {
		return nil
	}
	return NewS3CompatibleStore(config)
}

func s3CompatibleConfigFromViper() (S3CompatibleConfig, bool) {
	config := S3CompatibleConfig{
		Provider:        firstNonEmpty(viper.GetString("storage.object.provider"), "s3_compatible"),
		Endpoint:        strings.TrimSpace(viper.GetString("storage.object.endpoint")),
		Region:          firstNonEmpty(viper.GetString("storage.object.region"), "auto"),
		BucketName:      firstNonEmpty(viper.GetString("storage.object.bucketName"), viper.GetString("storage.object.bucket")),
		AccessKeyID:     strings.TrimSpace(viper.GetString("storage.object.accessKeyId")),
		AccessKeySecret: strings.TrimSpace(viper.GetString("storage.object.accessKeySecret")),
		SessionToken:    strings.TrimSpace(viper.GetString("storage.object.sessionToken")),
		PublicBaseURL:   firstNonEmpty(viper.GetString("storage.object.cdnurl"), viper.GetString("storage.object.publicBaseUrl")),
		ForcePathStyle:  viper.GetBool("storage.object.forcePathStyle"),
		PresignTTL:      configuredPresignTTL(),
	}

	if config.Endpoint == "" {
		config = legacyS3ConfigFromViper()
	}

	if config.Endpoint == "" || config.BucketName == "" || config.AccessKeyID == "" || config.AccessKeySecret == "" {
		return S3CompatibleConfig{}, false
	}
	return config, true
}

func legacyS3ConfigFromViper() S3CompatibleConfig {
	return S3CompatibleConfig{
		Provider:        "S3",
		Endpoint:        strings.TrimSpace(viper.GetString("storage.s3.endpoint")),
		Region:          "auto",
		BucketName:      strings.TrimSpace(viper.GetString("storage.s3.bucketName")),
		AccessKeyID:     strings.TrimSpace(viper.GetString("storage.s3.accessKeyId")),
		AccessKeySecret: strings.TrimSpace(viper.GetString("storage.s3.accessKeySecret")),
		PublicBaseURL:   strings.TrimSpace(viper.GetString("storage.s3.cdnurl")),
		ForcePathStyle:  true,
		PresignTTL:      configuredPresignTTL(),
	}
}

func configuredPresignTTL() time.Duration {
	seconds := viper.GetInt64("storage.object.presign_ttl_seconds")
	if seconds <= 0 {
		seconds = viper.GetInt64("storage.object.presignTTLSeconds")
	}
	if seconds <= 0 {
		return defaultPresignTTL
	}
	return time.Duration(seconds) * time.Second
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
