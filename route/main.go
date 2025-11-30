package route

import (
	"Gateway/internal/center"
	"Gateway/pkg/logger"
	"Gateway/pkg/middleware"
	"Gateway/pkg/monitor"
	"Gateway/user/ws"

	"github.com/gin-gonic/gin"
)

func InitRouter() *gin.Engine {
	r := gin.New()
	r.Use(logger.GinLogger(), logger.GinRecovery(true))
	// metrics endpoint for Prometheus
	r.GET("/metrics", gin.WrapH(monitor.Handler()))
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
