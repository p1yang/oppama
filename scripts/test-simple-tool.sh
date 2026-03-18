#!/bin/bash

# 超简单的工具调用测试

echo "发送最简单请求..."

curl -X POST http://localhost:8080/v1/messages \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test-key" \
  -d '{
    "model": "claude-3-sonnet",
    "max_tokens": 500,
    "messages": [
      {"role": "user", "content": "用工具查询北京天气"}
    ],
    "tools": [{
      "name": "get_weather",
      "description": "Get weather for a city",
      "input_schema": {
        "type": "object",
        "properties": {
          "city": {"type": "string"}
        },
        "required": ["city"]
      }
    }]
  }' | jq '.'
