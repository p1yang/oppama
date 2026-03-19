#!/bin/bash

# Oppama Docker 镜像打包脚本
# 在本地构建 Docker 镜像并导出为 tar 包

set -e

echo "╔════════════════════════════════════════════════════════════╗"
echo "║     Oppama Docker 镜像打包工具                            ║"
echo "╚════════════════════════════════════════════════════════════╝"
echo ""

# 配置变量
IMAGE_NAME="oppama:latest"
OUTPUT_FILE="./oppama-docker.tar.gz"

# 颜色定义
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[✓]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[⚠]${NC} $1"
}

# 检查 Docker
if ! command -v docker &> /dev/null; then
    log_error "Docker 未安装，请先安装 Docker"
    exit 1
fi

log_info "Docker 版本：$(docker --version)"
echo ""

# 构建镜像
log_info "正在构建 Docker 镜像..."
log_warning "首次构建可能需要较长时间，请耐心等待..."
echo ""

docker build -t ${IMAGE_NAME} .

log_success "Docker 镜像构建完成"
echo ""

# 导出镜像
log_info "正在导出镜像到 ${OUTPUT_FILE}..."
log_warning "导出过程可能需要几分钟..."
echo ""

docker save ${IMAGE_NAME} | gzip > ${OUTPUT_FILE}

# 显示文件大小
FILE_SIZE=$(du -h ${OUTPUT_FILE} | cut -f1)
log_success "镜像已导出：${OUTPUT_FILE} (大小：${FILE_SIZE})"
echo ""

# 显示使用说明
echo "╔════════════════════════════════════════════════════════════╗"
echo "║                    下一步操作                              ║"
echo "╚════════════════════════════════════════════════════════════╝"
echo ""
echo "1. 传输到服务器："
echo "   scp ${OUTPUT_FILE} user@your-server:/opt/oppama/"
echo ""
echo "2. 在服务器上加载镜像："
echo "   ssh user@your-server"
echo "   cd /opt/oppama"
echo "   docker load -i ${OUTPUT_FILE}"
echo ""
echo "3. 运行容器："
echo "   docker-compose up -d"
echo ""
echo "或者使用提供的自动化脚本："
echo "   ./scripts/deploy-to-server.sh <server_ip> [user] [port]"
echo ""
