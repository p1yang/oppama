package proxy

import (
	"encoding/json"
	"testing"
)

// TestToolCallsToXML 测试 JSON 到 XML 的转换
func TestToolCallsToXML(t *testing.T) {
	converter := NewToolCallConverter()

	// 测试单个工具调用
	t.Run("SingleToolCall", func(t *testing.T) {
		toolCall := ToolCall{
			ID:   "call_123",
			Type: "function",
			Function: FunctionCall{
				Name: "search",
				Arguments: map[string]interface{}{
					"query":     "golang tutorial",
					"maxResults": 10,
				},
			},
		}

		xml := converter.ToolCallToXML(&toolCall)

		if xml == "" {
			t.Fatal("Expected non-empty XML output")
		}

		// 验证 XML 包含必要的标签
		expectedSubstrings := []string{
			"<function_calls>",
			`<invoke name="search">`,
			`<parameter name="query"`,
			`golang tutorial`,
			`<parameter name="maxResults"`,
			`</invoke>`,
			"</function_calls>",
		}

		for _, expected := range expectedSubstrings {
			if !contains(xml, expected) {
				t.Errorf("XML output missing expected substring: %s\nGot: %s", expected, xml)
			}
		}
	})

	// 测试多个工具调用
	t.Run("MultipleToolCalls", func(t *testing.T) {
		toolCalls := []ToolCall{
			{
				ID:   "call_1",
				Type: "function",
				Function: FunctionCall{
					Name: "search",
					Arguments: map[string]interface{}{
						"query": "test",
					},
				},
			},
			{
				ID:   "call_2",
				Type: "function",
				Function: FunctionCall{
					Name: "calculate",
					Arguments: map[string]interface{}{
						"x": 10,
						"y": 20,
					},
				},
			},
		}

		xml := converter.ToolCallsToXML(toolCalls)

		if xml == "" {
			t.Fatal("Expected non-empty XML output")
		}

		// 验证包含两个工具调用
		if !contains(xml, `name="search"`) || !contains(xml, `name="calculate"`) {
			t.Errorf("XML output missing tool calls\nGot: %s", xml)
		}
	})

	// 测试空工具调用列表
	t.Run("EmptyToolCalls", func(t *testing.T) {
		xml := converter.ToolCallsToXML([]ToolCall{})
		if xml != "" {
			t.Errorf("Expected empty string for empty tool calls, got: %s", xml)
		}
	})
}

// TestXMLToToolCalls 测试 XML 到 JSON 的转换
func TestXMLToToolCalls(t *testing.T) {
	converter := NewToolCallConverter()

	// 测试有效的 XML 工具调用
	t.Run("ValidXMLToolCall", func(t *testing.T) {
		xml := `<function_calls>
<invoke name="search">
<parameter name="query" is_string="true">golang tutorial</parameter>
<parameter name="maxResults" is_string="false">10</parameter>
</invoke>
</function_calls>`

		toolCalls := converter.XMLToToolCalls(xml)

		if len(toolCalls) == 0 {
			t.Fatal("Expected at least one tool call")
		}

		if toolCalls[0].Function.Name != "search" {
			t.Errorf("Expected tool name 'search', got: %s", toolCalls[0].Function.Name)
		}

		// 验证参数
		var args map[string]interface{}
		argsStr, ok := toolCalls[0].Function.Arguments.(string)
		if !ok {
			t.Fatalf("Arguments should be string, got: %T", toolCalls[0].Function.Arguments)
		}
		if err := json.Unmarshal([]byte(argsStr), &args); err != nil {
			t.Fatalf("Failed to parse arguments: %v", err)
		}

		if args["query"] != "golang tutorial" {
			t.Errorf("Expected query 'golang tutorial', got: %v", args["query"])
		}

		if args["maxResults"] != float64(10) {
			t.Errorf("Expected maxResults 10, got: %v", args["maxResults"])
		}
	})

	// 测试多个工具调用
	t.Run("MultipleXMLToolCalls", func(t *testing.T) {
		xml := `<function_calls>
<invoke name="search">
<parameter name="query" is_string="true">test</parameter>
</invoke>
<invoke name="calculate">
<parameter name="x" is_string="false">10</parameter>
<parameter name="y" is_string="false">20</parameter>
</invoke>
</function_calls>`

		toolCalls := converter.XMLToToolCalls(xml)

		if len(toolCalls) != 2 {
			t.Fatalf("Expected 2 tool calls, got: %d", len(toolCalls))
		}

		if toolCalls[0].Function.Name != "search" {
			t.Errorf("First tool name should be 'search', got: %s", toolCalls[0].Function.Name)
		}

		if toolCalls[1].Function.Name != "calculate" {
			t.Errorf("Second tool name should be 'calculate', got: %s", toolCalls[1].Function.Name)
		}
	})

	// 测试无效的 XML
	t.Run("InvalidXML", func(t *testing.T) {
		xml := `not valid xml`

		toolCalls := converter.XMLToToolCalls(xml)

		if len(toolCalls) != 0 {
			t.Errorf("Expected no tool calls for invalid XML, got: %d", len(toolCalls))
		}
	})

	// 测试空 XML
	t.Run("EmptyXML", func(t *testing.T) {
		xml := ``

		toolCalls := converter.XMLToToolCalls(xml)

		if len(toolCalls) != 0 {
			t.Errorf("Expected no tool calls for empty XML, got: %d", len(toolCalls))
		}
	})
}

// TestConvertRequestToolCalls 测试请求格式转换
func TestConvertRequestToolCalls(t *testing.T) {
	converter := NewToolCallConverter()

	// 测试转换为 XML（DeepSeek 模型）
	t.Run("ConvertToXML", func(t *testing.T) {
		msg := &Message{
			Role:    "assistant",
			Content: "",
			ToolCalls: []ToolCall{
				{
					ID:   "call_1",
					Type: "function",
					Function: FunctionCall{
						Name: "search",
						Arguments: map[string]interface{}{
							"query": "test",
						},
					},
				},
			},
		}

		converted := converter.ConvertRequestToolCalls(msg, ModelTypeDeepSeek)

		contentStr := GetContentString(converted.Content)
		if contentStr == "" {
			t.Fatal("Expected non-empty content after conversion")
		}

		if !contains(contentStr, "<function_calls>") {
			t.Errorf("Expected XML content, got: %s", contentStr)
		}

		if len(converted.ToolCalls) != 0 {
			t.Errorf("Expected ToolCalls to be cleared after XML conversion")
		}
	})

	// 测试保持 JSON 格式（OpenAI 模型）
	t.Run("KeepJSONFormat", func(t *testing.T) {
		msg := &Message{
			Role:    "assistant",
			Content: "",
			ToolCalls: []ToolCall{
				{
					ID:   "call_1",
					Type: "function",
					Function: FunctionCall{
						Name: "search",
						Arguments: map[string]interface{}{
							"query": "test",
						},
					},
				},
			},
		}

		converted := converter.ConvertRequestToolCalls(msg, ModelTypeOpenAI)

		if len(converted.ToolCalls) == 0 {
			t.Fatal("Expected ToolCalls to be preserved for JSON format")
		}

		if converted.ToolCalls[0].Function.Name != "search" {
			t.Errorf("Expected tool name 'search', got: %s", converted.ToolCalls[0].Function.Name)
		}
	})

	// 测试没有工具调用的消息
	t.Run("NoToolCalls", func(t *testing.T) {
		msg := &Message{
			Role:    "user",
			Content: "hello",
		}

		converted := converter.ConvertRequestToolCalls(msg, ModelTypeDeepSeek)

		if converted.Content != "hello" {
			t.Errorf("Expected content to remain unchanged, got: %s", converted.Content)
		}
	})
}

// TestShouldConvertToolCalls 测试是否需要转换的判断
func TestShouldConvertToolCalls(t *testing.T) {
	tests := []struct {
		name       string
		clientType ClientType
		modelType  ModelType
		expected   bool
	}{
		{
			name:       "Opencode client with DeepSeek model",
			clientType: ClientTypeOpencode,
			modelType:  ModelTypeDeepSeek,
			expected:   true,
		},
		{
			name:       "Opencode client with OpenAI model",
			clientType: ClientTypeOpencode,
			modelType:  ModelTypeOpenAI,
			expected:   false,
		},
		{
			name:       "ClaudeCode client with OpenAI model",
			clientType: ClientTypeClaudeCode,
			modelType:  ModelTypeOpenAI,
			expected:   true,
		},
		{
			name:       "ClaudeCode client with Claude model",
			clientType: ClientTypeClaudeCode,
			modelType:  ModelTypeClaude,
			expected:   false,
		},
		{
			name:       "Unknown client with DeepSeek model",
			clientType: ClientTypeUnknown,
			modelType:  ModelTypeDeepSeek,
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShouldConvertToolCalls(tt.clientType, tt.modelType)
			if result != tt.expected {
				t.Errorf("ShouldConvertToolCalls(%v, %v) = %v; want %v",
					tt.clientType, tt.modelType, result, tt.expected)
			}
		})
	}
}

// TestDetectModelType 测试模型类型检测
func TestDetectModelType(t *testing.T) {
	tests := []struct {
		model    string
		expected ModelType
	}{
		{"deepseek-r1", ModelTypeDeepSeek},
		{"deepseek-coder", ModelTypeDeepSeek},
		{"claude-3-opus", ModelTypeClaude},
		{"claude-3.5-sonnet", ModelTypeClaude},
		{"ollama/llama3", ModelTypeOllama},
		{"gpt-4", ModelTypeOpenAI},
		{"openai/gpt-3.5", ModelTypeOpenAI},
		{"unknown-model", ModelTypeUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			result := detectModelType(tt.model)
			if result != tt.expected {
				t.Errorf("detectModelType(%s) = %v; want %v", tt.model, result, tt.expected)
			}
		})
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && findSubstr(s, substr)))
}

func findSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
