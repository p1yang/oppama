package proxy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"oppama/internal/storage"
	"oppama/internal/utils/logger"
)

// ChatCompletionRequest 增加了 SessionID 字段
type ChatCompletionRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	TopP        float64   `json:"top_p,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
	SessionID   string    `json:"session_id,omitempty"` // 会话 ID，用于多轮对话绑定
	// 工具调用相关字段
	Tools      []Tool      `json:"tools,omitempty"`
	ToolChoice interface{} `json:"tool_choice,omitempty"` // 可以是字符串或对象
}

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`   // 助手返回的工具调用
	ToolCallID string     `json:"tool_call_id,omitempty"` // 工具调用的 ID
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
	// Ollama 工具调用支持
	Tools []Tool `json:"tools,omitempty"`
}

// Tool 工具定义
type Tool struct {
	Type     string      `json:"type"`
	Function FunctionDef `json:"function"`
}

// FunctionDef 函数定义
type FunctionDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// ToolCall 工具调用
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall 函数调用
type FunctionCall struct {
	Name      string      `json:"name"`
	Arguments interface{} `json:"arguments"` // 可以是字符串或对象
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
	// 智能路由相关
	lastUsedIndex   map[string]int // 记录每个模型的最后使用索引（轮询）
	modelRoundRobin map[string]int // 每个模型对应的可用服务列表的轮询索引
	// 会话绑定相关
	sessionBindings map[string]*SessionBinding // session_id -> (service_id, model_name)
	sessionMu       sync.RWMutex               // 会话锁
	sessionTTL      time.Duration              // 会话过期时间，默认 30 分钟
}

// SessionBinding 会话绑定信息
type SessionBinding struct {
	ServiceID    string
	ServiceURL   string
	ModelName    string
	CreatedAt    time.Time
	LastUsedAt   time.Time
	RequestCount int64 // 该会话的请求次数
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
	// 创建主 HTTP Transport（用于代理请求）- 优化长连接和超时控制
	transport := &http.Transport{
		MaxIdleConns:        200,
		MaxIdleConnsPerHost: 50,
		DisableKeepAlives:   false, // 保持长连接，对流式传输至关重要
		DisableCompression:  true,  // 禁用压缩减少 CPU 开销和延迟
		IdleConnTimeout:     90 * time.Second,
		// 自定义拨号配置，优化连接建立
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second, // 连接建立超时
			KeepAlive: 30 * time.Second, // TCP keepalive
		}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second, // TLS 握手超时
		ResponseHeaderTimeout: 30 * time.Second, // 等待响应头的超时
		ExpectContinueTimeout: 1 * time.Second,
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
		logger.Proxy().Printf("已配置代理 - HTTP: %s, HTTPS: %s", cfg.HTTPProxy, cfg.HTTPSProxy)
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
		lastUsedIndex:   make(map[string]int),
		modelRoundRobin: make(map[string]int),
		sessionBindings: make(map[string]*SessionBinding),
		sessionTTL:      30 * time.Minute, // 默认会话过期时间 30 分钟
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

	// 创建新的 HTTP Transport - 优化长连接和超时控制
	transport := &http.Transport{
		MaxIdleConns:        200,
		MaxIdleConnsPerHost: 50,
		DisableKeepAlives:   false, // 保持长连接
		DisableCompression:  true,  // 禁用压缩
		IdleConnTimeout:     90 * time.Second,
		// 自定义拨号配置
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
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
		logger.Proxy().Printf("已更新代理配置 - HTTP: %s, HTTPS: %s", cfg.HTTPProxy, cfg.HTTPSProxy)
	} else {
		logger.Proxy().Printf("已清除代理配置")
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

	logger.Proxy().Printf("代理配置已更新：timeout=%v, auth=%v, fallback=%v, max_retries=%d, rate_limit=%d",
		cfg.Timeout, cfg.EnableAuth, cfg.FallbackEnabled, cfg.MaxRetries, cfg.RateLimitRPM)
}

// StreamChatCompletions 流式处理 Chat Completions 请求（优化版）
func (p *ProxyService) StreamChatCompletions(ctx context.Context, req *ChatCompletionRequest, callback func(*ChatCompletionResponse) error) error {
	// 1. 选择合适的服务和模型（支持会话绑定）
	service, ollamaModel, err := p.selectServiceAndModel(req.Model, req.SessionID)
	if err != nil {
		logger.Proxy().Printf("选择服务失败：%v", err)
		return fmt.Errorf("选择服务失败：%w", err)
	}

	logger.Proxy().Printf("选中服务：%s (%s), 模型：%s, 会话：%s", service.ID, service.URL, ollamaModel, req.SessionID)

	// 2. 转换为 Ollama 格式
	ollamaReq := p.convertToOllamaRequest(req, ollamaModel)

	// 3. 发送流式请求到 Ollama
	ollamaReq.Stream = true

	url := fmt.Sprintf("%s/api/chat", service.URL)
	jsonData, err := json.Marshal(ollamaReq)
	if err != nil {
		return err
	}

	// 4. 创建 HTTP 请求
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// 5. 发送请求
	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		logger.Proxy().Printf("流式请求失败：%v", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Ollama 返回错误 [%d]: %s", resp.StatusCode, string(body))
	}

	// 6. 使用 bufio.Reader 替代 Scanner，更精确的控制读取
	reader := bufio.NewReader(resp.Body)
	chunkCount := 0

	for {
		// 检查上下文是否已取消（客户端断开连接）
		select {
		case <-ctx.Done():
			logger.Proxy().Printf("客户端断开连接，停止流式传输（已发送 %d 个 chunk）", chunkCount)
			return nil
		default:
		}

		// 读取一行（包括换行符）
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			logger.Proxy().Printf("读取响应失败：%v", err)
			return fmt.Errorf("读取流式响应失败：%w", err)
		}

		if len(line) == 0 {
			continue
		}

		// 解析 JSON 响应
		var ollamaResp OllamaChatResponse
		if err := json.Unmarshal(line, &ollamaResp); err != nil {
			logger.Proxy().Printf("JSON 解析失败：%v, 数据：%s", err, string(line))
			continue
		}

		// 检查 Ollama 错误
		if ollamaResp.Error != "" {
			return fmt.Errorf("Ollama 错误：%s", ollamaResp.Error)
		}

		// 转换为 OpenAI 格式并回调
		chunk := p.convertToOpenAIStreamChunk(&ollamaResp, req.Model)
		if chunk != nil {
			if err := callback(chunk); err != nil {
				logger.Proxy().Printf("回调处理失败：%v", err)
				return err
			}
			chunkCount++
		}

		// 完成标志
		if ollamaResp.Done {
			logger.Proxy().Printf("流式传输完成，共发送 %d 个 chunk", chunkCount)
			break
		}
	}

	return nil
}

// convertToOpenAIStreamChunk 转换 Ollama 流式响应为 OpenAI 格式
func (p *ProxyService) convertToOpenAIStreamChunk(ollamaResp *OllamaChatResponse, requestedModel string) *ChatCompletionResponse {
	// 检查是否有内容或工具调用
	hasContent := ollamaResp.Message.Content != ""
	hasToolCalls := len(ollamaResp.Message.ToolCalls) > 0

	if !hasContent && !hasToolCalls {
		return nil // 跳过空响应
	}

	finishReason := ""
	if ollamaResp.Done {
		finishReason = "stop"
		// 如果有工具调用，finish_reason 应该是 tool_calls
		if hasToolCalls {
			finishReason = "tool_calls"
		}
	}

	delta := Message{
		Role:    "assistant",
		Content: ollamaResp.Message.Content,
	}

	// 复制工具调用（确保 Arguments 是字符串格式）
	if hasToolCalls {
		delta.ToolCalls = make([]ToolCall, len(ollamaResp.Message.ToolCalls))
		for i, toolCall := range ollamaResp.Message.ToolCalls {
			delta.ToolCalls[i] = ToolCall{
				ID:   toolCall.ID,
				Type: toolCall.Type,
				Function: FunctionCall{
					Name: toolCall.Function.Name,
				},
			}

			// 处理 Arguments 字段
			switch args := toolCall.Function.Arguments.(type) {
			case string:
				delta.ToolCalls[i].Function.Arguments = args
			default:
				// 将对象转换为 JSON 字符串
				if jsonBytes, err := json.Marshal(args); err == nil {
					delta.ToolCalls[i].Function.Arguments = string(jsonBytes)
				} else {
					delta.ToolCalls[i].Function.Arguments = "{}"
				}
			}
		}
		logger.Proxy().Printf("🔧 检测到原生工具调用：%d 个", len(delta.ToolCalls))
	} else if hasContent && ollamaResp.Done {
		// 【关键】如果 Ollama 不支持工具调用，尝试从文本中解析工具调用
		// 注意：只在最后一个 chunk (Done=true) 时尝试解析，因为需要完整的内容
		toolCall := tryParseToolUseFromText(ollamaResp.Message.Content)
		if toolCall != nil {
			delta.ToolCalls = []ToolCall{*toolCall}
			// 清除原始文本内容，因为工具调用已经包含在 ToolCalls 中
			delta.Content = ""
			finishReason = "tool_calls"
			logger.Proxy().Printf("✅ 从文本中成功解析工具调用：%s", toolCall.Function.Name)
		} else {
			logger.Proxy().Printf("📝 普通文本响应（未检测到工具调用）")
		}
	}

	return &ChatCompletionResponse{
		ID:      fmt.Sprintf("chatcmpl-%d", time.Now().Unix()),
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   requestedModel,
		Choices: []Choice{
			{
				Index:        0,
				Delta:        delta,
				FinishReason: finishReason,
			},
		},
	}
}

// tryParseToolUseFromText 尝试从文本中解析工具调用 JSON
func tryParseToolUseFromText(content string) *ToolCall {
	// 尝试查找 JSON 代码块
	jsonStart := strings.Index(content, "```json")
	if jsonStart == -1 {
		// 尝试直接查找 JSON 对象
		jsonStart = strings.Index(content, "{")
		if jsonStart == -1 {
			return nil
		}
	} else {
		// 跳过 ```json 标记
		jsonStart += 7
	}

	// 查找 JSON 结束位置
	jsonEnd := strings.LastIndex(content, "}")
	if jsonEnd == -1 || jsonEnd <= jsonStart {
		return nil
	}

	// 提取 JSON 字符串
	jsonStr := strings.TrimSpace(content[jsonStart : jsonEnd+1])

	// 尝试解析为工具调用格式
	var toolUse struct {
		Action    string                 `json:"action"`
		ToolName  string                 `json:"tool_name"`
		ToolInput map[string]interface{} `json:"tool_input"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &toolUse); err != nil {
		logger.Proxy().Printf("解析工具调用 JSON 失败：%v, 数据：%s", err, jsonStr)
		return nil
	}

	// 验证是否是工具调用格式
	if toolUse.Action != "tool_use" || toolUse.ToolName == "" {
		return nil
	}

	// 构造 ToolCall
	toolCall := &ToolCall{
		ID:   fmt.Sprintf("toolu_%d", time.Now().UnixNano()),
		Type: "function",
		Function: FunctionCall{
			Name: toolUse.ToolName,
		},
	}

	// 将输入参数转换为 JSON 字符串
	if argsJSON, err := json.Marshal(toolUse.ToolInput); err == nil {
		toolCall.Function.Arguments = string(argsJSON)
	} else {
		toolCall.Function.Arguments = "{}"
	}

	logger.Proxy().Printf("✅ 从文本中成功解析工具调用：%s", toolUse.ToolName)
	return toolCall
}

// ChatCompletions 处理 Chat Completions 请求（非流式） (OpenAI 兼容)
func (p *ProxyService) ChatCompletions(ctx context.Context, req *ChatCompletionRequest) (*ChatCompletionResponse, error) {
	// 1. 选择合适的服务和模型（支持会话绑定）
	service, ollamaModel, err := p.selectServiceAndModel(req.Model, req.SessionID)
	if err != nil {
		logger.Proxy().Printf("选择服务失败：%v", err)
		return nil, fmt.Errorf("选择服务失败：%w", err)
	}

	logger.Proxy().Printf("选中服务：%s (%s), 模型：%s, 会话：%s", service.ID, service.URL, ollamaModel, req.SessionID)

	// 2. 转换为 Ollama 格式
	ollamaReq := p.convertToOllamaRequest(req, ollamaModel)

	// 3. 发送请求到 Ollama（非流式）
	// 注意：即使客户端请求流式，我们也强制使用非流式调用 Ollama
	// 然后在 API 层将完整响应转换为 SSE 格式返回
	ollamaReq.Stream = false

	// 增加重试机制
	var ollamaResp *OllamaChatResponse
	maxRetries := p.maxRetries
	retryDelay := 2 * time.Second

	for attempt := 0; attempt <= maxRetries; attempt++ {
		ollamaResp, err = p.sendOllamaRequest(ctx, service.URL, ollamaReq)
		if err == nil {
			// 成功则跳出
			break
		}

		// 最后一次尝试失败
		if attempt >= maxRetries {
			logger.Proxy().Printf("主服务请求失败，已重试 %d 次：%v", maxRetries, err)
			// 如果启用 fallback，尝试其他服务
			if p.config.FallbackEnabled {
				return p.tryFallback(ctx, req, service.ID)
			}
			return nil, fmt.Errorf("请求 Ollama 失败：%w", err)
		}

		// 等待后重试
		logger.Proxy().Printf("主服务请求失败，%d秒后重试 (%d/%d): %v",
			int(retryDelay.Seconds())*(attempt+1), attempt+1, maxRetries, err)
		time.Sleep(retryDelay * time.Duration(attempt+1))
	}

	// 5. 转换为 OpenAI 格式
	openaiResp := p.convertToOpenAIResponse(ollamaResp, req.Model)

	return openaiResp, nil
}

// selectServiceAndModel 选择服务和模型（智能路由 + 会话绑定优化版）
func (p *ProxyService) selectServiceAndModel(requestedModel string, sessionID string) (*storage.OllamaService, string, error) {
	p.mu.RLock()

	// 【关键】检查是否有会话绑定
	if sessionID != "" {
		p.sessionMu.RLock()
		binding, exists := p.sessionBindings[sessionID]
		p.sessionMu.RUnlock()

		if exists {
			// 检查会话是否过期
			if time.Since(binding.LastUsedAt) > p.sessionTTL {
				logger.Proxy().Printf("会话 %s 已过期，删除绑定", sessionID)
				p.sessionMu.Lock()
				delete(p.sessionBindings, sessionID)
				p.sessionMu.Unlock()
			} else {
				// 使用绑定的服务
				logger.Proxy().Printf("使用会话绑定：%s -> %s (%s)", sessionID, binding.ServiceID, binding.ModelName)

				// 从 currentServices 中找到对应的服务
				for _, service := range p.currentServices {
					if service.ID == binding.ServiceID {
						// 更新最后使用时间
						p.sessionMu.Lock()
						binding.LastUsedAt = time.Now()
						binding.RequestCount++
						p.sessionMu.Unlock()
						p.mu.RUnlock() // Manually unlock before returning
						return service, binding.ModelName, nil
					}
				}

				// 服务不存在（可能已被删除），清理会话绑定
				logger.Proxy().Printf("警告：会话绑定的服务 %s 不存在，清理绑定", binding.ServiceID)
				p.sessionMu.Lock()
				delete(p.sessionBindings, sessionID)
				p.sessionMu.Unlock()
			}
		}
	}

	if len(p.currentServices) == 0 {
		// 从存储加载服务（包括 offline 和 unknown 状态）
		services, err := p.storage.ListServices(context.Background(), storage.ServiceFilter{})
		if err != nil {
			p.mu.RUnlock()
			return nil, "", err
		}
		// 过滤出有模型的服务
		for _, svc := range services {
			if len(svc.Models) > 0 {
				p.currentServices = append(p.currentServices, svc)
			}
		}
		// 记录加载结果
		logger.Proxy().Printf("从数据库加载了 %d 个服务，其中 %d 个有模型", len(services), len(p.currentServices))
		if len(p.currentServices) == 0 {
			logger.Proxy().Printf("警告：数据库中没有任何有模型的服务")
		}
	}

	// 如果指定了模型名，查找所有包含该模型的服务
	if requestedModel != "" {
		log.Printf("[Proxy] 查找模型：%s, currentServices 数量：%d", requestedModel, len(p.currentServices))

		// 收集所有匹配的服务
		var matchingServices []struct {
			service *storage.OllamaService
			model   string
		}

		for _, service := range p.currentServices {
			log.Printf("[Proxy] 检查服务：%s, URL: %s, 模型数量：%d, 状态：%s",
				service.ID, service.URL, len(service.Models), service.Status)
			for _, model := range service.Models {
				log.Printf("[Proxy]   检查模型：%s", model.Name)
				if p.modelMatches(requestedModel, model.Name) {
					log.Printf("[Proxy]   匹配成功！")
					matchingServices = append(matchingServices, struct {
						service *storage.OllamaService
						model   string
					}{service: service, model: model.Name})
				}
			}
		}

		if len(matchingServices) > 0 {
			// 智能选择策略
			service, modelName := p.selectBestService(matchingServices, requestedModel)
			if service != nil {
				log.Printf("[Proxy] 智能路由选择：%s (%s), 模型：%s", service.ID, service.URL, modelName)

				// 【关键】如果有 sessionID，创建绑定
				if sessionID != "" {
					p.sessionMu.Lock()
					p.sessionBindings[sessionID] = &SessionBinding{
						ServiceID:    service.ID,
						ServiceURL:   service.URL,
						ModelName:    modelName,
						CreatedAt:    time.Now(),
						LastUsedAt:   time.Now(),
						RequestCount: 1,
					}
					p.sessionMu.Unlock()
					logger.Proxy().Printf("创建会话绑定：%s -> %s (%s)", sessionID, service.ID, modelName)
				}
				p.mu.RUnlock()
				return service, modelName, nil
			}
		}
		p.mu.RUnlock() // Unlock before refresh

		// 如果没有找到，尝试刷新服务列表后再次查找
		logger.Proxy().Printf("未在缓存中找到模型 %s，尝试刷新服务列表...", requestedModel)
		if err := p.RefreshServices(); err != nil {
			logger.Proxy().Printf("刷新服务列表失败：%v", err)
		} else {
			p.mu.RLock() // Relock after successful refresh
			// 重新收集匹配的服务
			matchingServices = nil
			for _, service := range p.currentServices {
				for _, model := range service.Models {
					if p.modelMatches(requestedModel, model.Name) {
						matchingServices = append(matchingServices, struct {
							service *storage.OllamaService
							model   string
						}{service: service, model: model.Name})
					}
				}
			}

			if len(matchingServices) > 0 {
				service, modelName := p.selectBestService(matchingServices, requestedModel)
				if service != nil {
					logger.Proxy().Printf("刷新后选择：%s (%s), 模型：%s", service.ID, service.URL, modelName)

					// 创建会话绑定
					if sessionID != "" {
						p.sessionMu.Lock()
						p.sessionBindings[sessionID] = &SessionBinding{
							ServiceID:    service.ID,
							ServiceURL:   service.URL,
							ModelName:    modelName,
							CreatedAt:    time.Now(),
							LastUsedAt:   time.Now(),
							RequestCount: 1,
						}
						p.sessionMu.Unlock()
						log.Printf("[Proxy] 创建会话绑定：%s -> %s (%s)", sessionID, service.ID, modelName)
					}
					p.mu.RUnlock()
					return service, modelName, nil
				}
			}
			p.mu.RUnlock() // Unlock if not found after refresh
		}
		p.mu.RLock() // Relock if refresh failed or model still not found, for the final part of the function
	}

	// 使用默认模型或第一个可用服务
	if len(p.currentServices) > 0 {
		service := p.currentServices[0]
		if len(service.Models) > 0 {
			modelName := service.Models[0].Name

			// 为默认服务也创建会话绑定
			if sessionID != "" {
				p.sessionMu.Lock()
				p.sessionBindings[sessionID] = &SessionBinding{
					ServiceID:    service.ID,
					ServiceURL:   service.URL,
					ModelName:    modelName,
					CreatedAt:    time.Now(),
					LastUsedAt:   time.Now(),
					RequestCount: 1,
				}
				p.sessionMu.Unlock()
			}
			p.mu.RUnlock()
			return service, modelName, nil
		}
	}

	p.mu.RUnlock()
	return nil, "", fmt.Errorf("没有可用的服务")
}

// selectBestService 智能选择最佳服务（优先级 + 负载均衡）
func (p *ProxyService) selectBestService(matchingServices []struct {
	service *storage.OllamaService
	model   string
}, requestedModel string) (*storage.OllamaService, string) {
	if len(matchingServices) == 0 {
		return nil, ""
	}

	// 1. 按优先级和响应时间排序
	sort.SliceStable(matchingServices, func(i, j int) bool {
		si, sj := matchingServices[i].service, matchingServices[j].service

		// 优先级：online > unknown > offline
		priority := map[storage.ServiceStatus]int{
			storage.StatusOnline:   3,
			storage.StatusUnknown:  2,
			storage.StatusOffline:  1,
			storage.StatusHoneypot: 0,
		}

		pi, pj := priority[si.Status], priority[sj.Status]
		if pi != pj {
			return pi > pj // 优先级高的在前
		}

		// 同优先级时，响应时间短的优先（只比较在线服务）
		if si.Status == storage.StatusOnline && sj.Status == storage.StatusOnline {
			if si.ResponseTime != sj.ResponseTime {
				return si.ResponseTime < sj.ResponseTime
			}
		}

		// 其他情况保持原有顺序
		return false
	})

	// 2. 分组：优先选择在线服务
	var onlineServices []struct {
		service *storage.OllamaService
		model   string
	}

	for _, ms := range matchingServices {
		if ms.service.Status == storage.StatusOnline {
			onlineServices = append(onlineServices, ms)
		}
	}

	// 如果有在线服务，只在在线服务中轮询
	if len(onlineServices) > 0 {
		matchingServices = onlineServices
	}

	// 3. 轮询选择（避免总是选择第一个）
	index := p.modelRoundRobin[requestedModel]
	if index >= len(matchingServices) {
		index = 0
	}

	selected := matchingServices[index]

	// 更新轮询索引
	p.modelRoundRobin[requestedModel] = (index + 1) % len(matchingServices)

	logger.Proxy().Printf("轮询选择：索引=%d, 总数=%d, 选中=%s",
		index, len(matchingServices), selected.service.URL)

	return selected.service, selected.model
}

// tryFallback 尝试备用服务
func (p *ProxyService) tryFallback(ctx context.Context, req *ChatCompletionRequest, excludeServiceID string) (*ChatCompletionResponse, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	logger.Proxy().Printf("开始 fallback，排除服务：%s", excludeServiceID)

	for _, service := range p.currentServices {
		if service.ID == excludeServiceID {
			continue
		}

		if len(service.Models) == 0 {
			continue
		}

		logger.Proxy().Printf("尝试备用服务：%s (%s)", service.ID, service.URL)
		ollamaReq := p.convertToOllamaRequest(req, service.Models[0].Name)
		ollamaResp, err := p.sendOllamaRequest(ctx, service.URL, ollamaReq)
		if err == nil {
			logger.Proxy().Printf("备用服务成功：%s", service.ID)
			return p.convertToOpenAIResponse(ollamaResp, req.Model), nil
		}
		logger.Proxy().Printf("备用服务失败：%v", err)
	}

	return nil, fmt.Errorf("所有备用服务都不可用")
}

// convertToOllamaRequest 转换请求为 Ollama 格式
// convertToOllamaRequest 转换 OpenAI 格式为 Ollama 格式
func (p *ProxyService) convertToOllamaRequest(openaiReq *ChatCompletionRequest, model string) *OllamaChatRequest {
	// 复制消息
	messages := make([]Message, len(openaiReq.Messages))
	copy(messages, openaiReq.Messages)

	// 【关键】如果有工具定义，在第一个消息前添加系统提示，指导模型如何使用工具
	if len(openaiReq.Tools) > 0 {
		toolInstructions := buildToolInstructions(openaiReq.Tools)

		// 检查是否已有 system 消息
		hasSystem := false
		for i, msg := range messages {
			if msg.Role == "system" {
				// 合并到现有 system 消息
				messages[i].Content = msg.Content + "\n\n" + toolInstructions
				hasSystem = true
				break
			}
		}

		// 如果没有 system 消息，创建一个新的
		if !hasSystem {
			systemMsg := Message{
				Role:    "system",
				Content: toolInstructions,
			}
			// 插入到最前面
			messages = append([]Message{systemMsg}, messages...)
		}
	}

	return &OllamaChatRequest{
		Model:    model,
		Messages: messages,
		Stream:   openaiReq.Stream,
		Options: Options{
			Temperature: openaiReq.Temperature,
			TopP:        openaiReq.TopP,
			NumPredict:  openaiReq.MaxTokens,
		},
		// 仍然传递工具定义（如果 Ollama 支持会更好）
		Tools: openaiReq.Tools,
	}
}

// buildToolInstructions 构建工具使用说明
func buildToolInstructions(tools []Tool) string {
	var sb strings.Builder

	sb.WriteString("你是一个智能助手，可以使用以下工具来帮助用户：\n\n")
	sb.WriteString("## 可用工具\n\n")

	for i, tool := range tools {
		sb.WriteString(fmt.Sprintf("%d. **%s**\n", i+1, tool.Function.Name))
		sb.WriteString(fmt.Sprintf("   描述：%s\n", tool.Function.Description))

		// 添加参数说明
		if params, ok := tool.Function.Parameters["properties"]; ok {
			sb.WriteString("   参数:\n")
			if props, ok := params.(map[string]interface{}); ok {
				for paramName, paramDef := range props {
					if def, ok := paramDef.(map[string]interface{}); ok {
						if desc, ok := def["description"].(string); ok {
							sb.WriteString(fmt.Sprintf("      - %s: %s\n", paramName, desc))
						}
					}
				}
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## 使用规则\n\n")
	sb.WriteString("当你需要使用工具时，请严格按照以下 JSON 格式回复：\n\n")
	sb.WriteString("```json\n")
	sb.WriteString("{\n")
	sb.WriteString("  \"action\": \"tool_use\",\n")
	sb.WriteString("  \"tool_name\": \"工具名称\",\n")
	sb.WriteString("  \"tool_input\": {\n")
	sb.WriteString("    \"参数名\": \"参数值\"\n")
	sb.WriteString("  }\n")
	sb.WriteString("}\n")
	sb.WriteString("```\n\n")
	sb.WriteString("重要提示：\n")
	sb.WriteString("- 只在确实需要时才使用工具\n")
	sb.WriteString("- 确保提供所有必需的参数\n")
	sb.WriteString("- 如果你不确定，可以先向用户询问更多信息\n")
	sb.WriteString("- 不要编造工具返回的结果，等待系统为你提供结果\n")

	return sb.String()
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

	logger.Proxy().Printf("发送请求到：%s", url)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		logger.Proxy().Printf("请求失败：%v", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		logger.Proxy().Printf("Ollama 返回错误 [%d]: %s", resp.StatusCode, string(body))
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
			logger.Proxy().Printf("JSON 解析失败：%v, 数据：%s", err, string(line))
			continue
		}

		// 检查是否有错误
		if ollamaResp.Error != "" {
			logger.Proxy().Printf("Ollama 返回错误：%s", ollamaResp.Error)
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

	// 检查是否有工具调用
	hasToolCalls := len(ollamaResp.Message.ToolCalls) > 0
	if hasToolCalls {
		finishReason = "tool_calls"
	}

	// 处理 DeepSeek R1 等推理模型的 think 字段
	content := ollamaResp.Message.Content
	if content == "" && ollamaResp.EvalCount > 0 {
		// 如果模型有输出 token 但内容为空，可能是特殊模型
		logger.Proxy().Printf("警告：模型 %s 返回空内容，但有 %d 个 eval tokens", requestedModel, ollamaResp.EvalCount)

		// 检查是否有工具调用
		if len(ollamaResp.Message.ToolCalls) > 0 {
			logger.Proxy().Printf("检测到工具调用：%d 个工具调用", len(ollamaResp.Message.ToolCalls))
			for i, toolCall := range ollamaResp.Message.ToolCalls {
				logger.Proxy().Printf("工具调用 %d: %s, 参数: %v", i, toolCall.Function.Name, toolCall.Function.Arguments)
			}
		}
	}

	// 构建消息
	message := Message{
		Role:    "assistant",
		Content: content,
	}

	// 复制工具调用（确保 Arguments 是字符串格式）
	if hasToolCalls {
		message.ToolCalls = make([]ToolCall, len(ollamaResp.Message.ToolCalls))
		for i, toolCall := range ollamaResp.Message.ToolCalls {
			message.ToolCalls[i] = ToolCall{
				ID:   toolCall.ID,
				Type: toolCall.Type,
				Function: FunctionCall{
					Name: toolCall.Function.Name,
				},
			}

			// 处理 Arguments 字段
			switch args := toolCall.Function.Arguments.(type) {
			case string:
				message.ToolCalls[i].Function.Arguments = args
			default:
				// 将对象转换为 JSON 字符串
				if jsonBytes, err := json.Marshal(args); err == nil {
					message.ToolCalls[i].Function.Arguments = string(jsonBytes)
				} else {
					message.ToolCalls[i].Function.Arguments = "{}"
				}
			}
		}
	}

	return &ChatCompletionResponse{
		ID:      fmt.Sprintf("chatcmpl-%d", time.Now().Unix()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   requestedModel,
		Choices: []Choice{
			{
				Index:        0,
				Message:      message,
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

	logger.Proxy().Printf("Proxy 服务刷新：加载了 %d 个有模型的服务", len(p.currentServices))
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

// GetSessionBinding 获取会话绑定信息
func (p *ProxyService) GetSessionBinding(sessionID string) (*SessionBinding, bool) {
	p.sessionMu.RLock()
	defer p.sessionMu.RUnlock()

	binding, exists := p.sessionBindings[sessionID]
	if !exists {
		return nil, false
	}

	// 检查是否过期
	if time.Since(binding.LastUsedAt) > p.sessionTTL {
		return nil, false
	}

	return binding, true
}

// RemoveSessionBinding 移除会话绑定
func (p *ProxyService) RemoveSessionBinding(sessionID string) {
	p.sessionMu.Lock()
	defer p.sessionMu.Unlock()

	delete(p.sessionBindings, sessionID)
	logger.Proxy().Printf("删除会话绑定：%s", sessionID)
}

// CleanupExpiredSessions 清理过期会话（定期调用）
func (p *ProxyService) CleanupExpiredSessions() {
	p.sessionMu.Lock()
	defer p.sessionMu.Unlock()

	now := time.Now()
	count := 0

	for sessionID, binding := range p.sessionBindings {
		if now.Sub(binding.LastUsedAt) > p.sessionTTL {
			delete(p.sessionBindings, sessionID)
			count++
			logger.Proxy().Printf("清理过期会话：%s (存在时长：%v)",
				sessionID, now.Sub(binding.CreatedAt))
		}
	}

	if count > 0 {
		logger.Proxy().Printf("清理了 %d 个过期会话", count)
	}
}

// SetSessionTTL 设置会话过期时间
func (p *ProxyService) SetSessionTTL(ttl time.Duration) {
	p.sessionMu.Lock()
	defer p.sessionMu.Unlock()

	p.sessionTTL = ttl
	logger.Proxy().Printf("会话 TTL 已更新为：%v", ttl)
}

// GetSessionStats 获取会话统计信息
func (p *ProxyService) GetSessionStats() map[string]interface{} {
	p.sessionMu.RLock()
	defer p.sessionMu.RUnlock()

	activeCount := 0
	var totalRequests int64

	for _, binding := range p.sessionBindings {
		if time.Since(binding.LastUsedAt) <= p.sessionTTL {
			activeCount++
			totalRequests += binding.RequestCount
		}
	}

	return map[string]interface{}{
		"total_sessions":  len(p.sessionBindings),
		"active_sessions": activeCount,
		"total_requests":  totalRequests,
		"session_ttl":     p.sessionTTL.String(),
	}
}
