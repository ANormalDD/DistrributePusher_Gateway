package middleware

import (
	"strings"

	"Gateway/pkg/response"
	"Gateway/pkg/utils"

	"github.com/gin-gonic/gin"
)

func JWTAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.ReplyUnauthorized(c, "Authorization header is required")
			c.Abort()
			return
		}
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			response.ReplyUnauthorized(c, "Authorization header format must be Bearer {token}")
			c.Abort()
			return
		}
		claims, err := utils.ParseToken(parts[1])
		if err != nil {
			response.ReplyUnauthorized(c, "Invalid token: "+err.Error())
			c.Abort()
			return
		}
		c.Set("userID", claims.UserID)
		c.Set("username", claims.UserName)
		c.Next()
	}
}
