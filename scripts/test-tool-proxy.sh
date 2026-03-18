#!/bin/bash

echo "======================================"
echo "测试 Oppama 工具调用中间人功能"
echo "======================================"
echo ""

# 定义工具
TOOLS='[{
  "name": "get_weather",
  "description": "获取指定城市的天气信息",
  "input_schema": {
    "type": "object",
    "properties": {
      "city": {
        "type": "string",
        "description": "城市名称，例如：北京、上海"
      }
    },
    "required": ["city"]
  }
}]'

echo "📦 发送请求（包含工具定义）..."
echo ""

# 发送非流式请求测试
RESPONSE=$(curl -s -X POST http://localhost:8080/v1/messages \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test-key" \
  -d "{
    \"model\": \"claude-3-sonnet\",
    \"max_tokens\": 1024,
    \"messages\": [
      {
        \"role\": \"user\",
        \"content\": \"北京今天天气怎么样？我需要知道温度。\"
      }
    ],
    \"tools\": $TOOLS,
    \"tool_choice\": {\"type\": \"auto\"},
    \"stream\": false
  }")

echo "📥 响应:"
echo "$RESPONSE" | jq '.'

echo ""
echo "======================================"
echo "检查要点:"
echo "1. stop_reason 是否为 'tool_use'"
echo "2. content 中是否包含 type='tool_use' 的内容块"
echo "3. input 中是否包含 city 参数"
echo "======================================"
