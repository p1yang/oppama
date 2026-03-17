#!/bin/bash

# 测试时间间隔设置功能

echo "======================================"
echo "测试 Oppama 定时任务间隔设置功能"
echo "======================================"
echo ""

# 启动服务器（后台运行）
echo "1. 启动 Oppama 服务器..."
./oppama -config config.yaml &
SERVER_PID=$!
sleep 3

# 检查服务器是否启动成功
if ! kill -0 $SERVER_PID 2>/dev/null; then
    echo "❌ 服务器启动失败"
    exit 1
fi

echo "✅ 服务器已启动 (PID: $SERVER_PID)"
echo ""

# 获取当前的时间间隔设置
echo "2. 获取当前的时间间隔设置..."
curl -s -X GET "http://localhost:8080/v1/api/proxy/config" \
  -H "Content-Type: application/json" | jq '.data.detector'

echo ""
echo ""

# 更新时间间隔
echo "3. 更新健康检查和模型同步间隔..."
curl -s -X PUT "http://localhost:8080/v1/api/proxy/config" \
  -H "Content-Type: application/json" \
  -d '{
    "detector": {
      "health_check_interval": 3,
      "model_sync_interval": 7
    }
  }' | jq '.'

echo ""
echo ""

# 等待一下让设置生效
sleep 2

# 再次获取配置，验证更新
echo "4. 验证更新后的时间间隔设置..."
curl -s -X GET "http://localhost:8080/v1/api/proxy/config" \
  -H "Content-Type: application/json" | jq '.data.detector'

echo ""
echo ""

# 停止服务器
echo "5. 停止服务器..."
kill $SERVER_PID 2>/dev/null
wait $SERVER_PID 2>/dev/null

echo "✅ 测试完成!"
echo ""
echo "说明："
echo "- health_check_interval: 健康检查间隔（分钟），默认 5 分钟"
echo "- model_sync_interval: 模型同步间隔（分钟），默认 10 分钟"
echo ""
echo "可以在前端设置页面的「检测器配置」标签页中设置这两个参数"
