#!/bin/bash

# Oppama 快速启动脚本

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# 切换到项目根目录
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

echo "================================"
echo "Oppama 快速启动"
echo "================================"

# 检查配置文件
if [ ! -f "config.yaml" ]; then
    echo "生成配置文件..."
    cp deploy/config.example.yaml config.yaml
fi

if [ ! -f ".env" ]; then
    echo "生成环境变量配置..."
    cp .env.example .env
fi

# 创建必要的目录
mkdir -p data logs

# 检查是否已有编译好的二进制文件
if [ ! -f "bin/oppama" ]; then
    echo "编译项目..."
    make build
fi

echo ""
echo "启动服务..."
echo "API 地址：http://localhost:8080"
echo "管理界面：http://localhost:8080/admin"
echo ""

./bin/oppama -config config.yaml
