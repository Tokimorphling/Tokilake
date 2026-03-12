package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"one-api/pkg/log"
	"one-api/service/tokilake"
)

func main() {
	log.SetDefault(log.NewLogger(log.NewTerminalHandlerWithLevel(os.Stdout, tokiameLogLevel(), true)))

	config, err := tokilake.LoadClientConfigFromEnv()
	if err != nil {
		log.Crit("load tokiame config failed", "err", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Info("starting tokiame client",
		"namespace", config.Namespace,
		"node_name", config.NodeName,
		"group", config.Group,
		"gateway_url", config.GatewayURL,
		"models", config.ModelNames(),
		"backend_type", config.ControlPlaneBackendType(),
	)

	client := tokilake.NewClient(config)
	if err = client.Run(ctx); err != nil {
		log.Crit("tokiame exited with error", "err", err)
	}
}

func tokiameLogLevel() slog.Level {
	level := strings.ToLower(strings.TrimSpace(os.Getenv("TOKIAME_LOG_LEVEL")))
	if level == "" {
		level = strings.ToLower(strings.TrimSpace(os.Getenv("LOG_LEVEL")))
	}
	if level == "" {
		level = "info"
	}
	return log.StirngLevel(level)
}
