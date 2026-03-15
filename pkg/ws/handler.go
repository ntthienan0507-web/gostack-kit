package ws

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// AuthFunc authenticates a WebSocket upgrade request and returns a client ID,
// optional metadata, and an error if authentication fails.
type AuthFunc func(c *gin.Context) (clientID string, metadata map[string]any, err error)

// UpgradeHandler returns a gin.HandlerFunc that upgrades HTTP connections to WebSocket
// and registers the client with the hub.
func UpgradeHandler(hub *Hub, logger *zap.Logger, auth AuthFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		clientID := ""
		var metadata map[string]any

		if auth != nil {
			var err error
			clientID, metadata, err = auth(c)
			if err != nil {
				logger.Warn("websocket auth failed", zap.Error(err))
				c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
				return
			}
		}

		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			logger.Error("websocket upgrade failed", zap.Error(err))
			return
		}

		client := newClient(clientID, conn, hub, logger)
		if metadata != nil {
			for k, v := range metadata {
				client.SetMeta(k, v)
			}
		}

		hub.register <- client

		go client.writePump()
		go client.readPump()
	}
}
