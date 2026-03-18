package proxy

import (
	"encoding/json"
	"fmt"
	"sync"
)

// ToolExecutionResult 工具执行结果
type ToolExecutionResult struct {
	ToolCallID string                 `json:"tool_call_id"`
	Role       string                 `json:"role"`
	Name       string                 `json:"name"`
	Content    string                 `json:"content"`
	Result     map[string]interface{} `json:"result,omitempty"`
	Error      string                 `json:"error,omitempty"`
}

// ToolExecutionOptions 工具执行选项
type ToolExecutionOptions struct {
	MaxIterations int                                                             `json:"max_iterations"` // 最大迭代次数
	Timeout       int                                                             `json:"timeout"`        // 每个工具的超时时间（秒）
	Parallel      bool                                                            `json:"parallel"`       // 是否允许并行执行
	Handler       func(string, map[string]interface{}) (interface{}, error)      `json:"-"`              // 自定义工具处理器
}

// DefaultToolExecutionOptions 默认工具执行选项
func DefaultToolExecutionOptions() *ToolExecutionOptions {
	return &ToolExecutionOptions{
		MaxIterations: 5,
		Timeout:       30,
		Parallel:      true,
	}
}

// ExecuteToolCalls 执行工具调用（支持并行）
func ExecuteToolCalls(toolCalls []ToolCall, options *ToolExecutionOptions) []ToolExecutionResult {
	if options == nil {
		options = DefaultToolExecutionOptions()
	}

	results := make([]ToolExecutionResult, len(toolCalls))

	if options.Parallel && len(toolCalls) > 1 {
		// 并行执行所有工具调用（使用 goroutine pool）
		var wg sync.WaitGroup
		for i, toolCall := range toolCalls {
			wg.Add(1)
			go func(idx int, tc ToolCall) {
				defer wg.Done()
				results[idx] = executeSingleToolCall(tc, options)
			}(i, toolCall)
		}
		wg.Wait()
	} else {
		// 串行执行
		for i, toolCall := range toolCalls {
			results[i] = executeSingleToolCall(toolCall, options)
		}
	}

	return results
}

// executeSingleToolCall 执行单个工具调用
func executeSingleToolCall(toolCall ToolCall, options *ToolExecutionOptions) ToolExecutionResult {
	result := ToolExecutionResult{
		ToolCallID: toolCall.ID,
		Role:       "tool",
		Name:       toolCall.Function.Name,
	}

	// 解析参数
	var params map[string]interface{}
	switch args := toolCall.Function.Arguments.(type) {
	case string:
		if args != "" && args != "{}" {
			if err := json.Unmarshal([]byte(args), &params); err != nil {
				result.Error = fmt.Sprintf("参数解析失败: %v", err)
				result.Content = result.Error
				return result
			}
		} else {
			params = make(map[string]interface{})
		}
	case map[string]interface{}:
		params = args
	default:
		result.Error = "无效的参数格式"
		result.Content = result.Error
		return result
	}

	// 执行工具
	var execResult interface{}
	var err error

	if options.Handler != nil {
		// 使用自定义处理器
		execResult, err = options.Handler(toolCall.Function.Name, params)
	} else {
		// 默认处理器：返回参数信息
		execResult = map[string]interface{}{
			"tool":   toolCall.Function.Name,
			"params": params,
			"status": "executed",
		}
		err = nil
	}

	if err != nil {
		result.Error = err.Error()
		result.Content = result.Error
		return result
	}

	// 设置结果
	result.Result = map[string]interface{}{
		"output": execResult,
	}

	// 将结果转换为 JSON 字符串作为 content
	if jsonBytes, err := json.Marshal(execResult); err == nil {
		result.Content = string(jsonBytes)
	} else {
		result.Content = fmt.Sprintf("%v", execResult)
	}

	return result
}

// BuildToolResponseMessage 构建工具响应消息
func BuildToolResponseMessage(toolCallID string, content string) Message {
	return Message{
		Role:       "tool",
		Content:    content,
		ToolCallID: toolCallID,
	}
}

// RequiresToolCall 检查响应是否需要工具调用
func RequiresToolCall(choice Choice) bool {
	return len(choice.Message.ToolCalls) > 0 || choice.FinishReason == "tool_calls"
}

// ShouldContinueToolLoop 检查是否应该继续工具调用循环
func ShouldContinueToolLoop(lastResponse *ChatCompletionResponse) bool {
	if lastResponse == nil || len(lastResponse.Choices) == 0 {
		return false
	}

	choice := lastResponse.Choices[0]
	return RequiresToolCall(choice)
}
