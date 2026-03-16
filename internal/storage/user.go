package storage

import (
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// User 用户模型
type User struct {
	ID        string     `json:"id"`
	Username  string     `json:"username"`
	Password  string     `json:"-"` // 不序列化到 JSON
	Nickname  string     `json:"nickname"`
	Email     string     `json:"email"`
	Role      string     `json:"role"` // admin, user
	Status    string     `json:"status"` // active, disabled, locked, require_password_change
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`

	// 安全相关字段
	LastLoginAt   *time.Time `json:"last_login_at,omitempty"`
	FailedLogins  int        `json:"failed_logins,omitempty"`
	LockedUntil   *time.Time `json:"locked_until,omitempty"`
}

// UserStatus 用户状态常量
const (
	UserStatusActive               = "active"
	UserStatusDisabled             = "disabled"
	UserStatusLocked               = "locked"
	UserStatusRequirePasswordChange = "require_password_change"
)

// DefaultCost bcrypt 默认成本因子
const DefaultCost = bcrypt.DefaultCost

// HashPassword 对密码进行哈希（使用 bcrypt）
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword 验证密码（使用 bcrypt）
func (u *User) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password))
	return err == nil
}

// IsLocked 检查用户是否被锁定
func (u *User) IsLocked() bool {
	if u.Status == UserStatusLocked {
		return true
	}
	if u.LockedUntil != nil && time.Now().Before(*u.LockedUntil) {
		return true
	}
	return false
}

// IsDisabled 检查用户是否被禁用
func (u *User) IsDisabled() bool {
	return u.Status == UserStatusDisabled
}

// IsAccessible 检查用户是否可访问（可用于登录）
func (u *User) IsAccessible() bool {
	if u.IsDisabled() {
		return false
	}
	if u.IsLocked() {
		return false
	}
	return true
}

// RecordLoginSuccess 记录登录成功
func (u *User) RecordLoginSuccess() {
	now := time.Now()
	u.LastLoginAt = &now
	u.FailedLogins = 0
	u.LockedUntil = nil
	if u.Status == UserStatusLocked {
		u.Status = UserStatusActive
	}
}

// RecordLoginFailure 记录登录失败
func (u *User) RecordLoginFailure(maxAttempts int, lockoutDuration time.Duration) {
	u.FailedLogins++
	if u.FailedLogins >= maxAttempts {
		now := time.Now()
		lockUntil := now.Add(lockoutDuration)
		u.LockedUntil = &lockUntil
		u.Status = UserStatusLocked
	}
}

// DefaultAdminUserWithPassword 默认管理员账户（返回密码）
func DefaultAdminUserWithPassword() (*User, string) {
	// 固定密码
	password := "admin"
	hash, _ := HashPassword(password)

	user := &User{
		ID:       "admin",
		Username: "admin",
		Password: hash,
		Nickname: "系统管理员",
		Email:    "admin@oppama.local",
		Role:     "admin",
		Status:   UserStatusRequirePasswordChange,
	}
	return user, password
}

// DefaultAdminUser 默认管理员账户
func DefaultAdminUser() *User {
	user, _ := DefaultAdminUserWithPassword()
	return user
}

// CreateAdminUser 创建管理员账户（指定密码）
func CreateAdminUser(username, password string) *User {
	hash, _ := HashPassword(password)
	return &User{
		ID:       username,
		Username: username,
		Password: hash,
		Nickname: "管理员",
		Role:     "admin",
		Status:   UserStatusActive,
	}
}

// GenerateRandomPassword 生成随机密码
func GenerateRandomPassword(length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
	password := make([]byte, length)
	for i := range password {
		password[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(password), nil
}

var (
	ErrUserNotFound    = errors.New("用户不存在")
	ErrInvalidPassword = errors.New("密码错误")
	ErrUserExists      = errors.New("用户名已存在")
	ErrUserLocked      = errors.New("用户已被锁定")
	ErrUserDisabled    = errors.New("用户已被禁用")
)
