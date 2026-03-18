package v1

import (
	"fmt"
	"net/http"
	"time"

	"oppama/internal/api/middleware"
	"oppama/internal/config"
	"oppama/internal/storage"

	"github.com/gin-gonic/gin"
)

// AuthHandler 认证处理器
type AuthHandler struct {
	storage   storage.Storage
	authCfg   config.AuthConfig
	rateLimit *middleware.LoginRateLimiter
	blacklist storage.BlacklistStorage
}

// NewAuthHandler 创建认证处理器
func NewAuthHandler(store storage.Storage, authCfg config.AuthConfig, rateLimit *middleware.LoginRateLimiter, blacklist storage.BlacklistStorage) *AuthHandler {
	return &AuthHandler{
		storage:   store,
		authCfg:   authCfg,
		rateLimit: rateLimit,
		blacklist: blacklist,
	}
}

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	Token     string        `json:"token"`
	User      *storage.User `json:"user"`
	ExpiresIn int64         `json:"expires_in"` // 过期时间（秒）
}

// Login 用户登录
// @Summary 用户登录
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body LoginRequest true "登录信息"
// @Success 200 {object} LoginResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /api/v1/auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请求参数错误：" + err.Error(),
		})
		return
	}

	// 检查用户名限流
	if h.rateLimit != nil {
		locked, remaining := h.rateLimit.CheckUsername(req.Username)
		if locked {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": fmt.Sprintf("登录尝试过于频繁，请在 %v 后重试", remaining.Round(time.Second)),
			})
			return
		}
	}

	// 查询用户
	user, err := h.storage.GetUserByUsername(c.Request.Context(), req.Username)
	if err != nil {
		if err == storage.ErrUserNotFound {
			h.recordLoginFailure(c, req.Username)
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "用户名或密码错误",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "查询用户失败：" + err.Error(),
		})
		return
	}

	// 检查用户状态
	if user.IsDisabled() {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "账户已被禁用",
		})
		return
	}

	if user.IsLocked() {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "账户已被锁定",
		})
		return
	}

	// 验证密码
	if !user.CheckPassword(req.Password) {
		h.recordLoginFailure(c, req.Username)

		// 更新用户的失败次数
		user.RecordLoginFailure(h.authCfg.LoginRateLimit.MaxAttempts,
			time.Duration(h.authCfg.LoginRateLimit.LockoutMinutes)*time.Minute)
		h.storage.UpdateUser(c.Request.Context(), user)

		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "用户名或密码错误",
		})
		return
	}

	// 检查是否需要修改密码
	if user.Status == storage.UserStatusRequirePasswordChange {
		// 临时 Token，有效期较短
		tempToken, _ := h.generateToken(user, 15*time.Minute)
		user.Password = ""
		c.JSON(http.StatusOK, gin.H{
			"token":                   tempToken,
			"user":                    user,
			"require_password_change": true,
			"message":                 "首次登录请修改密码",
		})
		return
	}

	// 记录登录成功
	if h.rateLimit != nil {
		clientIP, _ := c.Get("client_ip")
		h.rateLimit.RecordSuccess(clientIP.(string), req.Username)
	}

	// 更新用户登录信息
	user.RecordLoginSuccess()
	h.storage.UpdateUser(c.Request.Context(), user)

	// 记录登录活动日志
	activityLog := &storage.ActivityLog{
		Type:     storage.ActivityAdd,
		Action:   "用户登录",
		Target:   fmt.Sprintf("%s (%s)", user.Username, user.Nickname),
		UserID:   user.ID,
		Metadata: fmt.Sprintf(`{"username":"%s","nickname":"%s","role":"%s"}`, user.Username, user.Nickname, user.Role),
	}
	h.storage.SaveActivityLog(c.Request.Context(), activityLog)

	// 生成 JWT Token
	token, err := h.generateToken(user, 0) // 0 表示使用默认配置
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "生成 Token 失败：" + err.Error(),
		})
		return
	}

	// 解析过期时间
	expiresIn := int64(24 * 3600) // 默认 24 小时
	if h.authCfg.JWTExpire != "" {
		if duration, err := time.ParseDuration(h.authCfg.JWTExpire); err == nil {
			expiresIn = int64(duration.Seconds())
		}
	}

	// 清除密码
	user.Password = ""

	c.JSON(http.StatusOK, LoginResponse{
		Token:     token,
		User:      user,
		ExpiresIn: expiresIn,
	})
}

// recordLoginFailure 记录登录失败
func (h *AuthHandler) recordLoginFailure(c *gin.Context, username string) {
	if h.rateLimit == nil {
		return
	}

	clientIP, exists := c.Get("client_ip")
	if !exists {
		return
	}

	h.rateLimit.RecordFailure(clientIP.(string), username)
}

// generateToken 生成 Token
func (h *AuthHandler) generateToken(user *storage.User, customExpire time.Duration) (string, error) {
	// 如果没有设置 JWT Secret，使用默认值
	secret := h.authCfg.JWTSecret
	if secret == "" {
		secret = "oppama-default-secret-change-me"
	}

	// 确定过期时间
	expireTime := 24 * time.Hour
	if h.authCfg.JWTExpire != "" {
		if duration, err := time.ParseDuration(h.authCfg.JWTExpire); err == nil {
			expireTime = duration
		}
	}
	if customExpire > 0 {
		expireTime = customExpire
	}

	claims := middleware.Claims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
	}

	return middleware.GenerateToken(middleware.JWTConfig{
		Secret:     secret,
		ExpireTime: expireTime,
	}, claims)
}

// Logout 登出
// @Summary 用户登出
// @Tags 认证
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]string
// @Router /api/v1/auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	// 获取当前用户
	userID, _ := c.Get("user_id")
	username, _ := c.Get("username")

	// 将 Token 加入黑名单
	if h.blacklist != nil && h.authCfg.EnableBlacklist {
		token, exists := middleware.GetToken(c)
		if exists {
			// 解析黑名单 TTL
			ttl := 24 * time.Hour
			if h.authCfg.BlacklistTTL != "" {
				if duration, err := time.ParseDuration(h.authCfg.BlacklistTTL); err == nil {
					ttl = duration
				}
			}
			expiresAt := time.Now().Add(ttl)

			// 添加到黑名单
			h.blacklist.AddToken(c.Request.Context(), token, expiresAt)
		}
	}

	// 记录登出活动日志
	if userIDStr, ok := userID.(string); ok {
		activityLog := &storage.ActivityLog{
			Type:     storage.ActivityCheck,
			Action:   "用户登出",
			Target:   username.(string),
			UserID:   userIDStr,
			Metadata: fmt.Sprintf(`{"username":"%s"}`, username),
		}
		h.storage.SaveActivityLog(c.Request.Context(), activityLog)
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "登出成功",
	})
}

// GetCurrentUser 获取当前用户信息
// @Summary 获取当前用户信息
// @Tags 认证
// @Produce json
// @Security BearerAuth
// @Success 200 {object} storage.User
// @Router /api/v1/auth/me [get]
func (h *AuthHandler) GetCurrentUser(c *gin.Context) {
	// 从上下文中获取用户信息
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "未授权",
		})
		return
	}

	user, err := h.storage.GetUser(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "查询用户失败：" + err.Error(),
		})
		return
	}

	// 不返回密码
	user.Password = ""
	c.JSON(http.StatusOK, user)
}

// ChangePasswordRequest 修改密码请求
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

// ChangePassword 修改密码
// @Summary 修改密码
// @Tags 认证
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body ChangePasswordRequest true "密码信息"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /api/v1/auth/change-password [post]
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "请求参数错误：" + err.Error(),
		})
		return
	}

	// 获取当前用户
	userID, _ := middleware.GetUserID(c)
	user, err := h.storage.GetUser(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "查询用户失败：" + err.Error(),
		})
		return
	}

	// 验证旧密码
	if !user.CheckPassword(req.OldPassword) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "原密码错误",
		})
		return
	}

	// 哈希新密码
	hash, err := storage.HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "密码加密失败",
		})
		return
	}

	// 更新密码
	user.Password = hash
	// 如果之前是需要修改密码状态，现在改为正常状态
	if user.Status == storage.UserStatusRequirePasswordChange {
		user.Status = storage.UserStatusActive
	}

	if err := h.storage.UpdateUser(c.Request.Context(), user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "更新密码失败：" + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "密码修改成功",
	})
}
