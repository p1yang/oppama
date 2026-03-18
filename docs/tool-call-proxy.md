# 工具调用中间人代理实现文档

## 🎯 核心功能

Oppama 作为中间人，让**不支持工具调用的后端（如旧版 Ollama）也能支持工具调用**。

## 📋 实现原理

### 1. Prompt 增强（在发送给后端前）

当检测到请求中包含工具定义时，Oppama 会自动在系统提示中添加详细的使用说明：

```
你是一个智能助手，可以使用以下工具来帮助用户：

## 可用工具

1. **get_weather**
   描述：获取指定城市的天气信息
   参数:
      - city: 城市名称

## 使用规则

当你需要使用工具时，请严格按照以下 JSON 格式回复：

```json
{
  "action": "tool_use",
  "tool_name": "工具名称",
  "tool_input": {
    "参数名": "参数值"
  }
}
```

重要提示：
- 只在确实需要时才使用工具
- 确保提供所有必需的参数
- 如果你不确定，可以先向用户询问更多信息
- 不要编造工具返回的结果，等待系统为你提供结果
```

### 2. 响应解析（从后端返回后）

Oppama 会检测模型的响应：

#### 情况 A：后端原生支持工具调用
- 直接使用 `message.tool_calls` 字段
- 无需额外处理

#### 情况 B：后端不支持工具调用
- 模型会以文本形式返回 JSON
- Oppama 自动解析文本中的 JSON 代码块
- 提取 `action`, `tool_name`, `tool_input` 字段
- 转换为标准的 OpenAI `ToolCall` 格式

### 3. 格式转换

**从文本中解析的格式：**
```json
{
  "action": "tool_use",
  "tool_name": "get_weather",
  "tool_input": {
    "city": "Beijing"
  }
}
```

**转换为 OpenAI 标准格式：**
```json
{
  "id": "toolu_1234567890",
  "type": "function",
  "function": {
    "name": "get_weather",
    "arguments": "{\"city\":\"Beijing\"}"
  }
}
```

## 🔄 完整流程

```
Claude Code (Client)
    ↓ (Anthropic 格式，包含 tools 定义)
    POST /v1/messages
    {
      "messages": [...],
      "tools": [{"name": "get_weather", ...}]
    }

Oppama (中间人)
    ↓ 1. 转换为 OpenAI 格式
    ↓ 2. 添加系统提示（工具使用说明）
    POST /api/chat
    {
      "messages": [
        {"role": "system", "content": "你可以使用以下工具..."},
        {"role": "user", "content": "..."}
      ],
      "tools": [...]
    }

Ollama (后端，不支持工具调用)
    ↓ 返回文本格式的工具调用
    {
      "message": {
        "content": "我将使用工具...\n```json\n{\"action\":\"tool_use\",...}\n```"
      }
    }

Oppama (中间人)
    ↓ 1. 检测并解析 JSON
    ↓ 2. 转换为 ToolCall 格式
    ↓ 3. 设置 finish_reason = "tool_calls"
    SSE Events:
    event: content_block_start
    data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use",...}}
    
    event: content_block_delta
    data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"..."}}
    
    event: content_block_stop
    data: {"type":"content_block_stop","index":0}
    
    event: message_delta
    data: {"stop_reason":"tool_use",...}

Claude Code (Client)
    ✅ 收到标准的 tool_use 格式
    可以继续执行工具调用流程
```

## 🛠️ 关键代码

### 1. buildToolInstructions (proxy.go)

构建详细的工具使用说明，包括：
- 工具列表和描述
- 参数说明
- JSON 格式示例
- 使用注意事项

### 2. tryParseToolUseFromText (proxy.go)

从文本中提取工具调用：
- 查找 ```json 代码块
- 解析 JSON 对象
- 验证必需字段 (action, tool_name)
- 转换为标准 ToolCall 格式

### 3. convertToOpenAIStreamChunk (proxy.go)

智能判断：
- 优先使用原生的 tool_calls
- 如果没有，尝试从文本解析
- 自动设置正确的 finish_reason

## ✅ 支持的场景

### ✅ 场景 1：后端原生支持工具调用
- Ollama >= 0.1.30 + 支持的模型
- 直接使用原生 API
- 最佳性能

### ✅ 场景 2：后端不支持工具调用
- 旧版 Ollama
- 不支持工具调用的模型
- 通过 Prompt 工程 + 文本解析实现
- 兼容性最好

### ✅ 场景 3：混合环境
- 自动检测后端能力
- 优先使用原生支持
- 降级到文本解析
- 对客户端透明

## 🎁 优势

1. **对客户端透明** - Claude Code 不需要知道后端是否支持工具调用
2. **向后兼容** - 支持旧版 Ollama 和其他不支持工具调用的后端
3. **统一接口** - 始终返回标准的 Anthropic/OpenAI 格式
4. **灵活扩展** - 可以轻松添加更多工具调用格式的支持

## 📝 示例工具定义

```json
{
  "name": "get_weather",
  "description": "获取指定城市的天气信息",
  "input_schema": {
    "type": "object",
    "properties": {
      "city": {
        "type": "string",
        "description": "城市名称"
      }
    },
    "required": ["city"]
  }
}
```

## 🔍 调试技巧

查看日志确认工具调用是否被正确解析：

```bash
# 启动服务
./oppama serve

# 观察日志
# 应该看到类似输出：
🔧 包含 1 个工具定义
  工具 1: get_weather (获取指定城市的天气信息)
✅ 从文本中成功解析工具调用：get_weather
```

## 🚀 未来扩展

1. **工具注册表** - 预定义常用工具，自动执行
2. **多轮对话支持** - 记住工具调用历史
3. **工具链编排** - 支持多个工具的顺序调用
4. **错误处理** - 更友好的工具调用失败提示
