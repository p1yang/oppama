#!/bin/bash

# Oppama 配置生成器
# 在服务器上生成交互式配置

set -e

echo "╔════════════════════════════════════════════════════════════╗"
echo "║     Oppama 配置生成器                                     ║"
echo "╚════════════════════════════════════════════════════════════╝"
echo ""

# 颜色定义
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
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

log_error() {
    echo -e "${RED}[✗]${NC} $1"
}

APP_DIR="/opt/oppama"

# 检查是否已安装 Docker
if ! command -v docker &> /dev/null; then
    log_error "Docker 未安装，请先安装 Docker"
    exit 1
fi

log_success "Docker 版本：$(docker --version)"
echo ""

# 交互式配置
echo "╔════════════════════════════════════════════════════════════╗"
echo "║                    配置向导                                ║"
echo "╚════════════════════════════════════════════════════════════╝"
echo ""

# 1. 服务端口
read -p "请输入服务端口 (默认：9001): " PORT
PORT=${PORT:-9001}
log_success "端口：$PORT"
echo ""

# 2. HTTPS 设置
echo "是否启用 HTTPS？"
echo "  1) 是 (推荐)"
echo "  2) 否"
echo ""
read -p "请选择 (1-2): " https_choice
case $https_choice in
    1|"")
        USE_HTTPS="true"
        log_success "已启用 HTTPS"
        ;;
    2)
        USE_HTTPS="false"
        log_warning "未启用 HTTPS"
        ;;
    *)
        log_error "无效选项"
        exit 1
        ;;
esac
echo ""

# 3. 域名（可选）
read -p "是否有域名？(y/n): " has_domain
DOMAIN=""
if [[ "$has_domain" =~ ^[Yy]$ ]]; then
    read -p "请输入域名： " domain
    DOMAIN="$domain"
    log_success "域名：$domain"
fi
echo ""

# 4. 生成安全凭证
log_info "正在生成安全凭证..."
API_KEY=$(openssl rand -hex 16)
JWT_SECRET=$(openssl rand -base64 32)
ADMIN_PASSWORD=$(openssl rand -base64 12)

echo ""
log_success "已生成安全凭证:"
echo "  API Key:     $API_KEY"
echo "  JWT Secret:  ${JWT_SECRET:0:20}..."
echo "  管理员密码： $ADMIN_PASSWORD"
echo ""

# 5. 确认配置
echo "╔════════════════════════════════════════════════════════════╗"
echo "║                    配置摘要                                ║"
echo "╚════════════════════════════════════════════════════════════╝"
echo ""
echo "  端口：      $PORT"
echo "  HTTPS:     $USE_HTTPS"
[ -n "$DOMAIN" ] && echo "  域名：     $DOMAIN"
echo ""

read -p "确认配置并生成？(y/n): " confirm
if [[ ! "$confirm" =~ ^[Yy]$ ]]; then
    log_error "配置已取消"
    exit 0
fi

# 创建目录
log_info "创建目录结构..."
mkdir -p $APP_DIR/{data,logs,certs}
log_success "目录已创建"
echo ""

# 生成配置文件
log_info "生成配置文件..."
cat > $APP_DIR/config.yaml << EOF
# Oppama 配置文件
# 生成时间：$(date '+%Y-%m-%d %H:%M:%S')

server:
    host: 0.0.0.0
    port: ${PORT}
    mode: release
    enable_https: ${USE_HTTPS}
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
      - "http://localhost:${PORT}"
      - "https://localhost:${PORT}"
EOF

if [ -n "$DOMAIN" ]; then
    cat >> $APP_DIR/config.yaml << EOF
      - "https://${DOMAIN}"
EOF
fi

log_success "配置文件已生成：$APP_DIR/config.yaml"
echo ""

# 生成 HTTPS 证书
if [ "$USE_HTTPS" = "true" ]; then
    log_info "正在生成 HTTPS 证书..."
    
    openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
      -keyout $APP_DIR/certs/server.key \
      -out $APP_DIR/certs/server.crt \
      -subj "/C=CN/ST=Beijing/L=Beijing/O=Oppama/CN=localhost"
    
    chmod 600 $APP_DIR/certs/server.key
    chmod 644 $APP_DIR/certs/server.crt
    
    log_success "HTTPS 证书已生成"
    echo ""
fi

# 生成 docker-compose.yml
log_info "生成 Docker Compose 配置..."
cat > $APP_DIR/docker-compose.yml << EOF
services:
  oppama:
    image: oppama:latest
    container_name: oppama
    restart: unless-stopped
    ports:
      - "${PORT}:${PORT}"
    volumes:
      - ./data:/app/data
      - ./logs:/app/logs
      - ./config.yaml:/app/config.yaml
      - ./certs:/app/certs
    healthcheck:
      test: ["CMD", "wget", "--spider", "-q", "http://localhost:${PORT}/health"]
      interval: 30s
      timeout: 3s
      retries: 3
      start_period: 5s
EOF

log_success "Docker Compose 配置已生成"
echo ""

# 显示完成信息
echo "╔════════════════════════════════════════════════════════════╗"
echo "║         🎉 配置完成！                                     ║"
echo "╚════════════════════════════════════════════════════════════╝"
echo ""
echo "📋 配置信息："
echo ""
echo "  应用目录：  $APP_DIR"
echo "  端口：      $PORT"
echo "  HTTPS:     $USE_HTTPS"
[ -n "$DOMAIN" ] && echo "  域名：     $DOMAIN"
echo ""
echo "🔐 安全凭证（请妥善保管）："
echo ""
echo "  API Key:     $API_KEY"
echo "  管理员密码： $ADMIN_PASSWORD"
echo ""
echo "🌐 访问地址："
echo ""
if [ "$USE_HTTPS" = "true" ]; then
    echo "  https://your-server-ip:${PORT}/admin"
else
    echo "  http://your-server-ip:${PORT}/admin"
fi
echo ""
echo "🔧 下一步操作："
echo ""
echo "  1. 加载 Docker 镜像："
echo "     cd $APP_DIR"
echo "     docker load -i oppama-docker.tar.gz"
echo ""
echo "  2. 启动服务："
echo "     docker-compose up -d"
echo ""
echo "  3. 查看状态："
echo "     docker-compose ps"
echo "     docker-compose logs -f"
echo ""
echo "⚠️  重要提示："
echo ""
echo "  1. 生产环境请使用正式 CA 证书"
echo "  2. 确保云服务器安全组开放 $PORT 端口"
echo "  3. 定期备份数据目录：$APP_DIR/data"
echo ""
