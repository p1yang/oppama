package proxy

import (
	"fmt"
	"net/http"
)

// ErrorCode 错误码类型
type ErrorCode string

const (
	// 通用错误
	ErrCodeInternal     ErrorCode = "INTERNAL_ERROR"
	ErrCodeInvalidReq   ErrorCode = "INVALID_REQUEST"
	ErrCodeTimeout      ErrorCode = "TIMEOUT"
	ErrCodeRateLimit    ErrorCode = "RATE_LIMIT_EXCEEDED"
	ErrCodeUnauthorized ErrorCode = "UNAUTHORIZED"

	// 服务相关错误
	ErrCodeServiceNotFound   ErrorCode = "SERVICE_NOT_FOUND"
	ErrCodeServiceUnavailable ErrorCode = "SERVICE_UNAVAILABLE"
	ErrCodeAllServicesFailed ErrorCode = "ALL_SERVICES_FAILED"
	ErrCodeModelNotSupported ErrorCode = "MODEL_NOT_SUPPORTED"

	// 请求相关错误
	ErrCodeInvalidModel  ErrorCode = "INVALID_MODEL"
	ErrCodeInvalidParams ErrorCode = "INVALID_PARAMS"
	ErrCodeContextLength ErrorCode = "CONTEXT_LENGTH_EXCEEDED"

	// 流式相关错误
	ErrCodeStreamInterrupted ErrorCode = "STREAM_INTERRUPTED"
	ErrCodeStreamDecode      ErrorCode = "STREAM_DECODE_ERROR"

	// 工具调用相关错误
	ErrCodeToolExecutionFailed ErrorCode = "TOOL_EXECUTION_FAILED"
	ErrCodeToolParseError      ErrorCode = "TOOL_PARSE_ERROR"
	ErrCodeToolTimeout         ErrorCode = "TOOL_TIMEOUT"
)

// ProxyError 代理服务错误
type ProxyError struct {
	Code       ErrorCode `json:"code"`
	Message    string    `json:"message"`
	StatusCode int       `json:"status_code"`
	Err        error     `json:"-"` // 原始错误
}

// Error 实现 error 接口
func (e *ProxyError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap 返回原始错误
func (e *ProxyError) Unwrap() error {
	return e.Err
}

// NewProxyError 创建代理错误
func NewProxyError(code ErrorCode, message string, statusCode int) *ProxyError {
	return &ProxyError{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
	}
}

// WrapError 包装错误
func WrapError(code ErrorCode, message string, err error) *ProxyError {
	return &ProxyError{
		Code:       code,
		Message:    message,
		StatusCode: http.StatusInternalServerError,
		Err:        err,
	}
}

// 预定义错误
var (
	ErrServiceNotFound = &ProxyError{
		Code:       ErrCodeServiceNotFound,
		Message:    "未找到可用的服务",
		StatusCode: http.StatusServiceUnavailable,
	}

	ErrModelNotSupported = &ProxyError{
		Code:       ErrCodeModelNotSupported,
		Message:    "模型不支持",
		StatusCode: http.StatusBadRequest,
	}

	ErrInvalidRequest = &ProxyError{
		Code:       ErrCodeInvalidReq,
		Message:    "请求参数无效",
		StatusCode: http.StatusBadRequest,
	}

	ErrUnauthorized = &ProxyError{
		Code:       ErrCodeUnauthorized,
		Message:    "未授权访问",
		StatusCode: http.StatusUnauthorized,
	}
)

// HTTPStatusFromError 从错误获取 HTTP 状态码
func HTTPStatusFromError(err error) int {
	if proxyErr, ok := err.(*ProxyError); ok {
		return proxyErr.StatusCode
	}

	// 默认返回 500
	return http.StatusInternalServerError
}

// IsRetryable 判断错误是否可重试
func IsRetryable(err error) bool {
	if proxyErr, ok := err.(*ProxyError); ok {
		switch proxyErr.Code {
		case ErrCodeTimeout,
			ErrCodeServiceUnavailable,
			ErrCodeInternal,
			ErrCodeAllServicesFailed:
			return true
		}
	}
	return false
}

// IsClientError 判断是否是客户端错误
func IsClientError(err error) bool {
	if proxyErr, ok := err.(*ProxyError); ok {
		status := proxyErr.StatusCode
		return status >= 400 && status < 500
	}
	return false
}
