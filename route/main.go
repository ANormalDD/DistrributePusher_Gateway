package route

import (
	"Gateway/internal/center"
	"Gateway/pkg/logger"
	"Gateway/pkg/middleware"
	"Gateway/user/ws"

	"github.com/gin-gonic/gin"
)

func InitRouter() *gin.Engine {
	r := gin.New()
	r.Use(logger.GinLogger(), logger.GinRecovery(true))
	User := r.Group("/api", middleware.JWTAuthMiddleware())
	{
		User.GET("/ws", ws.WebSocketHandler)
	}
	Center := r.Group("/center")
	{
		Center.POST("/forward", center.ForwardHandler)
	}
	return r
}
