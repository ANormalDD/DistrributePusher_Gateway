package route
import (
	"github.com/gin-gonic/gin"
	"Gateway/pkg/middleware"
	"Gateway/pkg/logger"
	"Gateway/user/ws"

)
func InitRouter() *gin.Engine {
	r := gin.New()
	r.Use(logger.GinLogger(), logger.GinRecovery(true))
	user := r.Group("/user", middleware.JWTAuthMiddleware())
	{
		user.GET("/ws", ws.WebSocketHandler)
	}
	return r
}