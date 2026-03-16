#!/bin/bash

# 测试一键检测时的列表刷新问题

echo "======================================"
echo "测试一键检测超时问题修复"
echo "======================================"
echo ""

# 获取 Token（需要先登录）
echo "步骤 1: 登录获取 Token..."
TOKEN=$(curl -s -X POST http://localhost:8080/v1/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin"}' | jq -r '.data.token')

if [ -z "$TOKEN" ] || [ "$TOKEN" = "null" ]; then
    echo "❌ 登录失败，请检查用户名和密码"
    exit 1
fi

echo "✅ 登录成功，Token: ${TOKEN:0:20}..."
echo ""

# 获取服务数量
echo "步骤 2: 获取当前服务数量..."
SERVICE_COUNT=$(curl -s http://localhost:8080/v1/api/services \
  -H "Authorization: Bearer $TOKEN" | jq '.total')

echo "当前服务数量：$SERVICE_COUNT"
echo ""

if [ "$SERVICE_COUNT" -eq 0 ]; then
    echo "⚠️  没有服务，请先添加一些服务再测试"
    echo "   访问：http://localhost:8080/admin/services"
    exit 0
fi

# 启动一键检测
echo "步骤 3: 启动一键检测..."
CHECK_RESPONSE=$(curl -s -X POST http://localhost:8080/v1/api/services/check-all \
  -H "Authorization: Bearer $TOKEN")

TASK_ID=$(echo $CHECK_RESPONSE | jq -r '.data.task_id')
MESSAGE=$(echo $CHECK_RESPONSE | jq -r '.data.message')

echo "响应：$MESSAGE"
echo "任务 ID: $TASK_ID"
echo ""

# 等待 1 秒让检测开始
sleep 1

# 多次刷新列表测试
echo "步骤 4: 测试列表刷新（连续 5 次）..."
for i in {1..5}; do
    echo ""
    echo "第 $i 次刷新..."
    START_TIME=$(date +%s%3N)
    
    HTTP_CODE=$(curl -s -w "%{http_code}" -o /tmp/response_$i.json \
      http://localhost:8080/v1/api/services \
      -H "Authorization: Bearer $TOKEN")
    
    END_TIME=$(date +%s%3N)
    DURATION=$((END_TIME - START_TIME))
    
    if [ "$HTTP_CODE" -eq 200 ]; then
        TOTAL=$(cat /tmp/response_$i.json | jq '.total')
        echo "  ✅ HTTP $HTTP_CODE - 服务总数：$TOTAL - 耗时：${DURATION}ms"
        
        if [ "$DURATION" -gt 10000 ]; then
            echo "  ⚠️  警告：响应时间超过 10 秒"
        fi
    else
        ERROR=$(cat /tmp/response_$i.json | jq -r '.error // "Unknown error"')
        echo "  ❌ HTTP $HTTP_CODE - 错误：$ERROR - 耗时：${DURATION}ms"
    fi
    
    # 等待 2 秒再次刷新
    sleep 2
done

echo ""
echo "步骤 5: 检查检测任务状态..."
TASK_STATUS=$(curl -s http://localhost:8080/v1/api/services/tasks/$TASK_ID \
  -H "Authorization: Bearer $TOKEN" | jq -r '.data.status')

TASK_PROGRESS=$(curl -s http://localhost:8080/v1/api/services/tasks/$TASK_ID \
  -H "Authorization: Bearer $TOKEN" | jq -r '.data.progress')

echo "任务状态：$TASK_STATUS"
echo "进度：$TASK_PROGRESS"

echo ""
echo "======================================"
echo "测试完成！"
echo "======================================"
echo ""
echo "预期结果:"
echo "  ✅ 所有刷新请求都应该在 10 秒内返回"
echo "  ✅ HTTP 状态码应该是 200"
echo "  ✅ 不应该出现 timeout 错误"
echo ""
echo "如果看到 timeout of 30000ms exceeded 错误，说明修复未生效"
