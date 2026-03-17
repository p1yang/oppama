package v1

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"oppama/internal/proxy"
	"oppama/internal/storage"

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

// MessagesRequest Anthropic Messages API 请求
type MessagesRequest struct {
	Model         string        `json:"model" binding:"required"`
	Messages      []Message     `json:"messages" binding:"required,min=1"`
	System        SystemContent `json:"system,omitempty"`
	MaxTokens     int           `json:"max_tokens" binding:"required"`
	Temperature   float64       `json:"temperature,omitempty"`
	TopP          float64       `json:"top_p,omitempty"`
	TopK          int           `json:"top_k,omitempty"`
	StopSequences []string      `json:"stop_sequences,omitempty"`
	Stream        bool          `json:"stream,omitempty"`
	Metadata      *Metadata     `json:"metadata,omitempty"`
}

// ContentBlock 内容块，支持文本和多模态
type ContentBlock struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	ImageURL *struct {
		URL string `json:"url"`
	} `json:"image_url,omitempty"`
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
			if block.Type == "text" {
				mc.RawString += block.Text
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
	Type    string            `json:"type"`
	Index   int               `json:"index,omitempty"`
	Delta   *Delta            `json:"delta,omitempty"`
	Message *MessagesResponse `json:"message,omitempty"`
	Usage   *UsageInfo        `json:"usage,omitempty"`
}

// Delta 增量内容
type Delta struct {
	Type         string `json:"type"`
	Text         string `json:"text,omitempty"`
	StopReason   string `json:"stop_reason,omitempty"`
	StopSequence string `json:"stop_sequence,omitempty"`
}

// Messages 处理 Messages API 请求
func (h *AnthropicHandler) Messages(c *gin.Context) {
	var req MessagesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"type": "error",
			"error": gin.H{
				"type":    "invalid_request_error",
				"message": fmt.Sprintf("请求参数错误：%v", err),
			},
		})
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
		c.JSON(http.StatusInternalServerError, gin.H{
			"type": "error",
			"error": gin.H{
				"type":    "api_error",
				"message": fmt.Sprintf("代理错误：%v", err),
			},
		})
		return
	}

	// 转换为 Anthropic 格式
	anthropicResp := h.convertToAnthropicFormat(resp, &req)
	c.JSON(http.StatusOK, anthropicResp)
}

// handleStreamMessages 处理流式 Messages API 请求
func (h *AnthropicHandler) handleStreamMessages(c *gin.Context, req *MessagesRequest) {
	// 设置 SSE 响应头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")
	c.Header("X-Accel-Buffering", "no")

	ctx := c.Request.Context()

	// 转换为 OpenAI 格式
	openaiReq := h.convertToOpenAIFormat(req)

	// 发送消息开始事件
	messageStart := StreamEvent{
		Type: "message_start",
		Message: &MessagesResponse{
			ID:      fmt.Sprintf("msg_%d", time.Now().UnixNano()),
			Type:    "message",
			Role:    "assistant",
			Content: []ContentBlock{{Type: "text", Text: ""}},
			Model:   req.Model,
			Usage:   UsageInfo{InputTokens: 0, OutputTokens: 0},
		},
	}
	data, _ := json.Marshal(messageStart)
	c.Writer.WriteString(fmt.Sprintf("event: message_start\ndata: %s\n\n", string(data)))
	c.Writer.Flush()

	contentBlockStart := StreamEvent{
		Type:  "content_block_start",
		Index: 0,
		Delta: &Delta{Type: "text_delta", Text: ""},
	}
	data, _ = json.Marshal(contentBlockStart)
	c.Writer.WriteString(fmt.Sprintf("event: content_block_start\ndata: %s\n\n", string(data)))
	c.Writer.Flush()

	// 调用流式接口
	err := h.proxy.StreamChatCompletions(ctx, openaiReq, func(chunk *proxy.ChatCompletionResponse) error {
		if len(chunk.Choices) == 0 {
			return nil
		}

		delta := chunk.Choices[0].Delta
		if delta.Content != "" {
			contentBlockDelta := StreamEvent{
				Type:  "content_block_delta",
				Index: 0,
				Delta: &Delta{
					Type: "text_delta",
					Text: delta.Content,
				},
			}
			data, err := json.Marshal(contentBlockDelta)
			if err != nil {
				return err
			}
			_, err = c.Writer.WriteString(fmt.Sprintf("event: content_block_delta\ndata: %s\n\n", string(data)))
			if err != nil {
				return err
			}
			c.Writer.Flush()
		}
		return nil
	})

	if err != nil {
		log.Printf("[Anthropic] 流式请求失败：%v", err)
		return
	}

	// 发送内容块结束事件
	contentBlockStop := StreamEvent{
		Type:  "content_block_stop",
		Index: 0,
	}
	data, _ = json.Marshal(contentBlockStop)
	c.Writer.WriteString(fmt.Sprintf("event: content_block_stop\ndata: %s\n\n", string(data)))
	c.Writer.Flush()

	// 发送消息结束事件
	messageDelta := StreamEvent{
		Type: "message_delta",
		Delta: &Delta{
			StopReason: "end_turn",
		},
		Usage: &UsageInfo{
			OutputTokens: 0,
		},
	}
	data, _ = json.Marshal(messageDelta)
	c.Writer.WriteString(fmt.Sprintf("event: message_delta\ndata: %s\n\n", string(data)))
	c.Writer.Flush()

	// 发送完成事件
	messageStop := StreamEvent{
		Type: "message_stop",
	}
	data, _ = json.Marshal(messageStop)
	c.Writer.WriteString(fmt.Sprintf("event: message_stop\ndata: %s\n\n", string(data)))
	c.Writer.Flush()

	// 发送 [DONE] 标记
	c.Writer.WriteString("data: [DONE]\n\n")
	c.Writer.Flush()
}

// convertToOpenAIFormat 将 Anthropic 请求转换为 OpenAI 格式
func (h *AnthropicHandler) convertToOpenAIFormat(req *MessagesRequest) *proxy.ChatCompletionRequest {
	messages := make([]proxy.Message, 0, len(req.Messages))

	// 如果有 system 提示，添加到消息中
	if req.System.String() != "" {
		messages = append(messages, proxy.Message{
			Role:    "system",
			Content: req.System.String(),
		})
	}

	// 转换消息
	for _, msg := range req.Messages {
		messages = append(messages, proxy.Message{
			Role:    msg.Role,
			Content: msg.Content.String(), // 转换为字符串
		})
	}

	return &proxy.ChatCompletionRequest{
		Model:       req.Model,
		Messages:    messages,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		MaxTokens:   req.MaxTokens,
		Stream:      req.Stream,
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

	content := openaiResp.Choices[0].Message.Content
	stopReason := "end_turn"
	if openaiResp.Choices[0].FinishReason == "length" {
		stopReason = "max_tokens"
	}

	return &MessagesResponse{
		ID:         openaiResp.ID,
		Type:       "message",
		Role:       "assistant",
		Content:    []ContentBlock{{Type: "text", Text: content}},
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
		c.JSON(http.StatusInternalServerError, gin.H{
			"type": "error",
			"error": gin.H{
				"type":    "api_error",
				"message": err.Error(),
			},
		})
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
