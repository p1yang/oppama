package scheduler

import (
	"context"
	"log"
	"sync"
	"time"

	"oppama/internal/detector"
	"oppama/internal/storage"
	"oppama/internal/task"
)

// Scheduler 定时任务调度器
type Scheduler struct {
	taskMgr       *task.Manager
	storage       storage.Storage
	interval      time.Duration
	stopChan      chan struct{}
	tickers       map[string]*time.Ticker
	detectorCfg   *detector.DetectorConfig
	detectorCfgMu sync.RWMutex
}

// NewScheduler 创建调度器
func NewScheduler(taskMgr *task.Manager, storage storage.Storage, detectorCfg *detector.DetectorConfig) *Scheduler {
	return &Scheduler{
		taskMgr:     taskMgr,
		storage:     storage,
		interval:    5 * time.Minute, // 默认 5 分钟
		stopChan:    make(chan struct{}),
		tickers:     make(map[string]*time.Ticker),
		detectorCfg: detectorCfg,
	}
}

// UpdateDetectorConfig 更新检测器配置
func (s *Scheduler) UpdateDetectorConfig(cfg *detector.DetectorConfig) {
	s.detectorCfgMu.Lock()
	defer s.detectorCfgMu.Unlock()
	s.detectorCfg = cfg
	log.Printf("[Scheduler] 检测器配置已更新: timeout=%v, concurrency=%d", cfg.Timeout, cfg.Concurrency)
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

	log.Println("定时任务调度器启动成功")
}

// Stop 停止调度器
func (s *Scheduler) Stop() {
	log.Println("停止定时任务调度器...")
	close(s.stopChan)

	// 停止所有 ticker
	for _, ticker := range s.tickers {
		ticker.Stop()
	}
}

// startHealthCheck 启动定期健康检查
func (s *Scheduler) startHealthCheck() {
	ticker := time.NewTicker(s.interval)
	s.tickers["health_check"] = ticker
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
	interval := 10 * time.Minute // 10 分钟同步一次模型
	ticker := time.NewTicker(interval)
	s.tickers["model_sync"] = ticker
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
