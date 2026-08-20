package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	"video-captions/enum"
	"video-captions/utils/response"
)

// TokenValidator JWT token 校验函数签名
type TokenValidator func(tokenString string) (map[string]interface{}, error)

// AuthRequired 返回 Gin 中间件，使用传入的 validate 函数校验 token
func AuthRequired(validate TokenValidator) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.FailByBizError(c, enum.ErrUnauthorized)
			c.Abort()
			return
		}

		// 解析 Bearer token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			response.FailByBizError(c, enum.ErrUnauthorized)
			c.Abort()
			return
		}

		tokenString := parts[1]
		claims, err := validate(tokenString)
		if err != nil {
			response.FailByBizError(c, enum.ErrUnauthorized.WithMsg(err.Error()))
			c.Abort()
			return
		}

		// 将用户信息写入上下文
		if userID, ok := claims["user_id"].(string); ok {
			c.Set("user_id", userID)
		}
		if username, ok := claims["username"].(string); ok {
			c.Set("username", username)
		}
		if email, ok := claims["email"].(string); ok {
			c.Set("email", email)
		}

		c.Next()
	}
}
