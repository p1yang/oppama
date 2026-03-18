package scheduler

import (
	"context"
	"log"
	"sync"
	"time"

	"oppama/internal/config"
	"oppama/internal/detector"
	"oppama/internal/storage"
	"oppama/internal/task"
)

// Scheduler 定时任务调度器
type Scheduler struct {
	taskMgr       *task.Manager
	storage       storage.Storage
	interval      time.Duration
	modelInterval time.Duration // 模型同步间隔
	stopChan      chan struct{}
	tickers       map[string]*time.Ticker
	tickersMu     sync.RWMutex // 保护 tickers map
	detectorCfg   *detector.DetectorConfig
	detectorCfgMu sync.RWMutex
	config        *config.DetectorConfig // 保存配置对象的引用以读取间隔设置
	// 代理会话清理相关
	proxyService       interface{ CleanupExpiredSessions() } // 代理服务接口
	sessionCleanTicker *time.Ticker
}

// NewScheduler 创建调度器
func NewScheduler(taskMgr *task.Manager, storage storage.Storage, detectorCfg *detector.DetectorConfig, config *config.DetectorConfig) *Scheduler {
	// 从配置中获取间隔设置（单位转换为分钟）
	healthCheckInterval := 5 * time.Minute // 默认 5 分钟
	modelSyncInterval := 10 * time.Minute  // 默认 10 分钟

	if config != nil {
		if config.CheckInterval > 0 {
			healthCheckInterval = time.Duration(config.CheckInterval) * time.Second
		}
		if config.ModelSyncInterval > 0 {
			modelSyncInterval = time.Duration(config.ModelSyncInterval) * time.Second
		}
	}

	return &Scheduler{
		taskMgr:       taskMgr,
		storage:       storage,
		interval:      healthCheckInterval,
		modelInterval: modelSyncInterval,
		stopChan:      make(chan struct{}),
		tickers:       make(map[string]*time.Ticker),
		detectorCfg:   detectorCfg,
		config:        config,
	}
}

// SetProxyService 设置代理服务（用于会话清理）
func (s *Scheduler) SetProxyService(proxySvc interface{ CleanupExpiredSessions() }) {
	s.proxyService = proxySvc
	log.Printf("[Scheduler] 代理服务已设置，将定期清理过期会话")
}

// UpdateDetectorConfig 更新检测器配置
func (s *Scheduler) UpdateDetectorConfig(cfg *detector.DetectorConfig) {
	s.detectorCfgMu.Lock()
	defer s.detectorCfgMu.Unlock()
	s.detectorCfg = cfg
	log.Printf("[Scheduler] 检测器配置已更新：timeout=%v, concurrency=%d", cfg.Timeout, cfg.Concurrency)
}

// SetHealthCheckInterval 设置健康检查间隔（单位：分钟）
func (s *Scheduler) SetHealthCheckInterval(minutes int) {
	if minutes < 1 {
		minutes = 1
	}
	newInterval := time.Duration(minutes) * time.Minute

	s.detectorCfgMu.Lock()
	oldInterval := s.interval
	s.interval = newInterval
	s.detectorCfgMu.Unlock()

	log.Printf("[Scheduler] 健康检查间隔已更新：%v -> %v", oldInterval, newInterval)

	// 重启健康检查任务以应用新间隔
	go func() {
		s.stopChan <- struct{}{} // 停止旧的 ticker
		close(s.stopChan)
		s.stopChan = make(chan struct{})
		go s.startHealthCheck()
	}()
}

// SetModelSyncInterval 设置模型同步间隔（单位：分钟）
func (s *Scheduler) SetModelSyncInterval(minutes int) {
	if minutes < 1 {
		minutes = 1
	}
	newInterval := time.Duration(minutes) * time.Minute

	s.detectorCfgMu.Lock()
	oldInterval := s.modelInterval
	s.modelInterval = newInterval
	s.detectorCfgMu.Unlock()

	log.Printf("[Scheduler] 模型同步间隔已更新：%v -> %v", oldInterval, newInterval)

	// 重启模型同步任务以应用新间隔
	go func() {
		s.stopChan <- struct{}{} // 停止旧的 ticker
		close(s.stopChan)
		s.stopChan = make(chan struct{})
		go s.startModelSync()
	}()
}

// GetIntervals 获取当前的时间间隔设置
func (s *Scheduler) GetIntervals() (healthCheck int, modelSync int) {
	s.detectorCfgMu.RLock()
	defer s.detectorCfgMu.RUnlock()
	return int(s.interval.Minutes()), int(s.modelInterval.Minutes())
}

// getDetectorConfig 获取当前检测器配置
func (s *Scheduler) getDetectorConfig() *detector.DetectorConfig {
	s.detectorCfgMu.RLock()
	defer s.detectorCfgMu.RUnlock()
	return s.detectorCfg
}

// Start 启动调度器
func (s *Scheduler) Start() {
	log.Println("启动定时任务调度器...")

	// 启动健康检查任务
	go s.startHealthCheck()

	// 启动模型同步任务
	go s.startModelSync()

	// 启动会话清理任务（如果有代理服务）
	if s.proxyService != nil {
		go s.startSessionCleanup()
	}

	log.Println("定时任务调度器启动成功")
}

// Stop 停止调度器
func (s *Scheduler) Stop() {
	log.Println("停止定时任务调度器...")
	close(s.stopChan)

	// 安全地读取并停止所有 ticker
	s.tickersMu.RLock()
	for _, ticker := range s.tickers {
		ticker.Stop()
	}
	s.tickersMu.RUnlock()
}

// startHealthCheck 启动定期健康检查
func (s *Scheduler) startHealthCheck() {
	s.detectorCfgMu.RLock()
	ticker := time.NewTicker(s.interval)
	s.detectorCfgMu.RUnlock()

	// 安全地写入 tickers map
	s.tickersMu.Lock()
	s.tickers["health_check"] = ticker
	s.tickersMu.Unlock()

	defer ticker.Stop()

	log.Printf("启动定期健康检查，间隔：%v", s.interval)

	for {
		select {
		case <-ticker.C:
			s.runHealthCheck()
		case <-s.stopChan:
			log.Println("停止健康检查任务")
			return
		}
	}
}

// runHealthCheck 执行健康检查（带并发控制）
func (s *Scheduler) runHealthCheck() {
	ctx := context.Background()

	// 获取所有在线服务
	services, err := s.storage.ListServices(ctx, storage.ServiceFilter{})
	if err != nil {
		log.Printf("获取服务列表失败：%v", err)
		return
	}

	log.Printf("开始健康检查，共 %d 个服务", len(services))

	// 使用当前配置创建检测器
	det := detector.NewDetector(s.getDetectorConfig())

	// 限制并发数
	maxConcurrent := s.getDetectorConfig().Concurrency
	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup

	for _, service := range services {
		wg.Add(1)
		go func(svc *storage.OllamaService) {
			defer wg.Done()

			// 获取信号量
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}

			// 使用检测器进行健康检查
			log.Printf("检查服务：%s (%s)", svc.Name, svc.URL)
			result, err := det.Detect(ctx, svc.URL)
			if err != nil {
				log.Printf("检测服务 %s 失败：%v", svc.URL, err)
				return
			}

			// 更新服务状态
			if result.IsValid && !result.IsHoneypot {
				svc.Status = storage.StatusOnline
			} else if result.IsHoneypot {
				svc.Status = storage.StatusHoneypot
			} else {
				svc.Status = storage.StatusOffline
			}
			svc.Version = result.Version
			svc.ResponseTime = result.ResponseTime
			svc.IsHoneypot = result.IsHoneypot
			svc.LastChecked = time.Now()

			if err := s.storage.SaveService(ctx, svc); err != nil {
				log.Printf("保存服务状态失败：%v", err)
			}
		}(service)
	}

	// 等待所有检测完成
	wg.Wait()
	log.Println("健康检查完成")
}

// startModelSync 启动定期模型同步
func (s *Scheduler) startModelSync() {
	s.detectorCfgMu.RLock()
	interval := s.modelInterval
	s.detectorCfgMu.RUnlock()

	ticker := time.NewTicker(interval)

	// 安全地写入 tickers map
	s.tickersMu.Lock()
	s.tickers["model_sync"] = ticker
	s.tickersMu.Unlock()

	defer ticker.Stop()

	log.Printf("启动定期模型同步，间隔：%v", interval)

	for {
		select {
		case <-ticker.C:
			s.runModelSync()
		case <-s.stopChan:
			log.Println("停止模型同步任务")
			return
		}
	}
}

// runModelSync 执行模型同步（带并发控制和过载保护）
func (s *Scheduler) runModelSync() {
	ctx := context.Background()

	// 获取所有在线服务
	services, err := s.storage.ListServices(ctx, storage.ServiceFilter{
		Status: func() *storage.ServiceStatus {
			status := storage.StatusOnline
			return &status
		}(),
	})
	if err != nil {
		log.Printf("获取在线服务列表失败：%v", err)
		return
	}

	log.Printf("开始模型同步，共 %d 个在线服务", len(services))

	// 使用当前配置创建检测器
	det := detector.NewDetector(s.getDetectorConfig())

	// 限制并发数
	maxConcurrent := s.getDetectorConfig().Concurrency
	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup

	for _, service := range services {
		wg.Add(1)
		go func(svc *storage.OllamaService) {
			defer wg.Done()

			// 获取信号量
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}

			log.Printf("同步服务模型：%s (%s)", svc.Name, svc.URL)

			// 调用检测器获取最新模型列表
			result, err := det.Detect(ctx, svc.URL)
			if err != nil {
				log.Printf("检测服务 %s 失败：%v", svc.URL, err)
				return
			}

			if !result.IsValid || len(result.Models) == 0 {
				log.Printf("服务 %s 无效或无模型", svc.URL)
				return
			}

			// 保存模型到数据库
			if err := s.storage.SaveModels(ctx, svc.ID, result.Models); err != nil {
				log.Printf("保存模型失败：%v", err)
				return
			}

			log.Printf("成功同步 %d 个模型到服务 %s", len(result.Models), svc.URL)
		}(service)
	}

	// 等待所有检测完成
	wg.Wait()
	log.Println("模型同步完成")
}

// startSessionCleanup 启动定期会话清理
func (s *Scheduler) startSessionCleanup() {
	if s.proxyService == nil {
		return
	}

	// 每 5 分钟清理一次过期会话
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	log.Printf("[Scheduler] 启动定期会话清理，间隔：5 分钟")

	for {
		select {
		case <-ticker.C:
			s.proxyService.CleanupExpiredSessions()
		case <-s.stopChan:
			log.Println("[Scheduler] 停止会话清理任务")
			return
		}
	}
}
