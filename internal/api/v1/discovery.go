package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"oppama/internal/config"
	"oppama/internal/discovery"
	"oppama/internal/storage"
	"oppama/internal/task"
	taskpkg "oppama/internal/task"

	"github.com/gin-gonic/gin"
)

// DiscoveryHandler 服务发现处理器
type DiscoveryHandler struct {
	storage    storage.Storage
	engineMgr  *discovery.EngineManager
	config     *config.Config
	configPath string
	taskMgr    *task.Manager
	detector   Detector // 添加检测器接口
	mu         sync.RWMutex
}

// Detector 检测器接口（简化版）
type Detector interface {
	Detect(ctx context.Context, url string) (*DetectionResult, error)
}

// DetectionResult 检测结果（简化版）
type DetectionResult struct {
	URL          string
	IsValid      bool
	Version      string
	ResponseTime time.Duration
	IsHoneypot   bool
	Models       []storage.ModelInfo
}

// NewDiscoveryHandler 创建服务发现处理器
func NewDiscoveryHandler(storage storage.Storage, cfg *config.Config, configPath string, taskMgr *task.Manager) *DiscoveryHandler {
	// 创建引擎管理器
	engineCfg := &discovery.EngineConfig{
		FOFA: discovery.FOFAConfig{
			Enabled:    cfg.Discovery.Engines.FOFA.Enabled,
			Email:      cfg.Discovery.Engines.FOFA.Email,
			Key:        cfg.Discovery.Engines.FOFA.Key,
			Query:      cfg.Discovery.Engines.FOFA.Query,
			MaxResults: cfg.Discovery.Engines.FOFA.MaxResults,
		},
		Hunter: discovery.HunterConfig{
			Enabled:    cfg.Discovery.Engines.Hunter.Enabled,
			Key:        cfg.Discovery.Engines.Hunter.Key,
			Query:      cfg.Discovery.Engines.Hunter.Query,
			MaxResults: cfg.Discovery.Engines.Hunter.MaxResults,
		},
		Shodan: discovery.ShodanConfig{
			Enabled:    cfg.Discovery.Engines.Shodan.Enabled,
			Key:        cfg.Discovery.Engines.Shodan.Key,
			Query:      cfg.Discovery.Engines.Shodan.Query,
			MaxResults: cfg.Discovery.Engines.Shodan.MaxResults,
		},
	}

	// 创建简单的检测器
	detector := createSimpleDetector()

	return &DiscoveryHandler{
		storage:    storage,
		engineMgr:  discovery.NewEngineManager(engineCfg),
		config:     cfg,
		configPath: configPath,
		taskMgr:    taskMgr,
		detector:   detector,
	}
}

// createSimpleDetector 创建简单的检测器
func createSimpleDetector() Detector {
	return &simpleDetector{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// simpleDetector 简单检测器实现
type simpleDetector struct {
	client *http.Client
}

// Detect 检测服务
func (d *simpleDetector) Detect(ctx context.Context, urlStr string) (*DetectionResult, error) {
	result := &DetectionResult{
		URL: urlStr,
	}

	// 标准化 URL
	urlStr = strings.TrimRight(urlStr, "/")
	if !strings.HasPrefix(urlStr, "http://") && !strings.HasPrefix(urlStr, "https://") {
		urlStr = "http://" + urlStr
	}

	// 检查版本信息
	version, responseTime, err := d.checkVersion(ctx, urlStr)
	if err != nil {
		result.IsValid = false
		return result, nil
	}

	result.Version = version
	result.ResponseTime = responseTime
	result.IsValid = (version != "")

	// 获取模型列表
	models, err := d.checkModels(ctx, urlStr)
	if err == nil && len(models) > 0 {
		result.Models = models
	}

	return result, nil
}

func (d *simpleDetector) checkVersion(ctx context.Context, baseURL string) (string, time.Duration, error) {
	url := fmt.Sprintf("%s/api/version", baseURL)

	startTime := time.Now()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", 0, err
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	responseTime := time.Since(startTime)

	if resp.StatusCode != http.StatusOK {
		return "", responseTime, fmt.Errorf("状态码：%d", resp.StatusCode)
	}

	var versionResp struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&versionResp); err != nil {
		return "", responseTime, err
	}

	return versionResp.Version, responseTime, nil
}

func (d *simpleDetector) checkModels(ctx context.Context, baseURL string) ([]storage.ModelInfo, error) {
	url := fmt.Sprintf("%s/api/tags", baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("状态码：%d", resp.StatusCode)
	}

	var modelsResp struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, err
	}

	modelInfos := make([]storage.ModelInfo, 0, len(modelsResp.Models))
	for _, m := range modelsResp.Models {
		modelInfos = append(modelInfos, storage.ModelInfo{
			Name: m.Name,
		})
	}

	return modelInfos, nil
}

// ReloadEngines 重新加载搜索引擎配置
func (h *DiscoveryHandler) ReloadEngines() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// 重新加载配置文件
	updatedCfg, err := config.Load(h.configPath)
	if err != nil {
		return err
	}

	// 更新配置指针
	h.config = updatedCfg

	// 重新创建引擎管理器
	engineCfg := &discovery.EngineConfig{
		FOFA: discovery.FOFAConfig{
			Enabled:    updatedCfg.Discovery.Engines.FOFA.Enabled,
			Email:      updatedCfg.Discovery.Engines.FOFA.Email,
			Key:        updatedCfg.Discovery.Engines.FOFA.Key,
			Query:      updatedCfg.Discovery.Engines.FOFA.Query,
			MaxResults: updatedCfg.Discovery.Engines.FOFA.MaxResults,
		},
		Hunter: discovery.HunterConfig{
			Enabled:    updatedCfg.Discovery.Engines.Hunter.Enabled,
			Key:        updatedCfg.Discovery.Engines.Hunter.Key,
			Query:      updatedCfg.Discovery.Engines.Hunter.Query,
			MaxResults: updatedCfg.Discovery.Engines.Hunter.MaxResults,
		},
		Shodan: discovery.ShodanConfig{
			Enabled:    updatedCfg.Discovery.Engines.Shodan.Enabled,
			Key:        updatedCfg.Discovery.Engines.Shodan.Key,
			Query:      updatedCfg.Discovery.Engines.Shodan.Query,
			MaxResults: updatedCfg.Discovery.Engines.Shodan.MaxResults,
		},
	}

	h.engineMgr = discovery.NewEngineManager(engineCfg)
	return nil
}

// Search 执行服务发现搜索
func (h *DiscoveryHandler) Search(c *gin.Context) {
	var req struct {
		Engines    []storage.DiscoverySource `json:"engines"`
		Query      string                    `json:"query"`
		MaxResults int                       `json:"max_results"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.MaxResults <= 0 {
		req.MaxResults = 100
	}

	// 创建任务
	task := h.taskMgr.CreateTask(
		task.TaskTypeDiscoverySearch,
		"搜索: "+req.Query,
		req.MaxResults,
	)

	// 在后台执行搜索
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		// 同时保存到 legacy task 表（兼容前端）
		legacyTask := &storage.DiscoveryTask{
			ID:         task.ID,
			Engines:    req.Engines,
			Query:      req.Query,
			MaxResults: req.MaxResults,
			Status:     storage.TaskRunning,
			StartedAt:  time.Now(),
		}
		h.storage.SaveTask(ctx, legacyTask)

		searchTask, err := h.engineMgr.Search(ctx, req.Engines, req.Query, req.MaxResults)
		if err != nil {
			h.taskMgr.SetTaskError(task.ID, err)
			legacyTask.Status = storage.TaskFailed
			legacyTask.CompletedAt = time.Now()
			h.storage.UpdateTask(context.Background(), legacyTask)
			return
		}

		// 保存发现的 URL 为服务
		foundCount := 0
		serviceIDs := make([]string, 0) // 收集新保存的服务 ID
		for source, urls := range searchTask.Results {
			for _, urlStr := range urls {
				service := &storage.OllamaService{
					ID:        generateID(),
					URL:       urlStr,
					Status:    storage.StatusUnknown,
					Source:    source,
					CreatedAt: time.Now(),
				}
				if err := h.storage.SaveService(context.Background(), service); err == nil {
					foundCount++
					serviceIDs = append(serviceIDs, service.ID) // 收集 ID 用于后续检测
				}
			}
		}

		// 如果有新发现的服务，自动触发批量检测
		if len(serviceIDs) > 0 {
			go func() {
				// 延迟 2 秒启动检测，确保事务已提交
				time.Sleep(2 * time.Second)

				// 创建批量检测任务
				batchTask := h.taskMgr.CreateTask(
					taskpkg.TaskTypeBatchCheck,
					fmt.Sprintf("自动检测 %d 个新服务", len(serviceIDs)),
					len(serviceIDs),
				)

				// 执行批量检测
				h.executeBatchDetection(context.Background(), batchTask, serviceIDs)
			}()
		}

		// 完成任务
		h.taskMgr.CompleteTask(task.ID, map[string]interface{}{
			"found_count": foundCount,
			"engines":     req.Engines,
		})

		legacyTask.Status = storage.TaskCompleted
		legacyTask.FoundCount = foundCount
		legacyTask.Progress = legacyTask.Total
		if task.CompletedAt != nil {
			legacyTask.CompletedAt = *task.CompletedAt
		}
		h.storage.UpdateTask(context.Background(), legacyTask)
	}()

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"task_id": task.ID,
			"status":  task.Status,
		},
	})
}

// GetTask 获取任务状态
func (h *DiscoveryHandler) GetTask(c *gin.Context) {
	taskID := c.Param("id")

	task, err := h.storage.GetTask(c.Request.Context(), taskID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if task == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	// 构建响应数据，包含 result 和 found_count 字段以兼容前端
	responseData := gin.H{
		"id":           task.ID,
		"engines":      task.Engines,
		"query":        task.Query,
		"max_results":  task.MaxResults,
		"status":       task.Status,
		"progress":     task.Progress,
		"total":        task.Total,
		"found_count":  task.FoundCount, // 直接返回 found_count，方便前端访问
		"started_at":   task.StartedAt,
		"completed_at": task.CompletedAt,
		"created_at":   task.CreatedAt,
	}

	// 添加 result 字段，方便前端访问（同时保留向后兼容）
	if task.Status == storage.TaskCompleted {
		responseData["result"] = gin.H{
			"found_count": task.FoundCount,
			"engines":     task.Engines,
		}
	}

	c.JSON(http.StatusOK, gin.H{"data": responseData})
}

// ImportURLs 从文件导入 URL 列表
func (h *DiscoveryHandler) ImportURLs(c *gin.Context) {
	var req struct {
		URLs   []string `json:"urls"`
		Source string   `json:"source"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	source := storage.DiscoverySource(req.Source)
	if source == "" {
		source = storage.SourceImport
	}

	importedCount := 0
	for _, urlStr := range req.URLs {
		service := &storage.OllamaService{
			ID:        generateID(),
			URL:       urlStr,
			Status:    storage.StatusUnknown,
			Source:    source,
			CreatedAt: time.Now(),
		}

		if err := h.storage.SaveService(c.Request.Context(), service); err != nil {
			continue
		}
		importedCount++
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"imported": importedCount,
			"total":    len(req.URLs),
		},
	})
}

// executeBatchDetection 执行批量检测（内部方法）
func (h *DiscoveryHandler) executeBatchDetection(ctx context.Context, batchTask *task.Task, serviceIDs []string) {
	// 获取所有服务的 URL
	urls := make([]string, 0, len(serviceIDs))
	for _, id := range serviceIDs {
		service, err := h.storage.GetService(ctx, id)
		if err != nil || service == nil {
			continue
		}
		urls = append(urls, service.URL)
	}

	if len(urls) == 0 {
		h.taskMgr.SetTaskError(batchTask.ID, fmt.Errorf("没有有效的服务 URL"))
		return
	}

	// 设置超时
	ctx, cancel := context.WithTimeout(ctx, 300*time.Second)
	defer cancel()

	// 并发检测
	sem := make(chan struct{}, 10)
	var wg sync.WaitGroup
	var mu sync.Mutex
	successCount := 0
	foundCount := 0

	for i, url := range urls {
		wg.Add(1)
		go func(idx int, urlStr string) {
			defer wg.Done()
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

				// 更新服务状态
				// 这里需要根据 URL 找到对应的服务 ID
				// 简化处理：遍历 serviceIDs 查找匹配的 URL
				for _, id := range serviceIDs {
					service, err := h.storage.GetService(context.Background(), id)
					if err == nil && service != nil && service.URL == urlStr {
						service.Status = storage.StatusOnline
						service.Version = result.Version
						service.ResponseTime = result.ResponseTime
						service.IsHoneypot = result.IsHoneypot
						service.LastChecked = time.Now()
						service.Models = result.Models
						h.storage.SaveService(context.Background(), service)
						if len(result.Models) > 0 {
							h.storage.SaveModels(context.Background(), service.ID, result.Models)
						}
						break
					}
				}
			} else {
				// 检测失败，标记为离线
				for _, id := range serviceIDs {
					service, err := h.storage.GetService(context.Background(), id)
					if err == nil && service != nil && service.URL == urlStr {
						service.Status = storage.StatusOffline
						service.LastChecked = time.Now()
						h.storage.SaveService(context.Background(), service)
						break
					}
				}
			}
		}(i, url)
	}

	wg.Wait()

	// 完成任务
	h.taskMgr.CompleteTask(batchTask.ID, map[string]interface{}{
		"total":       len(urls),
		"success":     successCount,
		"found_count": foundCount,
	})
}
