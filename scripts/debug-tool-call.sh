#!/bin/bash

echo "========================================"
echo "Claude Code 工具调用调试脚本"
echo "========================================"
echo ""

# API 配置
API_KEY="test-key"
BASE_URL="http://localhost:8080"

# 定义一个简单的工具
TOOLS='[{
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
}]'

echo "📦 测试 1: 非流式请求（包含工具定义）"
echo "----------------------------------------"

RESPONSE=$(curl -s -X POST "${BASE_URL}/v1/messages" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${API_KEY}" \
  -d "{
    \"model\": \"claude-3-sonnet\",
    \"max_tokens\": 1024,
    \"messages\": [
      {
        \"role\": \"user\",
        \"content\": \"北京今天天气怎么样？\"
      }
    ],
    \"tools\": ${TOOLS},
    \"tool_choice\": {\"type\": \"auto\"},
    \"stream\": false
  }")

echo "响应内容:"
echo "$RESPONSE" | jq '.' 2>/dev/null || echo "$RESPONSE"

echo ""
echo "检查要点:"
echo "1. stop_reason 是否为 'tool_use'"
echo "2. content 数组中是否有 type='tool_use' 的元素"
echo "3. input 字段是否包含 city 参数"
echo ""

# 提取关键信息
STOP_REASON=$(echo "$RESPONSE" | jq -r '.stop_reason' 2>/dev/null)
echo "stop_reason: $STOP_REASON"

if [ "$STOP_REASON" = "tool_use" ]; then
    echo "✅ stop_reason 正确"
else
    echo "❌ stop_reason 不正确：$STOP_REASON"
fi

echo ""
echo "========================================"
echo "测试 2: 流式请求（包含工具定义）"
echo "========================================"
echo ""

echo "发送流式请求..."
curl -s -X POST "${BASE_URL}/v1/messages" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${API_KEY}" \
  -d "{
    \"model\": \"claude-3-sonnet\",
    \"max_tokens\": 1024,
    \"messages\": [
      {
        \"role\": \"user\",
        \"content\": \"帮我查询上海的天气\"
      }
    ],
    \"tools\": ${TOOLS},
    \"tool_choice\": {\"type\": \"auto\"},
    \"stream\": true
  }" | while IFS= read -r line; do
    if [[ "$line" == data:* ]]; then
        data="${line#data: }"
        echo "收到事件: $(echo "$data" | jq -r '.type' 2>/dev/null || echo 'unknown')"
        
        # 检查是否是 tool_use
        if echo "$data" | jq -e '.content_block.type == "tool_use"' >/dev/null 2>&1; then
            echo "  ✅ 检测到 tool_use:"
            echo "$data" | jq '.' 2>/dev/null
        fi
        
        # 检查 stop_reason
        if echo "$data" | jq -e '.delta.stop_reason' >/dev/null 2>&1; then
            reason=$(echo "$data" | jq -r '.delta.stop_reason' 2>/dev/null)
            echo "  🎯 stop_reason: $reason"
        fi
    fi
done

echo ""
echo "========================================"
echo "调试提示"
echo "========================================"
echo ""
echo "请查看 Oppama 日志文件，确认以下信息："
echo "1. 是否看到 '🔧 包含 X 个工具定义'"
echo "2. 是否看到 '🔧 检测到原生工具调用'"
echo "3. 是否看到 '✅ 从文本中成功解析工具调用'"
echo ""
echo "如果只看到 '📝 普通文本响应'，说明模型没有按工具格式回复"
echo "这可能是因为："
echo "  - 模型不理解工具使用说明"
echo "  - 系统提示没有被正确添加"
echo "  - 模型本身不支持工具调用概念"
echo ""
