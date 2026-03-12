package controller

import (
	"fmt"
	"net/http"

	"one-api/common/logger"
	tokilakesvc "one-api/service/tokilake"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var tokilakeUpgrader = websocket.Upgrader{
	Subprotocols: []string{"tokilake.v1"},
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func TokilakeConnect(c *gin.Context) {
	tokenKey, token, err := tokilakesvc.AuthenticateConnectRequest(c.Request)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	wsConn, err := tokilakeUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": fmt.Sprintf("websocket upgrade failed: %v", err),
		})
		return
	}
	defer wsConn.Close()

	if err = tokilakesvc.HandleGatewayConnection(c.Request.Context(), wsConn, token, tokenKey, c.Request.RemoteAddr); err != nil {
		logger.SysLog(fmt.Sprintf("tokilake gateway session closed: remote=%s err=%v", c.Request.RemoteAddr, err))
	}
}
