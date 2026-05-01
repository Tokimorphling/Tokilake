package objectstore

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3CompatibleStore struct {
	config    S3CompatibleConfig
	client    *s3.Client
	presigner *s3.PresignClient
}

func NewS3CompatibleStore(config S3CompatibleConfig) *S3CompatibleStore {
	if strings.TrimSpace(config.Region) == "" {
		config.Region = "auto"
	}
	if config.PresignTTL <= 0 {
		config.PresignTTL = defaultPresignTTL
	}
	client := s3.New(s3.Options{
		Credentials: credentials.NewStaticCredentialsProvider(
			config.AccessKeyID,
			config.AccessKeySecret,
			config.SessionToken,
		),
		EndpointResolver: s3.EndpointResolverFromURL(config.Endpoint),
		Region:           config.Region,
		UsePathStyle:     config.ForcePathStyle,
	})
	return &S3CompatibleStore{
		config:    config,
		client:    client,
		presigner: s3.NewPresignClient(client),
	}
}

func (s *S3CompatibleStore) Provider() string {
	return firstNonEmpty(s.config.Provider, "s3_compatible")
}

func (s *S3CompatibleStore) BucketName() string {
	return s.config.BucketName
}

func (s *S3CompatibleStore) PutObject(ctx context.Context, key string, body io.Reader, contentType string) (*Object, error) {
	key = strings.TrimLeft(strings.TrimSpace(key), "/")
	if key == "" {
		return nil, fmt.Errorf("object key is required")
	}
	if body == nil {
		return nil, fmt.Errorf("object body is required")
	}
	input := &s3.PutObjectInput{
		Bucket: aws.String(s.config.BucketName),
		Key:    aws.String(key),
		Body:   body,
	}
	if contentType = strings.TrimSpace(contentType); contentType != "" {
		input.ContentType = aws.String(contentType)
	}
	if _, err := s.client.PutObject(ctx, input); err != nil {
		return nil, fmt.Errorf("put object: %w", err)
	}
	url, err := s.GetObjectURL(ctx, key, 0)
	if err != nil {
		return nil, err
	}
	return &Object{
		Provider: s.Provider(),
		Bucket:   s.config.BucketName,
		Key:      key,
		URL:      url,
	}, nil
}

func (s *S3CompatibleStore) GetObjectURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	key = strings.TrimLeft(strings.TrimSpace(key), "/")
	if key == "" {
		return "", fmt.Errorf("object key is required")
	}
	if baseURL := strings.TrimSpace(s.config.PublicBaseURL); baseURL != "" {
		return strings.TrimRight(baseURL, "/") + "/" + key, nil
	}
	request, err := s.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.config.BucketName),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(s.ttl(ttl)))
	if err != nil {
		return "", fmt.Errorf("presign get object: %w", err)
	}
	return request.URL, nil
}

func (s *S3CompatibleStore) PresignPutObject(ctx context.Context, key string, contentType string, ttl time.Duration) (*PresignedRequest, error) {
	key = strings.TrimLeft(strings.TrimSpace(key), "/")
	if key == "" {
		return nil, fmt.Errorf("object key is required")
	}
	input := &s3.PutObjectInput{
		Bucket: aws.String(s.config.BucketName),
		Key:    aws.String(key),
	}
	if contentType = strings.TrimSpace(contentType); contentType != "" {
		input.ContentType = aws.String(contentType)
	}
	expires := s.ttl(ttl)
	request, err := s.presigner.PresignPutObject(ctx, input, s3.WithPresignExpires(expires))
	if err != nil {
		return nil, fmt.Errorf("presign put object: %w", err)
	}
	headers := headerValues(request.SignedHeader)
	if contentType != "" {
		if headers == nil {
			headers = map[string]string{}
		}
		headers["Content-Type"] = contentType
	}
	return &PresignedRequest{
		Provider:  s.Provider(),
		Bucket:    s.config.BucketName,
		Key:       key,
		Method:    http.MethodPut,
		URL:       request.URL,
		Headers:   headers,
		ExpiresAt: time.Now().Add(expires).Unix(),
	}, nil
}

func (s *S3CompatibleStore) ttl(ttl time.Duration) time.Duration {
	if ttl > 0 {
		return ttl
	}
	if s.config.PresignTTL > 0 {
		return s.config.PresignTTL
	}
	return defaultPresignTTL
}

func headerValues(header http.Header) map[string]string {
	if len(header) == 0 {
		return nil
	}
	values := make(map[string]string, len(header))
	for key, list := range header {
		if len(list) == 0 {
			continue
		}
		values[key] = list[0]
	}
	return values
}
