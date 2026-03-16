package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// 角色常量
const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

// PermissionConfig 权限配置
type PermissionConfig struct {
	RequiredRole string   // 要求的角色
	AllowedRoles []string // 允许的角色列表
}

// RequireRole 要求特定角色的中间件
func RequireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, exists := GetRole(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "用户未认证",
			})
			c.Abort()
			return
		}

		if userRole != role {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "权限不足",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireRoles 要求多个角色之一
func RequireRoles(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, exists := GetRole(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "用户未认证",
			})
			c.Abort()
			return
		}

		// 管理员拥有所有权限
		if userRole == RoleAdmin {
			c.Next()
			return
		}

		// 检查用户角色是否在允许列表中
		for _, role := range roles {
			if userRole == role {
				c.Next()
				return
			}
		}

		c.JSON(http.StatusForbidden, gin.H{
			"error": "权限不足",
		})
		c.Abort()
	}
}

// RequireAdmin 要求管理员权限
func RequireAdmin() gin.HandlerFunc {
	return RequireRole(RoleAdmin)
}

// RequireUser 要求普通用户权限（admin 也可以访问）
func RequireUser() gin.HandlerFunc {
	return RequireRoles(RoleAdmin, RoleUser)
}

// Permission 通用权限检查中间件
func Permission(cfg PermissionConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, exists := GetRole(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "用户未认证",
			})
			c.Abort()
			return
		}

		// 如果指定了要求的具体角色
		if cfg.RequiredRole != "" {
			if userRole != cfg.RequiredRole && userRole != RoleAdmin {
				c.JSON(http.StatusForbidden, gin.H{
					"error": "权限不足",
				})
				c.Abort()
				return
			}
		}

		// 如果指定了允许的角色列表
		if len(cfg.AllowedRoles) > 0 {
			allowed := false
			for _, role := range cfg.AllowedRoles {
				if userRole == role || userRole == RoleAdmin {
					allowed = true
					break
				}
			}
			if !allowed {
				c.JSON(http.StatusForbidden, gin.H{
					"error": "权限不足",
				})
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

// IsAdmin 检查用户是否是管理员
func IsAdmin(c *gin.Context) bool {
	role, exists := GetRole(c)
	if !exists {
		return false
	}
	return role == RoleAdmin
}

// IsAuthenticated 检查用户是否已认证
func IsAuthenticated(c *gin.Context) bool {
	_, exists := GetUserID(c)
	return exists
}

// MustLogin 要求登录的中间件（不检查角色）
func MustLogin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !IsAuthenticated(c) {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "用户未认证",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
