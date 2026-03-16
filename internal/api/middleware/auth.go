package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// AuthConfig 认证配置
type AuthConfig struct {
	EnableAuth bool
	APIKey     string
	// 白名单路由，不需要认证的路由
	WhiteList []string
}

// Auth 认证中间件
func Auth(cfg AuthConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 如果未启用认证，直接通过
		if !cfg.EnableAuth {
			c.Next()
			return
		}

		// 检查是否在白名单中（支持前缀匹配）
		for _, whitePath := range cfg.WhiteList {
			if strings.HasPrefix(c.Request.URL.Path, whitePath) {
				c.Next()
				return
			}
		}

		// 获取 Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "缺少认证信息",
			})
			c.Abort()
			return
		}

		// 验证 Bearer Token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "认证格式错误，应为：Bearer <token>",
			})
			c.Abort()
			return
		}

		// 验证 API Key
		if parts[1] != cfg.APIKey {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "无效的 API Key",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
