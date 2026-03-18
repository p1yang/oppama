#!/bin/bash
# 完整诊断工具调用问题的脚本

echo "=========================================="
echo "  工具调用完整诊断"
echo "=========================================="
echo ""

# 配置
API_KEY="your-api-key"
BASE_URL="http://localhost:8080"

echo "📋 步骤 1: 检查 Ollama 是否支持工具调用"
echo "----------------------------------------"
echo "发送一个简单的工具调用请求到 Ollama..."
echo ""

# 非流式测试
curl -s -X POST "$BASE_URL/v1/chat/completions" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "qwen2.5:latest",
    "stream": false,
    "messages": [
      {
        "role": "user", 
        "content": "使用 get_weather 工具查询北京天气"
      }
    ],
    "tools": [
      {
        "type": "function",
        "function": {
          "name": "get_weather",
          "description": "获取指定城市的天气信息",
          "parameters": {
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
      }
    ]
  }' | jq '.'

echo ""
echo "✅ 期望看到："
echo "   - choices[0].message.tool_calls 数组"
echo "   - 或者至少有一个 function 调用"
echo ""
read -p "按回车继续..."

echo ""
echo "📋 步骤 2: 测试 Anthropic 兼容层"
echo "----------------------------------------"
echo "发送 Anthropic 格式的工具调用请求..."
echo ""

cat > /tmp/anthropic-tool-test.json << 'EOF'
{
  "model": "claude-opus-4-6",
  "max_tokens": 1024,
  "messages": [
    {
      "role": "user",
      "content": "请使用 get_weather 工具查询北京天气"
    }
  ],
  "tools": [
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
  ],
  "stream": false
}
EOF

curl -s -X POST "$BASE_URL/api/v1/messages" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -H "x-api-key: $API_KEY" \
  -d @/tmp/anthropic-tool-test.json | jq '.'

echo ""
echo "✅ 期望看到："
echo "   - content 数组中包含 type='tool_use' 的元素"
echo "   - stop_reason = 'tool_use'"
echo ""
read -p "按回车继续..."

echo ""
echo "📋 步骤 3: 查看 Oppama 日志"
echo "----------------------------------------"
echo "最近 30 条日志:"
echo ""
tail -n 30 logs/oppama.log | grep -E "(工具|tool|Tool|🔧|✅|⚠️)" || echo "没有找到相关日志"

echo ""
echo "📋 步骤 4: 实时日志监控"
echo "----------------------------------------"
echo "现在将发送一个流式请求，请观察日志输出..."
echo ""
read -p "按回车开始实时监控（然后按 Ctrl+C 退出）..."

# 后台监控日志
tail -f logs/oppama.log | grep -E "(工具|tool|Tool|🔧|✅|⚠️|解析)" &
TAIL_PID=$!

# 等待 2 秒让 tail 启动
sleep 2

# 发送流式请求
echo ""
echo "发送流式工具调用请求..."
curl -s -X POST "$BASE_URL/api/v1/messages" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -H "x-api-key: $API_KEY" \
  -d '{
    "model": "claude-opus-4-6",
    "max_tokens": 1024,
    "messages": [
      {
        "role": "user",
        "content": "使用 get_weather 工具查询北京天气"
      }
    ],
    "tools": [
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
    ],
    "stream": true
  }'

# 等待 3 秒
sleep 3

# 停止日志监控
kill $TAIL_PID 2>/dev/null

echo ""
echo ""
echo "=========================================="
echo "  诊断完成！"
echo "=========================================="
echo ""
echo "📝 请检查以上输出，特别关注："
echo ""
echo "1. Ollama 是否返回了 tool_calls？"
echo "   - 如果是 → 说明 Ollama 支持工具调用"
echo "   - 如果否 → 需要使用中间人代理模式"
echo ""
echo "2. Oppama 日志中是否有以下信息："
echo "   - 🔧 检测到原生工具调用 → 成功路径 1"
echo "   - ✅ 从文本中成功解析工具调用 → 成功路径 2"
echo "   - 📝 普通文本响应 → 模型没有理解工具调用"
echo ""
echo "3. Anthropic 响应中的 content_block.type 是什么？"
echo "   - 应该是 'tool_use'"
echo ""
