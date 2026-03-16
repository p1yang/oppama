package middleware

import (
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

// ConfigProvider 配置提供者接口
type ConfigProvider interface {
	GetAPIKey() string
	IsAuthEnabled() bool
}

// ProxyConfigProvider 代理配置提供者
type ProxyConfigProvider struct {
	mu          sync.RWMutex
	apiKey      string
	authEnabled bool
}

// NewProxyConfigProvider 创建代理配置提供者
func NewProxyConfigProvider() *ProxyConfigProvider {
	return &ProxyConfigProvider{
		apiKey:      "",
		authEnabled: false,
	}
}

// UpdateConfig 更新配置
func (p *ProxyConfigProvider) UpdateConfig(apiKey string, authEnabled bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.apiKey = apiKey
	p.authEnabled = authEnabled
}

// GetAPIKey 获取 API Key
func (p *ProxyConfigProvider) GetAPIKey() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.apiKey
}

// IsAuthEnabled 检查是否启用认证
func (p *ProxyConfigProvider) IsAuthEnabled() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.authEnabled
}

// OpenAIAuth OpenAI API 认证中间件
// 用于验证 OpenAI 兼容接口的 API Key
// provider 可以是 string、ConfigProvider 或 func() (string, bool)
func OpenAIAuth(provider interface{}) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 根据类型获取 API Key 和启用状态
		var apiKey string
		var enabled bool

		switch v := provider.(type) {
		case string:
			apiKey = v
			enabled = apiKey != ""
		case ConfigProvider:
			apiKey = v.GetAPIKey()
			enabled = v.IsAuthEnabled()
		case *ProxyConfigProvider:
			apiKey = v.GetAPIKey()
			enabled = v.IsAuthEnabled()
		case func() (string, bool):
			apiKey, enabled = v()
		}

		// 如果未启用认证，跳过认证（向后兼容）
		if !enabled {
			c.Next()
			return
		}

		// 如果启用了认证但没有配置 API Key，拒绝所有请求
		if apiKey == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "服务器未配置 API Key",
			})
			c.Abort()
			return
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

		// 验证 Bearer API Key
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "认证格式错误，应为：Bearer <api_key>",
			})
			c.Abort()
			return
		}

		// 验证 API Key
		if parts[1] != apiKey {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "无效的 API Key",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
