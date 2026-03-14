package main

import (
	"net"
	"net/http"
	"slices"
	"strings"
	"time"

	"one-api/common/logger"

	"github.com/gin-gonic/gin"
	"github.com/pires/go-proxyproto"
	"github.com/spf13/viper"
)

var defaultTrustedProxies = []string{"127.0.0.1", "::1"}

func buildRemoteIPHeaders(trustedHeader string) []string {
	headers := make([]string, 0, 3)
	for _, header := range []string{trustedHeader, "X-Forwarded-For", "X-Real-IP"} {
		header = strings.TrimSpace(header)
		if header == "" || slices.Contains(headers, header) {
			continue
		}
		headers = append(headers, header)
	}
	return headers
}

func normalizeTrustedProxyEntries(entries []string) []string {
	proxies := make([]string, 0, len(entries))
	for _, entry := range entries {
		for _, part := range strings.Split(entry, ",") {
			part = strings.TrimSpace(part)
			if part == "" || slices.Contains(proxies, part) {
				continue
			}
			proxies = append(proxies, part)
		}
	}
	return proxies
}

func parseTrustedProxies(proxyList []string, raw string, explicitlyConfigured bool) []string {
	proxies := normalizeTrustedProxyEntries(proxyList)
	if len(proxies) == 0 {
		proxies = normalizeTrustedProxyEntries([]string{raw})
	}

	if explicitlyConfigured {
		if len(proxies) == 1 && strings.EqualFold(proxies[0], "none") {
			return nil
		}
		return proxies
	}

	return append([]string(nil), defaultTrustedProxies...)
}

func configuredTrustedProxies() []string {
	explicitlyConfigured := viper.InConfig("trusted_proxies") || viper.IsSet("trusted_proxies")
	return parseTrustedProxies(viper.GetStringSlice("trusted_proxies"), viper.GetString("trusted_proxies"), explicitlyConfigured)
}

func applyServerProxyConfig(server *gin.Engine) error {
	server.RemoteIPHeaders = buildRemoteIPHeaders(viper.GetString("trusted_header"))
	return server.SetTrustedProxies(configuredTrustedProxies())
}

func proxyProtocolPolicy(trustedProxies []string) proxyproto.ConnPolicyFunc {
	if len(trustedProxies) == 0 {
		return func(proxyproto.ConnPolicyOptions) (proxyproto.Policy, error) {
			return proxyproto.IGNORE, nil
		}
	}
	return proxyproto.ConnMustLaxWhiteListPolicy(trustedProxies)
}

func runHTTPServer(server *gin.Engine, port string) error {
	addr := ":" + port
	if !viper.GetBool("proxy_protocol_enabled") {
		return server.Run(addr)
	}

	trustedProxies := configuredTrustedProxies()
	logger.SysLog("proxy protocol enabled")

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	proxyListener := &proxyproto.Listener{
		Listener:          listener,
		ConnPolicy:        proxyProtocolPolicy(trustedProxies),
		ReadHeaderTimeout: 5 * time.Second,
	}

	httpServer := &http.Server{
		Addr:    addr,
		Handler: server,
	}

	return httpServer.Serve(proxyListener)
}
