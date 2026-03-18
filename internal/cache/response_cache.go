package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"
)

// CacheEntry 缓存条目
type CacheEntry struct {
	Data      []byte
	ExpiresAt time.Time
	CreatedAt time.Time
	HitCount  int64
}

// IsExpired 检查是否过期
func (e *CacheEntry) IsExpired() bool {
	return time.Now().After(e.ExpiresAt)
}

// ResponseCache 响应缓存
type ResponseCache struct {
	mu         sync.RWMutex
entries     map[string]*CacheEntry
	maxSize    int
	ttl        time.Duration
	enabled    bool

	// 统计
	hits      int64
	misses    int64
	evictions int64
}

// ResponseCacheConfig 响应缓存配置
type ResponseCacheConfig struct {
	MaxSize int           // 最大缓存条目数
	TTL     time.Duration // 缓存过期时间
	Enabled bool          // 是否启用缓存
}

// NewResponseCache 创建响应缓存
func NewResponseCache(config ResponseCacheConfig) *ResponseCache {
	if config.MaxSize == 0 {
		config.MaxSize = 1000
	}
	if config.TTL == 0 {
		config.TTL = 5 * time.Minute
	}

	return &ResponseCache{
		entries:  make(map[string]*CacheEntry),
		maxSize:  config.MaxSize,
		ttl:      config.TTL,
		enabled:  config.Enabled,
	}
}

// Get 获取缓存
func (c *ResponseCache) Get(key string) ([]byte, bool) {
	if !c.enabled {
		c.misses++
		return nil, false
	}

	c.mu.RLock()
	entry, exists := c.entries[key]
	c.mu.RUnlock()

	if !exists || entry.IsExpired() {
		c.misses++
		if exists {
			// 清理过期条目
			c.Delete(key)
		}
		return nil, false
	}

	c.mu.Lock()
	entry.HitCount++
	c.mu.Unlock()

	c.hits++
	return entry.Data, true
}

// Set 设置缓存
func (c *ResponseCache) Set(key string, data []byte) {
	if !c.enabled {
		return
	}

	// 检查是否需要清理空间
	if len(c.entries) >= c.maxSize {
		c.evictOldest()
	}

	entry := &CacheEntry{
		Data:      data,
		ExpiresAt: time.Now().Add(c.ttl),
		CreatedAt: time.Now(),
		HitCount:  0,
	}

	c.mu.Lock()
	c.entries[key] = entry
	c.mu.Unlock()
}

// Delete 删除缓存
func (c *ResponseCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, key)
}

// evictOldest 清理最老的条目
func (c *ResponseCache) evictOldest() {
	c.mu.Lock()
	defer c.mu.Unlock()

	var oldestKey string
	var oldestTime time.Time

	for key, entry := range c.entries {
		if oldestKey == "" || entry.CreatedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.CreatedAt
		}
	}

	if oldestKey != "" {
		delete(c.entries, oldestKey)
		c.evictions++
	}
}

// Clear 清空缓存
func (c *ResponseCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*CacheEntry)
	c.evictions++
}

// GetStats 获取统计信息
func (c *ResponseCache) GetStats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := c.hits + c.misses
	hitRate := float64(0)
	if total > 0 {
		hitRate = float64(c.hits) / float64(total) * 100
	}

	return map[string]interface{}{
		"hits":      c.hits,
		"misses":    c.misses,
		"hit_rate":  hitRate,
		"evictions": c.evictions,
		"size":      len(c.entries),
		"max_size":  c.maxSize,
		"enabled":   c.enabled,
	}
}

// SetEnabled 设置是否启用
func (c *ResponseCache) SetEnabled(enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.enabled = enabled
	if !enabled {
		// 禁用时清空缓存
		c.entries = make(map[string]*CacheEntry)
	}
}

// GenerateCacheKey 生成缓存键
func GenerateCacheKey(request interface{}) (string, error) {
	// 序列化请求
	data, err := json.Marshal(request)
	if err != nil {
		return "", err
	}

	// 计算 SHA256 哈希
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

// CleanupExpired 清理过期条目
func (c *ResponseCache) CleanupExpired() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	expired := 0
	now := time.Now()

	for key, entry := range c.entries {
		if now.After(entry.ExpiresAt) {
			delete(c.entries, key)
			expired++
		}
	}

	return expired
}
