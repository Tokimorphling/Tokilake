package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"tokiame/api"
	"tokiame/pkg/cli"
	"tokiame/pkg/config"
	"tokiame/pkg/log"
	"tokiame/pkg/log/zapadaptor"
	"tokiame/pkg/rpc"

	"go.uber.org/zap"
)

func main() {
	log.SetLogger(zapadaptor.NewConsoleLogger(zapadaptor.WithZapOptions(zap.AddCaller(), zap.AddCallerSkip(3))))
	cli.Execute()
	arg := cli.GetArgs()

	namespace := arg.Namespace
	serverAddress := arg.Address
	apiAddress := arg.ApiAddress

	log.Infof("client namespace: %s tokilake address: %s", namespace, serverAddress)

	modelManager, err := config.NewManager("models.toml")
	if err != nil {
		log.Fatal("no model config file")
		panic("no model config")
	}

	go api.RunApiServer(apiAddress, modelManager)

	// // Create a main context that can be cancelled by SIGINT/SIGTERM
	mainCtx, mainCancel := context.WithCancel(context.Background())
	defer mainCancel()

	client := rpc.NewTokiameClient(mainCtx, serverAddress, namespace, modelManager)

	// // Handle OS signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		log.Infof("Received signal: %s. Shutting down...", sig)
		mainCancel()
	}()

	log.Infof("Starting Tokiame client: %s", namespace)

	go func() {
		if err := client.Run(); err != nil {
			log.Errorf("tokiame client run error: %v", err)

		}
		log.Infof("TokiameClient Run returned.")
	}()

	<-mainCtx.Done()
	log.Infof("Client [%s] performing final cleanup...", namespace)
	client.Close() // Ensure all resources are released
	log.Infof("Client [%s] shut down gracefully.", namespace)

}
