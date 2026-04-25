package tokilake_onehub

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"one-api/common/config"
	"one-api/middleware"
	"one-api/model"
	"one-api/providers"
	"one-api/relay/task"
	"one-api/router"

	"one-api/tokilake-onehub/gateway"
	hubprovider "one-api/tokilake-onehub/provider"
	hubtask "one-api/tokilake-onehub/task"

	tokilake "github.com/Tokimorphling/Tokilake/tokilake-core"
)

func InitGateway() func(ctx context.Context) (func(), error) {
	manager := tokilake.GetSessionManager()
	auth := NewHubAuthenticator()
	registry := NewHubWorkerRegistry(manager)
	logger := NewHubLogger()

	gateway.Global = tokilake.NewGateway(auth, registry, nil, logger, manager)

	providers.RegisterProvider(config.ChannelTypeTokiame, hubprovider.ProviderFactory{})
	task.RegisterTaskAdaptor(config.RelayModeVideos, model.TaskPlatformTokiameVideo, hubtask.TokiameVideoTaskFactory)

	router.RegisterPluginRouter(func(engine *gin.Engine) {
		tokilakeRouter := engine.Group("/api/tokilake")
		{
			tokilakeRouter.GET("/connect", TokilakeConnect)
		}
		relayV1Router := engine.Group("/v1")
		relayV1Router.Use(middleware.RelayPanicRecover(), middleware.OpenaiAuth(), middleware.Distribute(), middleware.DynamicRedisRateLimiter())
		{
			relayV1Router.GET("/videos", hubtask.ListVideos)
			relayV1Router.GET("/videos/:id/content", hubtask.GetVideoContent)
			relayV1Router.GET("/videos/:id", hubtask.GetVideoByID)
		}
	})

	return func(ctx context.Context) (func(), error) {
		quicPort := strings.TrimSpace(viper.GetString("quic.port"))
		if quicPort == "" {
			quicPort = strings.TrimSpace(viper.GetString("port"))
		}
		closeFn, err := gateway.Global.StartQUICGateway(ctx, tokilake.QUICGatewayConfig{
			Enable:   viper.GetBool("quic.enable"),
			Port:     quicPort,
			CertFile: viper.GetString("quic.cert_file"),
			KeyFile:  viper.GetString("quic.key_file"),
		})
		if err != nil {
			return nil, err
		}
		if closeFn == nil {
			return nil, nil
		}
		return func() { _ = closeFn() }, nil
	}
}

func Register(engine *gin.Engine) func(ctx context.Context) (func(), error) {
	return InitGateway()
}
