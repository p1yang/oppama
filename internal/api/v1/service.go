package v1

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"oppama/internal/config"
	"oppama/internal/detector"
	"oppama/internal/storage"
	taskpkg "oppama/internal/task"

	"github.com/gin-gonic/gin"
)

// ServiceHandler 服务管理处理器
type ServiceHandler struct {
	storage    storage.Storage
	detector   *detector.Detector
	taskMgr    *taskpkg.Manager
	cfg        *config.Config
	configPath string
	mu         sync.RWMutex
}

// NewServiceHandler 创建服务处理器
func NewServiceHandler(storage storage.Storage, cfg *config.Config, configPath string, taskMgr *taskpkg.Manager) *ServiceHandler {
	h := &ServiceHandler{
		storage:    storage,
		cfg:        cfg,
		configPath: configPath,
		taskMgr:    taskMgr,
	}
	h.detector = h.createDetector()
	return h
}

// createDetector 根据当前配置创建检测器
func (h *ServiceHandler) createDetector() *detector.Detector {
	// 构建虚假版本映射
	fakeVersions := make(map[string]bool)
	for _, v := range h.cfg.Detector.HoneypotDetection.FakeVersions {
		fakeVersions[v] = true
	}
	// 添加默认的虚假版本
	if fakeVersions["0.0.0"] == false {
		fakeVersions["0.0.0"] = true
	}
	if fakeVersions["unknown"] == false {
		fakeVersions["unknown"] = true
	}

	return detector.NewDetector(&detector.DetectorConfig{
		Timeout:         time.Duration(h.cfg.Detector.Timeout) * time.Second,
		Concurrency:     h.cfg.Detector.Concurrency,
		CheckHoneypot:   h.cfg.Detector.HoneypotDetection.Enabled,
		CheckModels:     true,
		SuspiciousPorts: h.cfg.Detector.HoneypotDetection.SuspiciousPorts,
		FakeVersions:    fakeVersions,
	})
}

// ReloadDetector 重新加载检测器配置
func (h *ServiceHandler) ReloadDetector() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// 重新加载配置文件
	updatedCfg, err := config.Load(h.configPath)
	if err != nil {
		return err
	}

	// 更新配置指针
	h.cfg = updatedCfg

	// 重新创建检测器
	h.detector = h.createDetector()

	fmt.Printf("[ServiceHandler] 检测器配置已重新加载: timeout=%v, concurrency=%d, honeypot=%v\n",
		time.Duration(h.cfg.Detector.Timeout)*time.Second,
		h.cfg.Detector.Concurrency,
		h.cfg.Detector.HoneypotDetection.Enabled)

	return nil
}

// ListServices 获取服务列表
func (h *ServiceHandler) ListServices(c *gin.Context) {
	// 创建带超时的上下文（快速失败，避免被检测任务阻塞）
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	var filter storage.ServiceFilter

	// 解析查询参数
	if status := c.Query("status"); status != "" {
		s := storage.ServiceStatus(status)
		filter.Status = &s
	}
	if source := c.Query("source"); source != "" {
		src := storage.DiscoverySource(source)
		filter.Source = &src
	}
	if search := c.Query("search"); search != "" {
		filter.Search = search
	}

	// 绑定查询参数（包括分页）
	c.ShouldBindQuery(&filter)

	// 设置默认分页参数
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}

	services, err := h.storage.ListServices(ctx, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 获取总数（用于分页）- 使用更简单的计数方式
	countFilter := filter
	countFilter.Page = 0
	countFilter.PageSize = 0
	totalServices, err := h.storage.ListServices(ctx, countFilter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  services,
		"total": len(totalServices),
	})
}

// CreateService 创建服务
func (h *ServiceHandler) CreateService(c *gin.Context) {
	var req struct {
		URL  string `json:"url" binding:"required"`
		Name string `json:"name"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	service := &storage.OllamaService{
		ID:        generateID(),
		URL:       req.URL,
		Name:      req.Name,
		Status:    storage.StatusUnknown,
		Source:    storage.SourceManual,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := h.storage.SaveService(c.Request.Context(), service); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 记录活动日志
	activityLog := &storage.ActivityLog{
		Type:     storage.ActivityAdd,
		Action:   "添加服务",
		Target:   service.URL,
		UserID:   c.GetString("user_id"),
		Metadata: fmt.Sprintf(`{"service_id":"%s","service_name":"%s"}`, service.ID, service.Name),
	}
	h.storage.SaveActivityLog(c.Request.Context(), activityLog)

	c.JSON(http.StatusCreated, gin.H{"data": service})
}

// GetService 获取服务详情
func (h *ServiceHandler) GetService(c *gin.Context) {
	id := c.Param("id")

	service, err := h.storage.GetService(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if service == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "服务不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": service})
}

// UpdateService 更新服务
func (h *ServiceHandler) UpdateService(c *gin.Context) {
	id := c.Param("id")

	var req storage.OllamaService
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req.ID = id
	req.UpdatedAt = time.Now()

	if err := h.storage.SaveService(c.Request.Context(), &req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": req})
}

// DeleteService 删除服务
func (h *ServiceHandler) DeleteService(c *gin.Context) {
	id := c.Param("id")

	// 获取要删除的服务信息（用于记录日志）
	service, err := h.storage.GetService(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := h.storage.DeleteService(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 记录活动日志
	activityLog := &storage.ActivityLog{
		Type:     storage.ActivityDelete,
		Action:   "删除服务",
		Target:   service.URL,
		UserID:   c.GetString("user_id"), // 从上下文中获取用户 ID
		Metadata: fmt.Sprintf(`{"service_id":"%s","service_name":"%s"}`, id, service.Name),
	}
	h.storage.SaveActivityLog(c.Request.Context(), activityLog)

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// CheckService 检测服务
func (h *ServiceHandler) CheckService(c *gin.Context) {
	id := c.Param("id")

	// 检查是否为异步模式
	asyncMode := c.Query("async") == "true"

	service, err := h.storage.GetService(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if service == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "服务不存在"})
		return
	}

	// 异步检测模式
	if asyncMode {
		// 先更新状态为检测中
		service.Status = "checking"
		h.storage.SaveService(c.Request.Context(), service)

		// 创建任务
		t := h.taskMgr.CreateTask(
			taskpkg.TaskTypeServiceCheck,
			"检测服务: "+service.Name,
			100,
		)

		// 在后台执行检测
		h.taskMgr.RunTask(c.Request.Context(), t, 60*time.Second, func(ctx context.Context, task *taskpkg.Task) error {
			// 执行检测
			result, err := h.detector.Detect(ctx, service.URL)
			if err != nil {
				return err
			}

			// 更新服务信息
			if result.IsValid && !result.IsHoneypot {
				service.Status = storage.StatusOnline
			} else if result.IsHoneypot {
				service.Status = storage.StatusHoneypot
			} else {
				service.Status = storage.StatusOffline
			}
			service.Version = result.Version
			service.ResponseTime = result.ResponseTime
			service.IsHoneypot = result.IsHoneypot
			service.LastChecked = time.Now()
			service.Models = result.Models

			if err := h.storage.SaveService(context.Background(), service); err != nil {
				return err
			}

			if len(result.Models) > 0 {
				h.storage.SaveModels(context.Background(), service.ID, result.Models)
			}

			// 更新任务结果
			h.taskMgr.SetTaskResult(task.ID, map[string]interface{}{
				"service_id": service.ID,
				"status":     service.Status,
				"version":    result.Version,
				"models":     len(result.Models),
			})

			h.taskMgr.SetProgress(task.ID, 100)
			return nil
		})

		c.JSON(http.StatusOK, gin.H{
			"data": gin.H{
				"task_id": t.ID,
				"message": "检测任务已启动",
			},
		})
		return
	}

	// 同步检测模式（原有逻辑）
	result, err := h.detector.Detect(c.Request.Context(), service.URL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 更新服务信息
	if result.IsValid && !result.IsHoneypot {
		service.Status = storage.StatusOnline
	} else if result.IsHoneypot {
		service.Status = storage.StatusHoneypot
	} else {
		service.Status = storage.StatusOffline
	}
	service.Version = result.Version
	service.ResponseTime = result.ResponseTime
	service.IsHoneypot = result.IsHoneypot
	service.LastChecked = time.Now()
	service.Models = result.Models

	if err := h.storage.SaveService(c.Request.Context(), service); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if len(result.Models) > 0 {
		if err := h.storage.SaveModels(c.Request.Context(), service.ID, result.Models); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	// 记录活动日志
	var activityType storage.ActivityType
	var action string
	if result.IsHoneypot {
		activityType = storage.ActivityWarning
		action = "检测到蜜罐服务"
	} else if result.IsValid {
		activityType = storage.ActivityCheck
		action = "完成服务健康检查"
	} else {
		activityType = storage.ActivityError
		action = "服务检查失败"
	}

	activityLog := &storage.ActivityLog{
		Type:     activityType,
		Action:   action,
		Target:   service.URL,
		UserID:   c.GetString("user_id"),
		Metadata: fmt.Sprintf(`{"service_id":"%s","status":"%s","is_honeypot":%v}`, service.ID, service.Status, result.IsHoneypot),
	}
	h.storage.SaveActivityLog(c.Request.Context(), activityLog)

	c.JSON(http.StatusOK, gin.H{"data": result})
}

// BatchCheck 批量检测
func (h *ServiceHandler) BatchCheck(c *gin.Context) {
	var req storage.BatchDetectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Concurrency <= 0 {
		req.Concurrency = 10
	}
	if req.Timeout <= 0 {
		req.Timeout = 30
	}

	// 创建批量检测任务
	batchTask := h.taskMgr.CreateTask(
		taskpkg.TaskTypeBatchCheck,
		"批量检测 "+string(rune(len(req.URLs)))+" 个服务",
		len(req.URLs),
	)

	// 在后台执行批量检测
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(req.Timeout)*time.Second)
		defer cancel()

		// 使用动态并发控制
		sem := make(chan struct{}, req.Concurrency)
		var wg sync.WaitGroup
		var mu sync.Mutex
		successCount := 0
		foundCount := 0

		// 批量保存的缓冲区
		const batchSize = 50 // 从20增加到50，减少刷新频率
		serviceBuffer := make([]*storage.OllamaService, 0, batchSize)
		modelBuffer := make(map[string][]storage.ModelInfo)
		bufferMu := sync.Mutex{}

		// 刷新缓冲区的函数
		flushBuffer := func() {
			bufferMu.Lock()
			defer bufferMu.Unlock()

			if len(serviceBuffer) > 0 {
				// 批量保存服务
				for _, svc := range serviceBuffer {
					h.storage.SaveService(context.Background(), svc)
					if models, ok := modelBuffer[svc.ID]; ok && len(models) > 0 {
						h.storage.SaveModels(context.Background(), svc.ID, models)
					}
				}
				serviceBuffer = serviceBuffer[:0]
				modelBuffer = make(map[string][]storage.ModelInfo)
			}
		}

		for i, url := range req.URLs {
			wg.Add(1)
			go func(idx int, urlStr string) {
				defer wg.Done()

				// 检查上下文是否取消
				select {
				case <-ctx.Done():
					return
				default:
				}

				sem <- struct{}{}        // 获取令牌
				defer func() { <-sem }() // 释放令牌

				// 执行检测
				result, err := h.detector.Detect(ctx, urlStr)

				// 更新进度
				h.taskMgr.IncrementProgress(batchTask.ID, 1)

				if err == nil && result.IsValid {
					mu.Lock()
					successCount++
					if result.Models != nil {
						foundCount += len(result.Models)
					}
					mu.Unlock()

					// 保存到缓冲区
					service := &storage.OllamaService{
						ID:           generateID(),
						URL:          result.URL,
						Status:       storage.StatusOnline,
						Version:      result.Version,
						ResponseTime: result.ResponseTime,
						IsHoneypot:   result.IsHoneypot,
						Models:       result.Models,
						Source:       storage.SourceImport,
						CreatedAt:    time.Now(),
					}

					bufferMu.Lock()
					serviceBuffer = append(serviceBuffer, service)
					if len(result.Models) > 0 {
						modelBuffer[service.ID] = result.Models
					}

					// 如果缓冲区满了，刷新
					if len(serviceBuffer) >= batchSize {
						flushBuffer()
					}
					bufferMu.Unlock()
				}
			}(i, url)
		}

		// 等待所有检测完成
		wg.Wait()

		// 刷新最后的缓冲区
		flushBuffer()

		// 完成任务
		h.taskMgr.CompleteTask(batchTask.ID, map[string]interface{}{
			"total":       len(req.URLs),
			"success":     successCount,
			"found_count": foundCount,
		})
	}()

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"task_id": batchTask.ID,
			"message": "批量检测任务已启动",
		},
	})
}

// CheckAllServices 一键检测所有服务
func (h *ServiceHandler) CheckAllServices(c *gin.Context) {
	ctx := c.Request.Context()

	// 获取所有服务
	services, err := h.storage.ListServices(ctx, storage.ServiceFilter{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if len(services) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"data": gin.H{
				"task_id": "",
				"message": "没有需要检测的服务",
			},
		})
		return
	}

	// 创建批量检测任务
	batchTask := h.taskMgr.CreateTask(
		taskpkg.TaskTypeBatchCheck,
		fmt.Sprintf("一键检测 %d 个服务", len(services)),
		len(services),
	)

	// 在后台执行批量检测
	go func() {
		taskCtx, cancel := context.WithTimeout(context.Background(), time.Duration(len(services))*time.Duration(h.cfg.Detector.Timeout)*time.Second)
		defer cancel()

		// 使用动态并发控制 - 保留至少 2 个并发给其他请求
		maxConcurrency := h.cfg.Detector.Concurrency
		if maxConcurrency > 2 {
			maxConcurrency = maxConcurrency - 2
		}
		sem := make(chan struct{}, maxConcurrency)
		var wg sync.WaitGroup
		var mu sync.Mutex
		successCount := 0
		foundCount := 0
		offlineCount := 0
		honeypotCount := 0

		// 批量保存的缓冲区（添加缓冲逻辑减少锁竞争）
		const batchSize = 50
		type bufferedService struct {
			service *storage.OllamaService
			models  []storage.ModelInfo
		}
		updateBuffer := make([]bufferedService, 0, batchSize)
		offlineBuffer := make([]*storage.OllamaService, 0, batchSize)
		bufferMu := sync.Mutex{}

		// 刷新缓冲区的函数
		flushBuffer := func() {
			bufferMu.Lock()
			defer bufferMu.Unlock()

			// 处理更新缓冲区
			if len(updateBuffer) > 0 {
				for _, bs := range updateBuffer {
					if err := h.storage.SaveService(context.Background(), bs.service); err != nil {
						fmt.Printf("[CheckAllServices] 保存服务失败：%v\n", err)
					}

					if len(bs.models) > 0 {
						if err := h.storage.SaveModels(context.Background(), bs.service.ID, bs.models); err != nil {
							fmt.Printf("[CheckAllServices] 保存模型失败：%v\n", err)
						}
					}
				}
				updateBuffer = updateBuffer[:0]
			}

			// 处理离线缓冲区
			if len(offlineBuffer) > 0 {
				for _, svc := range offlineBuffer {
					h.storage.SaveService(context.Background(), svc)
				}
				offlineBuffer = offlineBuffer[:0]
			}
		}

		// 定期检查缓冲区是否需要刷新（基于大小）
		tryFlushBuffer := func() {
			bufferMu.Lock()

			// 检查是否需要刷新
			shouldFlush := len(updateBuffer) >= batchSize || len(offlineBuffer) >= batchSize
			if !shouldFlush {
				bufferMu.Unlock()
				return
			}

			// 临时复制缓冲区，释放锁后执行保存
			tempUpdates := make([]bufferedService, len(updateBuffer))
			copy(tempUpdates, updateBuffer)
			updateBuffer = updateBuffer[:0]

			tempOfflines := make([]*storage.OllamaService, len(offlineBuffer))
			copy(tempOfflines, offlineBuffer)
			offlineBuffer = offlineBuffer[:0]

			bufferMu.Unlock()

			// 批量保存（不在锁内执行，避免阻塞其他goroutine）
			for _, bs := range tempUpdates {
				if err := h.storage.SaveService(context.Background(), bs.service); err != nil {
					fmt.Printf("[CheckAllServices] 保存服务失败：%v\n", err)
				}
				if len(bs.models) > 0 {
					h.storage.SaveModels(context.Background(), bs.service.ID, bs.models)
				}
			}
			for _, svc := range tempOfflines {
				h.storage.SaveService(context.Background(), svc)
			}
		}

		for _, service := range services {
			wg.Add(1)
			go func(svc *storage.OllamaService) {
				defer wg.Done()

				// 检查上下文是否取消
				select {
				case <-taskCtx.Done():
					return
				default:
				}

				sem <- struct{}{}        // 获取令牌
				defer func() { <-sem }() // 释放令牌

				// 先更新状态为检测中（这个需要立即更新，让前端看到状态变化）
				mu.Lock()
				h.storage.SaveService(context.Background(), &storage.OllamaService{
					ID:        svc.ID,
					Status:    "checking",
					UpdatedAt: time.Now(),
				})
				mu.Unlock()

				// 执行检测
				result, err := h.detector.Detect(taskCtx, svc.URL)

				// 更新进度
				h.taskMgr.IncrementProgress(batchTask.ID, 1)

				if err == nil && result.IsValid {
					mu.Lock()
					successCount++
					if result.Models != nil {
						foundCount += len(result.Models)
					}
					mu.Unlock()

					// 更新服务信息
					if result.IsValid && !result.IsHoneypot {
						svc.Status = storage.StatusOnline
					} else if result.IsHoneypot {
						svc.Status = storage.StatusHoneypot
						honeypotCount++
					} else {
						svc.Status = storage.StatusOffline
						offlineCount++
					}
					svc.Version = result.Version
					svc.ResponseTime = result.ResponseTime
					svc.IsHoneypot = result.IsHoneypot
					svc.LastChecked = time.Now()
					svc.Models = result.Models
					svc.UpdatedAt = time.Now()

					// 添加到更新缓冲区而不是直接保存
					bufferMu.Lock()
					updateBuffer = append(updateBuffer, bufferedService{
						service: svc,
						models:  result.Models,
					})
					bufferMu.Unlock()

					// 检查是否需要刷新缓冲区
					tryFlushBuffer()
				} else {
					// 检测失败，标记为离线
					mu.Lock()
					offlineCount++
					mu.Unlock()

					svc.Status = storage.StatusOffline
					svc.LastChecked = time.Now()
					svc.UpdatedAt = time.Now()

					// 添加到离线缓冲区而不是直接保存
					bufferMu.Lock()
					offlineBuffer = append(offlineBuffer, svc)
					bufferMu.Unlock()

					// 检查是否需要刷新缓冲区
					tryFlushBuffer()
				}
			}(service)
		}

		// 等待所有检测完成
		wg.Wait()

		// 刷新所有剩余的缓冲区数据
		flushBuffer()

		// 完成任务
		h.taskMgr.CompleteTask(batchTask.ID, map[string]interface{}{
			"total":       len(services),
			"success":     successCount,
			"found_count": foundCount,
			"offline":     offlineCount,
			"honeypot":    honeypotCount,
		})
	}()

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"task_id": batchTask.ID,
			"message": fmt.Sprintf("已启动检测 %d 个服务", len(services)),
		},
	})
}

// GetModels 获取服务的模型列表
func (h *ServiceHandler) GetModels(c *gin.Context) {
	id := c.Param("id")

	models, err := h.storage.GetModelsByService(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": models})
}

// generateID 生成唯一 ID
func generateID() string {
	return time.Now().Format("20060102150405") + "_" + randomString(8)
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
		time.Sleep(time.Nanosecond)
	}
	return string(b)
}

// GetServiceTask 获取服务检测任务状态
func (h *ServiceHandler) GetServiceTask(c *gin.Context) {
	taskID := c.Param("taskId")

	task := h.taskMgr.GetTask(taskID)
	if task == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"id":           task.ID,
			"type":         task.Type,
			"title":        task.Title,
			"status":       task.Status,
			"progress":     task.Progress,
			"total":        task.Total,
			"result":       task.Result,
			"error":        task.Error,
			"created_at":   task.CreatedAt,
			"updated_at":   task.UpdatedAt,
			"completed_at": task.CompletedAt,
		},
	})
}

// GetStats 获取服务统计数据
func (h *ServiceHandler) GetStats(c *gin.Context) {
	stats, err := h.storage.GetStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 添加检测器状态信息
	detectorStatus := gin.H{
		"active_tasks":   h.detector.GetActiveTasks(),
		"is_overloaded":  h.detector.IsOverloaded(),
		"max_concurrent": h.cfg.Detector.Concurrency,
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"total":           stats.TotalServices,
			"online":          stats.OnlineServices,
			"offline":         stats.OfflineServices,
			"honeypot":        stats.HoneypotServices,
			"detector_status": detectorStatus,
		},
	})
}

// GetRecentActivities 获取最近活动
func (h *ServiceHandler) GetRecentActivities(c *gin.Context) {
	limit := 20 // 默认返回最近 20 条
	if l := c.Query("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
		if limit > 100 {
			limit = 100 // 最多 100 条
		}
	}

	// 支持按服务 ID 筛选
	serviceID := c.Query("service_id")

	var activities []*storage.ActivityLog
	var err error

	if serviceID != "" {
		activities, err = h.storage.ListActivitiesByService(c.Request.Context(), serviceID, limit)
	} else {
		activities, err = h.storage.ListRecentActivities(c.Request.Context(), limit)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  activities,
		"total": len(activities),
	})
}
