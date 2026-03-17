#!/bin/bash

# Oppama Anthropic API 快速测试脚本

set -e

echo "================================"
echo "Oppama Anthropic API 测试"
echo "================================"

BASE_URL="http://localhost:8080"
API_KEY="${OLLAMA_PROXY_API_KEY:-test-key}"

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

# 测试非流式消息
echo ""
echo "2. 测试 Anthropic Messages API (非流式)..."
RESPONSE=$(curl -s -X POST "${BASE_URL}/v1/messages" \
  -H "Content-Type: application/json" \
  -H "x-api-key: ${API_KEY}" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "test",
    "max_tokens": 1024,
    "messages": [
      {"role": "user", "content": "Hello, Claude! Please respond with just the word SUCCESS if you receive this message."}
    ]
  }')

if echo "$RESPONSE" | jq -e '.type == "message"' > /dev/null 2>&1; then
    echo "   ✓ 非流式请求成功"
    echo "   响应内容：$(echo "$RESPONSE" | jq -r '.content[0].text' 2>/dev/null || echo '无法解析')"
else
    echo "   ✗ 非流式请求失败"
    echo "   响应：$RESPONSE"
fi

# 测试流式消息
echo ""
echo "3. 测试 Anthropic Messages API (流式)..."
STREAM_RESPONSE=$(curl -s -X POST "${BASE_URL}/v1/messages" \
  -H "Content-Type: application/json" \
  -H "x-api-key: ${API_KEY}" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "test",
    "max_tokens": 1024,
    "messages": [
      {"role": "user", "content": "Say hello in 3 words"}
    ],
    "stream": true
  }')

if echo "$STREAM_RESPONSE" | grep -q "message_start"; then
    echo "   ✓ 流式请求成功"
    # 提取实际内容
    CONTENT=$(echo "$STREAM_RESPONSE" | grep "content_block_delta" | head -n 1 | sed 's/.*"text":"\([^"]*\)".*/\1/' || echo "")
    if [ -n "$CONTENT" ]; then
        echo "   响应内容片段：$CONTENT"
    fi
else
    echo "   ✗ 流式请求失败"
    echo "   响应：$STREAM_RESPONSE"
fi

# 测试 Models API
echo ""
echo "4. 测试 Anthropic Models API..."
MODELS_RESPONSE=$(curl -s "${BASE_URL}/v1/models" \
  -H "x-api-key: ${API_KEY}" \
  -H "anthropic-version: 2023-06-01")

if echo "$MODELS_RESPONSE" | jq -e '.data' > /dev/null 2>&1; then
    echo "   ✓ Models API 请求成功"
    MODEL_COUNT=$(echo "$MODELS_RESPONSE" | jq '.data | length')
    echo "   可用模型数量：$MODEL_COUNT"
else
    echo "   ✗ Models API 请求失败"
    echo "   响应：$MODELS_RESPONSE"
fi

# 测试认证失败
echo ""
echo "5. 测试认证失败场景..."
AUTH_FAIL_RESPONSE=$(curl -s -X POST "${BASE_URL}/v1/messages" \
  -H "Content-Type: application/json" \
  -H "x-api-key: invalid-key" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "test",
    "max_tokens": 1024,
    "messages": [{"role": "user", "content": "Hello"}]
  }')

if echo "$AUTH_FAIL_RESPONSE" | jq -e '.error.type == "authentication_error"' > /dev/null 2>&1; then
    echo "   ✓ 认证失败处理正确"
else
    echo "   ✗ 认证失败处理异常"
    echo "   响应：$AUTH_FAIL_RESPONSE"
fi

echo ""
echo "================================"
echo "测试完成！"
echo "================================"
