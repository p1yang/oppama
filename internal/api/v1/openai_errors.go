package v1

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// OpenAIError OpenAI 标准错误格式
type OpenAIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Param   string `json:"param,omitempty"`
	Code    string `json:"code,omitempty"`
}

// OpenAIErrorResponse OpenAI 错误响应结构
type OpenAIErrorResponse struct {
	Error OpenAIError `json:"error"`
}

// 错误类型常量
const (
	ErrorTypeInvalidRequest    = "invalid_request_error"
	ErrorTypeInvalidAPIKey     = "invalid_api_key"
	ErrorTypeRateLimit         = "rate_limit_error"
	ErrorTypeServerError       = "server_error"
	ErrorTypeServiceUnavailable = "service_unavailable"
)

// 错误代码常量
const (
	ErrorCodeInvalidModel     = "invalid_model"
	ErrorCodeInvalidMessages  = "invalid_messages"
	ErrorCodeContextLength    = "context_length_exceeded"
	ErrorCodeServiceUnavail   = "service_unavailable"
	ErrorCodeRateLimited      = "rate_limited"
	ErrorCodeInternalError    = "internal_error"
)

// SendError 发送 OpenAI 标准错误响应
func SendError(c *gin.Context, statusCode int, errType, message, code string) {
	c.JSON(statusCode, OpenAIErrorResponse{
		Error: OpenAIError{
			Message: message,
			Type:    errType,
			Code:    code,
		},
	})
}

// SendInvalidRequestError 发送无效请求错误
func SendInvalidRequestError(c *gin.Context, message string) {
	SendError(c, http.StatusBadRequest, ErrorTypeInvalidRequest, message, ErrorCodeInvalidMessages)
}

// SendInvalidModelError 发送无效模型错误
func SendInvalidModelError(c *gin.Context, model string) {
	SendError(c, http.StatusBadRequest, ErrorTypeInvalidRequest,
		fmt.Sprintf("模型 '%s' 不存在或不可用", model), ErrorCodeInvalidModel)
}

// SendAPIKeyError 发送 API Key 错误
func SendAPIKeyError(c *gin.Context) {
	SendError(c, http.StatusUnauthorized, ErrorTypeInvalidAPIKey,
		"不正确的 API 密钥", "")
}

// SendRateLimitError 发送速率限制错误
func SendRateLimitError(c *gin.Context) {
	SendError(c, http.StatusTooManyRequests, ErrorTypeRateLimit,
		"请求过于频繁，请稍后再试", ErrorCodeRateLimited)
}

// SendServiceUnavailable 发送服务不可用错误
func SendServiceUnavailable(c *gin.Context, message string) {
	SendError(c, http.StatusServiceUnavailable, ErrorTypeServiceUnavailable,
		message, ErrorCodeServiceUnavail)
}

// SendInternalError 发送内部错误
func SendInternalError(c *gin.Context, err error) {
	SendError(c, http.StatusInternalServerError, ErrorTypeServerError,
		fmt.Sprintf("服务器内部错误: %v", err), ErrorCodeInternalError)
}
