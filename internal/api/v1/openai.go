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

// OpenAIHandler OpenAI 兼容接口处理器
type OpenAIHandler struct {
	storage storage.Storage
	proxy   *proxy.ProxyService
}

// NewOpenAIHandler 创建 OpenAI 处理器
func NewOpenAIHandler(storage storage.Storage, proxy *proxy.ProxyService) *OpenAIHandler {
	return &OpenAIHandler{
		storage: storage,
		proxy:   proxy,
	}
}

// ChatCompletions 处理 Chat Completions 请求
func (h *OpenAIHandler) ChatCompletions(c *gin.Context) {
	var req proxy.ChatCompletionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 如果是流式请求，使用流式处理
	if req.Stream {
		h.handleStreamChatCompletions(c, &req)
		return
	}

	// 调用代理服务
	resp, err := h.proxy.ChatCompletions(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("代理错误：%v", err)})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// handleStreamChatCompletions 处理流式 Chat Completions 请求（优化版）
func (h *OpenAIHandler) handleStreamChatCompletions(c *gin.Context, req *proxy.ChatCompletionRequest) {
	// 设置 SSE 响应头 - 优化配置
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")
	c.Header("X-Accel-Buffering", "no")           // 禁用 Nginx 缓冲
	c.Header("X-Content-Type-Options", "nosniff") // 防止 MIME 类型嗅探

	ctx := c.Request.Context()
	chunkCount := 0

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

	// 直接调用代理的流式方法
	err := h.proxy.StreamChatCompletions(ctx, req, func(chunk *proxy.ChatCompletionResponse) error {
		data, err := json.Marshal(chunk)
		if err != nil {
			return err
		}
		_, err = c.Writer.WriteString(fmt.Sprintf("data: %s\n\n", string(data)))
		if err != nil {
			return err
		}
		c.Writer.Flush() // 强制刷新缓冲区
		chunkCount++
		return nil
	})

	if err != nil {
		log.Printf("[OpenAI] 流式请求失败：%v", err)
		// 如果已经写入响应，不能再返回 JSON 错误
		return
	}

	// 发送结束标记
	c.Writer.WriteString("data: [DONE]\n\n")
	c.Writer.Flush()
	log.Printf("[OpenAI] 流式传输完成，共发送 %d 个 chunk", chunkCount)
}

// ListModels 列出可用模型
func (h *OpenAIHandler) ListModels(c *gin.Context) {
	models, err := h.storage.ListModels(c.Request.Context(), storage.ModelFilter{
		AvailableOnly: true,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 转换为 OpenAI 格式
	data := make([]gin.H, 0)
	for _, model := range models {
		data = append(data, gin.H{
			"id":       model.Name,
			"object":   "model",
			"created":  model.LastTested.Unix(),
			"owned_by": "ollama",
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   data,
	})
}

// Chat 处理简单对话请求
func (h *OpenAIHandler) Chat(c *gin.Context) {
	var req struct {
		Model   string `json:"model"`
		Message string `json:"message"`
		Stream  bool   `json:"stream"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	chatReq := &proxy.ChatCompletionRequest{
		Model: req.Model,
		Messages: []proxy.Message{
			{Role: "user", Content: req.Message},
		},
		Stream: req.Stream,
	}

	// 调用代理服务
	resp, err := h.proxy.ChatCompletions(c.Request.Context(), chatReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("对话失败：%v", err)})
		return
	}

	c.JSON(http.StatusOK, resp)
}
