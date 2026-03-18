#!/bin/bash

# 测试工具调用的请求
echo "测试 Anthropic API 工具调用..."

# 定义一个工具
TOOLS='[
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
]'

# 发送请求
curl -X POST http://localhost:8080/v1/messages \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test-key" \
  -d '{
    "model": "claude-3-sonnet",
    "max_tokens": 1024,
    "messages": [
      {
        "role": "user",
        "content": "北京今天天气怎么样？"
      }
    ],
    "tools": '"$TOOLS"',
    "tool_choice": {
      "type": "auto"
    },
    "stream": false
  }' | jq .

echo ""
echo "完成"
