package task

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"oppama/internal/storage"
)

// TaskType 任务类型
type TaskType string

const (
	TaskTypeServiceCheck    TaskType = "service-check"
	TaskTypeBatchCheck      TaskType = "batch-check"
	TaskTypeDiscoverySearch TaskType = "discovery-search"
	TaskTypeModelSync       TaskType = "model-sync"
)

// Task 异步任务
type Task struct {
	ID          string                 `json:"id"`
	Type        TaskType               `json:"type"`
	Title       string                 `json:"title"`
	Status      storage.TaskStatus     `json:"status"`
	Progress    int                    `json:"progress"`
	Total       int                    `json:"total"`
	Result      map[string]interface{} `json:"result,omitempty"`
	Error       string                 `json:"error,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`

	// 内部字段
	cancelFunc context.CancelFunc
	mu         sync.RWMutex
}

// Manager 任务管理器
type Manager struct {
	tasks    map[string]*Task
	mu       sync.RWMutex
	storage  storage.TaskStorage
	callback map[string][]func(*Task)
}

// NewManager 创建任务管理器
func NewManager(storage storage.TaskStorage) *Manager {
	m := &Manager{
		tasks:    make(map[string]*Task),
		storage:  storage,
		callback: make(map[string][]func(*Task)),
	}

	// 启动清理协程
	go m.cleanupRoutine()

	return m
}

// CreateTask 创建新任务
func (m *Manager) CreateTask(taskType TaskType, title string, total int) *Task {
	task := &Task{
		ID:        generateTaskID(),
		Type:      taskType,
		Title:     title,
		Status:    storage.TaskPending,
		Progress:  0,
		Total:     total,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	m.mu.Lock()
	m.tasks[task.ID] = task
	m.mu.Unlock()

	// 保存到存储
	if m.storage != nil {
		m.storage.SaveTask(context.Background(), taskToStorage(task))
	}

	return task
}

// GetTask 获取任务
func (m *Manager) GetTask(id string) *Task {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.tasks[id]
}

// UpdateTask 更新任务
func (m *Manager) UpdateTask(id string, updates func(*Task)) {
	m.mu.Lock()
	task := m.tasks[id]
	if task == nil {
		m.mu.Unlock()
		return
	}

	task.mu.Lock()
	updates(task)
	task.UpdatedAt = time.Now()
	task.mu.Unlock()

	m.mu.Unlock()

	// 保存到存储
	if m.storage != nil {
		m.storage.SaveTask(context.Background(), taskToStorage(task))
	}

	// 触发回调
	m.triggerCallbacks(task)
}

// SetTaskStatus 设置任务状态
func (m *Manager) SetTaskStatus(id string, status storage.TaskStatus) {
	m.UpdateTask(id, func(t *Task) {
		t.Status = status
		if status == storage.TaskCompleted || status == storage.TaskFailed {
			now := time.Now()
			t.CompletedAt = &now
			if status == storage.TaskCompleted {
				t.Progress = t.Total
			}
		}
	})
}

// IncrementProgress 增加任务进度
func (m *Manager) IncrementProgress(id string, delta int) {
	m.UpdateTask(id, func(t *Task) {
		t.Progress += delta
		if t.Progress > t.Total {
			t.Progress = t.Total
		}
	})
}

// SetProgress 设置任务进度
func (m *Manager) SetProgress(id string, progress int) {
	m.UpdateTask(id, func(t *Task) {
		t.Progress = progress
		if t.Progress > t.Total {
			t.Progress = t.Total
		}
	})
}

// SetTaskResult 设置任务结果
func (m *Manager) SetTaskResult(id string, result map[string]interface{}) {
	m.UpdateTask(id, func(t *Task) {
		t.Result = result
	})
}

// SetTaskError 设置任务错误
func (m *Manager) SetTaskError(id string, err error) {
	m.UpdateTask(id, func(t *Task) {
		t.Status = storage.TaskFailed
		if err != nil {
			t.Error = err.Error()
		}
		now := time.Now()
		t.CompletedAt = &now
	})
}

// CompleteTask 完成任务
func (m *Manager) CompleteTask(id string, result map[string]interface{}) {
	m.UpdateTask(id, func(t *Task) {
		t.Status = storage.TaskCompleted
		t.Progress = t.Total
		t.Result = result
		now := time.Now()
		t.CompletedAt = &now
	})
}

// RunTask 运行任务（带超时控制）
func (m *Manager) RunTask(ctx context.Context, task *Task, timeout time.Duration, fn func(context.Context, *Task) error) {
	// 创建带取消的上下文
	taskCtx, cancel := context.WithTimeout(ctx, timeout)
	task.cancelFunc = cancel

	// 更新状态为运行中
	m.SetTaskStatus(task.ID, storage.TaskRunning)

	// 在后台执行
	go func() {
		defer func() {
			if r := recover(); r != nil {
				m.SetTaskError(task.ID, fmt.Errorf("任务 panic: %v", r))
			}
		}()

		if err := fn(taskCtx, task); err != nil {
			m.SetTaskError(task.ID, err)
		} else {
			if task.Status != storage.TaskFailed {
				m.CompleteTask(task.ID, task.Result)
			}
		}
	}()
}

// CancelTask 取消任务
func (m *Manager) CancelTask(id string) bool {
	m.mu.RLock()
	task := m.tasks[id]
	m.mu.RUnlock()

	if task == nil || task.cancelFunc == nil {
		return false
	}

	task.cancelFunc()
	m.SetTaskStatus(id, storage.TaskFailed)
	m.SetTaskResult(id, map[string]interface{}{"cancelled": true})
	return true
}

// RegisterCallback 注册任务状态变更回调
func (m *Manager) RegisterCallback(id string, callback func(*Task)) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.callback[id] = append(m.callback[id], callback)
}

// triggerCallback 触发回调
func (m *Manager) triggerCallbacks(task *Task) {
	m.mu.RLock()
	callbacks := m.callback[task.ID]
	m.mu.RUnlock()

	for _, cb := range callbacks {
		go cb(task)
	}
}

// GetActiveTasks 获取活动任务
func (m *Manager) GetActiveTasks() []*Task {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var active []*Task
	for _, task := range m.tasks {
		if task.Status == storage.TaskPending || task.Status == storage.TaskRunning {
			active = append(active, task)
		}
	}
	return active
}

// GetTasksByType 获取指定类型的任务
func (m *Manager) GetTasksByType(taskType TaskType, limit int) []*Task {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var filtered []*Task
	for _, task := range m.tasks {
		if task.Type == taskType {
			filtered = append(filtered, task)
			if limit > 0 && len(filtered) >= limit {
				break
			}
		}
	}
	return filtered
}

// cleanupRoutine 清理已完成/失败的超期任务
func (m *Manager) cleanupRoutine() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		m.mu.Lock()
		now := time.Now()
		for id, task := range m.tasks {
			task.mu.RLock()
			completed := task.CompletedAt != nil && now.Sub(*task.CompletedAt) > 30*time.Minute
			task.mu.RUnlock()

			if completed {
				delete(m.tasks, id)
				delete(m.callback, id)
			}
		}
		m.mu.Unlock()
	}
}

// generateTaskID 生成任务 ID
func generateTaskID() string {
	return fmt.Sprintf("task_%d_%s", time.Now().UnixNano(), randomString(8))
}

// randomString 生成随机字符串
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	// 使用时间戳作为随机种子（对于生成 ID 来说足够安全）
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := range b {
		b[i] = letters[rng.Intn(len(letters))]
	}
	return string(b)
}

// taskToStorage 转换为存储格式
func taskToStorage(t *Task) *storage.Task {
	t.mu.RLock()
	defer t.mu.RUnlock()

	resultBytes, _ := json.Marshal(t.Result)

	return &storage.Task{
		ID:          t.ID,
		Type:        string(t.Type),
		Title:       t.Title,
		Status:      t.Status,
		Progress:    t.Progress,
		Total:       t.Total,
		Result:      string(resultBytes),
		Error:       t.Error,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
		CompletedAt: t.CompletedAt,
	}
}
