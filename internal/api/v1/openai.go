package v1

import (
	"encoding/base64"
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
		SendInvalidRequestError(c, fmt.Sprintf("请求参数无效: %v", err))
		return
	}

	// 验证模型
	if req.Model == "" {
		SendInvalidRequestError(c, "必须指定模型")
		return
	}

	// 验证消息
	if len(req.Messages) == 0 {
		SendInvalidRequestError(c, "消息列表不能为空")
		return
	}

	// 验证多模态内容（如果有图像）
	for _, msg := range req.Messages {
		if msg.Content != nil {
			switch content := msg.Content.(type) {
			case []interface{}:
				// 验证多模态内容格式
				for i, part := range content {
					partMap, ok := part.(map[string]interface{})
					if !ok {
						SendInvalidRequestError(c, fmt.Sprintf("消息 %d 的内容部分 %d 格式无效", msg.Role, i))
						return
					}

					partType, ok := partMap["type"].(string)
					if !ok {
						SendInvalidRequestError(c, fmt.Sprintf("消息 %d 的内容部分 %d 缺少 type 字段", msg.Role, i))
						return
					}

					switch partType {
					case "text":
						if _, ok := partMap["text"].(string); !ok {
							SendInvalidRequestError(c, fmt.Sprintf("消息 %d 的文本内容部分 %d 缺少 text 字段", msg.Role, i))
							return
						}
					case "image_url":
						imageURL, ok := partMap["image_url"].(map[string]interface{})
						if !ok {
							SendInvalidRequestError(c, fmt.Sprintf("消息 %d 的图像内容部分 %d 格式无效", msg.Role, i))
							return
						}
						if url, ok := imageURL["url"].(string); !ok || url == "" {
							SendInvalidRequestError(c, fmt.Sprintf("消息 %d 的图像内容部分 %d 缺少 url", msg.Role, i))
								return
						}
					default:
						SendInvalidRequestError(c, fmt.Sprintf("消息 %d 的内容部分 %d 包含不支持的类型: %s", msg.Role, i, partType))
						return
					}
				}
			}
		}
	}

	// 验证参数范围
	if req.Temperature < 0 || req.Temperature > 2 {
		SendInvalidRequestError(c, "temperature 必须在 0 到 2 之间")
		return
	}

	if req.TopP < 0 || req.TopP > 1 {
		SendInvalidRequestError(c, "top_p 必须在 0 到 1 之间")
		return
	}

	if req.PresencePenalty < -2 || req.PresencePenalty > 2 {
		SendInvalidRequestError(c, "presence_penalty 必须在 -2 到 2 之间")
		return
	}

	if req.FrequencyPenalty < -2 || req.FrequencyPenalty > 2 {
		SendInvalidRequestError(c, "frequency_penalty 必须在 -2 到 2 之间")
		return
	}

	if req.MaxTokens < 0 {
		SendInvalidRequestError(c, "max_tokens 不能为负数")
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
		// 检查是否是模型不可用错误
		if strings.Contains(err.Error(), "no available service") || strings.Contains(err.Error(), "model not found") {
			SendInvalidModelError(c, req.Model)
			return
		}
		SendInternalError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// handleStreamChatCompletions 处理流式 Chat Completions 请求（优化版）
func (h *OpenAIHandler) handleStreamChatCompletions(c *gin.Context, req *proxy.ChatCompletionRequest) {
	// 验证模型
	if req.Model == "" {
		SendInvalidRequestError(c, "必须指定模型")
		return
	}

	// 验证消息
	if len(req.Messages) == 0 {
		SendInvalidRequestError(c, "消息列表不能为空")
		return
	}

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
		logger.API().Error("流式请求失败：%v", err)
		// 如果已经写入响应，不能再返回 JSON 错误
		return
	}

	// 发送结束标记
	c.Writer.WriteString("data: [DONE]\n\n")
	c.Writer.Flush()
	logger.API().Debug("流式传输完成，共发送 %d 个 chunk", chunkCount)
}

// ListModels 列出可用模型
func (h *OpenAIHandler) ListModels(c *gin.Context) {
	models, err := h.storage.ListModels(c.Request.Context(), storage.ModelFilter{
		AvailableOnly: true,
	})
	if err != nil {
		SendInternalError(c, err)
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
		SendInvalidRequestError(c, fmt.Sprintf("请求参数无效: %v", err))
		return
	}

	// 验证模型
	if req.Model == "" {
		SendInvalidRequestError(c, "必须指定模型")
		return
	}

	// 验证消息
	if req.Message == "" {
		SendInvalidRequestError(c, "消息不能为空")
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
		SendInternalError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// Embeddings 处理 Embeddings 请求
func (h *OpenAIHandler) Embeddings(c *gin.Context) {
	var req proxy.EmbeddingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		SendInvalidRequestError(c, fmt.Sprintf("请求参数无效: %v", err))
		return
	}

	// 验证模型
	if req.Model == "" {
		SendInvalidRequestError(c, "必须指定模型")
		return
	}

	// 验证输入
	if req.Input == nil {
		SendInvalidRequestError(c, "input 不能为空")
		return
	}

	// 调用代理服务
	resp, err := h.proxy.Embeddings(c.Request.Context(), &req)
	if err != nil {
		// 检查是否是模型不可用错误
		if strings.Contains(err.Error(), "no service supports model") {
			SendInvalidModelError(c, req.Model)
			return
		}
		SendInternalError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// CreateTranscription 处理音频转录请求
func (h *OpenAIHandler) CreateTranscription(c *gin.Context) {
	// 解析表单数据
	model := c.PostForm("model")
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		SendInvalidRequestError(c, "必须提供音频文件")
		return
	}
	defer file.Close()

	if model == "" {
		SendInvalidRequestError(c, "必须指定模型")
		return
	}

	// 读取文件内容
	fileData := make([]byte, header.Size)
	if _, err := file.Read(fileData); err != nil {
		SendInternalError(c, fmt.Errorf("读取文件失败: %w", err))
		return
	}

	// 将文件数据编码为 base64
	encodedFile := base64.StdEncoding.EncodeToString(fileData)

	// 构建请求
	req := &proxy.AudioTranscriptionRequest{
		Model:          model,
		File:           encodedFile,
		Language:       c.PostForm("language"),
		Prompt:         c.PostForm("prompt"),
		ResponseFormat: c.PostForm("response_format"),
	}

	// 调用代理服务
	resp, err := h.proxy.CreateTranscription(c.Request.Context(), req)
	if err != nil {
		SendInternalError(c, err)
		return
	}

	// 根据响应格式返回
	switch req.ResponseFormat {
	case "text", "":
		c.String(http.StatusOK, resp.Text)
	default:
		c.JSON(http.StatusOK, resp)
	}
}

// CreateTranslation 处理音频翻译请求
func (h *OpenAIHandler) CreateTranslation(c *gin.Context) {
	model := c.PostForm("model")
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		SendInvalidRequestError(c, "必须提供音频文件")
		return
	}
	defer file.Close()

	if model == "" {
		SendInvalidRequestError(c, "必须指定模型")
		return
	}

	fileData := make([]byte, header.Size)
	if _, err := file.Read(fileData); err != nil {
		SendInternalError(c, fmt.Errorf("读取文件失败: %w", err))
		return
	}

	// 将文件数据编码为 base64
	encodedFile := base64.StdEncoding.EncodeToString(fileData)

	req := &proxy.AudioTranslationRequest{
		Model:          model,
		File:           encodedFile,
		ResponseFormat: c.PostForm("response_format"),
		Prompt:         c.PostForm("prompt"),
	}

	resp, err := h.proxy.CreateTranslation(c.Request.Context(), req)
	if err != nil {
		SendInternalError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// CreateSpeech 处理语音合成请求
func (h *OpenAIHandler) CreateSpeech(c *gin.Context) {
	var req proxy.SpeechRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		SendInvalidRequestError(c, fmt.Sprintf("请求参数无效: %v", err))
		return
	}

	// 验证参数
	if req.Model == "" {
		SendInvalidRequestError(c, "必须指定模型")
		return
	}

	if req.Input == "" {
		SendInvalidRequestError(c, "input 不能为空")
		return
	}

	if req.Voice == "" {
		req.Voice = "alloy" // 默认语音
	}

	// 验证速度
	if req.Speed == 0 {
		req.Speed = 1.0
	} else if req.Speed < 0.25 || req.Speed > 4.0 {
		SendInvalidRequestError(c, "speed 必须在 0.25 到 4.0 之间")
		return
	}

	// 调用代理服务
	resp, err := h.proxy.CreateSpeech(c.Request.Context(), &req)
	if err != nil {
		SendInternalError(c, err)
		return
	}

	// 设置响应头并返回音频数据
	c.Data(http.StatusOK, resp.ContentType, resp.AudioData)
}

// CreateModeration 处理内容审核请求
func (h *OpenAIHandler) CreateModeration(c *gin.Context) {
	var req proxy.ModerationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		SendInvalidRequestError(c, fmt.Sprintf("请求参数无效: %v", err))
		return
	}

	// 验证输入
	if req.Input == nil {
		SendInvalidRequestError(c, "input 不能为空")
		return
	}

	// 设置默认模型
	if req.Model == "" {
		req.Model = "llama3.2"
	}

	// 调用代理服务
	resp, err := h.proxy.CreateModeration(c.Request.Context(), &req)
	if err != nil {
		SendInternalError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// CreateBatch 处理批量任务请求
func (h *OpenAIHandler) CreateBatch(c *gin.Context) {
	var req proxy.BatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		SendInvalidRequestError(c, fmt.Sprintf("请求参数无效: %v", err))
		return
	}

	if len(req.Requests) == 0 {
		SendInvalidRequestError(c, "请求列表不能为空")
		return
	}

	// 调用代理服务
	resp, err := h.proxy.CreateBatch(c.Request.Context(), &req)
	if err != nil {
		SendInternalError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// ListFiles 列出文件
func (h *OpenAIHandler) ListFiles(c *gin.Context) {
	// 简化实现：返回空列表
	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   []string{},
	})
}

// UploadFile 上传文件
func (h *OpenAIHandler) UploadFile(c *gin.Context) {
	purpose := c.PostForm("purpose")
	if purpose == "" {
		purpose = "assistants"
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		SendInvalidRequestError(c, "必须提供文件")
		return
	}
	defer file.Close()

	// 简化实现：返回虚拟文件信息
	fileID := fmt.Sprintf("file-%d", time.Now().Unix())

	c.JSON(http.StatusOK, gin.H{
		"id":        fileID,
		"object":    "file",
		"bytes":     header.Size,
		"created_at": time.Now().Unix(),
		"filename":  header.Filename,
		"purpose":   purpose,
	})
}

// DeleteFile 删除文件
func (h *OpenAIHandler) DeleteFile(c *gin.Context) {
	fileID := c.Param("file_id")

	// 简化实现：返回删除成功
	c.JSON(http.StatusOK, gin.H{
		"id":      fileID,
		"object":  "file",
		"deleted": true,
	})
}

// CreateFineTuningJob 创建微调任务
func (h *OpenAIHandler) CreateFineTuningJob(c *gin.Context) {
	var req proxy.CreateFineTuningJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		SendInvalidRequestError(c, fmt.Sprintf("请求参数无效: %v", err))
		return
	}

	// 简化实现：返回虚拟任务信息
	jobID := fmt.Sprintf("ftjob-%d", time.Now().Unix())

	c.JSON(http.StatusOK, gin.H{
		"id":             jobID,
		"object":         "fine_tuning.job",
		"status":         "pending",
		"model":          req.Model,
		"training_file":  req.TrainingFile,
		"created_at":     time.Now().Unix(),
	})
}

// ListFineTuningJobs 列出微调任务
func (h *OpenAIHandler) ListFineTuningJobs(c *gin.Context) {
	// 简化实现：返回空列表
	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   []string{},
	})
}

// CreateAssistant 创建助手
func (h *OpenAIHandler) CreateAssistant(c *gin.Context) {
	var req Assistant
	if err := c.ShouldBindJSON(&req); err != nil {
		SendInvalidRequestError(c, fmt.Sprintf("请求参数无效: %v", err))
		return
	}

	// 设置默认值
	if req.Model == "" {
		req.Model = "llama3.2"
	}

	// 生成 ID
	req.ID = fmt.Sprintf("asst_%d", time.Now().Unix())
	req.Object = "assistant"
	req.CreatedAt = time.Now()

	c.JSON(http.StatusOK, req)
}

// ListAssistants 列出助手
func (h *OpenAIHandler) ListAssistants(c *gin.Context) {
	// 简化实现：返回空列表
	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   []Assistant{},
	})
}

// CreateThread 创建线程
func (h *OpenAIHandler) CreateThread(c *gin.Context) {
	thread := Thread{
		ID:        fmt.Sprintf("thread_%d", time.Now().Unix()),
		Object:    "thread",
		CreatedAt: time.Now(),
	}

	c.JSON(http.StatusOK, thread)
}

// CreateMessage 创建消息
func (h *OpenAIHandler) CreateMessage(c *gin.Context) {
	threadID := c.Param("thread_id")

	var req struct {
		Role    string      `json:"role"`
		Content interface{} `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		SendInvalidRequestError(c, fmt.Sprintf("请求参数无效: %v", err))
		return
	}

	message := ThreadMessage{
		ID:        fmt.Sprintf("msg_%d", time.Now().Unix()),
		Object:    "thread.message",
		CreatedAt: time.Now(),
		ThreadID:  threadID,
		Role:      req.Role,
		Content:   req.Content,
	}

	c.JSON(http.StatusOK, message)
}

// CreateRun 创建运行
func (h *OpenAIHandler) CreateRun(c *gin.Context) {
	threadID := c.Param("thread_id")

	var req struct {
		AssistantID string `json:"assistant_id"`
		Model       string `json:"model,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		SendInvalidRequestError(c, fmt.Sprintf("请求参数无效: %v", err))
		return
	}

	run := Run{
		ID:          fmt.Sprintf("run_%d", time.Now().Unix()),
		Object:      "thread.run",
		CreatedAt:   time.Now(),
		ThreadID:    threadID,
		AssistantID: req.AssistantID,
		Status:      "queued",
		Model:       req.Model,
	}

	c.JSON(http.StatusOK, run)
}

// CreateImages 生成图像
func (h *OpenAIHandler) CreateImages(c *gin.Context) {
	var req ImageGenerationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		SendInvalidRequestError(c, fmt.Sprintf("请求参数无效: %v", err))
		return
	}

	// 验证参数
	if req.Prompt == "" {
		SendInvalidRequestError(c, "prompt 不能为空")
		return
	}

	if req.N == 0 {
		req.N = 1
	}

	if req.Size == "" {
		req.Size = "1024x1024"
	}

	// 简化实现：返回虚拟图像响应
	c.JSON(http.StatusOK, ImageResponse{
		Created: int(time.Now().Unix()),
		Data:    make([]ImageData, req.N),
	})
}

