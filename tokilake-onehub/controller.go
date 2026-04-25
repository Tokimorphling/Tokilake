package tokilake_onehub

import (
	"fmt"
	"net"
	"net/http"
	"strings"

	"one-api/common/logger"
	"one-api/tokilake-onehub/gateway"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var tokilakeUpgrader = websocket.Upgrader{
	Subprotocols: []string{"tokilake.v1"},
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func resolveGatewayRemoteAddr(clientIP string, requestRemoteAddr string) string {
	clientIP = strings.TrimSpace(clientIP)
	requestRemoteAddr = strings.TrimSpace(requestRemoteAddr)
	if clientIP == "" {
		return requestRemoteAddr
	}
	host, _, err := net.SplitHostPort(requestRemoteAddr)
	if err == nil && host == clientIP {
		return requestRemoteAddr
	}
	return clientIP
}

func TokilakeConnect(c *gin.Context) {
	tokenKey, token, err := gateway.Global.AuthenticateConnectRequest(c.Request.Context(), c.Request)
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

	remoteAddr := resolveGatewayRemoteAddr(c.ClientIP(), c.Request.RemoteAddr)
	if err = gateway.Global.HandleGatewayConnection(c.Request.Context(), wsConn, token, tokenKey, remoteAddr); err != nil {
		logger.SysLog(fmt.Sprintf("tokilake gateway session closed: remote=%s err=%v", remoteAddr, err))
	}
}
