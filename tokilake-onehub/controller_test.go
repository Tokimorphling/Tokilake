package tokilake_onehub

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveGatewayRemoteAddrPrefersClientIPBehindProxy(t *testing.T) {
	addr := resolveGatewayRemoteAddr("203.0.113.9", "127.0.0.1:44132")
	assert.Equal(t, "203.0.113.9", addr)
}

func TestResolveGatewayRemoteAddrPreservesDirectConnectionPort(t *testing.T) {
	addr := resolveGatewayRemoteAddr("203.0.113.9", "203.0.113.9:44132")
	assert.Equal(t, "203.0.113.9:44132", addr)
}

func TestResolveGatewayRemoteAddrFallsBackWhenClientIPMissing(t *testing.T) {
	addr := resolveGatewayRemoteAddr("", "127.0.0.1:44132")
	assert.Equal(t, "127.0.0.1:44132", addr)
}
