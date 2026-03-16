package proxy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"oppama/internal/storage"
)

// OpenAI 请求格式
type ChatCompletionRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	TopP        float64   `json:"top_p,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OpenAI 响应格式
type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	Delta        Message `json:"delta,omitempty"` // 流式响应使用
	FinishReason string  `json:"finish_reason"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Ollama 请求格式
type OllamaChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream,omitempty"`
	Options  Options   `json:"options,omitempty"`
}

type Options struct {
	Temperature float64 `json:"temperature,omitempty"`
	TopP        float64 `json:"top_p,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"`
}

// Ollama 响应格式
type OllamaChatResponse struct {
	Model     string  `json:"model"`
	CreatedAt string  `json:"created_at"`
	Message   Message `json:"message"`
	Done      bool    `json:"done"`
	EvalCount int     `json:"eval_count,omitempty"`
	Error     string  `json:"error,omitempty"` // 新增：错误信息
}

// ProxyService 代理服务
type ProxyService struct {
	config          *ProxyConfig
	storage         storage.Storage
	httpClient      *http.Client
	detectorClient  *http.Client // 专用于检测的 HTTP 客户端
	mu              sync.RWMutex
	currentServices []*storage.OllamaService
	// 保存原始配置信息用于重新加载
	configPath      string
	enableAuth      bool
	apiKey          string
	fallbackEnabled bool
	maxRetries      int
	rateLimitRPM    int
	httpProxy       string
	httpsProxy      string
	noProxy         string
}

// ProxyConfig 代理配置
type ProxyConfig struct {
	ListenAddr      string
	ListenPort      int
	EnableAuth      bool
	APIKey          string
	DefaultModel    string
	FallbackEnabled bool
	MaxRetries      int
	Timeout         time.Duration
	RateLimitRPM    int
	// HTTP 代理配置（用于访问后端 Ollama 服务）
	HTTPProxy  string // HTTP 代理地址，如 http://proxy.example.com:8080
	HTTPSProxy string // HTTPS 代理地址
	NoProxy    string // 不使用代理的地址列表，逗号分隔
}

// NewProxyService 创建代理服务
func NewProxyService(cfg *ProxyConfig, store storage.Storage) *ProxyService {
	// 创建主 HTTP Transport（用于代理请求）
	transport := &http.Transport{
		MaxIdleConns:        200,
		MaxIdleConnsPerHost: 50,
		DisableKeepAlives:   false,
		DisableCompression:  true,
		IdleConnTimeout:     90 * time.Second,
	}

	// 创建检测专用 HTTP Transport（独立连接池）
	detectorTransport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		DisableKeepAlives:   false,
		DisableCompression:  true,
		IdleConnTimeout:     60 * time.Second,
	}

	// 配置代理
	if cfg.HTTPProxy != "" || cfg.HTTPSProxy != "" {
		transport.Proxy = createProxyFunc(cfg.HTTPProxy, cfg.HTTPSProxy, cfg.NoProxy)
		detectorTransport.Proxy = createProxyFunc(cfg.HTTPProxy, cfg.HTTPSProxy, cfg.NoProxy)
		log.Printf("[Proxy] 已配置代理 - HTTP: %s, HTTPS: %s", cfg.HTTPProxy, cfg.HTTPSProxy)
	}

	return &ProxyService{
		config:  cfg,
		storage: store,
		httpClient: &http.Client{
			Timeout:   cfg.Timeout,
			Transport: transport,
		},
		detectorClient: &http.Client{
			Timeout:   cfg.Timeout,
			Transport: detectorTransport,
		},
		currentServices: make([]*storage.OllamaService, 0),
		enableAuth:      cfg.EnableAuth,
		apiKey:          cfg.APIKey,
		fallbackEnabled: cfg.FallbackEnabled,
		maxRetries:      cfg.MaxRetries,
		rateLimitRPM:    cfg.RateLimitRPM,
		httpProxy:       cfg.HTTPProxy,
		httpsProxy:      cfg.HTTPSProxy,
		noProxy:         cfg.NoProxy,
	}
}

// createProxyFunc 创建代理函数
func createProxyFunc(httpProxy, httpsProxy, noProxy string) func(*http.Request) (*url.URL, error) {
	return func(req *http.Request) (*url.URL, error) {
		// 检查是否在 NoProxy 列表中
		if noProxy != "" {
			host := req.URL.Hostname()
			noProxyList := strings.Split(noProxy, ",")
			for _, np := range noProxyList {
				np = strings.TrimSpace(np)
				if np == "" {
					continue
				}
				// 精确匹配或后缀匹配
				if host == np || strings.HasSuffix(host, "."+np) || np == "*" {
					return nil, nil // 不使用代理
				}
			}
		}

		// 根据协议选择代理
		if req.URL.Scheme == "https" && httpsProxy != "" {
			return url.Parse(httpsProxy)
		}
		if httpProxy != "" {
			return url.Parse(httpProxy)
		}
		return nil, nil
	}
}

// UpdateConfig 更新代理配置
func (p *ProxyService) UpdateConfig(cfg *ProxyConfig) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 创建新的 HTTP Transport
	transport := &http.Transport{
		MaxIdleConns:        200,
		MaxIdleConnsPerHost: 50,
		DisableKeepAlives:   false,
		DisableCompression:  true,
		IdleConnTimeout:     90 * time.Second,
	}

	// 创建检测专用 Transport
	detectorTransport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		DisableKeepAlives:   false,
		DisableCompression:  true,
		IdleConnTimeout:     60 * time.Second,
	}

	// 配置代理
	if cfg.HTTPProxy != "" || cfg.HTTPSProxy != "" {
		transport.Proxy = createProxyFunc(cfg.HTTPProxy, cfg.HTTPSProxy, cfg.NoProxy)
		detectorTransport.Proxy = createProxyFunc(cfg.HTTPProxy, cfg.HTTPSProxy, cfg.NoProxy)
		log.Printf("[Proxy] 已更新代理配置 - HTTP: %s, HTTPS: %s", cfg.HTTPProxy, cfg.HTTPSProxy)
	} else {
		log.Printf("[Proxy] 已清除代理配置")
	}

	// 更新配置
	p.config = cfg
	p.enableAuth = cfg.EnableAuth
	p.apiKey = cfg.APIKey
	p.fallbackEnabled = cfg.FallbackEnabled
	p.maxRetries = cfg.MaxRetries
	p.rateLimitRPM = cfg.RateLimitRPM
	p.httpProxy = cfg.HTTPProxy
	p.httpsProxy = cfg.HTTPSProxy
	p.noProxy = cfg.NoProxy

	// 重新创建 HTTP Client
	p.httpClient = &http.Client{
		Timeout:   cfg.Timeout,
		Transport: transport,
	}

	p.detectorClient = &http.Client{
		Timeout:   cfg.Timeout,
		Transport: detectorTransport,
	}

	log.Printf("[Proxy] 代理配置已更新：timeout=%v, auth=%v, fallback=%v, max_retries=%d, rate_limit=%d",
		cfg.Timeout, cfg.EnableAuth, cfg.FallbackEnabled, cfg.MaxRetries, cfg.RateLimitRPM)
}

// StreamChatCompletions 流式处理 Chat Completions 请求
func (p *ProxyService) StreamChatCompletions(ctx context.Context, req *ChatCompletionRequest, callback func(*ChatCompletionResponse) error) error {
	// 1. 选择合适的服务和模型
	service, ollamaModel, err := p.selectServiceAndModel(req.Model)
	if err != nil {
		log.Printf("[Proxy] 选择服务失败：%v", err)
		return fmt.Errorf("选择服务失败：%w", err)
	}

	log.Printf("[Proxy] 选中服务：%s (%s), 模型：%s", service.ID, service.URL, ollamaModel)

	// 2. 转换为 Ollama 格式
	ollamaReq := p.convertToOllamaRequest(req, ollamaModel)

	// 3. 发送流式请求到 Ollama
	ollamaReq.Stream = true

	url := fmt.Sprintf("%s/api/chat", service.URL)
	jsonData, err := json.Marshal(ollamaReq)
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		log.Printf("[Proxy] 流式请求失败：%v", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Ollama 返回错误 [%d]: %s", resp.StatusCode, string(body))
	}

	// 4. 逐行读取并转换响应
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var ollamaResp OllamaChatResponse
		if err := json.Unmarshal(line, &ollamaResp); err != nil {
			log.Printf("[Proxy] JSON 解析失败：%v", err)
			continue
		}

		// 检查错误
		if ollamaResp.Error != "" {
			return fmt.Errorf("Ollama 错误：%s", ollamaResp.Error)
		}

		// 转换为 OpenAI 流式格式
		chunk := p.convertToOpenAIStreamChunk(&ollamaResp, req.Model)
		if chunk != nil {
			if err := callback(chunk); err != nil {
				return err
			}
		}

		if ollamaResp.Done {
			break
		}
	}

	return nil
}

// convertToOpenAIStreamChunk 转换 Ollama 流式响应为 OpenAI 格式
func (p *ProxyService) convertToOpenAIStreamChunk(ollamaResp *OllamaChatResponse, requestedModel string) *ChatCompletionResponse {
	if ollamaResp.Message.Content == "" {
		return nil // 跳过空内容
	}

	finishReason := ""
	if ollamaResp.Done {
		finishReason = "stop"
	}

	return &ChatCompletionResponse{
		ID:      fmt.Sprintf("chatcmpl-%d", time.Now().Unix()),
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   requestedModel,
		Choices: []Choice{
			{
				Index: 0,
				Delta: Message{
					Role:    "assistant",
					Content: ollamaResp.Message.Content,
				},
				FinishReason: finishReason,
			},
		},
	}
}

// ChatCompletions 处理 Chat Completions 请求（非流式） (OpenAI 兼容)
func (p *ProxyService) ChatCompletions(ctx context.Context, req *ChatCompletionRequest) (*ChatCompletionResponse, error) {
	// 1. 选择合适的服务和模型
	service, ollamaModel, err := p.selectServiceAndModel(req.Model)
	if err != nil {
		log.Printf("[Proxy] 选择服务失败：%v", err)
		return nil, fmt.Errorf("选择服务失败：%w", err)
	}

	log.Printf("[Proxy] 选中服务：%s (%s), 模型：%s", service.ID, service.URL, ollamaModel)

	// 2. 转换为 Ollama 格式
	ollamaReq := p.convertToOllamaRequest(req, ollamaModel)

	// 3. 发送请求到 Ollama（非流式）
	// 注意：即使客户端请求流式，我们也强制使用非流式调用 Ollama
	// 然后在 API 层将完整响应转换为 SSE 格式返回
	ollamaReq.Stream = false
	ollamaResp, err := p.sendOllamaRequest(ctx, service.URL, ollamaReq)
	if err != nil {
		log.Printf("[Proxy] 主服务请求失败，尝试 fallback: %v", err)
		// 如果启用 fallback，尝试其他服务
		if p.config.FallbackEnabled {
			return p.tryFallback(ctx, req, service.ID)
		}
		return nil, fmt.Errorf("请求 Ollama 失败：%w", err)
	}

	// 5. 转换为 OpenAI 格式
	openaiResp := p.convertToOpenAIResponse(ollamaResp, req.Model)

	return openaiResp, nil
}

// selectServiceAndModel 选择服务和模型
func (p *ProxyService) selectServiceAndModel(requestedModel string) (*storage.OllamaService, string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.currentServices) == 0 {
		// 从存储加载服务（包括 offline 和 unknown 状态）
		services, err := p.storage.ListServices(context.Background(), storage.ServiceFilter{})
		if err != nil {
			return nil, "", err
		}
		// 过滤出有模型的服务
		for _, svc := range services {
			if len(svc.Models) > 0 {
				p.currentServices = append(p.currentServices, svc)
			}
		}
		// 记录加载结果
		log.Printf("[Proxy] 从数据库加载了 %d 个服务，其中 %d 个有模型", len(services), len(p.currentServices))
		if len(p.currentServices) == 0 {
			log.Printf("[Proxy] 警告：数据库中没有任何有模型的服务")
		}
	}

	// 如果指定了模型名，查找包含该模型的服务
	if requestedModel != "" {
		log.Printf("[Proxy] 查找模型：%s, currentServices 数量：%d", requestedModel, len(p.currentServices))
		for _, service := range p.currentServices {
			log.Printf("[Proxy] 检查服务：%s, URL: %s, 模型数量：%d", service.ID, service.URL, len(service.Models))
			for _, model := range service.Models {
				log.Printf("[Proxy]   检查模型：%s", model.Name)
				if p.modelMatches(requestedModel, model.Name) {
					log.Printf("[Proxy]   匹配成功！")
					return service, model.Name, nil
				}
			}
		}

		// 如果没有找到，尝试刷新服务列表后再次查找
		log.Printf("[Proxy] 未在缓存中找到模型 %s，尝试刷新服务列表...", requestedModel)
		if err := p.RefreshServices(); err != nil {
			log.Printf("[Proxy] 刷新服务列表失败：%v", err)
		} else {
			// 再次查找
			for _, service := range p.currentServices {
				for _, model := range service.Models {
					if p.modelMatches(requestedModel, model.Name) {
						log.Printf("[Proxy] 刷新后找到模型：%s in %s", model.Name, service.URL)
						return service, model.Name, nil
					}
				}
			}
		}
	}

	// 使用默认模型或第一个可用服务
	if len(p.currentServices) > 0 {
		service := p.currentServices[0]
		if len(service.Models) > 0 {
			return service, service.Models[0].Name, nil
		}
	}

	return nil, "", fmt.Errorf("没有可用的服务")
}

// tryFallback 尝试备用服务
func (p *ProxyService) tryFallback(ctx context.Context, req *ChatCompletionRequest, excludeServiceID string) (*ChatCompletionResponse, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	log.Printf("[Proxy] 开始 fallback，排除服务：%s", excludeServiceID)

	for _, service := range p.currentServices {
		if service.ID == excludeServiceID {
			continue
		}

		if len(service.Models) == 0 {
			continue
		}

		log.Printf("[Proxy] 尝试备用服务：%s (%s)", service.ID, service.URL)
		ollamaReq := p.convertToOllamaRequest(req, service.Models[0].Name)
		ollamaResp, err := p.sendOllamaRequest(ctx, service.URL, ollamaReq)
		if err == nil {
			log.Printf("[Proxy] 备用服务成功：%s", service.ID)
			return p.convertToOpenAIResponse(ollamaResp, req.Model), nil
		}
		log.Printf("[Proxy] 备用服务失败：%v", err)
	}

	return nil, fmt.Errorf("所有备用服务都不可用")
}

// convertToOllamaRequest 转换请求为 Ollama 格式
func (p *ProxyService) convertToOllamaRequest(openaiReq *ChatCompletionRequest, model string) *OllamaChatRequest {
	return &OllamaChatRequest{
		Model:    model,
		Messages: openaiReq.Messages,
		Stream:   openaiReq.Stream,
		Options: Options{
			Temperature: openaiReq.Temperature,
			TopP:        openaiReq.TopP,
			NumPredict:  openaiReq.MaxTokens,
		},
	}
}

// sendOllamaRequest 发送请求到 Ollama
func (p *ProxyService) sendOllamaRequest(ctx context.Context, baseURL string, req *OllamaChatRequest) (*OllamaChatResponse, error) {
	url := fmt.Sprintf("%s/api/chat", baseURL)

	// 强制设置为非流式模式
	req.Stream = false

	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	log.Printf("[Proxy] 发送请求到：%s", url)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		log.Printf("[Proxy] 请求失败：%v", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("[Proxy] Ollama 返回错误 [%d]: %s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("Ollama 返回错误 [%d]: %s", resp.StatusCode, string(body))
	}

	// 逐行读取响应，直到 done=true
	var lastResp OllamaChatResponse
	var fullContent strings.Builder // 累积完整内容
	foundDone := false

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var ollamaResp OllamaChatResponse
		if err := json.Unmarshal(line, &ollamaResp); err != nil {
			log.Printf("[Proxy] JSON 解析失败：%v, 数据：%s", err, string(line))
			continue
		}

		// 检查是否有错误
		if ollamaResp.Error != "" {
			log.Printf("[Proxy] Ollama 返回错误：%s", ollamaResp.Error)
			return nil, fmt.Errorf("Ollama 错误：%s", ollamaResp.Error)
		}

		// 累积内容（Ollama 流式响应中每个 chunk 都有部分 content）
		if ollamaResp.Message.Content != "" {
			fullContent.WriteString(ollamaResp.Message.Content)
		}

		lastResp = ollamaResp
		if ollamaResp.Done {
			foundDone = true
			break
		}
	}

	if !foundDone {
		return nil, fmt.Errorf("未收到完成响应")
	}

	// 使用累积的完整内容
	lastResp.Message.Content = fullContent.String()

	return &lastResp, nil
}

// convertToOpenAIResponse 转换响应为 OpenAI 格式
func (p *ProxyService) convertToOpenAIResponse(ollamaResp *OllamaChatResponse, requestedModel string) *ChatCompletionResponse {
	finishReason := "stop"
	if !ollamaResp.Done {
		finishReason = "length"
	}

	// 处理 DeepSeek R1 等推理模型的 think 字段
	content := ollamaResp.Message.Content
	if content == "" && ollamaResp.EvalCount > 0 {
		// 如果模型有输出 token 但内容为空，可能是特殊模型
		log.Printf("[Proxy] 警告：模型 %s 返回空内容，但有 %d 个 eval tokens", requestedModel, ollamaResp.EvalCount)
	}

	return &ChatCompletionResponse{
		ID:      fmt.Sprintf("chatcmpl-%d", time.Now().Unix()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   requestedModel,
		Choices: []Choice{
			{
				Index: 0,
				Message: Message{
					Role:    "assistant",
					Content: content,
				},
				FinishReason: finishReason,
			},
		},
		Usage: Usage{
			PromptTokens:     0, // Ollama 不提供这个信息
			CompletionTokens: ollamaResp.EvalCount,
			TotalTokens:      ollamaResp.EvalCount,
		},
	}
}

// modelMatches 检查模型名是否匹配
func (p *ProxyService) modelMatches(requested, available string) bool {
	// 精确匹配
	if requested == available {
		return true
	}

	// 忽略标签匹配（如 llama2:7b 匹配 llama2）
	requestedBase := strings.Split(requested, ":")[0]
	availableBase := strings.Split(available, ":")[0]
	if requestedBase == availableBase {
		return true
	}

	// 包含匹配
	if strings.Contains(available, requested) || strings.Contains(requested, available) {
		return true
	}

	return false
}

// RefreshServices 刷新服务列表
func (p *ProxyService) RefreshServices() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	services, err := p.storage.ListServices(context.Background(), storage.ServiceFilter{})
	if err != nil {
		return err
	}

	// 过滤出有模型的服务
	p.currentServices = make([]*storage.OllamaService, 0)
	for _, svc := range services {
		if len(svc.Models) > 0 {
			p.currentServices = append(p.currentServices, svc)
		}
	}

	log.Printf("Proxy 服务刷新：加载了 %d 个有模型的服务", len(p.currentServices))
	return nil
}

// GetAvailableModels 获取可用模型列表
func (p *ProxyService) GetAvailableModels() ([]string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	models := make([]string, 0)
	seen := make(map[string]bool)

	for _, service := range p.currentServices {
		for _, model := range service.Models {
			if !seen[model.Name] {
				seen[model.Name] = true
				models = append(models, model.Name)
			}
		}
	}

	return models, nil
}
