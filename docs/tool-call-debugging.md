# 工具调用问题调试指南

## 🔍 问题现象

Claude Code 发送了工具调用请求，但 Oppama 返回的是普通文本而不是 `tool_use` 格式。

## 📋 调试步骤

### 步骤 1: 运行调试脚本

```bash
chmod +x scripts/debug-tool-call.sh
./scripts/debug-tool-call.sh
```

这个脚本会：
- ✅ 发送非流式请求测试工具调用
- ✅ 发送流式请求测试工具调用
- ✅ 自动检查响应中的关键字段
- ✅ 提供下一步的调试建议

### 步骤 2: 查看 Oppama 日志

```bash
tail -f logs/oppama.log
```

**关键日志信息：**

#### ✅ 正常情况应该看到：

```
🔧 包含 1 个工具定义
  工具 1: get_weather (获取指定城市的天气信息)
选中服务：ollama-1 (http://localhost:11434), 模型：llama2
🔧 检测到原生工具调用：1 个
```

或者（如果 Ollama 不支持原生工具调用）：

```
🔧 包含 1 个工具定义
  工具 1: get_weather (获取指定城市的天气信息)
✅ 从文本中成功解析工具调用：get_weather
```

#### ❌ 异常情况：

```
⚠️ 警告：请求中没有工具定义
```
或
```
📝 普通文本响应（未检测到工具调用）
```

### 步骤 3: 检查各个环节

#### A. 检查 Claude Code 请求

确保 Claude Code 发送的请求包含正确的工具定义：

```json
{
  "model": "claude-3-sonnet",
  "max_tokens": 1024,
  "messages": [{"role": "user", "content": "查询北京天气"}],
  "tools": [{
    "name": "get_weather",
    "description": "获取指定城市的天气信息",
    "input_schema": {
      "type": "object",
      "properties": {
        "city": {"type": "string", "description": "城市名称"}
      },
      "required": ["city"]
    }
  }]
}
```

#### B. 检查系统提示是否添加

在 Oppama 日志中查找发送给 Ollama 的请求内容，应该看到类似：

```
system: 你是一个智能助手，可以使用以下工具来帮助用户：

## 可用工具

1. **get_weather**
   描述：获取指定城市的天气信息
   参数:
      - city: 城市名称

## 使用规则

当你需要使用工具时，请严格按照以下 JSON 格式回复：
...
```

#### C. 检查 Ollama 响应

查看 Ollama 返回的内容是什么格式：

**情况 1: 原生工具调用（最佳）**
```json
{
  "message": {
    "role": "assistant",
    "content": "",
    "tool_calls": [{
      "id": "call_abc",
      "type": "function",
      "function": {
        "name": "get_weather",
        "arguments": {"city": "Beijing"}
      }
    }]
  }
}
```

**情况 2: 文本格式工具调用（可解析）**
```
我来帮你查询北京天气。

```json
{
  "action": "tool_use",
  "tool_name": "get_weather",
  "tool_input": {"city": "Beijing"}
}
```

**情况 3: 纯文本（无法解析）**
```
好的，我来查询一下北京今天的天气。根据天气预报，北京今天晴朗...
```

## 🛠️ 常见问题和解决方案

### 问题 1: 没有看到"包含 X 个工具定义"日志

**原因：** 工具定义没有从 Anthropic 格式转换为 OpenAI 格式

**检查：** 
- 查看 `convertToOpenAIFormat` 函数是否正确转换 tools
- 确认请求中确实包含了 tools 字段

### 问题 2: 看到"普通文本响应"

**原因：** 模型没有按照工具格式回复

**可能的问题：**
1. **系统提示没有被重视** - 有些模型会忽略 system prompt
2. **模型不理解工具调用概念** - 需要更明确的指令
3. **Prompt 格式不够清晰** - JSON 示例可能太复杂

**解决方案：**

尝试修改 `buildToolInstructions` 函数，使用更简洁直接的指令：

```go
func buildToolInstructions(tools []Tool) string {
    var sb strings.Builder
    
    sb.WriteString("【重要指令】你必须严格按照以下格式使用工具：\n\n")
    
    for _, tool := range tools {
        sb.WriteString(fmt.Sprintf("工具：%s\n", tool.Function.Name))
        sb.WriteString(fmt.Sprintf("功能：%s\n", tool.Function.Description))
        sb.WriteString("格式：{\"action\":\"tool_use\",\"tool_name\":\"")
        sb.WriteString(tool.Function.Name)
        sb.WriteString("\",\"tool_input\":{参数}}\n\n")
    }
    
    sb.WriteString("⚠️ 注意：只输出 JSON，不要输出其他解释！")
    
    return sb.String()
}
```

### 问题 3: 解析失败 "解析工具调用 JSON 失败"

**原因：** JSON 格式不正确

**检查 tryParseToolUseFromText 函数的日志输出**，看看实际收到的内容是什么。

可能的改进：增强 JSON 提取逻辑

```go
// 在 tryParseToolUseFromText 中添加更多容错
func tryParseToolUseFromText(content string) *ToolCall {
    // 尝试多种 JSON 提取策略
    patterns := []string{
        `\{[^{}]*"action"[^{}]*"tool_use"[^{}]*\}`,
        `\{[^{}]*"tool_name"[^{}]*\}`,
    }
    
    for _, pattern := range patterns {
        re := regexp.MustCompile(pattern)
        matches := re.FindStringSubmatch(content)
        if len(matches) > 0 {
            jsonStr := matches[0]
            // 尝试解析...
        }
    }
    
    return nil
}
```

### 问题 4: 流式响应中工具调用被分散

**原因：** 工具调用的 JSON 被分成多个 chunk 传输

**解决方案：** 已经修复！现在只在最后一个 chunk (`Done=true`) 时尝试解析，确保有完整内容。

## 📊 完整的调试流程图

```
Claude Code 请求
    ↓
Oppama 接收 (检查是否有 tools)
    ↓
有 tools → 添加系统提示
    ↓
发送给 Ollama
    ↓
等待 Ollama 响应
    ↓
Ollama 返回内容
    ↓
检查响应类型:
├─ 有 tool_calls 数组 → ✅ 原生支持
│   └─→ 直接转发给 Claude Code
│
└─ 只有 content 文本 → 🔍 尝试解析
    ├─ 找到 JSON 且格式正确 → ✅ 转换并转发
    │   └─→ 设置 finish_reason="tool_calls"
    │
    └─ 找不到 JSON 或格式错误 → ❌ 作为普通文本
        └─→ 设置 finish_reason="stop"
```

## 🎯 验证成功的标准

### ✅ 非流式响应验证

运行以下命令检查响应：

```bash
curl -X POST http://localhost:8080/v1/messages \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test-key" \
  -d '{
    "model": "claude-3-sonnet",
    "max_tokens": 1024,
    "messages": [{"role": "user", "content": "查询北京天气"}],
    "tools": [{"name": "get_weather", "description": "查询天气", "input_schema": {"type": "object", "properties": {"city": {"type": "string"}}}}]
  }' | jq '.'
```

**期望输出：**

```json
{
  "stop_reason": "tool_use",
  "content": [
    {
      "type": "tool_use",
      "id": "toolu_xxx",
      "name": "get_weather",
      "input": {
        "city": "Beijing"
      }
    }
  ]
}
```

### ✅ 流式响应验证

观察 SSE 事件序列：

```
event: message_start
data: {...}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_xxx","name":"get_weather","input":{}}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"city\":\"Beijing\"}"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"tool_use"},...}

event: message_stop
data: {"type":"message_stop"}
```

## 💡 终极调试技巧

如果以上都不行，可以临时添加更多日志：

在 `convertToOllamaRequest` 函数中添加：

```go
logger.Proxy().Printf("📋 原始消息数量：%d", len(openaiReq.Messages))
logger.Proxy().Printf("📋 添加工具指令后消息数量：%d", len(messages))
for i, msg := range messages {
    logger.Proxy().Printf("  消息 %d [%s]: %.50s...", i, msg.Role, msg.Content)
}
```

这样可以清楚看到系统提示是否被正确添加。

## 📞 需要帮助？

如果还是不行，请提供以下信息：

1. Oppama 日志输出（特别是带 🔧、✅、❌ emoji 的行）
2. 调试脚本的输出
3. Claude Code 的实际请求内容
4. Ollama 的实际响应内容
