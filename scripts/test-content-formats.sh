#!/bin/bash

# 测试 Anthropic API 内容格式兼容性

set -e

echo "================================"
echo "Anthropic API 内容格式测试"
echo "================================"

BASE_URL="http://localhost:8080"
API_KEY="${OLLAMA_PROXY_API_KEY:-admin123}"

echo ""
echo "配置信息:"
echo "  BASE_URL: ${BASE_URL}"
echo "  API_KEY: ${API_KEY}"
echo ""

# 检查服务器是否运行
echo "1. 检查服务器健康状态..."
if curl -s "${BASE_URL}/health" | grep -q "ok"; then
    echo "   ✓ 服务器运行正常"
else
    echo "   ✗ 服务器未运行或响应异常"
    exit 1
fi

# 测试字符串格式内容
echo ""
echo "2. 测试字符串格式内容..."
RESPONSE=$(curl -s -X POST "${BASE_URL}/v1/messages" \
  -H "Content-Type: application/json" \
  -H "x-api-key: ${API_KEY}" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "test",
    "max_tokens": 10,
    "messages": [
      {"role": "user", "content": "Hello, this is a string content!"}
    ]
  }')

if echo "$RESPONSE" | jq -e '.type == "message"' > /dev/null; then
    echo "   ✓ 字符串格式内容测试通过"
    echo "   响应示例：$(echo "$RESPONSE" | jq -c '.')"
else
    echo "   ✗ 字符串格式内容测试失败"
    echo "   响应：$RESPONSE"
    exit 1
fi

# 测试数组格式内容
echo ""
echo "3. 测试数组格式内容..."
RESPONSE=$(curl -s -X POST "${BASE_URL}/v1/messages" \
  -H "Content-Type: application/json" \
  -H "x-api-key: ${API_KEY}" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "test",
    "max_tokens": 10,
    "messages": [
      {
        "role": "user",
        "content": [
          {"type": "text", "text": "Hello from array!"}
        ]
      }
    ]
  }')

if echo "$RESPONSE" | jq -e '.type == "message"' > /dev/null; then
    echo "   ✓ 数组格式内容测试通过"
    echo "   响应示例：$(echo "$RESPONSE" | jq -c '.')"
else
    echo "   ✗ 数组格式内容测试失败"
    echo "   响应：$RESPONSE"
    exit 1
fi

# 测试多个内容块
echo ""
echo "4. 测试多个内容块..."
RESPONSE=$(curl -s -X POST "${BASE_URL}/v1/messages" \
  -H "Content-Type: application/json" \
  -H "x-api-key: ${API_KEY}" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "test",
    "max_tokens": 10,
    "messages": [
      {
        "role": "user",
        "content": [
          {"type": "text", "text": "First block"},
          {"type": "text", "text": "Second block"}
        ]
      }
    ]
  }')

if echo "$RESPONSE" | jq -e '.type == "message"' > /dev/null; then
    echo "   ✓ 多个内容块测试通过"
    echo "   响应示例：$(echo "$RESPONSE" | jq -c '.')"
else
    echo "   ✗ 多个内容块测试失败"
    echo "   响应：$RESPONSE"
    exit 1
fi

echo ""
echo "================================"
echo "✅ 所有测试通过！"
echo "================================"
echo ""
echo "总结："
echo "  ✓ 字符串格式 content: \"text\""
echo "  ✓ 数组格式 content: [{\"type\": \"text\", \"text\": \"...\"}]"
echo "  ✓ 多数组格式 content: [{...}, {...}]"
echo ""
