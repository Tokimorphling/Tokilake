package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"tokiame/internal/utils"
	"tokiame/pkg/cli"
	"tokiame/pkg/config"
	"tokiame/pkg/log"
	"tokiame/pkg/log/zapadaptor"
	"tokiame/pkg/rpc"

	"go.uber.org/zap"
)

func main() {
	log.SetLogger(zapadaptor.NewConsoleLogger(zapadaptor.WithZapOptions(zap.AddCaller(), zap.AddCallerSkip(3))))
	arg := cli.Parse()
	namespace := arg.Namespace
	serverAddress := arg.Address

	utils.Print()

	log.Infof("client namespace: %s Tokilake address: %s", namespace, serverAddress)

	modelManager, err := config.NewManager("models.toml")
	if err != nil {
		log.Fatal("no model config file")
		panic("no model config")
	}
	supportedModels := modelManager.Get().SupportedModels

	// clientUUID := uuid.New().String()
	// clientID := fmt.Sprintf("go-tokiame-%s", clientUUID[:8])

	client := rpc.NewTokiameClient(namespace, supportedModels)

	// // Create a main context that can be cancelled by SIGINT/SIGTERM
	mainCtx, mainCancel := context.WithCancel(context.Background())
	defer mainCancel()

	// // Handle OS signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		log.Infof("Received signal: %s. Shutting down...", sig)
		mainCancel()
	}()

	log.Infof("Starting Tokiame client: %s", namespace)
	err = client.Run(mainCtx, serverAddress)
	if err != nil {
		log.Fatalf("Client run failed: %v", err)
	}

	log.Infof("Client [%s] performing final cleanup...", namespace)
	client.Close() // Ensure all resources are released
	log.Infof("Client [%s] shut down gracefully.", namespace)
}
