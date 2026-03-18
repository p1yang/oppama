package storage

import (
	"context"
	"sync"
	"time"
)

// MemoryStorage 内存存储实现（用于开发/测试）
type MemoryStorage struct {
	mu           sync.RWMutex
	services     map[string]*OllamaService
	models       map[string][]ModelInfo // service_id -> models
	tasks        map[string]*DiscoveryTask
	universalTasks map[string]*Task
	users        map[string]*User
	tokenBlacklist map[string]*TokenBlacklistEntry
	activityLogs  []*ActivityLog
}

// TokenBlacklistEntry Token 黑名单条目
type TokenBlacklistEntry struct {
	Token     string
	ExpiresAt time.Time
}

// NewMemoryStorage 创建内存存储实例
func NewMemoryStorage() (*MemoryStorage, error) {
	return &MemoryStorage{
		services:       make(map[string]*OllamaService),
		models:         make(map[string][]ModelInfo),
		tasks:          make(map[string]*DiscoveryTask),
		universalTasks: make(map[string]*Task),
		users:          make(map[string]*User),
		tokenBlacklist: make(map[string]*TokenBlacklistEntry),
		activityLogs:   make([]*ActivityLog, 0),
	}, nil
}

// ========== 服务方法 ==========

// SaveService 保存服务
func (m *MemoryStorage) SaveService(ctx context.Context, svc *OllamaService) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	svc.UpdatedAt = now
	if svc.CreatedAt.IsZero() {
		svc.CreatedAt = now
	}

	m.services[svc.ID] = svc
	return nil
}

// GetService 获取服务
func (m *MemoryStorage) GetService(ctx context.Context, id string) (*OllamaService, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	svc, exists := m.services[id]
	if !exists {
		return nil, nil
	}

	// 复制服务对象以避免并发修改
	svcCopy := *svc
	svcCopy.Models = make([]ModelInfo, len(svc.Models))
	copy(svcCopy.Models, svc.Models)

	return &svcCopy, nil
}

// ListServices 列出服务
func (m *MemoryStorage) ListServices(ctx context.Context, filter ServiceFilter) ([]*OllamaService, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	services := make([]*OllamaService, 0)
	for _, svc := range m.services {
		// 应用过滤条件
		if filter.Status != nil && svc.Status != *filter.Status {
			continue
		}
		if filter.Source != nil && svc.Source != *filter.Source {
			continue
		}
		if filter.IsHoneypot != nil && svc.IsHoneypot != *filter.IsHoneypot {
			continue
		}
		if filter.Search != "" {
			searchTerm := filter.Search
			if !contains(svc.Name, searchTerm) &&
			   !contains(svc.URL, searchTerm) &&
			   !contains(svc.Version, searchTerm) {
				continue
			}
		}

		// 复制服务对象
		svcCopy := *svc
		svcCopy.Models = make([]ModelInfo, len(svc.Models))
		copy(svcCopy.Models, svc.Models)

		services = append(services, &svcCopy)
	}

	// 简单实现分页（在内存中）
	if filter.PageSize > 0 {
		start := 0
		if filter.Page > 0 {
			start = (filter.Page - 1) * filter.PageSize
		}
		end := start + filter.PageSize
		if start > len(services) {
			return []*OllamaService{}, nil
		}
		if end > len(services) {
			end = len(services)
		}
		services = services[start:end]
	}

	return services, nil
}

// DeleteService 删除服务
func (m *MemoryStorage) DeleteService(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.services, id)
	delete(m.models, id)
	return nil
}

// UpdateServiceStatus 更新服务状态
func (m *MemoryStorage) UpdateServiceStatus(ctx context.Context, id string, status ServiceStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if svc, exists := m.services[id]; exists {
		svc.Status = status
		svc.UpdatedAt = time.Now()
	}
	return nil
}

// ========== 模型方法 ==========

// SaveModels 保存模型列表
func (m *MemoryStorage) SaveModels(ctx context.Context, serviceID string, models []ModelInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.models[serviceID] = models
	return nil
}

// GetModelsByService 获取服务的模型列表
func (m *MemoryStorage) GetModelsByService(ctx context.Context, serviceID string) ([]ModelInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	models, exists := m.models[serviceID]
	if !exists {
		return []ModelInfo{}, nil
	}

	// 返回副本
	result := make([]ModelInfo, len(models))
	copy(result, models)
	return result, nil
}

// ListModels 列出所有模型
func (m *MemoryStorage) ListModels(ctx context.Context, filter ModelFilter) ([]ModelInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	allModels := make([]ModelInfo, 0)
	for _, models := range m.models {
		allModels = append(allModels, models...)
	}

	// 应用过滤条件
	result := make([]ModelInfo, 0)
	for _, model := range allModels {
		if filter.Family != "" && model.Family != filter.Family {
			continue
		}
		if filter.MinSize > 0 && model.Size < filter.MinSize {
			continue
		}
		if filter.AvailableOnly && !model.IsAvailable {
			continue
		}
		if filter.ServiceID != "" && model.ServiceID != filter.ServiceID {
			continue
		}
		result = append(result, model)
	}

	return result, nil
}

// ========== 任务方法 ==========

// SaveTask 保存任务
func (m *MemoryStorage) SaveTask(ctx context.Context, task *DiscoveryTask) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.tasks[task.ID] = task
	return nil
}

// GetTask 获取任务
func (m *MemoryStorage) GetTask(ctx context.Context, id string) (*DiscoveryTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, exists := m.tasks[id]
	if !exists {
		return nil, nil
	}

	// 返回副本
	taskCopy := *task
	return &taskCopy, nil
}

// UpdateTask 更新任务
func (m *MemoryStorage) UpdateTask(ctx context.Context, task *DiscoveryTask) error {
	return m.SaveTask(ctx, task)
}

// GetStats 获取统计数据
func (m *MemoryStorage) GetStats(ctx context.Context) (*Stats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &Stats{
		Timestamp: time.Now(),
	}

	// 统计服务
	for _, svc := range m.services {
		stats.TotalServices++
		switch svc.Status {
		case StatusOnline:
			stats.OnlineServices++
		case StatusOffline:
			stats.OfflineServices++
		}
		if svc.IsHoneypot {
			stats.HoneypotServices++
		}
	}

	// 统计模型
	for _, models := range m.models {
		for _, model := range models {
			stats.TotalModels++
			if model.IsAvailable {
				stats.AvailableModels++
			}
		}
	}

	return stats, nil
}

// Ping 健康检查
func (m *MemoryStorage) Ping(ctx context.Context) error {
	return nil
}

// Close 关闭存储
func (m *MemoryStorage) Close() error {
	return nil
}

// ========== 通用任务方法 ==========

// SaveUniversalTask 保存通用任务
func (m *MemoryStorage) SaveUniversalTask(ctx context.Context, task *Task) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.universalTasks[task.ID] = task
	return nil
}

// GetUniversalTask 获取通用任务
func (m *MemoryStorage) GetUniversalTask(ctx context.Context, id string) (*Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, exists := m.universalTasks[id]
	if !exists {
		return nil, nil
	}

	// 返回副本
	taskCopy := *task
	return &taskCopy, nil
}

// ListUniversalTasks 列出通用任务
func (m *MemoryStorage) ListUniversalTasks(ctx context.Context, filter TaskFilter) ([]*Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]*Task, 0)
	for _, task := range m.universalTasks {
		// 应用过滤条件
		if filter.Type != "" && task.Type != filter.Type {
			continue
		}
		if filter.Status != "" && task.Status != filter.Status {
			continue
		}

		// 返回副本
		taskCopy := *task
		tasks = append(tasks, &taskCopy)
	}

	// 简单实现限制
	if filter.Limit > 0 && len(tasks) > filter.Limit {
		tasks = tasks[:filter.Limit]
	}

	return tasks, nil
}

// DeleteUniversalTask 删除通用任务
func (m *MemoryStorage) DeleteUniversalTask(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.universalTasks, id)
	return nil
}

// ========== 用户方法 ==========

// SaveUser 保存用户
func (m *MemoryStorage) SaveUser(ctx context.Context, user *User) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	user.UpdatedAt = now
	if user.CreatedAt.IsZero() {
		user.CreatedAt = now
	}

	m.users[user.ID] = user
	return nil
}

// GetUserByUsername 根据用户名获取用户
func (m *MemoryStorage) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, user := range m.users {
		if user.Username == username {
			// 返回副本
			userCopy := *user
			return &userCopy, nil
		}
	}

	return nil, ErrUserNotFound
}

// GetUser 根据 ID 获取用户
func (m *MemoryStorage) GetUser(ctx context.Context, id string) (*User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	user, exists := m.users[id]
	if !exists {
		return nil, ErrUserNotFound
	}

	// 返回副本
	userCopy := *user
	return &userCopy, nil
}

// ListUsers 获取所有用户
func (m *MemoryStorage) ListUsers(ctx context.Context) ([]*User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	users := make([]*User, 0, len(m.users))
	for _, user := range m.users {
		// 返回副本
		userCopy := *user
		users = append(users, &userCopy)
	}

	return users, nil
}

// DeleteUser 删除用户
func (m *MemoryStorage) DeleteUser(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.users, id)
	return nil
}

// UpdateUser 更新用户
func (m *MemoryStorage) UpdateUser(ctx context.Context, user *User) error {
	return m.SaveUser(ctx, user)
}

// ========== Token 黑名单方法 ==========

// AddToken 添加 Token 到黑名单
func (m *MemoryStorage) AddToken(ctx context.Context, token string, expiresAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.tokenBlacklist[token] = &TokenBlacklistEntry{
		Token:     token,
		ExpiresAt: expiresAt,
	}
	return nil
}

// IsTokenBlacklisted 检查 Token 是否在黑名单中
func (m *MemoryStorage) IsTokenBlacklisted(ctx context.Context, token string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, exists := m.tokenBlacklist[token]
	if !exists {
		return false, nil
	}

	// 检查是否过期
	if time.Now().After(entry.ExpiresAt) {
		// 过期了，删除它
		m.mu.RUnlock()
		m.mu.Lock()
		delete(m.tokenBlacklist, token)
		m.mu.Unlock()
		m.mu.RLock()
		return false, nil
	}

	return true, nil
}

// DeleteToken 从黑名单删除 Token
func (m *MemoryStorage) DeleteToken(ctx context.Context, token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.tokenBlacklist, token)
	return nil
}

// CleanExpiredTokens 清理过期的 Token
func (m *MemoryStorage) CleanExpiredTokens(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for token, entry := range m.tokenBlacklist {
		if now.After(entry.ExpiresAt) {
			delete(m.tokenBlacklist, token)
		}
	}

	return nil
}

// ========== 活动日志方法 ==========

// SaveActivityLog 保存活动日志
func (m *MemoryStorage) SaveActivityLog(ctx context.Context, log *ActivityLog) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	if log.CreatedAt.IsZero() {
		log.CreatedAt = now
	}

	m.activityLogs = append(m.activityLogs, log)
	return nil
}

// ListRecentActivities 查询最近活动日志
func (m *MemoryStorage) ListRecentActivities(ctx context.Context, limit int) ([]*ActivityLog, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 简单实现：返回最新的 N 条
	count := len(m.activityLogs)
	start := 0
	if count > limit {
		start = count - limit
	}

	result := make([]*ActivityLog, 0)
	for i := start; i < count; i++ {
		logCopy := *m.activityLogs[i]
		result = append(result, &logCopy)
	}

	return result, nil
}

// ListActivitiesByService 按服务 ID 查询活动日志
func (m *MemoryStorage) ListActivitiesByService(ctx context.Context, serviceID string, limit int) ([]*ActivityLog, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 简单实现：遍历所有日志
	result := make([]*ActivityLog, 0)
	for _, log := range m.activityLogs {
		if contains(log.Metadata, serviceID) {
			logCopy := *log
			result = append(result, &logCopy)
			if len(result) >= limit {
				break
			}
		}
	}

	return result, nil
}

// ========== 辅助函数 ==========

// contains 检查字符串是否包含子串（不区分大小写）
func contains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) == 0 {
		return false
	}

	// 简单的不区分大小写匹配
	sLower := toLower(s)
	substrLower := toLower(substr)

	return len(sLower) >= len(substrLower) &&
	       (sLower == substrLower ||
	        len(sLower) > len(substrLower) && indexOf(sLower, substrLower) >= 0)
}

// toLower 转换为小写
func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			result[i] = c + 32
		} else {
			result[i] = c
		}
	}
	return string(result)
}

// indexOf 查找子串位置
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}
