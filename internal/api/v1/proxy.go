package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"oppama/internal/config"
	"oppama/internal/discovery/fofa"
	"oppama/internal/discovery/hunter"
	"oppama/internal/discovery/shodan"
	"oppama/internal/proxy"
	"oppama/internal/storage"

	"github.com/gin-gonic/gin"
)

// ProxyHandler 代理服务处理器
type ProxyHandler struct {
	storage        storage.Storage
	config         *config.Config
	configPath     string
	proxyService   *proxy.ProxyService
	onConfigSaved  func() // 配置保存后的回调函数
}

// NewProxyHandler 创建代理处理器
func NewProxyHandler(storage storage.Storage, cfg *config.Config, configPath string) *ProxyHandler {
	return &ProxyHandler{
		storage:    storage,
		config:     cfg,
		configPath: configPath,
	}
}

// SetProxyService 设置代理服务引用
func (h *ProxyHandler) SetProxyService(ps *proxy.ProxyService) {
	h.proxyService = ps
}

// SetConfigSavedCallback 设置配置保存后的回调
func (h *ProxyHandler) SetConfigSavedCallback(fn func()) {
	h.onConfigSaved = fn
}

// GetConfig 获取代理配置、搜索引擎配置和检测器配置
func (h *ProxyHandler) GetConfig(c *gin.Context) {
	proxyCfg := h.config.Proxy
	discoveryCfg := h.config.Discovery
	detectorCfg := h.config.Detector

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			// 代理配置
			"enabled":          proxyCfg.Enabled,
			"enable_auth":      proxyCfg.EnableAuth,
			"api_key":          proxyCfg.APIKey,
			"default_model":    proxyCfg.DefaultModel,
			"fallback_enabled": proxyCfg.FallbackEnabled,
			"max_retries":      proxyCfg.MaxRetries,
			"timeout":          proxyCfg.Timeout,
			"http_proxy":       proxyCfg.HTTPProxy,
			"https_proxy":      proxyCfg.HTTPSProxy,
			"no_proxy":         proxyCfg.NoProxy,
			"rate_limit": gin.H{
				"enabled":             proxyCfg.RateLimit.Enabled,
				"requests_per_minute": proxyCfg.RateLimit.RequestsPerMinute,
			},
			// 搜索引擎配置
			"search_engines": gin.H{
				"fofa_enabled":   discoveryCfg.Engines.FOFA.Enabled,
				"fofa_email":     discoveryCfg.Engines.FOFA.Email,
				"fofa_key":       discoveryCfg.Engines.FOFA.Key,
				"hunter_enabled": discoveryCfg.Engines.Hunter.Enabled,
				"hunter_key":     discoveryCfg.Engines.Hunter.Key,
				"shodan_enabled": discoveryCfg.Engines.Shodan.Enabled,
				"shodan_key":     discoveryCfg.Engines.Shodan.Key,
			},
			// 检测器配置
			"detector": gin.H{
				"concurrency":         detectorCfg.Concurrency,
				"timeout":             detectorCfg.Timeout,
				"honeypot_enabled":    detectorCfg.HoneypotDetection.Enabled,
				"honeypot_threshold":  detectorCfg.HoneypotDetection.Threshold,
			},
		},
	})
}

// UpdateConfig 更新代理配置、搜索引擎配置和检测器配置
// 支持部分更新：只更新请求中包含的字段
func (h *ProxyHandler) UpdateConfig(c *gin.Context) {
	// 使用 map 来接收请求，这样可以检测哪些字段存在
	var req map[string]json.RawMessage
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 检查是否有任何字段
	if len(req) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体为空"})
		return
	}

	// 处理代理配置字段
	if _, exists := req["enable_auth"]; exists {
		var val bool
		json.Unmarshal(req["enable_auth"], &val)
		h.config.Proxy.EnableAuth = val
	}
	if val, exists := req["api_key"]; exists {
		var s string
		json.Unmarshal(val, &s)
		h.config.Proxy.APIKey = s
	}
	if val, exists := req["default_model"]; exists {
		var s string
		json.Unmarshal(val, &s)
		h.config.Proxy.DefaultModel = s
	}
	if _, exists := req["fallback_enabled"]; exists {
		var val bool
		json.Unmarshal(req["fallback_enabled"], &val)
		h.config.Proxy.FallbackEnabled = val
	}
	if val, exists := req["max_retries"]; exists {
		var i int
		json.Unmarshal(val, &i)
		if i > 0 {
			h.config.Proxy.MaxRetries = i
		}
	}
	if val, exists := req["timeout"]; exists {
		var i int
		json.Unmarshal(val, &i)
		if i > 0 {
			h.config.Proxy.Timeout = i
		}
	}
	if val, exists := req["http_proxy"]; exists {
		var s string
		json.Unmarshal(val, &s)
		h.config.Proxy.HTTPProxy = s
	}
	if val, exists := req["https_proxy"]; exists {
		var s string
		json.Unmarshal(val, &s)
		h.config.Proxy.HTTPSProxy = s
	}
	if val, exists := req["no_proxy"]; exists {
		var s string
		json.Unmarshal(val, &s)
		h.config.Proxy.NoProxy = s
	}
	if _, exists := req["rate_limit"]; exists {
		var rateLimit struct {
			Enabled           *bool `json:"enabled"`
			RequestsPerMinute *int  `json:"requests_per_minute"`
		}
		if err := json.Unmarshal(req["rate_limit"], &rateLimit); err == nil {
			if rateLimit.Enabled != nil {
				h.config.Proxy.RateLimit.Enabled = *rateLimit.Enabled
			}
			if rateLimit.RequestsPerMinute != nil && *rateLimit.RequestsPerMinute > 0 {
				h.config.Proxy.RateLimit.RequestsPerMinute = *rateLimit.RequestsPerMinute
			}
		}
	}

	// 处理搜索引擎配置
	if val, exists := req["search_engines"]; exists {
		var engines struct {
			FofaEnabled   *bool   `json:"fofa_enabled"`
			FofaEmail     *string `json:"fofa_email"`
			FofaKey       *string `json:"fofa_key"`
			HunterEnabled *bool   `json:"hunter_enabled"`
			HunterKey     *string `json:"hunter_key"`
			ShodanEnabled *bool   `json:"shodan_enabled"`
			ShodanKey     *string `json:"shodan_key"`
		}
		if err := json.Unmarshal(val, &engines); err == nil {
			if engines.FofaEnabled != nil {
				h.config.Discovery.Engines.FOFA.Enabled = *engines.FofaEnabled
			}
			if engines.FofaEmail != nil {
				h.config.Discovery.Engines.FOFA.Email = *engines.FofaEmail
			}
			if engines.FofaKey != nil {
				h.config.Discovery.Engines.FOFA.Key = *engines.FofaKey
			}
			if engines.HunterEnabled != nil {
				h.config.Discovery.Engines.Hunter.Enabled = *engines.HunterEnabled
			}
			if engines.HunterKey != nil {
				h.config.Discovery.Engines.Hunter.Key = *engines.HunterKey
			}
			if engines.ShodanEnabled != nil {
				h.config.Discovery.Engines.Shodan.Enabled = *engines.ShodanEnabled
			}
			if engines.ShodanKey != nil {
				h.config.Discovery.Engines.Shodan.Key = *engines.ShodanKey
			}
		}
	}

	// 处理检测器配置
	if val, exists := req["detector"]; exists {
		var detector struct {
			Concurrency       *int  `json:"concurrency"`
			Timeout           *int  `json:"timeout"`
			HoneypotEnabled   *bool `json:"honeypot_enabled"`
			HoneypotThreshold *int  `json:"honeypot_threshold"`
		}
		if err := json.Unmarshal(val, &detector); err == nil {
			if detector.Concurrency != nil && *detector.Concurrency > 0 {
				h.config.Detector.Concurrency = *detector.Concurrency
			}
			if detector.Timeout != nil && *detector.Timeout > 0 {
				h.config.Detector.Timeout = *detector.Timeout
			}
			if detector.HoneypotEnabled != nil {
				h.config.Detector.HoneypotDetection.Enabled = *detector.HoneypotEnabled
			}
			if detector.HoneypotThreshold != nil && *detector.HoneypotThreshold >= 0 {
				h.config.Detector.HoneypotDetection.Threshold = *detector.HoneypotThreshold
			}
		}
	}

	// 保存配置到文件
	if err := h.config.Save(h.configPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "配置更新成功，但保存失败",
			"error":   err.Error(),
		})
		return
	}

	// 更新 ProxyService 的配置
	if h.proxyService != nil {
		proxyCfg := &proxy.ProxyConfig{
			ListenAddr:      "",
			ListenPort:      0,
			EnableAuth:      h.config.Proxy.EnableAuth,
			APIKey:          h.config.Proxy.APIKey,
			DefaultModel:    h.config.Proxy.DefaultModel,
			FallbackEnabled: h.config.Proxy.FallbackEnabled,
			MaxRetries:      h.config.Proxy.MaxRetries,
			Timeout:         time.Duration(h.config.Proxy.Timeout) * time.Second,
			RateLimitRPM:    h.config.Proxy.RateLimit.RequestsPerMinute,
			HTTPProxy:       h.config.Proxy.HTTPProxy,
			HTTPSProxy:      h.config.Proxy.HTTPSProxy,
			NoProxy:         h.config.Proxy.NoProxy,
		}
		h.proxyService.UpdateConfig(proxyCfg)
		fmt.Printf("[ProxyHandler] 已更新 ProxyService 配置\n")
	}

	// 调用配置保存后的回调（用于重新加载搜索引擎等）
	if h.onConfigSaved != nil {
		h.onConfigSaved()
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "配置已更新并保存",
	})
}

// GetStatus 获取代理服务状态
func (h *ProxyHandler) GetStatus(c *gin.Context) {
	// 检查代理服务是否运行
	stats, err := h.storage.GetStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"running":     true,
			"listen_addr": "0.0.0.0",
			"listen_port": h.config.Server.Port,
			"stats":       stats,
		},
	})
}

// EngineTestResult 搜索引擎测试结果
type EngineTestResult struct {
	Engine  string `json:"engine"`
	Success bool   `json:"success"`
	Message string `json:"message"`
	Quota   int    `json:"quota,omitempty"`
}

// TestEngines 测试搜索引擎连接
func (h *ProxyHandler) TestEngines(c *gin.Context) {
	var req struct {
		Engines []string `json:"engines"` // 要测试的引擎列表，如 ["fofa", "hunter", "zoomeye", "shodan"]
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 默认测试所有启用的引擎
	if len(req.Engines) == 0 {
		if h.config.Discovery.Engines.FOFA.Enabled {
			req.Engines = append(req.Engines, "fofa")
		}
		if h.config.Discovery.Engines.Hunter.Enabled {
			req.Engines = append(req.Engines, "hunter")
		}
		if h.config.Discovery.Engines.Shodan.Enabled {
			req.Engines = append(req.Engines, "shodan")
		}
	}

	results := make([]EngineTestResult, 0, len(req.Engines))

	// 测试 FOFA
	for _, engine := range req.Engines {
		switch engine {
		case "fofa":
			result := h.testFOFA(c.Request.Context())
			results = append(results, result)
		case "hunter":
			result := h.testHunter(c.Request.Context())
			results = append(results, result)
		case "shodan":
			result := h.testShodan(c.Request.Context())
			results = append(results, result)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"data": results,
	})
}

// testFOFA 测试 FOFA 连接
func (h *ProxyHandler) testFOFA(ctx context.Context) EngineTestResult {
	// 动态导入以避免循环依赖
	cfg := h.config.Discovery.Engines.FOFA
	if cfg.Email == "" || cfg.Key == "" {
		return EngineTestResult{
			Engine:  "fofa",
			Success: false,
			Message: "API 凭证未配置",
		}
	}

	// 导入 FOFA 客户端
	fofaClient := fofa.NewClient(fofa.Config{
		Email: cfg.Email,
		Key:   cfg.Key,
	})

	if err := fofaClient.ValidateCredentials(ctx); err != nil {
		return EngineTestResult{
			Engine:  "fofa",
			Success: false,
			Message: err.Error(),
		}
	}

	// 获取配额信息
	quota, _ := fofaClient.GetQuota(ctx)

	return EngineTestResult{
		Engine:  "fofa",
		Success: true,
		Message: "连接成功",
		Quota:   quota,
	}
}

// testHunter 测试 Hunter 连接
func (h *ProxyHandler) testHunter(ctx context.Context) EngineTestResult {
	cfg := h.config.Discovery.Engines.Hunter
	if cfg.Key == "" {
		return EngineTestResult{
			Engine:  "hunter",
			Success: false,
			Message: "API Key 未配置",
		}
	}

	hunterClient := hunter.NewClient(hunter.Config{
		Key: cfg.Key,
	})

	if err := hunterClient.ValidateCredentials(ctx); err != nil {
		return EngineTestResult{
			Engine:  "hunter",
			Success: false,
			Message: err.Error(),
		}
	}

	// 获取配额信息
	quota, _ := hunterClient.GetQuota(ctx)

	return EngineTestResult{
		Engine:  "hunter",
		Success: true,
		Message: "连接成功",
		Quota:   quota,
	}
}

// testShodan 测试 Shodan 连接
func (h *ProxyHandler) testShodan(ctx context.Context) EngineTestResult {
	cfg := h.config.Discovery.Engines.Shodan
	if cfg.Key == "" {
		return EngineTestResult{
			Engine:  "shodan",
			Success: false,
			Message: "API Key 未配置",
		}
	}

	shodanClient := shodan.NewClient(shodan.Config{
		Key: cfg.Key,
	})

	if err := shodanClient.ValidateCredentials(ctx); err != nil {
		return EngineTestResult{
			Engine:  "shodan",
			Success: false,
			Message: err.Error(),
		}
	}

	// 获取配额信息
	quota, _ := shodanClient.GetQuota(ctx)

	return EngineTestResult{
		Engine:  "shodan",
		Success: true,
		Message: "连接成功",
		Quota:   quota,
	}
}
