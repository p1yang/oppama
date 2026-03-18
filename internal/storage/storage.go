package storage

import (
	"context"
	"time"
)

// Storage 存储接口
type Storage interface {
	// 服务 CRUD
	SaveService(ctx context.Context, svc *OllamaService) error
	GetService(ctx context.Context, id string) (*OllamaService, error)
	ListServices(ctx context.Context, filter ServiceFilter) ([]*OllamaService, error)
	DeleteService(ctx context.Context, id string) error
	UpdateServiceStatus(ctx context.Context, id string, status ServiceStatus) error

	// 模型 CRUD
	SaveModels(ctx context.Context, serviceID string, models []ModelInfo) error
	GetModelsByService(ctx context.Context, serviceID string) ([]ModelInfo, error)
	ListModels(ctx context.Context, filter ModelFilter) ([]ModelInfo, error)

	// 发现任务（遗留，保持兼容）
	SaveTask(ctx context.Context, task *DiscoveryTask) error
	GetTask(ctx context.Context, id string) (*DiscoveryTask, error)
	UpdateTask(ctx context.Context, task *DiscoveryTask) error

	// 活动日志
	SaveActivityLog(ctx context.Context, log *ActivityLog) error
	ListRecentActivities(ctx context.Context, limit int) ([]*ActivityLog, error)
	ListActivitiesByService(ctx context.Context, serviceID string, limit int) ([]*ActivityLog, error)

	// 统计查询
	GetStats(ctx context.Context) (*Stats, error)

	// 用户管理
	SaveUser(ctx context.Context, user *User) error
	GetUserByUsername(ctx context.Context, username string) (*User, error)
	GetUser(ctx context.Context, id string) (*User, error)
	ListUsers(ctx context.Context) ([]*User, error)
	DeleteUser(ctx context.Context, id string) error
	UpdateUser(ctx context.Context, user *User) error

	// 健康检查
	Ping(ctx context.Context) error
	Close() error
}

// BlacklistStorage Token 黑名单存储接口
type BlacklistStorage interface {
	AddToken(ctx context.Context, token string, expiresAt time.Time) error
	IsTokenBlacklisted(ctx context.Context, token string) (bool, error)
	DeleteToken(ctx context.Context, token string) error
	CleanExpiredTokens(ctx context.Context) error
}

// TaskStorage 通用任务存储接口
type TaskStorage interface {
	SaveTask(ctx context.Context, task *Task) error
	GetTask(ctx context.Context, id string) (*Task, error)
	ListTasks(ctx context.Context, filter TaskFilter) ([]*Task, error)
	DeleteTask(ctx context.Context, id string) error
}

// NewBlacklistMemory 创建内存黑名单
func NewBlacklistMemory() BlacklistStorage {
	return &blacklistMemory{
		tokens: make(map[string]time.Time),
	}
}

// blacklistMemory 内存实现的 Token 黑名单（用于测试）
type blacklistMemory struct {
	tokens map[string]time.Time
}

func (b *blacklistMemory) AddToken(ctx context.Context, token string, expiresAt time.Time) error {
	b.tokens[token] = expiresAt
	return nil
}

func (b *blacklistMemory) IsTokenBlacklisted(ctx context.Context, token string) (bool, error) {
	expiresAt, exists := b.tokens[token]
	if !exists {
		return false, nil
	}
	// 如果已过期，自动删除
	if time.Now().After(expiresAt) {
		delete(b.tokens, token)
		return false, nil
	}
	return true, nil
}

func (b *blacklistMemory) DeleteToken(ctx context.Context, token string) error {
	delete(b.tokens, token)
	return nil
}

func (b *blacklistMemory) CleanExpiredTokens(ctx context.Context) error {
	now := time.Now()
	for token, expiresAt := range b.tokens {
		if now.After(expiresAt) {
			delete(b.tokens, token)
		}
	}
	return nil
}
