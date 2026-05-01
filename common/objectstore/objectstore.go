package objectstore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

const defaultPresignTTL = time.Hour

var (
	ErrNotConfigured = errors.New("object store is not configured")

	storeMu      sync.RWMutex
	currentStore Store
)

type Store interface {
	Provider() string
	BucketName() string
	PutObject(ctx context.Context, key string, body io.Reader, contentType string) (*Object, error)
	GetObjectURL(ctx context.Context, key string, ttl time.Duration) (string, error)
	PresignPutObject(ctx context.Context, key string, contentType string, ttl time.Duration) (*PresignedRequest, error)
}

type Object struct {
	Provider string
	Bucket   string
	Key      string
	URL      string
}

type PresignedRequest struct {
	Provider  string
	Bucket    string
	Key       string
	Method    string
	URL       string
	Headers   map[string]string
	ExpiresAt int64
}

func InitObjectStore() {
	storeMu.Lock()
	defer storeMu.Unlock()
	currentStore = newStoreFromConfig()
}

func Configured() bool {
	return defaultStore() != nil
}

func PutObject(ctx context.Context, key string, body io.Reader, contentType string) (*Object, error) {
	store := defaultStore()
	if store == nil {
		return nil, ErrNotConfigured
	}
	return store.PutObject(ctx, key, body, contentType)
}

func GetObjectURL(ctx context.Context, provider string, key string) (string, error) {
	store := defaultStore()
	if store == nil {
		return "", ErrNotConfigured
	}
	if provider = strings.TrimSpace(provider); provider != "" && !strings.EqualFold(provider, store.Provider()) {
		return "", fmt.Errorf("object store provider %s is not configured", provider)
	}
	return store.GetObjectURL(ctx, key, 0)
}

func PresignPutObject(ctx context.Context, key string, contentType string) (*PresignedRequest, error) {
	store := defaultStore()
	if store == nil {
		return nil, ErrNotConfigured
	}
	return store.PresignPutObject(ctx, key, contentType, 0)
}

func SetStoreForTest(store Store) func() {
	storeMu.Lock()
	previous := currentStore
	currentStore = store
	storeMu.Unlock()
	return func() {
		storeMu.Lock()
		currentStore = previous
		storeMu.Unlock()
	}
}

func defaultStore() Store {
	storeMu.RLock()
	store := currentStore
	storeMu.RUnlock()
	if store != nil {
		return store
	}
	InitObjectStore()
	storeMu.RLock()
	store = currentStore
	storeMu.RUnlock()
	return store
}
