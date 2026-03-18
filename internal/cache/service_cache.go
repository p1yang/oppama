package cache

import (
	"sync"
	"time"

	"oppama/internal/storage"
)

// ServiceCache 服务列表缓存
type ServiceCache struct {
	mu              sync.RWMutex
	services        []*storage.OllamaService
	lastRefresh      time.Time
	refreshInterval time.Duration
	ttl             time.Duration
	// 缓存统计
	hits     int64
	misses   int64
	refreshes int64
}

// ServiceCacheConfig 缓存配置
type ServiceCacheConfig struct {
	RefreshInterval time.Duration // 缓存刷新间隔
	TTL             time.Duration // 缓存过期时间
}

// NewServiceCache 创建服务缓存
func NewServiceCache(config ServiceCacheConfig) *ServiceCache {
	if config.RefreshInterval == 0 {
		config.RefreshInterval = 5 * time.Minute
	}
	if config.TTL == 0 {
		config.TTL = 10 * time.Minute
	}

	return &ServiceCache{
		services:        make([]*storage.OllamaService, 0),
		refreshInterval: config.RefreshInterval,
		ttl:             config.TTL,
		lastRefresh:     time.Time{},
	}
}

// Get 获取缓存的服务列表
func (c *ServiceCache) Get() ([]*storage.OllamaService, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 检查是否过期
	if time.Since(c.lastRefresh) > c.ttl {
		c.mu.RUnlock()
		c.mu.Lock()
		defer c.mu.Unlock()

		// 双重检查
		if time.Since(c.lastRefresh) > c.ttl {
			c.misses++
			return nil, false
		}
	}

	c.hits++
	// 返回副本，避免外部修改
	result := make([]*storage.OllamaService, len(c.services))
	copy(result, c.services)
	return result, true
}

// Set 设置缓存的服务列表
func (c *ServiceCache) Set(services []*storage.OllamaService) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.services = services
	c.lastRefresh = time.Now()
	c.refreshes++
}

// ShouldRefresh 检查是否应该刷新缓存
func (c *ServiceCache) ShouldRefresh() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.lastRefresh.IsZero() {
		return true
	}

	return time.Since(c.lastRefresh) > c.refreshInterval
}

// GetStats 获取缓存统计信息
func (c *ServiceCache) GetStats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := c.hits + c.misses
	hitRate := float64(0)
	if total > 0 {
		hitRate = float64(c.hits) / float64(total) * 100
	}

	return map[string]interface{}{
		"hits":          c.hits,
		"misses":        c.misses,
		"hit_rate":      hitRate,
		"refreshes":     c.refreshes,
		"last_refresh":  c.lastRefresh,
		"service_count": len(c.services),
		"is_expired":    time.Since(c.lastRefresh) > c.ttl,
	}
}

// Invalidate 使缓存失效
func (c *ServiceCache) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.services = make([]*storage.OllamaService, 0)
	c.lastRefresh = time.Time{}
}

// Clear 清理资源
func (c *ServiceCache) Clear() {
	c.Invalidate()
}
