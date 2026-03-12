package router

import (
	"one-api/controller"

	"github.com/gin-gonic/gin"
)

func SetTokilakeRouter(router *gin.Engine) {
	tokilakeRouter := router.Group("/api/tokilake")
	{
		tokilakeRouter.GET("/connect", controller.TokilakeConnect)
	}
}
