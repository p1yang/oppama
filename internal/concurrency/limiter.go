package concurrency

import (
	"context"
	"sync"
)

// Limiter 全局并发限制器
type Limiter struct {
	maxConcurrent int
	currentCount  int
	mu            sync.Mutex
	sem           chan struct{}
	waiting       int // 等待中的任务数
}

// NewLimiter 创建并发限制器
func NewLimiter(maxConcurrent int) *Limiter {
	if maxConcurrent <= 0 {
		maxConcurrent = 10 // 默认值
	}

	return &Limiter{
		maxConcurrent: maxConcurrent,
		sem:           make(chan struct{}, maxConcurrent),
	}
}

// Acquire 获取并发许可（阻塞直到获取到许可）
func (l *Limiter) Acquire(ctx context.Context) error {
	l.mu.Lock()
	l.waiting++
	l.mu.Unlock()

	select {
	case l.sem <- struct{}{}:
		l.mu.Lock()
		l.waiting--
		l.currentCount++
		l.mu.Unlock()
		return nil
	case <-ctx.Done():
		l.mu.Lock()
		l.waiting--
		l.mu.Unlock()
		return ctx.Err()
	}
}

// TryAcquire 尝试获取并发许可（非阻塞）
func (l *Limiter) TryAcquire() bool {
	select {
	case l.sem <- struct{}{}:
		l.mu.Lock()
		l.currentCount++
		l.mu.Unlock()
		return true
	default:
		return false
	}
}

// Release 释放并发许可
func (l *Limiter) Release() {
	<-l.sem
	l.mu.Lock()
	l.currentCount--
	l.mu.Unlock()
}

// GetCurrentCount 获取当前并发数
func (l *Limiter) GetCurrentCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.currentCount
}

// GetWaitingCount 获取等待中的任务数
func (l *Limiter) GetWaitingCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.waiting
}

// GetMaxConcurrent 获取最大并发数
func (l *Limiter) GetMaxConcurrent() int {
	return l.maxConcurrent
}

// SetMaxConcurrent 设置最大并发数
func (l *Limiter) SetMaxConcurrent(max int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if max <= 0 {
		return
	}

	oldMax := l.maxConcurrent
	l.maxConcurrent = max

	// 如果新的限制更大，需要更新信号量
	if max > oldMax {
		newSem := make(chan struct{}, max)
		// 复制旧信号量中的许可
		for i := 0; i < oldMax; i++ {
			select {
			case <-l.sem:
				newSem <- struct{}{}
			default:
				break
			}
		}
		l.sem = newSem
	}
}

// Stats 获取统计信息
func (l *Limiter) Stats() map[string]interface{} {
	l.mu.Lock()
	defer l.mu.Unlock()

	return map[string]interface{}{
		"current":       l.currentCount,
		"waiting":       l.waiting,
		"max_concurrent": l.maxConcurrent,
		"available":     l.maxConcurrent - l.currentCount,
	}
}
