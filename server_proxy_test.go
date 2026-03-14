package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildRemoteIPHeaders(t *testing.T) {
	headers := buildRemoteIPHeaders("CF-Connecting-IP")
	assert.Equal(t, []string{"CF-Connecting-IP", "X-Forwarded-For", "X-Real-IP"}, headers)
}

func TestBuildRemoteIPHeadersDeduplicatesKnownHeaders(t *testing.T) {
	headers := buildRemoteIPHeaders("X-Real-IP")
	assert.Equal(t, []string{"X-Real-IP", "X-Forwarded-For"}, headers)
}

func TestParseTrustedProxiesUsesSafeDefaults(t *testing.T) {
	proxies := parseTrustedProxies(nil, "", false)
	assert.Equal(t, []string{"127.0.0.1", "::1"}, proxies)
}

func TestParseTrustedProxiesAcceptsCommaSeparatedValues(t *testing.T) {
	proxies := parseTrustedProxies(nil, "127.0.0.1, 10.0.0.0/8", true)
	assert.Equal(t, []string{"127.0.0.1", "10.0.0.0/8"}, proxies)
}

func TestParseTrustedProxiesSupportsDisableSentinel(t *testing.T) {
	proxies := parseTrustedProxies(nil, "none", true)
	assert.Nil(t, proxies)
}
