package discovery

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"oppama/internal/discovery/fofa"
	"oppama/internal/discovery/hunter"
	"oppama/internal/discovery/shodan"
	"oppama/internal/storage"
)

// DiscoverySource 发现源接口
type DiscoverySource interface {
	Search(ctx context.Context, query string, limit int) ([]string, error)
	ValidateCredentials(ctx context.Context) error
}

// EngineManager 引擎管理器
type EngineManager struct {
	engines map[storage.DiscoverySource]DiscoverySource
	config  *EngineConfig
}

// EngineConfig 引擎配置
type EngineConfig struct {
	FOFA   FOFAConfig
	Hunter HunterConfig
	Shodan ShodanConfig
}

type FOFAConfig struct {
	Enabled    bool
	Email      string
	Key        string
	Query      string
	MaxResults int
}

type HunterConfig struct {
	Enabled    bool
	Key        string
	Query      string
	MaxResults int
}

type ShodanConfig struct {
	Enabled    bool
	Key        string
	Query      string
	MaxResults int
}

// NewEngineManager 创建引擎管理器
func NewEngineManager(cfg *EngineConfig) *EngineManager {
	manager := &EngineManager{
		engines: make(map[storage.DiscoverySource]DiscoverySource),
		config:  cfg,
	}

	// 初始化启用的引擎
	if cfg.FOFA.Enabled && cfg.FOFA.Email != "" && cfg.FOFA.Key != "" {
		manager.engines[storage.SourceFOFA] = fofa.NewClient(fofa.Config{
			Email:      cfg.FOFA.Email,
			Key:        cfg.FOFA.Key,
			MaxResults: cfg.FOFA.MaxResults,
		})
	}

	if cfg.Hunter.Enabled && cfg.Hunter.Key != "" {
		manager.engines[storage.SourceHunter] = hunter.NewClient(hunter.Config{
			Key:        cfg.Hunter.Key,
			MaxResults: cfg.Hunter.MaxResults,
		})
	}

	if cfg.Shodan.Enabled && cfg.Shodan.Key != "" {
		manager.engines[storage.SourceShodan] = shodan.NewClient(shodan.Config{
			Key:        cfg.Shodan.Key,
			MaxResults: cfg.Shodan.MaxResults,
		})
	}

	return manager
}

// SearchTask 搜索任务结果
type SearchTask struct {
	ID         string
	Engines    []storage.DiscoverySource
	Query      string
	MaxResults int
	Status     storage.TaskStatus
	Progress   int
	Total      int
	FoundCount int
	Results    map[storage.DiscoverySource][]string
	Error      error
}

// Search 执行多引擎搜索
func (m *EngineManager) Search(ctx context.Context, engines []storage.DiscoverySource, query string, maxResults int) (*SearchTask, error) {
	if len(engines) == 0 {
		// 默认使用所有启用的引擎
		engines = m.GetEnabledEngines()
	}

	if len(engines) == 0 {
		return nil, fmt.Errorf("没有启用的搜索引擎")
	}

	task := &SearchTask{
		ID:         generateTaskID(),
		Engines:    engines,
		Query:      query,
		MaxResults: maxResults,
		Status:     storage.TaskRunning,
		Results:    make(map[storage.DiscoverySource][]string),
		Total:      len(engines),
	}

	// 并发执行搜索
	var wg sync.WaitGroup
	resultChan := make(chan searchResult, len(engines))

	for _, engine := range engines {
		wg.Add(1)
		go func(src storage.DiscoverySource) {
			defer wg.Done()

			searcher, ok := m.engines[src]
			if !ok {
				resultChan <- searchResult{
					Source: src,
					Error:  fmt.Errorf("引擎未启用"),
				}
				return
			}

			// 获取该引擎的查询配置
			queryStr := m.getQueryForEngine(src, query)
			limit := m.getLimitForEngine(src, maxResults)

			urls, err := searcher.Search(ctx, queryStr, limit)
			resultChan <- searchResult{
				Source: src,
				URLs:   urls,
				Error:  err,
			}
		}(engine)
	}

	// 等待所有搜索完成
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// 收集结果
	allURLs := make([]string, 0)
	for result := range resultChan {
		task.Progress++
		if result.Error != nil {
			// 记录错误但继续处理其他结果
			log.Printf("[Discovery] %s 搜索失败: %v", result.Source, result.Error)
			continue
		}
		log.Printf("[Discovery] %s 搜索成功，找到 %d 条结果", result.Source, len(result.URLs))
		task.Results[result.Source] = result.URLs
		task.FoundCount += len(result.URLs)
		allURLs = append(allURLs, result.URLs...)
	}

	// 去重
	task.Results[storage.SourceManual] = deduplicateURLs(allURLs)
	task.FoundCount = len(task.Results[storage.SourceManual])
	task.Status = storage.TaskCompleted

	return task, nil
}

// searchResult 单个搜索结果
type searchResult struct {
	Source storage.DiscoverySource
	URLs   []string
	Error  error
}

// GetEnabledEngines 获取启用的引擎列表
func (m *EngineManager) GetEnabledEngines() []storage.DiscoverySource {
	enabled := make([]storage.DiscoverySource, 0)
	for source := range m.engines {
		enabled = append(enabled, source)
	}
	return enabled
}

// ValidateAllCredentials 验证所有引擎的凭证
func (m *EngineManager) ValidateAllCredentials(ctx context.Context) map[storage.DiscoverySource]error {
	results := make(map[storage.DiscoverySource]error)

	for source, engine := range m.engines {
		err := engine.ValidateCredentials(ctx)
		results[source] = err
	}

	return results
}

// getQueryForEngine 获取适合特定引擎的查询语句
func (m *EngineManager) getQueryForEngine(source storage.DiscoverySource, defaultQuery string) string {
	switch source {
	case storage.SourceFOFA:
		if m.config.FOFA.Query != "" {
			return m.config.FOFA.Query
		}
		return `app="Ollama"`
	case storage.SourceHunter:
		if m.config.Hunter.Query != "" {
			return m.config.Hunter.Query
		}
		return `app.name="Ollama"`
	case storage.SourceShodan:
		if m.config.Shodan.Query != "" {
			return m.config.Shodan.Query
		}
		return `http.title:"Ollama"`
	default:
		return defaultQuery
	}
}

// getLimitForEngine 获取引擎的搜索结果限制
func (m *EngineManager) getLimitForEngine(source storage.DiscoverySource, defaultLimit int) int {
	switch source {
	case storage.SourceFOFA:
		if m.config.FOFA.MaxResults > 0 {
			return m.config.FOFA.MaxResults
		}
	case storage.SourceHunter:
		if m.config.Hunter.MaxResults > 0 {
			return m.config.Hunter.MaxResults
		}
	case storage.SourceShodan:
		if m.config.Shodan.MaxResults > 0 {
			return m.config.Shodan.MaxResults
		}
	}
	return defaultLimit
}

// deduplicateURLs URL 去重
func deduplicateURLs(urls []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0)

	for _, url := range urls {
		if !seen[url] {
			seen[url] = true
			result = append(result, url)
		}
	}

	return result
}

// generateTaskID 生成任务 ID
func generateTaskID() string {
	return fmt.Sprintf("task_%d", time.Now().UnixNano())
}
