package proxy

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"oppama/internal/utils/logger"
)

// ============================================================================
// 智能客户端检测和格式转换
// ============================================================================


// detectModelType 根据模型名称判断模型类型
func detectModelType(model string) ModelType {
	modelLower := strings.ToLower(model)

	if strings.Contains(modelLower, "deepseek") {
		logger.Proxy().Printf("🤖 识别为 DeepSeek 模型 (XML 格式)")
		return ModelTypeDeepSeek
	}
	if strings.Contains(modelLower, "claude") {
		logger.Proxy().Printf("🤖 识别为 Claude 模型 (XML 格式)")
		return ModelTypeClaude
	}
	if strings.Contains(modelLower, "ollama") {
		logger.Proxy().Printf("🤖 识别为 Ollama 模型 (JSON 格式)")
		return ModelTypeOllama
	}
	if strings.Contains(modelLower, "gpt-") || strings.Contains(modelLower, "openai") {
		logger.Proxy().Printf("🤖 识别为 OpenAI 模型 (JSON 格式)")
		return ModelTypeOpenAI
	}

	logger.Proxy().Printf("🤖 未知模型类型，默认使用 JSON 格式：%s", model)
	return ModelTypeUnknown
}


// isStringValue 判断值是否为字符串类型
func isStringValue(value interface{}) bool {
	_, ok := value.(string)
	return ok
}

// ToolCallToXML 将 ToolCall 转换为 XML 格式（用于返回给 Claude Code 客户端）
func (c *ToolCallConverter) ToolCallToXML(toolCall *ToolCall) string {
	if toolCall == nil || toolCall.Type != "function" {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("<function_calls>\n")
	sb.WriteString(fmt.Sprintf("<invoke name=\"%s\">\n", toolCall.Function.Name))

	var params map[string]interface{}
	if toolCall.Function.Arguments != nil {
		switch v := toolCall.Function.Arguments.(type) {
		case string:
			// 如果是 JSON字符串，尝试解析
			json.Unmarshal([]byte(v), &params)
		case map[string]interface{}:
			params = v
		}
	}

	// 生成 parameter 标签
	for name, value := range params {
		isString := isStringValue(value)
		sb.WriteString(fmt.Sprintf("<parameter name=\"%s\" is_string=\"%t\">%v</parameter>\n", name, isString, value))
	}
	sb.WriteString("</invoke>\n")
	sb.WriteString("</function_calls>\n")
	return sb.String()
}

// ToolCallsToXML 将多个 ToolCall 转换为 XML 格式
func (c *ToolCallConverter) ToolCallsToXML(toolCalls []ToolCall) string {
	if len(toolCalls) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("<function_calls>\n")

	for _, toolCall := range toolCalls {
		if toolCall.Type != "function" {
			continue
		}

		sb.WriteString(fmt.Sprintf("<invoke name=\"%s\">\n", toolCall.Function.Name))

		var params map[string]interface{}
		if toolCall.Function.Arguments != nil {
			switch v := toolCall.Function.Arguments.(type) {
			case string:
				// 如果是 JSON字符串，尝试解析
				json.Unmarshal([]byte(v), &params)
			case map[string]interface{}:
				params = v
			}
		}

		// 生成 parameter 标签
		for name, value := range params {
			isString := isStringValue(value)
			sb.WriteString(fmt.Sprintf("<parameter name=\"%s\" is_string=\"%t\">%v</parameter>\n", name, isString, value))
		}

		sb.WriteString("</invoke>\n")
	}

	sb.WriteString("</function_calls>\n")
	return sb.String()
}

// XMLToToolCalls 将 XML 格式的工具调用转换为 ToolCall 数组
// 用于解析 DeepSeek/Claude 模型返回的 XML 工具调用
func (c *ToolCallConverter) XMLToToolCalls(xmlContent string) []ToolCall {
	// 检查是否包含 function_calls 标签
	if !strings.Contains(xmlContent, "<function_calls>") {
		return nil
	}

	var toolCalls []ToolCall

	// 查找所有 <invoke> 标签
	invokeStart := strings.Index(xmlContent, "<invoke ")
	if invokeStart == -1 {
		return nil
	}

	// 循环处理每个 invoke
	remaining := xmlContent
	for {
		invokeStart := strings.Index(remaining, "<invoke ")
		if invokeStart == -1 {
			break
		}

		invokeEnd := strings.Index(remaining[invokeStart:], ">")
		if invokeEnd == -1 {
			break
		}
		invokeEnd += invokeStart

		// 解析 invoke 标签的属性
		invokeTag := remaining[invokeStart:invokeEnd]
		nameMatch := regexp.MustCompile(`name="([^"]+)"`).FindStringSubmatch(invokeTag)
		if len(nameMatch) < 2 {
			// 移动到下一个位置
			remaining = remaining[invokeEnd+1:]
			continue
		}
		toolName := nameMatch[1]

		logger.Proxy().Printf("🔍 解析 XML 工具调用：%s", toolName)

		// 查找对应的 </invoke> 标签
		invokeCloseTag := strings.Index(remaining[invokeEnd:], "</invoke>")
		if invokeCloseTag == -1 {
			break
		}
		invokeCloseEnd := invokeEnd + invokeCloseTag + len("</invoke>")

		// 提取 invoke 内部的 XML（包含 parameter 标签）
		invokeInner := remaining[invokeEnd:invokeCloseEnd]

		// 提取所有 <parameter> 标签
		params := make(map[string]interface{})
		paramRegex := regexp.MustCompile(`<parameter\s+name="([^"]+)"(?:\s+is_string="(true|false)")?>([^<]*)</parameter>`)
		matches := paramRegex.FindAllStringSubmatch(invokeInner, -1)
		for _, match := range matches {
			if len(match) >= 4 {
				paramName := match[1]
				paramValue := match[3]
				isString := match[2] == "true"

				if isString {
					params[paramName] = paramValue
				} else {
					// 尝试解析为数字或布尔值
					if paramValue == "true" {
						params[paramName] = true
					} else if paramValue == "false" {
						params[paramName] = false
					} else if num, err := strconv.ParseFloat(paramValue, 64); err == nil {
						params[paramName] = num
					} else {
						params[paramName] = paramValue
					}
				}
			}
		}

		// 构造 ToolCall
		toolCall := ToolCall{
			ID:   fmt.Sprintf("toolu_xml_%d", time.Now().UnixNano()),
			Type: "function",
			Function: FunctionCall{
				Name: toolName,
			},
		}

		// 将参数转换为 JSON 字符串
		if argsJSON, err := json.Marshal(params); err == nil {
			toolCall.Function.Arguments = string(argsJSON)
		} else {
			toolCall.Function.Arguments = "{}"
		}

		toolCalls = append(toolCalls, toolCall)
		logger.Proxy().Printf("✅ 成功解析工具调用：%s, 参数：%v", toolName, params)

		// 移动到下一个 invoke
		remaining = remaining[invokeCloseEnd:]
	}

	return toolCalls
}

// ConvertRequestToolCalls 根据模型类型转换请求中的工具调用格式
// 如果模型需要 XML 格式，将 JSON tool_calls 转换为 XML content
func (c *ToolCallConverter) ConvertRequestToolCalls(msg *Message, modelType ModelType) *Message {
	// 复制消息
	convertedMsg := *msg
	convertedMsg.ToolCalls = nil // 清空原有的 tool_calls

	if len(msg.ToolCalls) == 0 {
		return &convertedMsg
	}

	// 判断是否需要转换为 XML
	if modelType == ModelTypeDeepSeek || modelType == ModelTypeClaude {
		// 转换为 XML 格式
		xmlContent := c.ToolCallsToXML(msg.ToolCalls)

		// 将 XML 放入 content 字段
		convertedMsg.Content = xmlContent
		logger.Proxy().Printf("🔄 将 JSON tool_calls 转换为 XML 格式（%s 模型）", modelType)
	} else {
		// 保持 JSON 格式
		convertedMsg.ToolCalls = msg.ToolCalls
		logger.Proxy().Printf("📋 保持 JSON tool_calls 格式（%s 模型）", modelType)
	}

	return &convertedMsg
}

// ShouldConvertToolCalls 判断是否需要转换工具调用格式
func ShouldConvertToolCalls(clientType ClientType, modelType ModelType) bool {
	// 如果客户端发送 JSON，但模型需要 XML，则需要转换
	if clientType == ClientTypeOpencode && (modelType == ModelTypeDeepSeek || modelType == ModelTypeClaude) {
		return true
	}

	// 如果客户端发送 XML，但模型需要 JSON，则需要转换
	if clientType == ClientTypeClaudeCode && (modelType == ModelTypeOllama || modelType == ModelTypeOpenAI) {
		return true
	}

	return false
}
