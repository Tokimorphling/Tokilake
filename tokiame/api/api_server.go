package api

import (
	"tokiame/pkg/config"
	"tokiame/pkg/log"

	"github.com/gin-gonic/gin"
)

func RunApiServer(addr string, cfgManager *config.Manager) {
	log.Infof("Running Tokiame API server on: %s", addr)
	r := gin.Default()
	r.Use(log.ToGinLogger(), log.ToGinRecovery(true))

	RegisterModelConfigAPI(r, cfgManager)

	if err := r.Run(addr); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}

}
