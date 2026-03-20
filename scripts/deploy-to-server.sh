#!/bin/bash

# Oppama 服务器部署脚本
# 传输镜像到服务器并自动部署

set -e

# 配置变量
SERVER_IP="${1:-}"
SERVER_USER="${2:-root}"
SERVER_PORT="${3:-22}"
IMAGE_FILE="./oppama-docker.tar.gz"
REMOTE_DIR="/opt/oppama"

# 颜色定义
RED='\033[0;31m'
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

log_error() {
    echo -e "${RED}[✗]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[⚠]${NC} $1"
}

# 显示使用说明
show_usage() {
    echo "用法：$0 <服务器 IP> [用户名] [SSH 端口]"
    echo ""
    echo "示例："
    echo "  $0 192.168.1.100 root 22"
    echo "  $0 example.com admin 2222"
    echo ""
    exit 1
}

# 检查参数
if [ -z "$SERVER_IP" ]; then
    show_usage
fi

# 检查镜像文件
if [ ! -f "$IMAGE_FILE" ]; then
    log_error "镜像文件不存在：$IMAGE_FILE"
    log_info "请先运行：./scripts/package-docker-image.sh"
    exit 1
fi

echo ""
echo "╔════════════════════════════════════════════════════════════╗"
echo "║     Oppama 服务器部署工具                                 ║"
echo "╚════════════════════════════════════════════════════════════╝"
echo ""

log_info "服务器信息:"
echo "  IP 地址：   $SERVER_IP"
echo "  用户名：   $SERVER_USER"
echo "  SSH 端口：  $SERVER_PORT"
echo "  镜像文件： $IMAGE_FILE"
echo ""

# 交互式配置
echo "╔════════════════════════════════════════════════════════════╗"
echo "║                    部署配置                                ║"
echo "╚════════════════════════════════════════════════════════════╝"
echo ""

# 配置服务端口
read -p "请输入服务端口 (默认：9001): " SERVICE_PORT
SERVICE_PORT=${SERVICE_PORT:-9001}

# 配置 HTTPS
read -p "是否启用 HTTPS? (y/n, 默认：n): " ENABLE_HTTPS
ENABLE_HTTPS=${ENABLE_HTTPS:-n}

# 生成随机密码
ADMIN_PASSWORD=$(openssl rand -base64 12)
API_KEY=$(openssl rand -hex 16)
JWT_SECRET=$(openssl rand -base64 32)

echo ""
log_info "已生成安全凭证:"
echo "  API Key:     $API_KEY"
echo "  管理员密码： $ADMIN_PASSWORD"
echo "  JWT Secret:  ${JWT_SECRET:0:20}..."
echo ""

read -p "确认开始部署？(y/n): " CONFIRM
if [[ ! "$CONFIRM" =~ ^[Yy]$ ]]; then
    log_error "部署已取消"
    exit 0
fi

# 测试 SSH 连接
log_info "测试 SSH 连接..."
if ! ssh -p $SERVER_PORT -o ConnectTimeout=5 -o StrictHostKeyChecking=no $SERVER_USER@$SERVER_IP "echo '连接成功'" > /dev/null 2>&1; then
    log_error "无法连接到服务器，请检查："
    echo "  1. 服务器 IP 是否正确"
    echo "  2. SSH 端口是否正确"
    echo "  3. 防火墙是否开放 SSH 端口"
    echo "  4. SSH 密钥或密码是否正确"
    exit 1
fi
log_success "SSH 连接正常"
echo ""

# 上传镜像文件
log_info "正在上传 Docker 镜像到服务器..."
log_warning "这可能需要几分钟时间，请耐心等待..."

scp -P $SERVER_PORT "$IMAGE_FILE" $SERVER_USER@$SERVER_IP:$REMOTE_DIR/

if [ $? -eq 0 ]; then
    log_success "镜像文件已上传到服务器"
else
    log_error "上传失败"
    exit 1
fi
echo ""

# 创建配置文件
log_info "创建远程配置文件..."

# 根据 HTTPS 设置生成配置
if [[ "$ENABLE_HTTPS" =~ ^[Yy]$ ]]; then
    HTTPS_ENABLED="true"
else
    HTTPS_ENABLED="false"
fi

# 通过 SSH 创建配置文件
ssh -p $SERVER_PORT $SERVER_USER@$SERVER_IP << EOF
    # 创建目录
    mkdir -p $REMOTE_DIR/data
    mkdir -p $REMOTE_DIR/logs
    mkdir -p $REMOTE_DIR/certs
    
    # 创建配置文件
    cat > $REMOTE_DIR/config.yaml << 'CONFIGEOF'
server:
    host: 0.0.0.0
    port: ${SERVICE_PORT}
    mode: release
    enable_https: ${HTTPS_ENABLED}
    cert_file: "./certs/server.crt"
    key_file: "./certs/server.key"
    min_tls_version: "1.2"

auth:
    enabled: true
    jwt_secret: "${JWT_SECRET}"
    jwt_expire: 24h
    enable_blacklist: true
    blacklist_ttl: 24h

proxy:
    enabled: true
    enable_auth: true
    api_key: "${API_KEY}"
    fallback_enabled: true
    max_retries: 3
    timeout: 120

cors:
    allowed_origins:
      - "http://localhost:${SERVICE_PORT}"
      - "https://localhost:${SERVICE_PORT}"
CONFIGEOF

    # 如果需要 HTTPS，生成自签名证书
    if [ "${HTTPS_ENABLED}" = "true" ]; then
        openssl req -x509 -nodes -days 365 -newkey rsa:4096 \
          -keyout $REMOTE_DIR/certs/server.key \
          -out $REMOTE_DIR/certs/server.crt \
          -subj "/CN=localhost" \
          -addext "subjectAltName=IP:${SERVER_IP},DNS:localhost"
        chmod 600 $REMOTE_DIR/certs/server.key
        chmod 644 $REMOTE_DIR/certs/server.crt
    fi
EOF

log_success "远程配置文件已创建"
echo ""

# 加载 Docker 镜像
log_info "正在加载 Docker 镜像..."
ssh -p $SERVER_PORT $SERVER_USER@$SERVER_IP << EOF
    cd $REMOTE_DIR
    docker load -i oppama-docker.tar.gz
EOF

log_success "Docker 镜像已加载"
echo ""

# 创建 docker-compose.yml
log_info "创建 Docker Compose 配置..."
ssh -p $SERVER_PORT $SERVER_USER@$SERVER_IP << EOF
    cd $REMOTE_DIR
    
    # 备份旧的 docker-compose.yml
    [ -f docker-compose.yml ] && mv docker-compose.yml docker-compose.yml.bak
    
    # 创建新的 docker-compose.yml
    cat > docker-compose.yml << 'COMPOSEEOF'
services:
  oppama:
    image: oppama:latest
    container_name: oppama
    restart: unless-stopped
    ports:
      - "${SERVICE_PORT}:${SERVICE_PORT}"
    volumes:
      - ./data:/app/data
      - ./logs:/app/logs
      - ./config.yaml:/app/config.yaml
      - ./certs:/app/certs
    healthcheck:
      test: ["CMD", "wget", "--spider", "-q", "http://localhost:${SERVICE_PORT}/health"]
      interval: 30s
      timeout: 3s
      retries: 3
      start_period: 5s
COMPOSEEOF
EOF

log_success "Docker Compose 配置已创建"
echo ""

# 启动服务
log_info "正在启动服务..."
ssh -p $SERVER_PORT $SERVER_USER@$SERVER_IP << EOF
    cd $REMOTE_DIR
    docker-compose up -d
    sleep 5
    docker-compose ps
EOF

log_success "服务已启动"
echo ""

# 显示部署信息
echo "╔════════════════════════════════════════════════════════════╗"
echo "║         🎉 部署完成！                                     ║"
echo "╚════════════════════════════════════════════════════════════╝"
echo ""
echo "📋 部署信息："
echo ""
echo "  服务器：    $SERVER_IP"
echo "  端口：      $SERVICE_PORT"
echo "  HTTPS:     ${HTTPS_ENABLED}"
echo ""
echo "🔐 安全凭证（请妥善保管）："
echo ""
echo "  API Key:     $API_KEY"
echo "  管理员密码： $ADMIN_PASSWORD"
echo ""
echo "🌐 访问地址："
echo ""
if [[ "$ENABLE_HTTPS" =~ ^[Yy]$ ]]; then
    echo "  https://${SERVER_IP}:${SERVICE_PORT}/admin"
else
    echo "  http://${SERVER_IP}:${SERVICE_PORT}/admin"
fi
echo ""
echo "🔧 常用命令："
echo ""
echo "  ssh -p $SERVER_PORT $SERVER_USER@$SERVER_IP"
echo "  cd $REMOTE_DIR"
echo "  docker-compose ps          # 查看状态"
echo "  docker-compose logs -f     # 查看日志"
echo "  docker-compose restart     # 重启服务"
echo ""
echo "⚠️  重要提示："
echo ""
echo "  1. 请立即修改管理员密码"
echo "  2. 生产环境请使用正式 CA 证书"
echo "  3. 确保云服务器安全组开放 $SERVICE_PORT 端口"
echo "  4. 定期备份数据目录：$REMOTE_DIR/data"
echo ""
