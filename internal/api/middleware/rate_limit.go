package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// LoginRateLimitConfig 登录限流配置
type LoginRateLimitConfig struct {
	MaxAttempts    int           // 最大失败次数
	WindowMinutes  int           // 时间窗口（分钟）
	LockoutMinutes int           // 锁定时间（分钟）
}

// loginAttempt 登录尝试记录
type loginAttempt struct {
	count      int
	windowStart time.Time
}

// LoginRateLimiter 登录限流器
type LoginRateLimiter struct {
	cfg    LoginRateLimitConfig
	mu     sync.RWMutex
	ipAttempts   map[string]*loginAttempt  // IP 维度的限流
	userAttempts map[string]*loginAttempt  // 用户名维度的限流
}

// NewLoginRateLimiter 创建登录限流器
func NewLoginRateLimiter(cfg LoginRateLimitConfig) *LoginRateLimiter {
	limiter := &LoginRateLimiter{
		cfg:          cfg,
		ipAttempts:   make(map[string]*loginAttempt),
		userAttempts: make(map[string]*loginAttempt),
	}

	// 启动清理协程
	go limiter.cleanupLoop()

	return limiter
}

// cleanupLoop 定期清理过期记录
func (r *LoginRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		r.cleanup()
	}
}

// cleanup 清理过期的限流记录
func (r *LoginRateLimiter) cleanup() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	windowDuration := time.Duration(r.cfg.WindowMinutes) * time.Minute

	// 清理 IP 记录
	for ip, attempt := range r.ipAttempts {
		if now.Sub(attempt.windowStart) > windowDuration {
			delete(r.ipAttempts, ip)
		}
	}

	// 清理用户名记录
	for username, attempt := range r.userAttempts {
		if now.Sub(attempt.windowStart) > windowDuration {
			delete(r.userAttempts, username)
		}
	}
}

// CheckIP 检查 IP 是否被限流
func (r *LoginRateLimiter) CheckIP(ip string) (bool, time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	attempt, exists := r.ipAttempts[ip]
	if !exists {
		return false, 0
	}

	now := time.Now()
	windowDuration := time.Duration(r.cfg.WindowMinutes) * time.Minute

	// 如果窗口已过期，重置
	if now.Sub(attempt.windowStart) > windowDuration {
		delete(r.ipAttempts, ip)
		return false, 0
	}

	// 检查是否超过限制
	if attempt.count >= r.cfg.MaxAttempts {
		lockoutDuration := time.Duration(r.cfg.LockoutMinutes) * time.Minute
		remaining := windowDuration - now.Sub(attempt.windowStart)
		if remaining < lockoutDuration {
			remaining = lockoutDuration
		}
		return true, remaining
	}

	return false, 0
}

// CheckUsername 检查用户名是否被限流
func (r *LoginRateLimiter) CheckUsername(username string) (bool, time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	attempt, exists := r.userAttempts[username]
	if !exists {
		return false, 0
	}

	now := time.Now()
	windowDuration := time.Duration(r.cfg.WindowMinutes) * time.Minute

	// 如果窗口已过期，重置
	if now.Sub(attempt.windowStart) > windowDuration {
		delete(r.userAttempts, username)
		return false, 0
	}

	// 检查是否超过限制
	if attempt.count >= r.cfg.MaxAttempts {
		lockoutDuration := time.Duration(r.cfg.LockoutMinutes) * time.Minute
		remaining := windowDuration - now.Sub(attempt.windowStart)
		if remaining < lockoutDuration {
			remaining = lockoutDuration
		}
		return true, remaining
	}

	return false, 0
}

// RecordFailure 记录登录失败
func (r *LoginRateLimiter) RecordFailure(ip, username string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	windowDuration := time.Duration(r.cfg.WindowMinutes) * time.Minute

	// 更新 IP 记录
	if attempt, exists := r.ipAttempts[ip]; exists {
		if now.Sub(attempt.windowStart) > windowDuration {
			// 窗口已过期，重置
			r.ipAttempts[ip] = &loginAttempt{count: 1, windowStart: now}
		} else {
			attempt.count++
		}
	} else {
		r.ipAttempts[ip] = &loginAttempt{count: 1, windowStart: now}
	}

	// 更新用户名记录
	if username != "" {
		if attempt, exists := r.userAttempts[username]; exists {
			if now.Sub(attempt.windowStart) > windowDuration {
				// 窗口已过期，重置
				r.userAttempts[username] = &loginAttempt{count: 1, windowStart: now}
			} else {
				attempt.count++
			}
		} else {
			r.userAttempts[username] = &loginAttempt{count: 1, windowStart: now}
		}
	}
}

// RecordSuccess 记录登录成功，清除限制
func (r *LoginRateLimiter) RecordSuccess(ip, username string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.ipAttempts, ip)
	if username != "" {
		delete(r.userAttempts, username)
	}
}

// GetClientIP 获取客户端 IP
func GetClientIP(c *gin.Context) string {
	// 检查 X-Forwarded-For 头
	if xff := c.GetHeader("X-Forwarded-For"); xff != "" {
		// 取第一个 IP
		for i, c := range xff {
			if c == ' ' || c == ',' {
				return xff[:i]
			}
		}
		return xff
	}

	// 检查 X-Real-IP 头
	if xri := c.GetHeader("X-Real-IP"); xri != "" {
		return xri
	}

	// 使用 RemoteAddr
	return c.ClientIP()
}

// LoginRateLimit 登录限流中间件
func LoginRateLimit(limiter *LoginRateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 只对登录接口进行限流
		if c.Request.URL.Path != "/v1/api/auth/login" {
			c.Next()
			return
		}

		ip := GetClientIP(c)

		// 检查 IP 限流
		locked, remaining := limiter.CheckIP(ip)
		if locked {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": fmt.Sprintf("登录尝试过于频繁，请在 %v 后重试", remaining.Round(time.Second)),
			})
			c.Abort()
			return
		}

		// 将 limiter 存入上下文，供登录处理器使用
		c.Set("rate_limiter", limiter)
		c.Set("client_ip", ip)

		c.Next()
	}
}

// GetRateLimiter 从上下文获取限流器
func GetRateLimiter(c *gin.Context) *LoginRateLimiter {
	if limiter, exists := c.Get("rate_limiter"); exists {
		return limiter.(*LoginRateLimiter)
	}
	return nil
}
