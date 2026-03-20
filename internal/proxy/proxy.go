package proxy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"oppama/internal/cache"
	"oppama/internal/storage"
	"oppama/internal/utils/logger"
)

// ChatCompletionRequest 增加了 SessionID 字段
type ChatCompletionRequest struct {
	Model            string    `json:"model"`
	Messages         []Message `json:"messages"`
	System           string    `json:"system,omitempty"` // 系统提示
	Temperature      float64   `json:"temperature,omitempty"`
	TopP             float64   `json:"top_p,omitempty"`
	MaxTokens        int       `json:"max_tokens,omitempty"`
	Stream           bool      `json:"stream,omitempty"`
	SessionID        string    `json:"session_id,omitempty"` // 会话 ID，用于多轮对话绑定
	PresencePenalty  float64   `json:"presence_penalty,omitempty"`
	FrequencyPenalty float64   `json:"frequency_penalty,omitempty"`
	Stop             []string  `json:"stop,omitempty"` // 停止序列
	// 工具调用相关字段
	Tools      []Tool      `json:"tools,omitempty"`
	ToolChoice interface{} `json:"tool_choice,omitempty"` // 可以是字符串或对象
}

// ContentPart 内容部分（用于多模态）
type ContentPart interface {
	IsContentPart()
}

// TextContentPart 文本内容
type TextContentPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func (t TextContentPart) IsContentPart() {}

// ImageURLContentPart 图像 URL 内容
type ImageURLContentPart struct {
	Type     string   `json:"type"`
	ImageURL ImageURL `json:"image_url"`
}

type ImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"` // auto, low, high
}

func (i ImageURLContentPart) IsContentPart() {}

type Message struct {
	Role       string      `json:"role"`
	Content    interface{} `json:"content"`                // 可以是 string 或 []ContentPart
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`   // 助手返回的工具调用
	ToolCallID string      `json:"tool_call_id,omitempty"` // 工具调用的 ID
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
	Index    int          `json:"index"` // 流式响应中的索引
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
	Temperature      float64  `json:"temperature,omitempty"`
	TopP             float64  `json:"top_p,omitempty"`
	NumPredict       int      `json:"num_predict,omitempty"`
	PresencePenalty  float64  `json:"presence_penalty,omitempty"`
	FrequencyPenalty float64  `json:"frequency_penalty,omitempty"`
	RepeatPenalty    float64  `json:"repeat_penalty,omitempty"` // Ollama: 防止重复生成
	RepeatLastN      int      `json:"repeat_last_n,omitempty"`  // Ollama: 对最近 N 个 token 应用惩罚
	Stop             []string `json:"stop,omitempty"`
}

// Embeddings 相关结构
type EmbeddingRequest struct {
	Input any    `json:"input"` // 可以是字符串或字符串数组
	Model string `json:"model"`
}

type Embedding struct {
	Object    string    `json:"object"`
	Embedding []float64 `json:"embedding"`
	Index     int       `json:"index"`
}

type EmbeddingResponse struct {
	Object string      `json:"object"`
	Data   []Embedding `json:"data"`
	Model  string      `json:"model"`
	Usage  Usage       `json:"usage"`
}

// Ollama Embedding 请求/响应
type OllamaEmbeddingRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type OllamaEmbeddingResponse struct {
	Embedding []float64 `json:"embedding"`
}

// Audio 相关结构
type AudioTranscriptionRequest struct {
	File           string  `json:"file"`                      // base64 编码的音频数据
	Model          string  `json:"model"`                     // 模型名称
	Language       string  `json:"language,omitempty"`        // 语言代码
	Prompt         string  `json:"prompt,omitempty"`          // 可选提示
	ResponseFormat string  `json:"response_format,omitempty"` // json, text, srt, verbose_json, vtt
	Temperature    float64 `json:"temperature,omitempty"`     // 采样温度
}

type AudioTranslationRequest struct {
	File           string `json:"file"`
	Model          string `json:"model"`
	ResponseFormat string `json:"response_format,omitempty"`
	Prompt         string `json:"prompt,omitempty"`
}

type AudioTranscriptionResponse struct {
	Text     string  `json:"text"`
	Language string  `json:"language,omitempty"`
	Duration float64 `json:"duration,omitempty"`
	Words    []Word  `json:"words,omitempty"`
}

type AudioTranslationResponse struct {
	Text string `json:"text"`
}

type Word struct {
	Word  string  `json:"word"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

type SpeechRequest struct {
	Model          string  `json:"model"`
	Input          string  `json:"input"`                     // 要转换的文本
	Voice          string  `json:"voice"`                     // 语音类型
	ResponseFormat string  `json:"response_format,omitempty"` // mp3, opus, aac, flac
	Speed          float64 `json:"speed,omitempty"`           // 0.25 to 4.0
}

type SpeechResponse struct {
	AudioData   []byte `json:"-"` // 二进制音频数据
	ContentType string `json:"content_type"`
}

// Moderations 相关结构
type ModerationRequest struct {
	Input any    `json:"input"` // 可以是字符串或字符串数组
	Model string `json:"model"`
}

type ModerationResult struct {
	Categories     Categories     `json:"categories"`
	CategoryScores CategoryScores `json:"category_scores"`
	Flagged        bool           `json:"flagged"`
}

type Categories struct {
	Hate            bool `json:"hate"`
	HateThreatening bool `json:"hate/threatening"`
	Harassment      bool `json:"harassment"`
	SelfHarm        bool `json:"self-harm"`
	Sexual          bool `json:"sexual"`
	Violence        bool `json:"violence"`
	ViolenceGraphic bool `json:"violence/graphic"`
}

type CategoryScores struct {
	Hate            float64 `json:"hate"`
	HateThreatening float64 `json:"hate/threatening"`
	Harassment      float64 `json:"harassment"`
	SelfHarm        float64 `json:"self-harm"`
	Sexual          float64 `json:"sexual"`
	Violence        float64 `json:"violence"`
	ViolenceGraphic float64 `json:"violence/graphic"`
}

type ModerationResponse struct {
	ID      string             `json:"id"`
	Model   string             `json:"model"`
	Results []ModerationResult `json:"results"`
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
	serviceCache    *cache.ServiceCache  // 服务列表缓存
	responseCache   *cache.ResponseCache // 响应缓存
	metrics         *Metrics             // Prometheus 指标
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
	roundRobinMu    sync.RWMutex   // 保护 modelRoundRobin 的锁
	// 会话绑定相关
	sessionBindings map[string]*SessionBinding // session_id -> (service_id, model_name)
	sessionMu       sync.RWMutex               // 会话锁
	sessionTTL      time.Duration              // 会话过期时间，默认 30 分钟
	// 格式转换相关
	converter *ToolCallConverter // 工具调用格式转换器
	// 模型上下文长度缓存
	modelContextCache map[string]int // model_name -> context_length (token 数)
	contextCacheMu    sync.RWMutex   // 保护模型上下文缓存的锁
	contextCacheTTL   time.Duration  // 缓存过期时间，默认 1 小时
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
	// 创建优化的 HTTP Transport
	transport := createTransport()
	detectorTransport := createDetectorTransport()

	// 配置代理
	if cfg.HTTPProxy != "" || cfg.HTTPSProxy != "" {
		proxyFunc := createProxyFunc(cfg.HTTPProxy, cfg.HTTPSProxy, cfg.NoProxy)
		transport.Proxy = proxyFunc
		detectorTransport.Proxy = proxyFunc
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
		serviceCache: cache.NewServiceCache(cache.ServiceCacheConfig{
			RefreshInterval: 5 * time.Minute,
			TTL:             10 * time.Minute,
		}),
		responseCache: cache.NewResponseCache(cache.ResponseCacheConfig{
			MaxSize: 1000,
			TTL:     5 * time.Minute,
			Enabled: true, // 默认启用响应缓存
		}),
		metrics:           NewMetrics(), // 初始化指标收集器
		enableAuth:        cfg.EnableAuth,
		apiKey:            cfg.APIKey,
		fallbackEnabled:   cfg.FallbackEnabled,
		maxRetries:        cfg.MaxRetries,
		rateLimitRPM:      cfg.RateLimitRPM,
		httpProxy:         cfg.HTTPProxy,
		httpsProxy:        cfg.HTTPSProxy,
		noProxy:           cfg.NoProxy,
		lastUsedIndex:     make(map[string]int),
		modelRoundRobin:   make(map[string]int),
		sessionBindings:   make(map[string]*SessionBinding),
		sessionTTL:        30 * time.Minute,       // 默认会话过期时间 30 分钟
		converter:         NewToolCallConverter(), // 初始化格式转换器
		modelContextCache: make(map[string]int),   // 初始化模型上下文长度缓存
		contextCacheTTL:   time.Hour,              // 缓存 1 小时
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

// createTransport 创建优化的 HTTP Transport
func createTransport() *http.Transport {
	return &http.Transport{
		// 连接池配置
		MaxIdleConns:        200,              // 最大空闲连接数
		MaxIdleConnsPerHost: 100,              // 每个主机的最大空闲连接数（提升）
		MaxConnsPerHost:     0,                // 0 表示无限制（移除连接数上限）
		DisableKeepAlives:   false,            // 保持长连接，对流式传输至关重要
		DisableCompression:  true,             // 禁用压缩减少 CPU 开销和延迟
		IdleConnTimeout:     90 * time.Second, // 空闲连接超时
		// 自定义拨号配置，优化连接建立
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second, // 连接建立超时
			KeepAlive: 30 * time.Second, // TCP keepalive
		}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second, // TLS 握手超时
		ResponseHeaderTimeout: 60 * time.Second, // 响应头超时（提升）
		ExpectContinueTimeout: 2 * time.Second,  // Expect: 100-continue 超时
		ForceAttemptHTTP2:     false,            // 禁用 HTTP/2 以提高兼容性
	}
}

// createDetectorTransport 创建检测专用的 HTTP Transport（独立连接池）
func createDetectorTransport() *http.Transport {
	return &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 50,
		MaxConnsPerHost:     0, // 0 表示无限制
		DisableKeepAlives:   false,
		DisableCompression:  true,
		IdleConnTimeout:     60 * time.Second,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second, // 检测超时更短
			KeepAlive: 15 * time.Second,
		}).DialContext,
		ResponseHeaderTimeout: 15 * time.Second, // 检测响应超时更短
	}
}

// UpdateConfig 更新代理配置
func (p *ProxyService) UpdateConfig(cfg *ProxyConfig) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 创建优化的 HTTP Transport
	transport := createTransport()
	detectorTransport := createDetectorTransport()

	// 配置代理
	if cfg.HTTPProxy != "" || cfg.HTTPSProxy != "" {
		proxyFunc := createProxyFunc(cfg.HTTPProxy, cfg.HTTPSProxy, cfg.NoProxy)
		transport.Proxy = proxyFunc
		detectorTransport.Proxy = proxyFunc
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

// ChatCompletionsWithTools Chat Completions with automatic tool execution
func (p *ProxyService) ChatCompletionsWithTools(ctx context.Context, req *ChatCompletionRequest, options *ToolExecutionOptions) (*ChatCompletionResponse, error) {
	messages := req.Messages
	maxIterations := 5
	parallel := true

	if options != nil {
		maxIterations = options.MaxIterations
		if maxIterations <= 0 {
			maxIterations = 5
		}
		parallel = options.Parallel
	}

	for iteration := 0; iteration < maxIterations; iteration++ {
		// 执行聊天完成
		currentReq := *req
		currentReq.Messages = messages
		currentReq.Stream = false

		resp, err := p.ChatCompletions(ctx, &currentReq)
		if err != nil {
			return nil, fmt.Errorf("迭代 %d 失败: %w", iteration+1, err)
		}

		// 检查是否需要工具调用
		if len(resp.Choices) == 0 {
			return resp, nil
		}

		choice := resp.Choices[0]

		// 如果没有工具调用，返回结果
		if !RequiresToolCall(choice) {
			return resp, nil
		}

		// 执行工具调用
		toolCalls := choice.Message.ToolCalls
		execOptions := &ToolExecutionOptions{
			MaxIterations: maxIterations,
			Timeout:       30,
			Parallel:      parallel,
			Handler:       nil, // 使用默认处理器
		}

		if options != nil && options.Handler != nil {
			execOptions.Handler = options.Handler
		}

		results := ExecuteToolCalls(toolCalls, execOptions)

		// 将工具结果添加到消息列表
		for _, result := range results {
			messages = append(messages, Message{
				Role:       "tool",
				Content:    result.Content,
				ToolCallID: result.ToolCallID,
			})
		}

		logger.Proxy().Printf("工具调用迭代 %d/%d 完成，执行了 %d 个工具", iteration+1, maxIterations, len(results))
	}

	// 达到最大迭代次数，执行最后一次请求
	currentReq := *req
	currentReq.Messages = messages
	currentReq.Stream = false

	return p.ChatCompletions(ctx, &currentReq)
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

	// 3. 发送流式请求到 Ollama（带故障转移）
	ollamaReq.Stream = true

	url := fmt.Sprintf("%s/api/chat", service.URL)
	jsonData, err := json.Marshal(ollamaReq)
	if err != nil {
		return err
	}

	// 【调试】打印流式请求的完整内容
	logger.Proxy().Printf("\n========================================")
	logger.Proxy().Printf("📤 [流式中转] 发送到：%s", url)
	logger.Proxy().Printf("🕐 时间：%s", time.Now().Format("2006-01-02 15:04:05.000"))
	logger.Proxy().Printf("📄 请求体 (JSON):")
	logger.Proxy().Printf("%s", formatJSON(string(jsonData)))
	logger.Proxy().Printf("📊 消息数量：%d", len(ollamaReq.Messages))
	for i, msg := range ollamaReq.Messages {
		logger.Proxy().Printf("  📨 消息 %d:", i+1)
		logger.Proxy().Printf("     Role: %s", msg.Role)
		logger.Proxy().Printf("     Content: %v", msg.Content)
		if len(msg.ToolCalls) > 0 {
			logger.Proxy().Printf("     ToolCalls: %d 个", len(msg.ToolCalls))
			for j, tc := range msg.ToolCalls {
				logger.Proxy().Printf("       [%d] ID: %s, Type: %s, Name: %s", j, tc.ID, tc.Type, tc.Function.Name)
				logger.Proxy().Printf("           Args: %v", tc.Function.Arguments)
			}
		}
		if msg.ToolCallID != "" {
			logger.Proxy().Printf("     ToolCallID: %s", msg.ToolCallID)
		}
	}
	logger.Proxy().Printf("🔧 Tools: %d 个", len(ollamaReq.Tools))
	for i, tool := range ollamaReq.Tools {
		logger.Proxy().Printf("  🛠️  工具 %d: %s (%s)", i+1, tool.Function.Name, tool.Type)
	}
	logger.Proxy().Printf("========================================\n")

	// 4. 创建 HTTP 请求
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// 5. 发送请求（带故障转移逻辑）
	resp, err := p.sendRequestWithFailover(ctx, httpReq, service, req.Model, req.SessionID)
	if err != nil {
		logger.Proxy().Printf("❌ 流式请求失败：%v", err)
		return err
	}
	defer resp.Body.Close()

	logger.Proxy().Printf("✅ 收到响应：HTTP %d", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		logger.Proxy().Printf("❌ Ollama 返回错误 [%d]: %s", resp.StatusCode, string(body))
		return fmt.Errorf("Ollama 返回错误 [%d]: %s", resp.StatusCode, string(body))
	}

	// 6. 使用 bufio.Reader 替代 Scanner，更精确的控制读取
	reader := bufio.NewReader(resp.Body)
	chunkCount := 0
	var rawResponse strings.Builder // 保存原始响应

	logger.Proxy().Printf("📥 开始接收流式数据...")

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

		chunkCount++
		rawResponse.Write(line)
		rawResponse.WriteByte('\n')

		// 解析 JSON 响应
		var ollamaResp OllamaChatResponse
		if err := json.Unmarshal(line, &ollamaResp); err != nil {
			logger.Proxy().Printf("⚠️  JSON 解析失败：%v, 数据：%s", err, string(line))
			continue
		}

		// 【调试】打印每个 chunk 的详细内容
		content := GetContentString(ollamaResp.Message.Content)
		logger.Proxy().Printf("📦 [Chunk #%d] Content: \"%s\", Done: %v", chunkCount, truncateString(content, 50), ollamaResp.Done)
		if len(ollamaResp.Message.ToolCalls) > 0 {
			logger.Proxy().Printf("  🔧 ToolCalls: %d 个", len(ollamaResp.Message.ToolCalls))
			for i, tc := range ollamaResp.Message.ToolCalls {
				logger.Proxy().Printf("    [%d] ID: %s, Type: %s, Name: %s", i, tc.ID, tc.Type, tc.Function.Name)
				logger.Proxy().Printf("        Args: %v", tc.Function.Arguments)
			}
		} else if strings.Contains(content, "<function_calls>") {
			logger.Proxy().Printf("  🔍 检测到 XML 工具调用标记")
		}

		// 检查 Ollama 错误
		if ollamaResp.Error != "" {
			logger.Proxy().Printf("❌ Ollama 错误：%s", ollamaResp.Error)
			return fmt.Errorf("Ollama 错误：%s", ollamaResp.Error)
		}

		// 转换为 OpenAI 格式并回调
		chunk := p.convertToOpenAIStreamChunk(&ollamaResp, req.Model)
		if chunk != nil {
			if err := callback(chunk); err != nil {
				logger.Proxy().Printf("❌ 回调处理失败：%v", err)
				return err
			}
			logger.Proxy().Printf("✅ Chunk 已转发到客户端")
		}

		// 完成标志
		if ollamaResp.Done {
			logger.Proxy().Printf("✅ 收到完成标志 (Done=true)")
			logger.Proxy().Printf("📊 流式传输完成，共接收 %d 个 chunk, 已转发 %d 个", chunkCount, chunkCount)
			break
		}
	}

	// 【调试】打印流式响应的完整内容
	logger.Proxy().Printf("\n========================================")
	logger.Proxy().Printf("📥 [流式响应] 来自：%s", url)
	logger.Proxy().Printf("🕐 时间：%s", time.Now().Format("2006-01-02 15:04:05.000"))
	logger.Proxy().Printf("📄 响应体 (JSON):")
	logger.Proxy().Printf("%s", formatJSON(rawResponse.String()))
	logger.Proxy().Printf("📊 响应统计：共 %d 个 chunk", chunkCount)
	logger.Proxy().Printf("========================================\n")

	return nil
}

// sendRequestWithFailover 发送请求并带故障转移逻辑
func (p *ProxyService) sendRequestWithFailover(ctx context.Context, httpReq *http.Request, initialService *storage.OllamaService, requestedModel string, sessionID string) (*http.Response, error) {
	// 如果是会话绑定的请求，需要排除的服务 ID
	excludeServiceIDs := make(map[string]bool)
	excludeServiceIDs[initialService.ID] = true

	// 尝试当前服务
	resp, err := p.httpClient.Do(httpReq)
	if err == nil && resp.StatusCode == http.StatusOK {
		return resp, nil
	}

	// 检查是否是可重试的错误（403, 503, 502, 504 等）
	isRetryable := false
	if resp != nil {
		if resp.StatusCode == http.StatusForbidden ||
			resp.StatusCode == http.StatusServiceUnavailable ||
			resp.StatusCode == http.StatusBadGateway ||
			resp.StatusCode == http.StatusGatewayTimeout {
			isRetryable = true
			logger.Proxy().Printf("⚠️  检测到可重试错误：HTTP %d，尝试故障转移...", resp.StatusCode)
			resp.Body.Close() // 关闭当前响应
		}
	}

	if err != nil {
		logger.Proxy().Printf("⚠️  请求失败：%v，尝试故障转移...", err)
		isRetryable = true
	}

	// 如果不是可重试的错误，直接返回
	if !isRetryable {
		return resp, err
	}

	// 清除会话绑定（如果存在）
	if sessionID != "" {
		p.removeSessionBinding(sessionID)
	}

	// 获取所有匹配的服务
	p.mu.RLock()
	matchingServices := p.findMatchingServices(requestedModel)
	p.mu.RUnlock()

	// 尝试其他可用服务
	for _, ms := range matchingServices {
		if excludeServiceIDs[ms.service.ID] {
			continue // 跳过已排除的服务
		}

		logger.Proxy().Printf("🔄 尝试备用服务：%s (%s), 模型：%s", ms.service.ID, ms.service.URL, ms.model)

		// 更新请求 URL
		newURL := fmt.Sprintf("%s/api/chat", ms.service.URL)
		parsedURL, err := url.Parse(newURL)
		if err != nil {
			logger.Proxy().Printf("❌ 解析 URL 失败：%v", err)
			continue
		}
		httpReq.URL = parsedURL
		httpReq.Host = httpReq.URL.Host

		// 尝试新服务
		resp, err = p.httpClient.Do(httpReq)
		if err == nil && resp.StatusCode == http.StatusOK {
			logger.Proxy().Printf("✅ 备用服务成功：%s", ms.service.ID)
			// 更新会话绑定到新服务
			if sessionID != "" {
				p.createSessionBinding(sessionID, ms.service, ms.model)
			}
			return resp, nil
		}

		// 记录失败并继续尝试下一个
		if resp != nil {
			logger.Proxy().Printf("❌ 备用服务失败：HTTP %d", resp.StatusCode)
			resp.Body.Close()
		} else {
			logger.Proxy().Printf("❌ 备用服务失败：%v", err)
		}

		excludeServiceIDs[ms.service.ID] = true
	}

	// 所有服务都失败
	if resp != nil {
		return resp, fmt.Errorf("所有服务都不可用：%w", err)
	}
	return nil, fmt.Errorf("所有服务都不可用：%w", err)
}

// tryFallbackServices 尝试备用服务（非流式请求）
func (p *ProxyService) tryFallbackServices(ctx context.Context, req *OllamaChatRequest, excludeServiceID string, sessionID string) (*OllamaChatResponse, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	logger.Proxy().Printf("开始 fallback，排除服务：%s", excludeServiceID)

	// 获取所有匹配的服务
	modelName := req.Model
	matchingServices := p.findMatchingServices(modelName)

	for _, ms := range matchingServices {
		if ms.service.ID == excludeServiceID {
			continue // 跳过已排除的服务
		}

		logger.Proxy().Printf("🔄 尝试备用服务：%s (%s), 模型：%s", ms.service.ID, ms.service.URL, ms.model)

		// 递归调用自己，但不传入 excludeServiceID 以避免无限循环
		ollamaResp, err := p.sendOllamaRequestWithRetry(ctx, ms.service.URL, req, "", sessionID)
		if err == nil {
			logger.Proxy().Printf("✅ 备用服务成功：%s", ms.service.ID)
			// 更新会话绑定到新服务
			if sessionID != "" {
				p.createSessionBinding(sessionID, ms.service, ms.model)
			}
			return ollamaResp, nil
		}
		logger.Proxy().Printf("❌ 备用服务失败：%v", err)
	}

	return nil, fmt.Errorf("所有备用服务都不可用")
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
				Index: i, // 添加 index 字段
				ID:    toolCall.ID,
				Type:  toolCall.Type,
				Function: FunctionCall{
					Name: toolCall.Function.Name,
				},
			}

			// 处理 Arguments 字段，原封不动传递
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
		toolCall := tryParseToolUseFromText(GetContentString(ollamaResp.Message.Content))
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

// ToolCallConverter 提供 JSON 和 XML 工具调用格式的相互转换
type ToolCallConverter struct{}

func NewToolCallConverter() *ToolCallConverter {
	return &ToolCallConverter{}
}

// ClientType 客户端类型
type ClientType string

const (
	ClientTypeOpencode   ClientType = "opencode"    // JSON 格式
	ClientTypeClaudeCode ClientType = "claude-code" // XML 格式
	ClientTypeUnknown    ClientType = "unknown"
)

// ModelType 模型类型
type ModelType string

const (
	ModelTypeDeepSeek ModelType = "deepseek" // 使用 XML
	ModelTypeClaude   ModelType = "claude"   // 使用 XML
	ModelTypeOllama   ModelType = "ollama"   // 使用 JSON
	ModelTypeOpenAI   ModelType = "openai"   // 使用 JSON
	ModelTypeUnknown  ModelType = "unknown"
)

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

	// 增加重试机制和故障转移
	var ollamaResp *OllamaChatResponse
	maxRetries := p.maxRetries
	retryDelay := 2 * time.Second

	for attempt := 0; attempt <= maxRetries; attempt++ {
		ollamaResp, err = p.sendOllamaRequestWithRetry(ctx, service.URL, ollamaReq, service.ID, req.SessionID)
		if err == nil {
			// 成功则跳出
			break
		}

		// 最后一次尝试失败
		if attempt >= maxRetries {
			logger.Proxy().Printf("主服务请求失败，已重试 %d 次：%v", maxRetries, err)
			// 如果启用 fallback，尝试其他服务
			if p.config.FallbackEnabled {
				// 如果是会话绑定的服务失败，清除绑定
				if req.SessionID != "" {
					p.removeSessionBinding(req.SessionID)
				}
				return p.tryFallback(ctx, req, service.ID, req.SessionID)
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
	// 1. 首先检查会话绑定（如果有）
	if sessionID != "" {
		if service, modelName := p.getSessionBinding(sessionID); service != nil {
			return service, modelName, nil
		}
	}

	// 2. 确保服务列表已加载
	if err := p.ensureServicesLoaded(); err != nil {
		return nil, "", err
	}

	// 3. 查找匹配的服务和模型
	p.mu.RLock()
	defer p.mu.RUnlock()

	// 如果指定了模型名，查找所有匹配的服务
	if requestedModel != "" {
		matchingServices := p.findMatchingServices(requestedModel)
		if len(matchingServices) > 0 {
			service, modelName := p.selectBestService(matchingServices, requestedModel)
			if service != nil {
				logger.Proxy().Debug("智能路由选择：%s (%s), 模型：%s", service.ID, service.URL, modelName)
				// 创建会话绑定
				if sessionID != "" {
					p.createSessionBinding(sessionID, service, modelName)
				}
				return service, modelName, nil
			}
		}

		// 没找到匹配的服务，尝试刷新后重新查找
		p.mu.RUnlock()
		if err := p.RefreshServices(); err != nil {
			logger.Proxy().Printf("刷新服务列表失败：%v", err)
		} else {
			p.mu.RLock()
			matchingServices = p.findMatchingServices(requestedModel)
			if len(matchingServices) > 0 {
				service, modelName := p.selectBestService(matchingServices, requestedModel)
				if service != nil {
					logger.Proxy().Printf("刷新后选择：%s (%s), 模型：%s", service.ID, service.URL, modelName)
					if sessionID != "" {
						p.createSessionBinding(sessionID, service, modelName)
					}
					return service, modelName, nil
				}
			}
		}
	}

	// 4. 使用第一个可用服务
	if len(p.currentServices) > 0 && len(p.currentServices[0].Models) > 0 {
		service := p.currentServices[0]
		modelName := service.Models[0].Name
		if sessionID != "" {
			p.createSessionBinding(sessionID, service, modelName)
		}
		return service, modelName, nil
	}

	return nil, "", fmt.Errorf("没有可用的服务")
}

// getSessionBinding 获取会话绑定的服务
func (p *ProxyService) getSessionBinding(sessionID string) (*storage.OllamaService, string) {
	p.sessionMu.RLock()
	binding, exists := p.sessionBindings[sessionID]
	p.sessionMu.RUnlock()

	if !exists {
		return nil, ""
	}

	// 检查会话是否过期
	if time.Since(binding.LastUsedAt) > p.sessionTTL {
		p.sessionMu.Lock()
		delete(p.sessionBindings, sessionID)
		p.sessionMu.Unlock()
		return nil, ""
	}

	// 查找绑定的服务是否仍然存在
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, service := range p.currentServices {
		if service.ID == binding.ServiceID {
			// 更新最后使用时间
			p.sessionMu.Lock()
			binding.LastUsedAt = time.Now()
			binding.RequestCount++
			p.sessionMu.Unlock()
			logger.Proxy().Printf("使用会话绑定：%s -> %s (%s)", sessionID, binding.ServiceID, binding.ModelName)
			return service, binding.ModelName
		}
	}

	// 服务不存在，清理绑定
	p.sessionMu.Lock()
	delete(p.sessionBindings, sessionID)
	p.sessionMu.Unlock()
	return nil, ""
}

// ensureServicesLoaded 确保服务列表已加载
func (p *ProxyService) ensureServicesLoaded() error {
	p.mu.RLock()
	hasServices := len(p.currentServices) > 0
	p.mu.RUnlock()

	if hasServices {
		return nil
	}

	// 从存储加载服务
	services, err := p.storage.ListServices(context.Background(), storage.ServiceFilter{})
	if err != nil {
		return err
	}

	// 过滤出有模型的服务
	filteredServices := make([]*storage.OllamaService, 0)
	for _, svc := range services {
		if len(svc.Models) > 0 {
			filteredServices = append(filteredServices, svc)
		}
	}

	// 原子性更新：先更新缓存，再更新 currentServices
	p.serviceCache.Set(filteredServices)

	p.mu.Lock()
	p.currentServices = filteredServices
	p.mu.Unlock()

	logger.Proxy().Printf("从数据库加载了 %d 个服务，其中 %d 个有模型", len(services), len(filteredServices))
	if len(filteredServices) == 0 {
		logger.Proxy().Printf("警告：数据库中没有任何有模型的服务")
	}

	return nil
}

// findMatchingServices 查找匹配指定模型的服务
func (p *ProxyService) findMatchingServices(requestedModel string) []struct {
	service *storage.OllamaService
	model   string
} {
	var matchingServices []struct {
		service *storage.OllamaService
		model   string
	}

	logger.Proxy().Debug("查找模型：%s, currentServices 数量：%d", requestedModel, len(p.currentServices))

	for _, service := range p.currentServices {
		for _, model := range service.Models {
			if p.modelMatches(requestedModel, model.Name) {
				logger.Proxy().Debug("  匹配成功：%s (%s)", model.Name, service.URL)
				matchingServices = append(matchingServices, struct {
					service *storage.OllamaService
					model   string
				}{service: service, model: model.Name})
			}
		}
	}

	return matchingServices
}

// createSessionBinding 创建会话绑定
func (p *ProxyService) createSessionBinding(sessionID string, service *storage.OllamaService, modelName string) {
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

// removeSessionBinding 移除会话绑定
func (p *ProxyService) removeSessionBinding(sessionID string) {
	p.sessionMu.Lock()
	delete(p.sessionBindings, sessionID)
	p.sessionMu.Unlock()
	logger.Proxy().Printf("移除会话绑定：%s", sessionID)
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
	p.roundRobinMu.Lock()
	index := p.modelRoundRobin[requestedModel]
	if index >= len(matchingServices) {
		index = 0
	}

	selected := matchingServices[index]

	// 更新轮询索引
	p.modelRoundRobin[requestedModel] = (index + 1) % len(matchingServices)
	p.roundRobinMu.Unlock()

	logger.Proxy().Printf("轮询选择：索引=%d, 总数=%d, 选中=%s",
		index, len(matchingServices), selected.service.URL)

	return selected.service, selected.model
}

// tryFallback 尝试备用服务
func (p *ProxyService) tryFallback(ctx context.Context, req *ChatCompletionRequest, excludeServiceID string, sessionID string) (*ChatCompletionResponse, error) {
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
			// 如果有会话绑定，更新到新的服务
			if sessionID != "" {
				p.createSessionBinding(sessionID, service, service.Models[0].Name)
			}
			return p.convertToOpenAIResponse(ollamaResp, req.Model), nil
		}
		logger.Proxy().Printf("备用服务失败：%v", err)
	}

	return nil, fmt.Errorf("所有备用服务都不可用")
}

// convertToOllamaRequest 转换请求为 Ollama 格式
// convertToOllamaRequest 转换 OpenAI 格式为 Ollama 格式
func (p *ProxyService) convertToOllamaRequest(openaiReq *ChatCompletionRequest, model string) *OllamaChatRequest {
	// 检测模型类型
	modelType := detectModelType(model)
	logger.Proxy().Printf("🔍 检测到模型类型：%s (模型: %s)", modelType, model)

	// 复制消息（深度复制，清理 ToolCalls 中的 Arguments）
	messages := make([]Message, len(openaiReq.Messages))
	for i, msg := range openaiReq.Messages {
		// 处理工具调用消息（assistant消息包含tool_calls）
		if len(msg.ToolCalls) > 0 {
			messages[i] = Message{
				Role:    msg.Role,
				Content: msg.Content,
			}
			messages[i].ToolCalls = make([]ToolCall, len(msg.ToolCalls))
			for j, tc := range msg.ToolCalls {
				// 对于Ollama后端，总是将arguments从JSON字符串转换为对象
				var argsObj map[string]interface{}
				if argsStr, ok := tc.Function.Arguments.(string); ok {
					if err := json.Unmarshal([]byte(argsStr), &argsObj); err == nil {
						// 解析成功，使用对象格式
						messages[i].ToolCalls[j] = ToolCall{
							Index: tc.Index,
							ID:    tc.ID,
							Type:  tc.Type,
							Function: FunctionCall{
								Name:      tc.Function.Name,
								Arguments: argsObj,
							},
						}
					} else {
						// 解析失败，保持原样
						logger.Proxy().Warnf("解析 arguments 失败：%v，保持原样", err)
						messages[i].ToolCalls[j] = ToolCall{
							Index: tc.Index,
							ID:    tc.ID,
							Type:  tc.Type,
							Function: FunctionCall{
								Name:      tc.Function.Name,
								Arguments: tc.Function.Arguments,
							},
						}
					}
				} else {
					// 已经是对象格式，直接使用
					messages[i].ToolCalls[j] = ToolCall{
						Index: tc.Index,
						ID:    tc.ID,
						Type:  tc.Type,
						Function: FunctionCall{
							Name:      tc.Function.Name,
							Arguments: tc.Function.Arguments,
						},
					}
				}
			}
			logger.Proxy().Printf("🔄 转换消息 %d 的 tool_calls 为对象格式（Ollama后端）", i)
		} else if msg.ToolCallID != "" {
			// 对于工具响应消息，从 tool_call_id 中提取工具名称
			// Ollama 后端使用 tool_name 而不是 tool_call_id
			toolName := msg.ToolCallID
			if strings.HasPrefix(msg.ToolCallID, "call") {
				// 尝试从 tool_call_id 中提取工具名
				// 格式可能是 call_xxx 或 call_xxx_toolname
				parts := strings.Split(msg.ToolCallID, "_")
				if len(parts) > 1 {
					// 取最后一个部分作为工具名
					toolName = parts[len(parts)-1]
				}
			}
			messages[i] = Message{
				Role:    msg.Role,
				Content: msg.Content,
				// Ollama 使用 tool_name 字段
				ToolCallID: toolName,
			}
			logger.Proxy().Printf("🔄 转换工具响应消息 tool_call_id=%s -> tool_name=%s", msg.ToolCallID, toolName)
		} else {
			// 普通消息，保持原样
			messages[i] = Message{
				Role:       msg.Role,
				Content:    msg.Content,
				ToolCallID: msg.ToolCallID,
			}
		}
	}

	// 获取模型的上下文长度（用于智能设置 max_tokens）
	ctx := context.Background()
	modelContextLength := p.GetModelContextLength(ctx, model)

	// 设置 max_tokens
	// 1. 如果客户端传了 max_tokens，使用客户端的值（但不超过模型上下文长度的 75%）
	// 2. 如果客户端没传，使用模型上下文长度的 50%（确保留足够空间给输入）
	maxTokens := openaiReq.MaxTokens
	if maxTokens <= 0 {
		// 客户端没传，使用模型上下文长度的 50%
		maxTokens = modelContextLength / 2
		if maxTokens < 512 {
			maxTokens = 512 // 最小 512 个 token
		}
		logger.Proxy().Debug("模型 %s 上下文长度：%d，设置 max_tokens：%d（50%%）", model, modelContextLength, maxTokens)
	} else {
		// 客户端传了，但确保不超过模型上下文长度的 75%（保留空间给输入）
		maxAllowed := modelContextLength * 3 / 4
		if maxTokens > maxAllowed {
			logger.Proxy().Warn("客户端请求的 max_tokens (%d) 超过模型上下文长度的 75%% (%d)，调整为：%d", maxTokens, maxAllowed, maxAllowed)
			maxTokens = maxAllowed
		}
	}

	// 设置更强的重复惩罚参数（防止模型陷入循环）
	repeatPenalty := 1.5 // 从 1.1 提高到 1.5，更强地惩罚重复内容
	repeatLastN := 256   // 从 128 提高到 256，对更多历史 token 应用惩罚

	logger.Proxy().Debug("设置防循环参数：repeat_penalty=%.2f, repeat_last_n=%d", repeatPenalty, repeatLastN)

	return &OllamaChatRequest{
		Model:    model,
		Messages: messages,
		Stream:   openaiReq.Stream,
		Options: Options{
			Temperature:      openaiReq.Temperature,
			TopP:             openaiReq.TopP,
			NumPredict:       maxTokens,
			PresencePenalty:  openaiReq.PresencePenalty,
			FrequencyPenalty: openaiReq.FrequencyPenalty,
			RepeatPenalty:    repeatPenalty, // 使用更强的重复惩罚（1.5）
			RepeatLastN:      repeatLastN,   // 对更多历史 token 应用惩罚（256）
			Stop:             openaiReq.Stop,
		},
		// 直接传递工具定义（不做任何修改）
		Tools: openaiReq.Tools,
	}
}

// sendOllamaRequest 发送请求到 Ollama（带故障转移）
func (p *ProxyService) sendOllamaRequest(ctx context.Context, baseURL string, req *OllamaChatRequest) (*OllamaChatResponse, error) {
	return p.sendOllamaRequestWithRetry(ctx, baseURL, req, "", "")
}

// sendOllamaRequestWithRetry 发送请求到 Ollama 并支持重试和故障转移
func (p *ProxyService) sendOllamaRequestWithRetry(ctx context.Context, baseURL string, req *OllamaChatRequest, excludeServiceID string, sessionID string) (*OllamaChatResponse, error) {
	url := fmt.Sprintf("%s/api/chat", baseURL)

	// 强制设置为非流式模式
	req.Stream = false

	jsonData, err := json.Marshal(req)
	if err != nil {
		logger.Proxy().Printf("❌ JSON 序列化失败：%v", err)
		return nil, err
	}

	// 【调试】打印完整的请求内容
	logger.Proxy().Printf("\n========================================")
	logger.Proxy().Printf("📤 [中转请求] 发送到：%s", url)
	logger.Proxy().Printf("🕐 时间：%s", time.Now().Format("2006-01-02 15:04:05.000"))
	logger.Proxy().Printf("📄 请求体 (JSON):")
	logger.Proxy().Printf("%s", formatJSON(string(jsonData)))
	logger.Proxy().Printf("📊 消息数量：%d", len(req.Messages))
	for i, msg := range req.Messages {
		logger.Proxy().Printf("  📨 消息 %d:", i+1)
		logger.Proxy().Printf("     Role: %s", msg.Role)
		logger.Proxy().Printf("     Content: %v", msg.Content)
		if len(msg.ToolCalls) > 0 {
			logger.Proxy().Printf("     ToolCalls: %d 个", len(msg.ToolCalls))
			for j, tc := range msg.ToolCalls {
				logger.Proxy().Printf("       [%d] ID: %s, Type: %s, Name: %s", j, tc.ID, tc.Type, tc.Function.Name)
				logger.Proxy().Printf("           Args: %v", tc.Function.Arguments)
			}
		}
		if msg.ToolCallID != "" {
			logger.Proxy().Printf("     ToolCallID: %s", msg.ToolCallID)
		}
	}
	logger.Proxy().Printf("🔧 Tools: %d 个", len(req.Tools))
	for i, tool := range req.Tools {
		logger.Proxy().Printf("  🛠️  工具 %d: %s (%s)", i+1, tool.Function.Name, tool.Type)
	}
	logger.Proxy().Printf("========================================\n")

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	logger.Proxy().Printf("⏳ 正在发送 HTTP 请求...")
	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		logger.Proxy().Printf("❌ 请求失败：%v", err)
		return nil, err
	}
	defer resp.Body.Close()

	logger.Proxy().Printf("✅ 收到响应：HTTP %d", resp.StatusCode)

	// 检查是否需要故障转移（403, 503 等错误）
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		logger.Proxy().Printf("❌ Ollama 返回错误 [%d]: %s", resp.StatusCode, string(body))

		// 检查是否是可重试的错误
		isRetryable := resp.StatusCode == http.StatusForbidden ||
			resp.StatusCode == http.StatusServiceUnavailable ||
			resp.StatusCode == http.StatusBadGateway ||
			resp.StatusCode == http.StatusGatewayTimeout

		if isRetryable && excludeServiceID != "" {
			logger.Proxy().Printf("⚠️  检测到可重试错误：HTTP %d，尝试故障转移...", resp.StatusCode)
			// 清除会话绑定
			if sessionID != "" {
				p.removeSessionBinding(sessionID)
			}
			// 尝试其他服务
			return p.tryFallbackServices(ctx, req, excludeServiceID, sessionID)
		}

		return nil, fmt.Errorf("Ollama 返回错误 [%d]: %s", resp.StatusCode, string(body))
	}

	// 逐行读取响应，直到 done=true
	var lastResp OllamaChatResponse
	var fullContent strings.Builder // 累积完整内容
	var rawResponse strings.Builder // 保存原始响应
	foundDone := false

	logger.Proxy().Printf("📥 开始读取响应数据...")
	scanner := bufio.NewScanner(resp.Body)
	chunkCount := 0
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		chunkCount++
		rawResponse.Write(line)
		rawResponse.WriteByte('\n')

		var ollamaResp OllamaChatResponse
		if err := json.Unmarshal(line, &ollamaResp); err != nil {
			logger.Proxy().Printf("⚠️  JSON 解析失败：%v, 数据：%s", err, string(line))
			continue
		}

		// 检查是否有错误
		if ollamaResp.Error != "" {
			logger.Proxy().Printf("❌ Ollama 返回错误：%s", ollamaResp.Error)
			return nil, fmt.Errorf("Ollama 错误：%s", ollamaResp.Error)
		}

		// 累积内容（Ollama 流式响应中每个 chunk 都有部分 content）
		content := GetContentString(ollamaResp.Message.Content)
		if content != "" {
			fullContent.WriteString(content)
		}

		lastResp = ollamaResp
		if ollamaResp.Done {
			foundDone = true
			logger.Proxy().Printf("✅ 收到完成标志 (Done=true)")
			break
		}
	}

	logger.Proxy().Printf("📊 响应统计：共 %d 个 chunk, 解析成功 %d 个", chunkCount, lastResp.EvalCount)

	if !foundDone {
		logger.Proxy().Printf("⚠️  警告：未收到完成响应")
		return nil, fmt.Errorf("未收到完成响应")
	}

	// 使用累积的完整内容
	lastResp.Message.Content = fullContent.String()

	// 【调试】打印完整的响应内容
	logger.Proxy().Printf("\n========================================")
	logger.Proxy().Printf("📥 [中转响应] 来自：%s", url)
	logger.Proxy().Printf("🕐 时间：%s", time.Now().Format("2006-01-02 15:04:05.000"))
	logger.Proxy().Printf("📄 响应体 (JSON):")
	logger.Proxy().Printf("%s", formatJSON(rawResponse.String()))
	logger.Proxy().Printf("📊 响应详情:")
	logger.Proxy().Printf("  Model: %s", lastResp.Model)
	logger.Proxy().Printf("  CreatedAt: %s", lastResp.CreatedAt)
	logger.Proxy().Printf("  Done: %v", lastResp.Done)
	logger.Proxy().Printf("  EvalCount: %d", lastResp.EvalCount)
	logger.Proxy().Printf("  Content Length: %d", len(GetContentString(lastResp.Message.Content)))
	if len(lastResp.Message.ToolCalls) > 0 {
		logger.Proxy().Printf("  🔧 ToolCalls: %d 个", len(lastResp.Message.ToolCalls))
		for i, tc := range lastResp.Message.ToolCalls {
			logger.Proxy().Printf("    [%d] ID: %s, Type: %s, Name: %s", i, tc.ID, tc.Type, tc.Function.Name)
			logger.Proxy().Printf("        Args: %v", tc.Function.Arguments)
		}
	}
	logger.Proxy().Printf("========================================\n")

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
				Index: i, // 添加 index 字段
				ID:    toolCall.ID,
				Type:  toolCall.Type,
				Function: FunctionCall{
					Name: toolCall.Function.Name,
				},
			}

			// 处理 Arguments 字段，原封不动传递
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
		logger.Proxy().Printf("🔧 检测到原生工具调用：%d 个", len(message.ToolCalls))
	} else {
		logger.Proxy().Printf("📝 普通文本响应（未检测到工具调用）")
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
	// 1. 精确匹配
	if requested == available {
		return true
	}

	// 2. 忽略标签匹配（如 llama2:7b 匹配 llama2:latest）
	requestedParts := strings.Split(requested, ":")
	availableParts := strings.Split(available, ":")

	requestedBase := requestedParts[0]
	availableBase := availableParts[0]

	// 基础名称必须完全相同
	if requestedBase != availableBase {
		return false
	}

	// 3. 如果请求方没有指定标签，匹配任何标签（如 llama2 匹配 llama2:latest）
	if len(requestedParts) == 1 {
		return true
	}

	// 4. 如果都指定了标签，使用更严格的匹配规则
	if len(requestedParts) > 1 && len(availableParts) > 1 {
		requestedTag := requestedParts[1]
		availableTag := availableParts[1]

		// 4.1 标签精确匹配（如 latest == latest）
		if requestedTag == availableTag {
			return true
		}

		// 4.2 允许量化版本匹配（如 latest 匹配 latest-q4_K_M）
		// 规则：可用标签必须是 "请求标签-量化后缀" 格式
		// 这避免了 7b 匹配到 70b 的问题（因为 "70b" != "7b-*"）
		if len(availableTag) > len(requestedTag)+1 {
			prefix := requestedTag + "-"
			if strings.HasPrefix(availableTag, prefix) {
				return true
			}
		}

		return false
	}

	return false
}

// RefreshServices 刷新服务列表（使用缓存优化）
func (p *ProxyService) RefreshServices() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 检查是否应该刷新缓存
	if !p.serviceCache.ShouldRefresh() {
		logger.Proxy().Debug("缓存未过期，跳过刷新")
		return nil
	}

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

	// 更新缓存
	p.serviceCache.Set(p.currentServices)

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

// GetCacheStats 获取缓存统计信息
func (p *ProxyService) GetCacheStats() map[string]interface{} {
	return map[string]interface{}{
		"service":  p.serviceCache.GetStats(),
		"response": p.responseCache.GetStats(),
	}
}

// GetMetrics 获取 Prometheus 指标收集器
func (p *ProxyService) GetMetrics() *Metrics {
	return p.metrics
}

// InvalidateCache 使缓存失效
func (p *ProxyService) InvalidateCache() {
	p.serviceCache.Invalidate()
	logger.Proxy().Printf("服务缓存已失效")
}

// GetContentString 从 Message.Content 获取字符串内容
func GetContentString(content interface{}) string {
	switch v := content.(type) {
	case string:
		return v
	case []interface{}:
		// 处理多模态内容
		var result strings.Builder
		for _, part := range v {
			if partMap, ok := part.(map[string]interface{}); ok {
				if partType, ok := partMap["type"].(string); ok {
					if partType == "text" {
						if text, ok := partMap["text"].(string); ok {
							result.WriteString(text)
						}
					}
					// 忽略图像，因为 Ollama 会直接处理
				}
			}
		}
		return result.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

// Embeddings 生成文本嵌入向量
func (p *ProxyService) Embeddings(ctx context.Context, req *EmbeddingRequest) (*EmbeddingResponse, error) {
	p.mu.RLock()
	services := p.currentServices

	// 如果没有服务，尝试从缓存获取
	if len(services) == 0 {
		p.mu.RUnlock()
		if cachedServices, found := p.serviceCache.Get(); found {
			p.mu.Lock()
			p.currentServices = cachedServices
			services = cachedServices
			p.mu.Unlock()
		} else {
			return nil, fmt.Errorf("no available service")
		}
	} else {
		p.mu.RUnlock()
	}

	// 智能路由：选择支持该模型的服务
	svc := p.selectServiceForModel(req.Model, services)
	if svc == nil {
		return nil, fmt.Errorf("no service supports model: %s", req.Model)
	}

	logger.Proxy().Printf("Embeddings 请求 -> 服务：%s, 模型：%s", svc.Name, req.Model)

	// 处理输入（支持字符串或字符串数组）
	var inputs []string
	switch v := req.Input.(type) {
	case string:
		inputs = []string{v}
	case []interface{}:
		inputs = make([]string, 0, len(v))
		for _, item := range v {
			if str, ok := item.(string); ok {
				inputs = append(inputs, str)
			}
		}
	case []string:
		inputs = v
	default:
		return nil, fmt.Errorf("invalid input type")
	}

	if len(inputs) == 0 {
		return nil, fmt.Errorf("empty input")
	}

	// 调用 Ollama Embeddings API
	embeddings := make([]Embedding, 0, len(inputs))
	totalTokens := 0

	for i, input := range inputs {
		ollamaReq := OllamaEmbeddingRequest{
			Model:  req.Model,
			Prompt: input,
		}

		reqBody, err := json.Marshal(ollamaReq)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}

		// 构建 Ollama API URL
		apiURL := fmt.Sprintf("%s/api/embeddings", svc.URL)

		// 创建请求
		httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(reqBody))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		httpReq.Header.Set("Content-Type", "application/json")

		// 发送请求
		resp, err := p.httpClient.Do(httpReq)
		if err != nil {
			return nil, fmt.Errorf("failed to call Ollama: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("Ollama returned status %d: %s", resp.StatusCode, string(body))
		}

		// 解析响应
		var ollamaResp OllamaEmbeddingResponse
		if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		// 添加到结果
		embeddings = append(embeddings, Embedding{
			Object:    "embedding",
			Embedding: ollamaResp.Embedding,
			Index:     i,
		})

		// 简单估算 token 数（实际应该使用 tokenizer）
		totalTokens += len(input) / 4
	}

	// 构建响应
	return &EmbeddingResponse{
		Object: "list",
		Data:   embeddings,
		Model:  req.Model,
		Usage: Usage{
			PromptTokens:     totalTokens,
			CompletionTokens: 0,
			TotalTokens:      totalTokens,
		},
	}, nil
}

// selectServiceForModel 选择支持指定模型的服务
func (p *ProxyService) selectServiceForModel(modelName string, services []*storage.OllamaService) *storage.OllamaService {
	// 首先尝试精确匹配
	for _, svc := range services {
		for _, model := range svc.Models {
			if model.Name == modelName && model.IsAvailable {
				return svc
			}
		}
	}

	// 如果没有精确匹配，使用第一个可用服务
	if len(services) > 0 {
		return services[0]
	}

	return nil
}

// CreateTranscription 创建音频转录
func (p *ProxyService) CreateTranscription(ctx context.Context, req *AudioTranscriptionRequest) (*AudioTranscriptionResponse, error) {
	p.mu.RLock()
	services := p.currentServices

	if len(services) == 0 {
		p.mu.RUnlock()
		if cachedServices, found := p.serviceCache.Get(); found {
			p.mu.Lock()
			p.currentServices = cachedServices
			services = cachedServices
			p.mu.Unlock()
		} else {
			return nil, fmt.Errorf("no available service")
		}
	} else {
		p.mu.RUnlock()
	}

	svc := p.selectServiceForModel(req.Model, services)
	if svc == nil {
		return nil, fmt.Errorf("no service supports model: %s", req.Model)
	}

	logger.Proxy().Printf("Audio Transcription 请求 -> 服务：%s, 模型：%s", svc.Name, req.Model)

	ollamaReq := map[string]interface{}{
		"model":  req.Model,
		"prompt": "Transcribe the following audio:",
		"audio":  req.File,
	}

	reqBody, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	apiURL := fmt.Sprintf("%s/api/generate", svc.URL)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to call Ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Ollama returned status %d: %s", resp.StatusCode, string(body))
	}

	var ollamaResp struct {
		Response string `json:"response"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &AudioTranscriptionResponse{
		Text: ollamaResp.Response,
	}, nil
}

// CreateTranslation 创建音频翻译
func (p *ProxyService) CreateTranslation(ctx context.Context, req *AudioTranslationRequest) (*AudioTranslationResponse, error) {
	p.mu.RLock()
	services := p.currentServices

	if len(services) == 0 {
		p.mu.RUnlock()
		if cachedServices, found := p.serviceCache.Get(); found {
			p.mu.Lock()
			p.currentServices = cachedServices
			services = cachedServices
			p.mu.Unlock()
		} else {
			return nil, fmt.Errorf("no available service")
		}
	} else {
		p.mu.RUnlock()
	}

	svc := p.selectServiceForModel(req.Model, services)
	if svc == nil {
		return nil, fmt.Errorf("no service supports model: %s", req.Model)
	}

	logger.Proxy().Printf("Audio Translation 请求 -> 服务：%s, 模型：%s", svc.Name, req.Model)

	ollamaReq := map[string]interface{}{
		"model":  req.Model,
		"prompt": "Translate the following audio to English:",
		"audio":  req.File,
	}

	reqBody, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	apiURL := fmt.Sprintf("%s/api/generate", svc.URL)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to call Ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Ollama returned status %d: %s", resp.StatusCode, string(body))
	}

	var ollamaResp struct {
		Response string `json:"response"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &AudioTranslationResponse{
		Text: ollamaResp.Response,
	}, nil
}

// CreateSpeech 创建语音合成（TTS）
func (p *ProxyService) CreateSpeech(ctx context.Context, req *SpeechRequest) (*SpeechResponse, error) {
	p.mu.RLock()
	services := p.currentServices

	if len(services) == 0 {
		p.mu.RUnlock()
		if cachedServices, found := p.serviceCache.Get(); found {
			p.mu.Lock()
			p.currentServices = cachedServices
			services = cachedServices
			p.mu.Unlock()
		} else {
			return nil, fmt.Errorf("no available service")
		}
	} else {
		p.mu.RUnlock()
	}

	svc := p.selectServiceForModel(req.Model, services)
	if svc == nil {
		return nil, fmt.Errorf("no service supports model: %s", req.Model)
	}

	logger.Proxy().Printf("Speech 请求 -> 服务：%s, 模型：%s", svc.Name, req.Model)

	ollamaReq := map[string]interface{}{
		"model": req.Model,
		"input": req.Input,
		"voice": req.Voice,
		"speed": req.Speed,
	}

	reqBody, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	apiURL := fmt.Sprintf("%s/api/tts", svc.URL)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to call Ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Ollama returned status %d: %s", resp.StatusCode, string(body))
	}

	audioData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read audio data: %w", err)
	}

	contentType := "audio/mpeg"
	if req.ResponseFormat == "opus" {
		contentType = "audio/opus"
	} else if req.ResponseFormat == "aac" {
		contentType = "audio/aac"
	} else if req.ResponseFormat == "flac" {
		contentType = "audio/flac"
	}

	return &SpeechResponse{
		AudioData:   audioData,
		ContentType: contentType,
	}, nil
}

// CreateModeration 创建内容审核
func (p *ProxyService) CreateModeration(ctx context.Context, req *ModerationRequest) (*ModerationResponse, error) {
	p.mu.RLock()
	services := p.currentServices

	if len(services) == 0 {
		p.mu.RUnlock()
		if cachedServices, found := p.serviceCache.Get(); found {
			p.mu.Lock()
			p.currentServices = cachedServices
			services = cachedServices
			p.mu.Unlock()
		} else {
			return nil, fmt.Errorf("no available service")
		}
	} else {
		p.mu.RUnlock()
	}

	svc := p.selectServiceForModel(req.Model, services)
	if svc == nil {
		return nil, fmt.Errorf("no service supports model: %s", req.Model)
	}

	logger.Proxy().Printf("Moderation 请求 -> 服务：%s, 模型：%s", svc.Name, req.Model)

	// 处理输入
	var inputs []string
	switch v := req.Input.(type) {
	case string:
		inputs = []string{v}
	case []interface{}:
		inputs = make([]string, 0, len(v))
		for _, item := range v {
			if str, ok := item.(string); ok {
				inputs = append(inputs, str)
			}
		}
	case []string:
		inputs = v
	default:
		return nil, fmt.Errorf("invalid input type")
	}

	if len(inputs) == 0 {
		return nil, fmt.Errorf("empty input")
	}

	results := make([]ModerationResult, 0, len(inputs))

	for _, input := range inputs {
		messages := []Message{
			{
				Role: "system",
				Content: `You are a content moderator. Analyze the given text and respond with a JSON object indicating if it contains:
- hate speech
- hate/threatening content
- harassment
- self-harm
- sexual content
- violence
- graphic violence

Respond ONLY with a JSON object in this format:
{
  "hate": false,
  "hate_threatening": false,
  "harassment": false,
  "self_harm": false,
  "sexual": false,
  "violence": false,
  "violence_graphic": false
}`,
			},
			{
				Role:    "user",
				Content: fmt.Sprintf("Analyze this text for safety: %s", input),
			},
		}

		chatReq := &ChatCompletionRequest{
			Model:    req.Model,
			Messages: messages,
		}

		resp, err := p.ChatCompletions(ctx, chatReq)
		if err != nil {
			results = append(results, ModerationResult{
				Categories:     Categories{},
				CategoryScores: CategoryScores{},
				Flagged:        false,
			})
			continue
		}

		var analysis struct {
			Hate            bool `json:"hate"`
			HateThreatening bool `json:"hate_threatening"`
			Harassment      bool `json:"harassment"`
			SelfHarm        bool `json:"self_harm"`
			Sexual          bool `json:"sexual"`
			Violence        bool `json:"violence"`
			ViolenceGraphic bool `json:"violence_graphic"`
		}

		content := GetContentString(resp.Choices[0].Message.Content)
		if err := json.Unmarshal([]byte(content), &analysis); err != nil {
			results = append(results, ModerationResult{
				Categories:     Categories{},
				CategoryScores: CategoryScores{},
				Flagged:        false,
			})
			continue
		}

		categoryScores := CategoryScores{
			Hate:            boolToFloat(analysis.Hate),
			HateThreatening: boolToFloat(analysis.HateThreatening),
			Harassment:      boolToFloat(analysis.Harassment),
			SelfHarm:        boolToFloat(analysis.SelfHarm),
			Sexual:          boolToFloat(analysis.Sexual),
			Violence:        boolToFloat(analysis.Violence),
			ViolenceGraphic: boolToFloat(analysis.ViolenceGraphic),
		}

		categories := Categories{
			Hate:            analysis.Hate,
			HateThreatening: analysis.HateThreatening,
			Harassment:      analysis.Harassment,
			SelfHarm:        analysis.SelfHarm,
			Sexual:          analysis.Sexual,
			Violence:        analysis.Violence,
			ViolenceGraphic: analysis.ViolenceGraphic,
		}

		flagged := analysis.Hate || analysis.HateThreatening || analysis.Harassment ||
			analysis.SelfHarm || analysis.Sexual || analysis.Violence || analysis.ViolenceGraphic

		results = append(results, ModerationResult{
			Categories:     categories,
			CategoryScores: categoryScores,
			Flagged:        flagged,
		})
	}

	return &ModerationResponse{
		ID:      fmt.Sprintf("modr-%d", time.Now().Unix()),
		Model:   req.Model,
		Results: results,
	}, nil
}

func boolToFloat(b bool) float64 {
	if b {
		return 1.0
	}
	return 0.0
}

// truncateString 截断字符串（用于日志显示）
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// formatJSON 格式化 JSON 字符串（美化输出）
func formatJSON(jsonStr string) string {
	var prettyJSON bytes.Buffer
	err := json.Indent(&prettyJSON, []byte(jsonStr), "", "  ")
	if err != nil {
		// 如果格式化失败，返回原始字符串
		return jsonStr
	}
	return prettyJSON.String()
}

// GetResponseCacheStats 获取响应缓存统计信息
func (p *ProxyService) GetResponseCacheStats() map[string]interface{} {
	return p.responseCache.GetStats()
}

// SetResponseCacheEnabled 设置响应缓存开关
func (p *ProxyService) SetResponseCacheEnabled(enabled bool) {
	p.responseCache.SetEnabled(enabled)
	logger.Proxy().Printf("响应缓存已%s", map[bool]string{true: "启用", false: "禁用"}[enabled])
}

// ClearResponseCache 清空响应缓存
func (p *ProxyService) ClearResponseCache() {
	p.responseCache.Clear()
	logger.Proxy().Printf("响应缓存已清空")
}

// CleanupExpiredResponseCache 清理过期缓存条目
func (p *ProxyService) CleanupExpiredResponseCache() int {
	count := p.responseCache.CleanupExpired()
	if count > 0 {
		logger.Proxy().Printf("清理了 %d 个过期缓存条目", count)
	}
	return count
}

// GetModelContextLength 获取模型的上下文长度（带缓存）
// 优先从缓存读取，缓存未命中则从数据库或 Ollama API 获取
func (p *ProxyService) GetModelContextLength(ctx context.Context, modelName string) int {
	// 1. 先检查缓存
	p.contextCacheMu.RLock()
	if contextLength, exists := p.modelContextCache[modelName]; exists {
		p.contextCacheMu.RUnlock()
		logger.Proxy().Debug("从缓存获取模型上下文长度：%s = %d", modelName, contextLength)
		return contextLength
	}
	p.contextCacheMu.RUnlock()

	// 2. 从数据库获取模型信息
	services, err := p.storage.ListServices(ctx, storage.ServiceFilter{})
	if err == nil {
		for _, service := range services {
			for _, model := range service.Models {
				if model.Name == modelName && model.ContextLength > 0 {
					// 找到了模型信息，存入缓存
					p.contextCacheMu.Lock()
					p.modelContextCache[modelName] = model.ContextLength
					p.contextCacheMu.Unlock()
					logger.Proxy().Debug("从数据库获取模型上下文长度：%s = %d", modelName, model.ContextLength)
					return model.ContextLength
				}
			}
		}
	}

	// 3. 从 Ollama API 获取（使用第一个可用的服务）
	if len(services) > 0 {
		for _, service := range services {
			if service.Status == storage.StatusOnline {
				// 直接调用 Ollama API 获取模型详情
				contextLength, err := p.fetchModelContextFromOllama(ctx, service.URL, modelName)
				if err == nil && contextLength > 0 {
					// 存入缓存
					p.contextCacheMu.Lock()
					p.modelContextCache[modelName] = contextLength
					p.contextCacheMu.Unlock()

					logger.Proxy().Debug("从 Ollama API 获取模型上下文长度：%s = %d", modelName, contextLength)
					return contextLength
				}
			}
		}
	}

	// 4. 使用默认值
	defaultContextLength := 2048
	logger.Proxy().Warn("无法获取模型 %s 的上下文长度，使用默认值：%d", modelName, defaultContextLength)
	return defaultContextLength
}

// fetchModelContextFromOllama 从 Ollama API 获取模型的上下文长度
func (p *ProxyService) fetchModelContextFromOllama(ctx context.Context, baseURL, modelName string) (int, error) {
	url := fmt.Sprintf("%s/api/show", baseURL)

	payload := map[string]interface{}{
		"name": modelName,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.detectorClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("状态码：%d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var showResp struct {
		Parameters struct {
			NumCtx int `json:"num_ctx"` // 上下文长度
		} `json:"parameters"`
	}

	if err := json.Unmarshal(body, &showResp); err != nil {
		return 0, err
	}

	// 如果没有返回上下文长度，使用默认值
	contextLength := showResp.Parameters.NumCtx
	if contextLength == 0 {
		contextLength = 2048 // 默认 2048
	}

	return contextLength, nil
}

// ClearModelContextCache 清空模型上下文长度缓存
func (p *ProxyService) ClearModelContextCache() {
	p.contextCacheMu.Lock()
	defer p.contextCacheMu.Unlock()
	p.modelContextCache = make(map[string]int)
	logger.Proxy().Printf("模型上下文长度缓存已清空")
}
