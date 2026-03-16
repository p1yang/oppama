#!/bin/bash

# Oppama 测试脚本

set -e

echo "================================"
echo "Oppama 功能测试"
echo "================================"

BASE_URL="http://localhost:8080"
API_URL="${BASE_URL}/v1/api"

# 测试健康检查
echo ""
echo "1. 测试健康检查..."
curl -s "${BASE_URL}/health" | jq .

# 测试添加服务
echo ""
echo "2. 测试添加本地 Ollama 服务..."
curl -s -X POST "${API_URL}/services" \
  -H "Content-Type: application/json" \
  -d '{"url": "http://localhost:11434", "name": "本地测试"}' | jq .

# 测试获取服务列表
echo ""
echo "3. 测试获取服务列表..."
curl -s "${API_URL}/services" | jq '.data | length'

# 测试获取模型列表
echo ""
echo "4. 测试获取模型列表..."
curl -s "${API_URL}/models" | jq .

# 测试 OpenAI 兼容接口
echo ""
echo "5. 测试 OpenAI 兼容接口..."
curl -s -X POST "${BASE_URL}/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "test",
    "messages": [{"role": "user", "content": "Hello"}]
  }' | jq .

echo ""
echo "================================"
echo "测试完成！"
echo "================================"
