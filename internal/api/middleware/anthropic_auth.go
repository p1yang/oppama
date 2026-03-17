package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// AnthropicAuth Anthropic API 认证中间件
// 使用 x-api-key header 进行认证，与 OpenAI API 权限一致
func AnthropicAuth(configProvider *ProxyConfigProvider) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := configProvider.GetAPIKey()
		enabled := configProvider.IsAuthEnabled()

		// 如果未启用认证，直接通过
		if !enabled {
			c.Next()
			return
		}

		// 获取 x-api-key header (Anthropic 标准)
		apiKeyHeader := c.GetHeader("x-api-key")

		// 兼容 Bearer token 格式
		if apiKeyHeader == "" {
			authHeader := c.GetHeader("Authorization")
			if authHeader != "" {
				parts := strings.SplitN(authHeader, " ", 2)
				if len(parts) == 2 && parts[0] == "Bearer" {
					apiKeyHeader = parts[1]
				}
			}
		}

		if apiKeyHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"type": "error",
				"error": gin.H{
					"type":    "authentication_error",
					"message": "缺少认证信息，请提供 x-api-key header 或 Authorization: Bearer token",
				},
			})
			c.Abort()
			return
		}

		// 验证 API Key
		if apiKeyHeader != apiKey {
			c.JSON(http.StatusUnauthorized, gin.H{
				"type": "error",
				"error": gin.H{
					"type":    "authentication_error",
					"message": "无效的 API Key",
				},
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
