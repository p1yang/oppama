#!/bin/bash

# Oppama 全能部署脚本
# 适用于 Ubuntu/Debian 系统
# 支持 Docker 和二进制两种部署方式，提供交互式配置

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# 全局配置变量
declare -A CONFIG
CONFIG[DEPLOY_MODE]=""
CONFIG[PORT]="9001"
CONFIG[HOST]="0.0.0.0"
CONFIG[JWT_SECRET]=""
CONFIG[API_KEY]=""
CONFIG[ADMIN_PASSWORD]=""
CONFIG[USE_HTTPS]="true"
CONFIG[DOMAIN]=""

# 日志函数
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

log_step() {
    echo -e "\n${CYAN}═══════════════════════════════════════════════════════════${NC}"
    echo -e "${CYAN}  $1${NC}"
    echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}\n"
}

# 检查是否以 root 运行
check_root() {
    if [ "$EUID" -ne 0 ]; then
        log_error "请使用 sudo 运行此脚本"
        exit 1
    fi
}

# 检查操作系统
check_os() {
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        OS=$NAME
        VER=$VERSION_ID
        log_success "检测到操作系统：$OS $VER"
    else
        log_error "无法检测操作系统版本"
        exit 1
    fi
}

# 显示欢迎信息
show_welcome() {
    echo ""
    echo -e "${GREEN}╔════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║     Oppama 全能部署向导                                    ║${NC}"
    echo -e "${GREEN}╚════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    log_info "本脚本将引导您完成 Oppama 的部署配置"
    log_info "支持 Docker 和二进制两种部署方式"
    echo ""
}

# 选择部署模式
select_deploy_mode() {
    log_step "步骤 1: 选择部署方式"
    
    echo "请选择部署方式："
    echo "  1) Docker 部署 (推荐 - 隔离性好，易管理)"
    echo "  2) 二进制部署 (简单，资源占用少)"
    echo ""
    
    while true; do
        read -p "请输入选项 (1-2): " choice
        case $choice in
            1)
                CONFIG[DEPLOY_MODE]="docker"
                log_success "已选择：Docker 部署"
                break
                ;;
            2)
                CONFIG[DEPLOY_MODE]="binary"
                log_success "已选择：二进制部署"
                break
                ;;
            *)
                log_error "无效选项，请重新输入"
                ;;
        esac
    done
}

# 配置服务端口
configure_port() {
    log_step "步骤 2: 配置服务端口"
    
    echo "默认端口：9001"
    echo ""
    read -p "请输入服务端口 (直接回车使用默认值): " port
    if [ -n "$port" ]; then
        CONFIG[PORT]="$port"
        log_success "端口已配置：$port"
    else
        log_success "使用默认端口：${CONFIG[PORT]}"
    fi
}

# 配置 HTTPS
configure_https() {
    log_step "步骤 3: 配置 HTTPS"
    
    echo "是否启用 HTTPS？(推荐启用)"
    echo "  1) 是 (默认)"
    echo "  2) 否"
    echo ""
    
    while true; do
        read -p "请选择 (1-2): " choice
        case $choice in
            1|"")
                CONFIG[USE_HTTPS]="true"
                log_success "已启用 HTTPS"
                break
                ;;
            2)
                CONFIG[USE_HTTPS]="false"
                log_warning "未启用 HTTPS (生产环境建议启用)"
                break
                ;;
            *)
                log_error "无效选项"
                ;;
        esac
    done
    
    # 配置域名（可选）
    echo ""
    read -p "是否有域名？(y/n): " has_domain
    if [[ "$has_domain" =~ ^[Yy]$ ]]; then
        read -p "请输入域名 (如：example.com): " domain
        CONFIG[DOMAIN]="$domain"
        log_success "域名已配置：$domain"
    fi
}

# 生成安全密钥
generate_secrets() {
    log_step "步骤 4: 生成安全密钥"
    
    # JWT Secret
    log_info "正在生成 JWT Secret..."
    CONFIG[JWT_SECRET]=$(openssl rand -base64 32)
    log_success "JWT Secret 已生成"
    
    # API Key
    log_info "正在生成 API Key..."
    CONFIG[API_KEY]=$(openssl rand -hex 16)
    log_success "API Key 已生成"
    
    # 管理员密码
    echo ""
    log_info "设置管理员密码："
    read -sp "请输入密码 (直接回车使用随机密码): " admin_pwd
    echo ""
    if [ -n "$admin_pwd" ]; then
        CONFIG[ADMIN_PASSWORD]="$admin_pwd"
        log_success "管理员密码已设置"
    else
        CONFIG[ADMIN_PASSWORD]=$(openssl rand -base64 12)
        log_success "已生成随机管理员密码：${CONFIG[ADMIN_PASSWORD]}"
    fi
}

# 确认配置
confirm_config() {
    log_step "步骤 5: 确认配置"
    
    echo "╔════════════════════════════════════════════════════════════╗"
    echo "║                    配置摘要                                ║"
    echo "╚════════════════════════════════════════════════════════════╝"
    echo ""
    echo "部署方式：    ${CONFIG[DEPLOY_MODE]}"
    echo "服务端口：    ${CONFIG[PORT]}"
    echo "HTTPS:       ${CONFIG[USE_HTTPS]}"
    [ -n "${CONFIG[DOMAIN]}" ] && echo "域名：        ${CONFIG[DOMAIN]}"
    echo ""
    echo "安全凭证:"
    echo "  JWT Secret: ${CONFIG[JWT_SECRET]:0:20}... (已隐藏)"
    echo "  API Key:    ${CONFIG[API_KEY]}"
    echo "  管理员密码：${CONFIG[ADMIN_PASSWORD]}"
    echo ""
    
    read -p "确认以上配置并开始部署？(y/n): " confirm
    if [[ ! "$confirm" =~ ^[Yy]$ ]]; then
        log_error "部署已取消"
        exit 0
    fi
    
    log_success "配置已确认，开始部署..."
}

# 安装 Docker
install_docker() {
    log_info "检查 Docker..."
    
    if command -v docker &> /dev/null; then
        DOCKER_VERSION=$(docker --version)
        log_success "Docker 已安装：$DOCKER_VERSION"
        
        if systemctl is-active --quiet docker; then
            log_success "Docker 服务正在运行"
        else
            log_warning "Docker 已安装但未运行，正在启动..."
            systemctl start docker
            systemctl enable docker
            log_success "Docker 服务已启动"
        fi
        return 0
    fi
    
    log_info "正在安装 Docker..."
    apt-get update
    apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin
    systemctl start docker
    systemctl enable docker
    log_success "Docker 安装完成"
}

# 安装 Docker Compose
install_docker_compose() {
    log_info "检查 Docker Compose..."
    
    if command -v docker-compose &> /dev/null || command -v docker compose &> /dev/null; then
        if command -v docker-compose &> /dev/null; then
            COMPOSE_VERSION=$(docker-compose --version 2>&1 | head -n1)
        else
            COMPOSE_VERSION=$(docker compose version 2>&1 | head -n1)
        fi
        log_success "Docker Compose 已安装：$COMPOSE_VERSION"
        return 0
    fi
    
    log_info "正在安装 Docker Compose..."
    COMPOSE_VERSION=$(curl -s https://api.github.com/repos/docker/compose/releases/latest | grep 'tag_name' | awk '{print substr($2, 3, length($2)-4)}')
    curl -L "https://github.com/docker/compose/releases/download/v${COMPOSE_VERSION}/docker-compose-$(uname -s)-$(uname -m)" \
      -o /usr/local/bin/docker-compose
    chmod +x /usr/local/bin/docker-compose
    ln -sf /usr/local/bin/docker-compose /usr/bin/docker-compose
    log_success "Docker Compose 安装完成 (v${COMPOSE_VERSION})"
}

# 安装基础依赖（二进制部署）
install_dependencies() {
    log_info "正在安装基础依赖..."
    apt-get update
    apt-get install -y wget curl openssl jq
    log_success "基础依赖安装完成"
}

# 配置防火墙
configure_firewall() {
    log_info "正在配置防火墙..."
    
    if command -v ufw &> /dev/null; then
        ufw allow 22/tcp comment 'SSH'
        ufw allow ${CONFIG[PORT]}/tcp comment 'Oppama Service'
        
        if ! ufw status | grep -q "Status: active"; then
            log_warning "UFW 未启用，建议手动启用防火墙"
        else
            ufw reload
        fi
        
        log_success "防火墙规则已配置"
    else
        log_warning "未检测到 UFW，请手动配置防火墙"
    fi
}

# Docker 部署
deploy_docker() {
    log_step "Docker 部署"
    
    APP_DIR="/opt/oppama"
    mkdir -p $APP_DIR
    
    # 复制项目文件
    log_info "准备项目文件..."
    if [ -d "./web/dist" ]; then
        cp -r ./web/dist $APP_DIR/
        log_success "前端文件已复制"
    fi
    
    # 创建配置文件
    log_info "创建配置文件..."
    cat > $APP_DIR/config.yaml << EOF
server:
    host: ${CONFIG[HOST]}
    port: ${CONFIG[PORT]}
    mode: release
    enable_https: ${CONFIG[USE_HTTPS]}
    cert_file: "./certs/server.crt"
    key_file: "./certs/server.key"
    min_tls_version: "1.2"

auth:
    enabled: true
    jwt_secret: "${CONFIG[JWT_SECRET]}"
    jwt_expire: 24h
    enable_blacklist: true
    blacklist_ttl: 24h

proxy:
    enabled: true
    enable_auth: true
    api_key: "${CONFIG[API_KEY]}"
    fallback_enabled: true
    max_retries: 3
    timeout: 120

cors:
    allowed_origins:
      - "http://localhost:${CONFIG[PORT]}"
      - "https://localhost:${CONFIG[PORT]}"
EOF
    
    if [ -n "${CONFIG[DOMAIN]}" ]; then
        cat >> $APP_DIR/config.yaml << EOF
      - "https://${CONFIG[DOMAIN]}"
EOF
    fi
    
    log_success "配置文件已创建"
    
    # 创建 docker-compose.yml
    log_info "创建 Docker Compose 配置..."
    cat > $APP_DIR/docker-compose.yml << EOF
services:
  oppama:
    build:
      context: .
      dockerfile: Dockerfile
    container_name: oppama
    restart: unless-stopped
    ports:
      - "${CONFIG[PORT]}:${CONFIG[PORT]}"
    volumes:
      - ./data:/app/data
      - ./logs:/app/logs
      - ./config.yaml:/app/config.yaml
      - ./certs:/app/certs
    healthcheck:
      test: ["CMD", "wget", "--spider", "-q", "http://localhost:${CONFIG[PORT]}/health"]
      interval: 30s
      timeout: 3s
      retries: 3
      start_period: 5s
EOF
    
    log_success "Docker Compose 配置已创建"
    
    # 创建 systemd 服务
    cat > /etc/systemd/system/oppama.service << EOF
[Unit]
Description=Oppama Server (Docker)
After=network.target docker.service
Requires=docker.service

[Service]
Type=simple
Restart=always
RestartSec=5
WorkingDirectory=$APP_DIR
ExecStart=/usr/bin/docker-compose up -d
ExecStop=/usr/bin/docker-compose down
TimeoutStartSec=0

[Install]
WantedBy=multi-user.target
EOF
    
    systemctl daemon-reload
    systemctl enable oppama
    
    log_success "Systemd 服务已创建"
    
    # 启动服务
    cd $APP_DIR
    log_info "正在启动服务..."
    docker-compose up -d
    log_success "服务已启动"
}

# 二进制部署
deploy_binary() {
    log_step "二进制部署"
    
    APP_DIR="/opt/oppama"
    mkdir -p $APP_DIR/{data,logs,certs,bin}
    
    # 编译或复制二进制文件
    log_info "准备二进制文件..."
    if [ -f "./oppama" ]; then
        cp ./oppama $APP_DIR/bin/
        chmod +x $APP_DIR/bin/oppama
        log_success "二进制文件已复制"
    elif command -v go &> /dev/null; then
        log_info "正在编译二进制文件..."
        cd /opt/oppama
        CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/oppama ./cmd/server/main.go
        log_success "二进制文件已编译"
    else
        log_error "未找到二进制文件且未安装 Go，无法继续"
        exit 1
    fi
    
    # 创建配置文件
    log_info "创建配置文件..."
    cat > $APP_DIR/config.yaml << EOF
server:
    host: ${CONFIG[HOST]}
    port: ${CONFIG[PORT]}
    mode: release
    enable_https: ${CONFIG[USE_HTTPS]}
    cert_file: "./certs/server.crt"
    key_file: "./certs/server.key"
    min_tls_version: "1.2"

auth:
    enabled: true
    jwt_secret: "${CONFIG[JWT_SECRET]}"
    jwt_expire: 24h
    enable_blacklist: true
    blacklist_ttl: 24h

proxy:
    enabled: true
    enable_auth: true
    api_key: "${CONFIG[API_KEY]}"
    fallback_enabled: true
    max_retries: 3
    timeout: 120

cors:
    allowed_origins:
      - "http://localhost:${CONFIG[PORT]}"
      - "https://localhost:${CONFIG[PORT]}"
EOF
    
    if [ -n "${CONFIG[DOMAIN]}" ]; then
        cat >> $APP_DIR/config.yaml << EOF
      - "https://${CONFIG[DOMAIN]}"
EOF
    fi
    
    log_success "配置文件已创建"
    
    # 生成证书
    if [ "${CONFIG[USE_HTTPS]}" = "true" ]; then
        log_info "正在生成 HTTPS 证书..."
        openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
          -keyout $APP_DIR/certs/server.key \
          -out $APP_DIR/certs/server.crt \
          -subj "/C=CN/ST=Beijing/L=Beijing/O=Oppama/CN=localhost"
        chmod 600 $APP_DIR/certs/server.key
        chmod 644 $APP_DIR/certs/server.crt
        log_success "HTTPS 证书已生成"
    fi
    
    # 创建 systemd 服务
    cat > /etc/systemd/system/oppama.service << EOF
[Unit]
Description=Oppama Server
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=$APP_DIR
ExecStart=$APP_DIR/bin/oppama -config $APP_DIR/config.yaml
Restart=always
RestartSec=5
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF
    
    systemctl daemon-reload
    systemctl enable oppama
    
    log_success "Systemd 服务已创建"
    
    # 启动服务
    log_info "正在启动服务..."
    systemctl start oppama
    log_success "服务已启动"
}

# 显示部署完成信息
show_complete() {
    log_step "部署完成"
    
    echo -e "${GREEN}╔════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║         🎉 Oppama 部署成功！                               ║${NC}"
    echo -e "${GREEN}╚════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    echo "📋 部署信息摘要："
    echo ""
    echo "  部署方式：   ${CONFIG[DEPLOY_MODE]}"
    echo "  服务端口：   ${CONFIG[PORT]}"
    echo "  HTTPS:      ${CONFIG[USE_HTTPS]}"
    [ -n "${CONFIG[DOMAIN]}" ] && echo "  域名：       ${CONFIG[DOMAIN]}"
    echo ""
    echo "🔐 安全凭证："
    echo ""
    echo "  API Key:     ${CONFIG[API_KEY]}"
    echo "  管理员密码： ${CONFIG[ADMIN_PASSWORD]}"
    echo ""
    echo "🌐 访问地址："
    echo ""
    if [ "${CONFIG[USE_HTTPS]}" = "true" ]; then
        echo "  管理界面：   https://your-server-ip:${CONFIG[PORT]}/admin"
    else
        echo "  管理界面：   http://your-server-ip:${CONFIG[PORT]}/admin"
    fi
    echo ""
    echo "🔧 常用命令："
    echo ""
    if [ "${CONFIG[DEPLOY_MODE]}" = "docker" ]; then
        echo "  查看状态：   docker-compose ps"
        echo "  查看日志：   docker-compose logs -f"
        echo "  重启服务：   docker-compose restart"
        echo "  停止服务：   docker-compose down"
    else
        echo "  查看状态：   systemctl status oppama"
        echo "  查看日志：   journalctl -u oppama -f"
        echo "  重启服务：   systemctl restart oppama"
        echo "  停止服务：   systemctl stop oppama"
    fi
    echo ""
    echo "⚠️  重要提示："
    echo ""
    echo "  1. 请立即修改管理员密码"
    echo "  2. 生产环境请使用正式 CA 证书"
    echo "  3. 确保云服务器安全组开放 ${CONFIG[PORT]} 端口"
    echo "  4. 定期备份数据目录"
    echo ""
    echo "📖 详细文档：docs/DEPLOYMENT.md"
    echo ""
}

# 主函数
main() {
    show_welcome
    select_deploy_mode
    configure_port
    configure_https
    generate_secrets
    confirm_config
    
    # 执行部署
    if [ "${CONFIG[DEPLOY_MODE]}" = "docker" ]; then
        install_docker
        install_docker_compose
    else
        install_dependencies
    fi
    
    configure_firewall
    
    if [ "${CONFIG[DEPLOY_MODE]}" = "docker" ]; then
        deploy_docker
    else
        deploy_binary
    fi
    
    show_complete
}

# 执行主函数
main
