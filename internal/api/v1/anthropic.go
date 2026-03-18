package v1

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"oppama/internal/proxy"
	"oppama/internal/storage"
	"oppama/internal/utils/logger"

	"github.com/gin-gonic/gin"
)

// AnthropicHandler Anthropic API 兼容处理器
type AnthropicHandler struct {
	storage storage.Storage
	proxy   *proxy.ProxyService
}

// NewAnthropicHandler 创建 Anthropic API 处理器
func NewAnthropicHandler(storage storage.Storage, proxy *proxy.ProxyService) *AnthropicHandler {
	return &AnthropicHandler{
		storage: storage,
		proxy:   proxy,
	}
}

// SystemContent 系统提示内容，支持字符串或内容块数组
type SystemContent struct {
	RawString   string         `json:"-"`
	ContentList []ContentBlock `json:"-"`
	IsArray     bool           `json:"-"`
}

// UnmarshalJSON 实现自定义 JSON 解包，兼容字符串和数组格式
func (sc *SystemContent) UnmarshalJSON(data []byte) error {
	// 尝试解析为字符串
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		sc.RawString = s
		sc.IsArray = false
		return nil
	}

	// 尝试解析为内容块数组
	var blocks []ContentBlock
	if err := json.Unmarshal(data, &blocks); err == nil {
		sc.ContentList = blocks
		sc.IsArray = true
		// 合并所有文本内容
		for _, block := range blocks {
			if block.Type == "text" {
				sc.RawString += block.Text
			}
			// 对于图像内容，添加到原始字符串中作为描述
			if block.Type == "image" && block.Source != nil {
				sc.RawString += "[图像]"
			}
		}
		return nil
	}

	return fmt.Errorf("system must be either a string or an array of content blocks")
}

// MarshalJSON 实现自定义 JSON 打包
func (sc *SystemContent) MarshalJSON() ([]byte, error) {
	if sc.IsArray {
		return json.Marshal(sc.ContentList)
	}
	return json.Marshal(sc.RawString)
}

// String 返回字符串表示
func (sc *SystemContent) String() string {
	return sc.RawString
}

// Tool 工具定义
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"input_schema"` // JSON Schema
}

// ToolChoice 工具选择
type ToolChoice struct {
	Type string `json:"type"` // "auto", "any", "tool"
	Name string `json:"name,omitempty"`
}

// ResponseFormat 响应格式
type ResponseFormat struct {
	Type string `json:"type"` // "text" 或 "json_object"
}

// MessagesRequest Anthropic Messages API 请求
type MessagesRequest struct {
	Model          string          `json:"model" binding:"required"`
	Messages       []Message       `json:"messages" binding:"required,min=1"`
	System         SystemContent   `json:"system,omitempty"`
	MaxTokens      int             `json:"max_tokens" binding:"required"`
	Temperature    float64         `json:"temperature,omitempty"`
	TopP           float64         `json:"top_p,omitempty"`
	TopK           int             `json:"top_k,omitempty"`
	StopSequences  []string        `json:"stop_sequences,omitempty"`
	Stream         bool            `json:"stream,omitempty"`
	Metadata       *Metadata       `json:"metadata,omitempty"`
	Tools          []Tool          `json:"tools,omitempty"`           // 可用工具列表
	ToolChoice     *ToolChoice     `json:"tool_choice,omitempty"`     // 工具选择策略
	ResponseFormat *ResponseFormat `json:"response_format,omitempty"` // 响应格式
}

// ImageSource 图像源，支持 base64 和 url
type ImageSource struct {
	Type      string `json:"type"`           // "base64" 或 "url"
	MediaType string `json:"media_type"`     // "image/jpeg", "image/png", "image/gif", "image/webp"
	Data      string `json:"data,omitempty"` // base64 编码的图像数据
	URL       string `json:"url,omitempty"`  // 图像 URL
}

// ContentBlock 内容块，支持文本、图像和工具使用
type ContentBlock struct {
	Type    string       `json:"type"`               // "text", "image", "tool_use", "tool_result"
	Text    string       `json:"text,omitempty"`     // 文本内容
	Source  *ImageSource `json:"source,omitempty"`   // 图像源
	ID      string       `json:"id,omitempty"`       // 工具使用 ID
	Name    string       `json:"name,omitempty"`     // 工具名称
	Input   interface{}  `json:"input,omitempty"`    // 工具输入
	Content interface{}  `json:"content,omitempty"`  // 工具结果内容
	IsError bool         `json:"is_error,omitempty"` // 工具调用是否出错
}

// MessageContent 消息内容，支持字符串或内容块数组
type MessageContent struct {
	RawString   string
	ContentList []ContentBlock
	IsArray     bool
}

// UnmarshalJSON 实现自定义 JSON 解包，兼容字符串和数组格式
func (mc *MessageContent) UnmarshalJSON(data []byte) error {
	// 尝试解析为字符串
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		mc.RawString = s
		mc.IsArray = false
		return nil
	}

	// 尝试解析为内容块数组
	var blocks []ContentBlock
	if err := json.Unmarshal(data, &blocks); err == nil {
		mc.ContentList = blocks
		mc.IsArray = true
		// 合并所有文本内容
		for _, block := range blocks {
			switch block.Type {
			case "text":
				mc.RawString += block.Text
			case "image":
				if block.Source != nil {
					mc.RawString += "[图像]"
				}
			case "tool_use":
				mc.RawString += fmt.Sprintf("[工具调用：%s]", block.Name)
			case "tool_result":
				if result, ok := block.Content.(string); ok {
					mc.RawString += fmt.Sprintf("[工具结果：%s]", result)
				}
			}
		}
		return nil
	}

	return fmt.Errorf("content must be either a string or an array of content blocks")
}

// MarshalJSON 实现自定义 JSON 打包
func (mc *MessageContent) MarshalJSON() ([]byte, error) {
	if mc.IsArray {
		return json.Marshal(mc.ContentList)
	}
	return json.Marshal(mc.RawString)
}

// String 返回字符串表示
func (mc *MessageContent) String() string {
	return mc.RawString
}

// Message 消息结构
type Message struct {
	Role    string         `json:"role"`
	Content MessageContent `json:"content"`
}

// Metadata 元数据
type Metadata struct {
	UserID string `json:"user_id,omitempty"`
}

// MessagesResponse Anthropic Messages API 响应
type MessagesResponse struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Role         string         `json:"role"`
	Content      []ContentBlock `json:"content"`
	Model        string         `json:"model"`
	StopReason   string         `json:"stop_reason"`
	StopSequence string         `json:"stop_sequence,omitempty"`
	Usage        UsageInfo      `json:"usage"`
}

// UsageInfo 使用信息
type UsageInfo struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// StreamEvent 流式事件
type StreamEvent struct {
	Type         string            `json:"type"`
	Index        int               `json:"index,omitempty"`
	Delta        *Delta            `json:"delta,omitempty"`
	Message      *MessagesResponse `json:"message,omitempty"`
	Usage        *UsageInfo        `json:"usage,omitempty"`
	ContentBlock *ContentBlock     `json:"content_block,omitempty"` // 用于 content_block_start/stop
}

// Delta 增量内容
type Delta struct {
	Type        string `json:"type"`
	Text        string `json:"text,omitempty"`
	StopReason  string `json:"stop_reason,omitempty"`
	PartialJSON string `json:"partial_json,omitempty"` // 用于工具调用的部分 JSON
}

// Messages 处理 Messages API 请求
func (h *AnthropicHandler) Messages(c *gin.Context) {
	var req MessagesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.sendError(c, http.StatusBadRequest, "invalid_request_error",
			fmt.Sprintf("请求参数错误：%v", err))
		return
	}

	// 检查预填充（已弃用）
	if h.hasPrefill(&req) {
		h.sendError(c, http.StatusBadRequest, "invalid_request_error",
			"预填充(prefill)在 Claude Opus 4.6 和 Claude Sonnet 4.5 上已被弃用，请使用结构化输出或系统提示指令")
		return
	}

	// 如果是流式请求，使用流式处理
	if req.Stream {
		h.handleStreamMessages(c, &req)
		return
	}

	// 转换为 OpenAI 格式并调用代理服务
	openaiReq := h.convertToOpenAIFormat(&req)
	resp, err := h.proxy.ChatCompletions(c.Request.Context(), openaiReq)
	if err != nil {
		h.sendError(c, http.StatusInternalServerError, "api_error",
			fmt.Sprintf("代理错误：%v", err))
		return
	}

	// 转换为 Anthropic 格式
	anthropicResp := h.convertToAnthropicFormat(resp, &req)
	c.JSON(http.StatusOK, anthropicResp)
}

// hasPrefill 检查请求是否包含预填充
func (h *AnthropicHandler) hasPrefill(req *MessagesRequest) bool {
	// 预填充通常表现为最后一条消息是 assistant 消息
	if len(req.Messages) > 0 {
		lastMessage := req.Messages[len(req.Messages)-1]
		if lastMessage.Role == "assistant" {
			// 检查是否是部分内容（预填充的特征）
			content := lastMessage.Content.String()
			if strings.Contains(content, "(") || strings.Contains(content, "The answer is") {
				return true
			}
		}
	}
	return false
}

// handleStreamMessages 处理流式 Messages API 请求
func (h *AnthropicHandler) handleStreamMessages(c *gin.Context, req *MessagesRequest) {
	// 设置 SSE 响应头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")
	c.Header("X-Accel-Buffering", "no")
	c.Header("Access-Control-Allow-Origin", "*")

	ctx := c.Request.Context()

	// 转换为 OpenAI 格式
	openaiReq := h.convertToOpenAIFormat(req)

	// 生成消息 ID
	messageID := fmt.Sprintf("msg_%d", time.Now().UnixNano())

	// 跟踪 token 使用
	var inputTokens, outputTokens int

	// 跟踪是否已经发送了某个类型的内容块开始事件
	sentTextStart := false
	sentToolStart := make(map[int]bool) // tool_index -> started
	toolIndex := 0                      // 工具调用索引

	// 启动心跳包 goroutine，防止连接超时
	heartbeatTicker := time.NewTicker(15 * time.Second)
	defer heartbeatTicker.Stop()

	done := make(chan struct{})
	defer close(done)

	// 心跳包发送协程
	go func() {
		for {
			select {
			case <-done:
				return
			case <-heartbeatTicker.C:
				// 发送 SSE 注释行作为心跳（不会触发客户端事件）
				c.Writer.WriteString(": heartbeat\n\n")
				c.Writer.Flush()
			}
		}
	}()

	// 调用流式接口
	err := h.proxy.StreamChatCompletions(ctx, openaiReq, func(chunk *proxy.ChatCompletionResponse) error {
		if len(chunk.Choices) == 0 {
			return nil
		}

		delta := chunk.Choices[0].Delta

		// 更新 token 统计
		if chunk.Usage.PromptTokens > 0 {
			inputTokens = chunk.Usage.PromptTokens
		}
		if chunk.Usage.CompletionTokens > 0 {
			outputTokens = chunk.Usage.CompletionTokens
		}

		// 1. 处理文本内容（如果有）
		if delta.Content != "" {
			// 如果是第一次有文本内容，先发送 message_start 和 content_block_start
			if !sentTextStart {
				// 发送消息开始事件（简化格式）
				messageStartJSON := fmt.Sprintf(`{"type":"message_start","message":{"id":"%s","type":"message","role":"assistant","content":[{"type":"text","text":""}],"model":"%s","stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":0,"output_tokens":0}}}`, messageID, req.Model)
				c.Writer.WriteString(fmt.Sprintf("event: message_start\ndata:%s\n\n", messageStartJSON))
				c.Writer.Flush()

				// 发送内容块开始事件（简化格式）
				c.Writer.WriteString("event: content_block_start\ndata:{\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n")
				c.Writer.Flush()

				sentTextStart = true
			}

			// 发送文本增量
			jsonData := fmt.Sprintf(`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":%q}}`, delta.Content)
			_, writeErr := c.Writer.WriteString(fmt.Sprintf("event: content_block_delta\ndata:%s\n\n", jsonData))
			if writeErr != nil {
				return writeErr
			}
			c.Writer.Flush()
		}

		// 2. 处理工具调用
		if len(delta.ToolCalls) > 0 {
			for _, toolCall := range delta.ToolCalls {
				// 检查这个工具调用是否已经开始
				if !sentToolStart[toolIndex] {
					// 如果是第一个内容块，且之前没有发送过开始事件，需要发送 message_start
					if !sentTextStart && len(sentToolStart) == 0 {
						messageStartJSON := fmt.Sprintf(`{"type":"message_start","message":{"id":"%s","type":"message","role":"assistant","content":[{"type":"text","text":""}],"model":"%s","stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":0,"output_tokens":0}}}`, messageID, req.Model)
						c.Writer.WriteString(fmt.Sprintf("event: message_start\ndata:%s\n\n", messageStartJSON))
						c.Writer.Flush()
					}

					// 发送工具调用内容块开始事件
					toolStartData := fmt.Sprintf(`{"type":"content_block_start","index":%d,"content_block":{"type":"tool_use","id":"%s","name":"%s","input":{}}}`, toolIndex, toolCall.ID, toolCall.Function.Name)
					c.Writer.WriteString(fmt.Sprintf("event: content_block_start\ndata:%s\n\n", toolStartData))
					c.Writer.Flush()

					sentToolStart[toolIndex] = true
				}

				// 发送工具调用输入数据（input_json_delta）
				if toolCall.Function.Arguments != nil {
					var argsStr string
					switch args := toolCall.Function.Arguments.(type) {
					case string:
						argsStr = args
					default:
						if argsJSON, err := json.Marshal(args); err == nil {
							argsStr = string(argsJSON)
						}
					}

					if argsStr != "" {
						toolDeltaData := fmt.Sprintf(`{"type":"content_block_delta","index":%d,"delta":{"type":"input_json_delta","partial_json":%q}}`, toolIndex, argsStr)
						c.Writer.WriteString(fmt.Sprintf("event: content_block_delta\ndata:%s\n\n", toolDeltaData))
						c.Writer.Flush()
					}
				}

				toolIndex++
			}
		}

		return nil
	})

	if err != nil {
		// 检查是否是客户端主动断开连接（context canceled）
		if strings.Contains(err.Error(), "context canceled") {
			logger.API().Printf("客户端断开流式连接（正常行为）")
			// 不发送错误响应，直接返回
			return
		}
		logger.API().Printf("流式请求失败：%v", err)
		h.sendStreamError(c, "api_error", fmt.Sprintf("流式请求失败：%v", err))
		return
	}

	// 发送所有内容块的结束事件
	// 如果发送过文本开始，才发送文本结束
	if sentTextStart {
		c.Writer.WriteString("event: content_block_stop\ndata:{\"type\":\"content_block_stop\",\"index\":0}\n\n")
		c.Writer.Flush()
	}

	// 发送所有工具调用的结束事件
	for idx := range sentToolStart {
		toolStopData := fmt.Sprintf(`{"type":"content_block_stop","index":%d}`, idx)
		c.Writer.WriteString(fmt.Sprintf("event: content_block_stop\ndata:%s\n\n", toolStopData))
		c.Writer.Flush()
	}

	// 确定 stop_reason
	stopReason := "end_turn"
	if len(sentToolStart) > 0 {
		stopReason = "tool_use"
	}

	// 发送消息结束事件（包含 usage 统计，简化格式）
	messageDeltaJSON := fmt.Sprintf(`{"type":"message_delta","delta":{"stop_reason":"%s","stop_sequence":null},"usage":{"input_tokens":%d,"output_tokens":%d}}`, stopReason, inputTokens, outputTokens)
	c.Writer.WriteString(fmt.Sprintf("event: message_delta\ndata:%s\n\n", messageDeltaJSON))
	c.Writer.Flush()

	// 发送完成事件（简化格式）
	c.Writer.WriteString("event: message_stop\ndata:{\"type\":\"message_stop\"}\n\n")
	c.Writer.Flush()
}

// convertToOpenAIFormat 将 Anthropic 请求转换为 OpenAI 格式
func (h *AnthropicHandler) convertToOpenAIFormat(req *MessagesRequest) *proxy.ChatCompletionRequest {
	messages := make([]proxy.Message, 0, len(req.Messages))

	// 转换消息
	for _, msg := range req.Messages {
		// 检查消息内容是否包含工具调用或图像
		var toolCalls []proxy.ToolCall
		var content string

		if msg.Content.IsArray {
			// 处理内容块数组（可能包含文本、图像、工具调用）
			var textParts []string
			for _, block := range msg.Content.ContentList {
				switch block.Type {
				case "text":
					textParts = append(textParts, block.Text)
				case "tool_use":
					// 转换 Anthropic 工具调用为 OpenAI 格式
					toolCall := proxy.ToolCall{
						ID:   block.ID,
						Type: "function",
						Function: proxy.FunctionCall{
							Name:      block.Name,
							Arguments: "",
						},
					}
					// 尝试将 Input 转换为 JSON 字符串
					if block.Input != nil {
						if inputJSON, err := json.Marshal(block.Input); err == nil {
							toolCall.Function.Arguments = string(inputJSON)
						}
					}
					toolCalls = append(toolCalls, toolCall)
				case "tool_result":
					// 工具调用结果，作为用户消息的内容
					if toolResult, ok := block.Content.(string); ok {
						textParts = append(textParts, fmt.Sprintf("工具 %s 的结果：%s", block.Name, toolResult))
					}
				}
			}
			content = strings.Join(textParts, "\n")
		} else {
			// 简单字符串内容
			content = msg.Content.String()
		}

		messages = append(messages, proxy.Message{
			Role:      msg.Role,
			Content:   content,
			ToolCalls: toolCalls,
		})
	}

	// 转换工具定义
	var tools []proxy.Tool
	if req.Tools != nil {
		for _, tool := range req.Tools {
			tools = append(tools, proxy.Tool{
				Type: "function",
				Function: proxy.FunctionDef{
					Name:        tool.Name,
					Description: tool.Description,
					Parameters:  tool.InputSchema,
				},
			})
		}
	}

	// 转换工具选择
	var toolChoice interface{}
	if req.ToolChoice != nil {
		if req.ToolChoice.Type == "auto" {
			toolChoice = "auto"
		} else if req.ToolChoice.Type == "any" {
			toolChoice = "any"
		} else if req.ToolChoice.Type == "tool" && req.ToolChoice.Name != "" {
			toolChoice = map[string]interface{}{
				"type": "function",
				"function": map[string]string{
					"name": req.ToolChoice.Name,
				},
			}
		}
	}

	// 处理系统提示和 JSON 格式请求
	systemPrompt := req.System.String()
	if req.ResponseFormat != nil && req.ResponseFormat.Type == "json_object" {
		if systemPrompt != "" {
			systemPrompt += "\n请以 JSON 格式回复。"
		} else {
			systemPrompt = "请以 JSON 格式回复。"
		}
	}

	return &proxy.ChatCompletionRequest{
		Model:       req.Model,
		Messages:    messages,
		System:      systemPrompt,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		MaxTokens:   req.MaxTokens,
		Stream:      req.Stream,
		Tools:       tools,
		ToolChoice:  toolChoice,
	}
}

// convertToAnthropicFormat 将 OpenAI 响应转换为 Anthropic 格式
func (h *AnthropicHandler) convertToAnthropicFormat(openaiResp *proxy.ChatCompletionResponse, req *MessagesRequest) *MessagesResponse {
	if len(openaiResp.Choices) == 0 {
		return &MessagesResponse{
			ID:         openaiResp.ID,
			Type:       "message",
			Role:       "assistant",
			Content:    []ContentBlock{{Type: "text", Text: ""}},
			Model:      req.Model,
			StopReason: "end_turn",
			Usage: UsageInfo{
				InputTokens:  openaiResp.Usage.PromptTokens,
				OutputTokens: openaiResp.Usage.CompletionTokens,
			},
		}
	}

	choice := openaiResp.Choices[0]
	content := proxy.GetContentString(choice.Message.Content)
	stopReason := "end_turn"

	// 转换停止原因
	switch choice.FinishReason {
	case "length":
		stopReason = "max_tokens"
	case "tool_calls":
		stopReason = "tool_use"
	case "stop":
		stopReason = "end_turn"
	case "content_filter":
		stopReason = "content_filter"
	}

	// 构建内容块数组 - 始终是数组格式
	var contentBlocks []ContentBlock

	// 1. 添加文本内容（如果有）
	if content != "" {
		contentBlocks = append(contentBlocks, ContentBlock{
			Type: "text",
			Text: content,
		})
	}

	// 2. 添加工具调用内容（如果有）
	if len(choice.Message.ToolCalls) > 0 {
		for _, toolCall := range choice.Message.ToolCalls {
			// 解析参数为对象
			var input interface{}
			switch args := toolCall.Function.Arguments.(type) {
			case string:
				if args != "" {
					if err := json.Unmarshal([]byte(args), &input); err != nil {
						logger.API().Printf("解析工具参数失败：%v", err)
						input = toolCall.Function.Arguments // 保持原始字符串
					}
				} else {
					input = make(map[string]interface{})
				}
			default:
				// 如果已经是对象，直接使用
				input = args
			}

			contentBlocks = append(contentBlocks, ContentBlock{
				Type:  "tool_use",
				ID:    toolCall.ID,
				Name:  toolCall.Function.Name,
				Input: input,
			})
		}
	}

	// 确保至少有一个内容块
	if len(contentBlocks) == 0 {
		contentBlocks = append(contentBlocks, ContentBlock{
			Type: "text",
			Text: "",
		})
	}

	return &MessagesResponse{
		ID:         openaiResp.ID,
		Type:       "message",
		Role:       "assistant",
		Content:    contentBlocks,
		Model:      req.Model,
		StopReason: stopReason,
		Usage: UsageInfo{
			InputTokens:  openaiResp.Usage.PromptTokens,
			OutputTokens: openaiResp.Usage.CompletionTokens,
		},
	}
}

// ListModels 列出可用模型 (与 OpenAI 兼容)
func (h *AnthropicHandler) ListModels(c *gin.Context) {
	models, err := h.storage.ListModels(c.Request.Context(), storage.ModelFilter{
		AvailableOnly: true,
	})
	if err != nil {
		h.sendError(c, http.StatusInternalServerError, "api_error", err.Error())
		return
	}

	// 转换为 Anthropic 格式
	data := make([]gin.H, 0)
	for _, model := range models {
		data = append(data, gin.H{
			"name":         model.Name,
			"display_name": model.Name,
			"description":  fmt.Sprintf("%s model", model.Name),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"data": data,
	})
}


// sendError 发送统一的错误响应
func (h *AnthropicHandler) sendError(c *gin.Context, statusCode int, errorType, message string) {
	c.JSON(statusCode, gin.H{
		"type": "error",
		"error": gin.H{
			"type":    errorType,
			"message": message,
		},
	})
}

// sendStreamError 发送流式错误响应
func (h *AnthropicHandler) sendStreamError(c *gin.Context, errorType, message string) {
	errorEvent := StreamEvent{
		Type: "error",
		Delta: &Delta{
			Type: "error",
			Text: message,
		},
	}
	data, _ := json.Marshal(errorEvent)
	c.Writer.WriteString(fmt.Sprintf("event: error\ndata: %s\n\n", string(data)))
	c.Writer.Flush()
}
