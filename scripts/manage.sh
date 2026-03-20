#!/bin/bash

# Oppama 统一安装/配置/更新管理脚本
# 支持：全新安装、配置更新、版本升级、备份恢复
# 纯交互式界面，无需记忆参数

set -e

# ==================== 配置变量 ====================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
INSTALL_DIR="/opt/oppama"
DATA_DIR="$INSTALL_DIR/data"
LOGS_DIR="$INSTALL_DIR/logs"
CERTS_DIR="$INSTALL_DIR/certs"
CONFIG_FILE="$INSTALL_DIR/config.yaml"
BACKUP_DIR="$INSTALL_DIR/backups"
IMAGE_FILE="$PROJECT_ROOT/oppama-docker.tar.gz"

# 默认配置
DEFAULT_PORT=9001
DEFAULT_HTTPS="n"
DEFAULT_USER="root"
DEFAULT_SSH_PORT=22

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

# ==================== 日志函数 ====================
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[✓]${NC} $1"
}

log_error() {
    echo -e "${RED}[✗]${NC} $1" >&2
}

log_warning() {
    echo -e "${YELLOW}[⚠]${NC} $1"
}

log_step() {
    echo -e "\n${CYAN}${BOLD}>>> $1${NC}"
}

# ==================== 帮助信息 ====================
show_menu() {
    echo -e "\n${BOLD}╔══════════════════════════════════════════════════╗${NC}"
    echo -e "${BOLD}║     Oppama 统一安装/配置/更新管理工具            ║${NC}"
    echo -e "${BOLD}╚══════════════════════════════════════════════════╝${NC}\n"
    
    echo -e "${CYAN}请选择操作：${NC}\n"
    echo "  1) 安装（本地）"
    echo "  2) 安装（远程服务器）"
    echo "  3) 更新（本地）"
    echo "  4) 更新（远程服务器）"
    echo "  5) 更新配置"
    echo "  6) 备份数据"
    echo "  7) 从备份恢复"
    echo "  8) 查看状态"
    echo "  9) 查看日志"
    echo " 10) 重启服务"
    echo " 11) 卸载服务"
    echo "  0) 退出"
    echo ""
}

show_help() {
    cat << EOF
${BOLD}Oppama 统一安装/配置/更新管理工具${NC}

这是一个纯交互式管理工具，所有操作都通过菜单引导完成。

${BOLD}主要功能:${NC}
  - 本地/远程安装部署
  - 版本自动更新
  - 配置文件管理
  - 数据备份恢复
  - 运行状态监控

${BOLD}使用说明:${NC}
  直接运行脚本即可进入交互菜单:
  $ ./scripts/manage.sh

EOF
}

# ==================== 工具函数 ====================
check_docker() {
    if ! command -v docker &> /dev/null; then
        log_error "Docker 未安装"
        return 1
    fi
    
    if ! docker info &> /dev/null; then
        log_error "Docker 未运行"
        return 1
    fi
    
    log_success "Docker 环境正常"
    return 0
}

check_docker_compose() {
    if command -v docker-compose &> /dev/null; then
        DOCKER_COMPOSE_CMD="docker-compose"
    elif docker compose version &> /dev/null; then
        DOCKER_COMPOSE_CMD="docker compose"
    else
        log_error "docker-compose 未安装"
        return 1
    fi
    
    log_success "docker-compose 可用 ($DOCKER_COMPOSE_CMD)"
    return 0
}

# 检测远程服务器的 docker-compose 命令
get_remote_docker_compose_cmd() {
    local server_ip=$1
    local server_user=$2
    local ssh_port=$3
    
    # 尝试检测远程的 docker-compose 命令
    local remote_cmd
    remote_cmd=$(ssh -p "$ssh_port" "$server_user@$server_ip" \
        "if command -v docker-compose &> /dev/null; then echo 'docker-compose'; elif docker compose version &> /dev/null; then echo 'docker compose'; else echo ''; fi" 2>/dev/null)
    
    if [ -n "$remote_cmd" ]; then
        echo "$remote_cmd"
        return 0
    else
        return 1
    fi
}

generate_password() {
    openssl rand -base64 12
}

generate_api_key() {
    openssl rand -hex 16
}

generate_jwt_secret() {
    openssl rand -base64 32
}

# ==================== SSH 执行函数 ====================
ssh_exec() {
    local server_ip=$1
    local server_user=$2
    local ssh_port=$3
    shift 3
    
    if [ "$LOCAL_MODE" = true ]; then
        eval "$@"
    else
        ssh -p "$ssh_port" -o StrictHostKeyChecking=no "$server_user@$server_ip" "$@"
    fi
}

ssh_upload() {
    local server_ip=$1
    local server_user=$2
    local ssh_port=$3
    local src=$4
    local dest=$5
    
    if [ "$LOCAL_MODE" = true ]; then
        cp "$src" "$dest"
    else
        scp -P "$ssh_port" "$src" "$server_user@$server_ip:$dest"
    fi
}

# ==================== 配置生成 ====================
generate_config() {
    local port=$1
    local https_enabled=$2
    local api_key=$3
    local jwt_secret=$4
    
    cat > "$CONFIG_FILE" << CONFIGEOF
server:
    host: 0.0.0.0
    port: ${port}
    mode: release
    enable_https: ${https_enabled}
    cert_file: "./certs/server.crt"
    key_file: "./certs/server.key"
    min_tls_version: "1.2"

auth:
    enabled: true
    jwt_secret: "${jwt_secret}"
    jwt_expire: 24h
    enable_blacklist: true
    blacklist_ttl: 24h

proxy:
    enabled: true
    enable_auth: true
    api_key: "${api_key}"
    fallback_enabled: true
    max_retries: 3
    timeout: 120

cors:
    allowed_origins:
      - "http://localhost:${port}"
      - "https://localhost:${port}"
CONFIGEOF
}

generate_docker_compose() {
    local port=$1
    
    cat > "$INSTALL_DIR/docker-compose.yml" << COMPOSEEOF
services:
  oppama:
    image: oppama:latest
    container_name: oppama
    restart: unless-stopped
    ports:
      - "${port}:${port}"
    volumes:
      - ./data:/app/data
      - ./logs:/app/logs
      - ./config.yaml:/app/config.yaml
      - ./certs:/app/certs
    healthcheck:
      test: ["CMD", "wget", "--spider", "-q", "http://localhost:${port}/health"]
      interval: 30s
      timeout: 3s
      retries: 3
      start_period: 5s
COMPOSEEOF
}

generate_ssl_cert() {
    local server_ip=$1
    
    log_info "生成自签名 SSL 证书..."
    
    openssl req -x509 -nodes -days 365 -newkey rsa:4096 \
      -keyout "$CERTS_DIR/server.key" \
      -out "$CERTS_DIR/server.crt" \
      -subj "/CN=localhost" \
      -addext "subjectAltName=IP:${server_ip},DNS:localhost" 2>/dev/null
    
    chmod 600 "$CERTS_DIR/server.key"
    chmod 644 "$CERTS_DIR/server.crt"
    
    log_success "SSL 证书已生成"
}

# ==================== 备份恢复 ====================
do_backup() {
    log_step "备份数据和配置"
    
    # 检查安装目录是否存在
    if [ "$LOCAL_MODE" = false ]; then
        # 远程模式：先检查服务器是否有安装目录
        if ! ssh_exec "$SERVER_IP" "$SERVER_USER" "$SSH_PORT" \
            "[ -d $INSTALL_DIR ] && echo 'exists'" 2>/dev/null | grep -q 'exists'; then
            log_error "远程服务器未检测到安装目录"
            exit 1
        fi
        log_success "远程服务器连接正常"
    fi
    
    local backup_name="oppama-backup-$(date +%Y%m%d-%H%M%S)"
    local backup_path="$BACKUP_DIR/$backup_name"
    
    # 备份数据目录
    if [ -d "$DATA_DIR" ]; then
        cp -r "$DATA_DIR" "$backup_path/"
        log_success "数据已备份"
    fi
    
    # 备份配置文件
    if [ -f "$CONFIG_FILE" ]; then
        cp "$CONFIG_FILE" "$backup_path/"
        log_success "配置文件已备份"
    fi
    
    # 备份证书
    if [ -d "$CERTS_DIR" ]; then
        cp -r "$CERTS_DIR" "$backup_path/"
        log_success "证书已备份"
    fi
    
    log_success "备份完成：$backup_path"
}

do_restore() {
    log_step "从备份恢复"
    
    local backup_path=$1
    
    if [ ! -d "$backup_path" ]; then
        log_error "备份不存在：$backup_path"
        exit 1
    fi
    
    # 停止服务
    log_info "停止服务..."
    ssh_exec "$SERVER_IP" "$SERVER_USER" "$SSH_PORT" \
        "cd $INSTALL_DIR && $DOCKER_COMPOSE_CMD down"
    
    # 恢复数据
    if [ -d "$backup_path/data" ]; then
        rm -rf "$DATA_DIR"
        cp -r "$backup_path/data" "$DATA_DIR"
        log_success "数据已恢复"
    fi
    
    # 恢复配置
    if [ -f "$backup_path/config.yaml" ]; then
        cp "$backup_path/config.yaml" "$CONFIG_FILE"
        log_success "配置已恢复"
    fi
    
    # 恢复证书
    if [ -d "$backup_path/certs" ]; then
        rm -rf "$CERTS_DIR"
        cp -r "$backup_path/certs" "$CERTS_DIR"
        log_success "证书已恢复"
    fi
    
    # 启动服务
    log_info "启动服务..."
    ssh_exec "$SERVER_IP" "$SERVER_USER" "$SSH_PORT" \
        "cd $INSTALL_DIR && $DOCKER_COMPOSE_CMD up -d"
    
    log_success "恢复完成"
}

# ==================== 安装流程 ====================
do_install() {
    log_step "开始安装 Oppama"
    
    # 远程模式下先检测环境
    if [ "$LOCAL_MODE" = false ]; then
        log_info "检测远程服务器环境..."
        REMOTE_DOCKER_COMPOSE_CMD=$(get_remote_docker_compose_cmd "$SERVER_IP" "$SERVER_USER" "$SSH_PORT")
        if [ -z "$REMOTE_DOCKER_COMPOSE_CMD" ]; then
            log_error "远程服务器未检测到 docker-compose 命令"
            exit 1
        fi
        log_success "远程 docker-compose 可用：$REMOTE_DOCKER_COMPOSE_CMD"
    fi
    
    # 检查是否已安装
    if [ -d "$INSTALL_DIR" ] && [ -f "$INSTALL_DIR/docker-compose.yml" ]; then
        log_warning "检测到已安装实例"
        read -p "是否继续安装？(这将覆盖现有配置) (y/n): " confirm
        if [[ ! "$confirm" =~ ^[Yy]$ ]]; then
            log_error "安装已取消"
            exit 0
        fi
    fi
    
    # 创建目录
    log_info "创建安装目录..."
    ssh_exec "$SERVER_IP" "$SERVER_USER" "$SSH_PORT" \
        "mkdir -p $DATA_DIR $LOGS_DIR $CERTS_DIR $BACKUP_DIR"
    
    # 交互式配置
    echo -e "\n${BOLD}╔════════════════════════════════════════════════╗${NC}"
    echo -e "${BOLD}║         安装配置                                ║${NC}"
    echo -e "${BOLD}╚════════════════════════════════════════════════╝${NC}\n"
    
    read -p "请输入服务端口 (默认：$DEFAULT_PORT): " SERVICE_PORT
    SERVICE_PORT=${SERVICE_PORT:-$DEFAULT_PORT}
    
    read -p "是否启用 HTTPS? (y/n, 默认：$DEFAULT_HTTPS): " ENABLE_HTTPS
    ENABLE_HTTPS=${ENABLE_HTTPS:-$DEFAULT_HTTPS}
    
    # 生成安全凭证
    ADMIN_PASSWORD=$(generate_password)
    API_KEY=$(generate_api_key)
    JWT_SECRET=$(generate_jwt_secret)
    
    echo ""
    log_info "已生成安全凭证:"
    echo "  API Key:     $API_KEY"
    echo "  管理员密码： $ADMIN_PASSWORD"
    echo "  JWT Secret:  ${JWT_SECRET:0:20}..."
    echo ""
    
    if [[ "$FORCE" != true ]]; then
        read -p "确认开始安装？(y/n): " CONFIRM
        if [[ ! "$CONFIRM" =~ ^[Yy]$ ]]; then
            log_error "安装已取消"
            exit 0
        fi
    fi
    
    # 构建 Docker 镜像
    log_info "构建 Docker 镜像..."
    cd "$PROJECT_ROOT"
    ./scripts/package-docker-image.sh
    
    # 上传镜像
    log_info "上传镜像到服务器..."
    ssh_upload "$SERVER_IP" "$SERVER_USER" "$SSH_PORT" \
        "$IMAGE_FILE" "$INSTALL_DIR/"
    
    # 加载镜像
    log_info "加载 Docker 镜像..."
    ssh_exec "$SERVER_IP" "$SERVER_USER" "$SSH_PORT" \
        "cd $INSTALL_DIR && docker load -i oppama-docker.tar.gz"
    
    # 生成配置
    log_info "生成配置文件..."
    if [[ "$ENABLE_HTTPS" =~ ^[Yy]$ ]]; then
        HTTPS_ENABLED="true"
        generate_ssl_cert "$SERVER_IP"
        # 上传证书
        ssh_upload "$SERVER_IP" "$SERVER_USER" "$SSH_PORT" \
            "$CERTS_DIR" "$INSTALL_DIR/"
    else
        HTTPS_ENABLED="false"
    fi
    
    generate_config "$SERVICE_PORT" "$HTTPS_ENABLED" "$API_KEY" "$JWT_SECRET"
    ssh_upload "$SERVER_IP" "$SERVER_USER" "$SSH_PORT" \
        "$CONFIG_FILE" "$INSTALL_DIR/"
    
    # 生成 docker-compose
    generate_docker_compose "$SERVICE_PORT"
    ssh_upload "$SERVER_IP" "$SERVER_USER" "$SSH_PORT" \
        "$INSTALL_DIR/docker-compose.yml" "$INSTALL_DIR/"
    
    # 启动服务
    log_info "启动服务..."
    if [ "$LOCAL_MODE" = false ]; then
        ssh_exec "$SERVER_IP" "$SERVER_USER" "$SSH_PORT" \
            "cd $INSTALL_DIR && $REMOTE_DOCKER_COMPOSE_CMD up -d"
    else
        ssh_exec "$SERVER_IP" "$SERVER_USER" "$SSH_PORT" \
            "cd $INSTALL_DIR && $DOCKER_COMPOSE_CMD up -d"
    fi
    
    sleep 3
    
    # 显示部署信息
    echo -e "\n${GREEN}${BOLD}╔════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}${BOLD}║     🎉 安装完成！                              ║${NC}"
    echo -e "${GREEN}${BOLD}╚════════════════════════════════════════════════╝${NC}${NC}\n"
    
    echo "${BOLD}📋 部署信息：${NC}"
    echo "  安装路径： $INSTALL_DIR"
    echo "  服务端口： $SERVICE_PORT"
    echo "  HTTPS:     ${HTTPS_ENABLED}"
    echo ""
    echo "${BOLD}🔐 安全凭证（请妥善保管）：${NC}"
    echo "  API Key:     $API_KEY"
    echo "  管理员密码： $ADMIN_PASSWORD"
    echo ""
    echo "${BOLD}🌐 访问地址：${NC}"
    if [[ "$ENABLE_HTTPS" =~ ^[Yy]$ ]]; then
        echo "  https://${SERVER_IP}:${SERVICE_PORT}/admin"
    else
        echo "  http://${SERVER_IP}:${SERVICE_PORT}/admin"
    fi
    echo ""
    echo "${BOLD}🔧 常用命令：${NC}"
    echo "  $0 status          # 查看状态"
    echo "  $0 logs            # 查看日志"
    echo "  $0 restart         # 重启服务"
    echo "  $0 backup          # 备份数据"
    echo ""
}

# ==================== 更新流程 ====================
do_update() {
    log_step "更新 Oppama"
    
    # 远程模式下先检测环境
    if [ "$LOCAL_MODE" = false ]; then
        log_info "检测远程服务器环境..."
        REMOTE_DOCKER_COMPOSE_CMD=$(get_remote_docker_compose_cmd "$SERVER_IP" "$SERVER_USER" "$SSH_PORT")
        if [ -z "$REMOTE_DOCKER_COMPOSE_CMD" ]; then
            log_error "远程服务器未检测到 docker-compose 命令"
            exit 1
        fi
        log_success "远程 docker-compose 可用：$REMOTE_DOCKER_COMPOSE_CMD"
    fi
    
    # 检查是否已安装
    if [ "$LOCAL_MODE" = true ]; then
        if [ ! -f "$INSTALL_DIR/docker-compose.yml" ]; then
            log_error "未检测到安装实例，请先运行安装"
            exit 1
        fi
    else
        # 远程模式：检查服务器上是否存在
        if ! ssh_exec "$SERVER_IP" "$SERVER_USER" "$SSH_PORT" \
            "[ -f $INSTALL_DIR/docker-compose.yml ] && echo 'exists'" 2>/dev/null | grep -q 'exists'; then
            log_error "远程服务器未检测到安装实例，请先运行安装"
            exit 1
        fi
        log_success "远程服务器连接正常，已检测到安装实例"
    fi
    
    # 备份
    if [[ "$AUTO_BACKUP" == true ]]; then
        do_backup
    fi
    
    # 构建新版本
    log_info "构建新版本..."
    cd "$PROJECT_ROOT"
    ./scripts/package-docker-image.sh
    
    # 上传并加载
    log_info "上传新版本..."
    ssh_upload "$SERVER_IP" "$SERVER_USER" "$SSH_PORT" \
        "$IMAGE_FILE" "$INSTALL_DIR/"
    
    log_info "加载新镜像..."
    ssh_exec "$SERVER_IP" "$SERVER_USER" "$SSH_PORT" \
        "cd $INSTALL_DIR && docker load -i oppama-docker.tar.gz"
    
    # 停止旧版本
    log_info "停止旧版本..."
    if [ "$LOCAL_MODE" = false ]; then
        ssh_exec "$SERVER_IP" "$SERVER_USER" "$SSH_PORT" \
            "cd $INSTALL_DIR && $REMOTE_DOCKER_COMPOSE_CMD down"
    else
        ssh_exec "$SERVER_IP" "$SERVER_USER" "$SSH_PORT" \
            "cd $INSTALL_DIR && $DOCKER_COMPOSE_CMD down"
    fi
    
    # 启动新版本
    log_info "启动新版本..."
    if [ "$LOCAL_MODE" = false ]; then
        ssh_exec "$SERVER_IP" "$SERVER_USER" "$SSH_PORT" \
            "cd $INSTALL_DIR && $REMOTE_DOCKER_COMPOSE_CMD up -d"
    else
        ssh_exec "$SERVER_IP" "$SERVER_USER" "$SSH_PORT" \
            "cd $INSTALL_DIR && $DOCKER_COMPOSE_CMD up -d"
    fi
    
    sleep 3
    
    # 检查状态
    log_info "检查服务状态..."
    ssh_exec "$SERVER_IP" "$SERVER_USER" "$SSH_PORT" \
        "cd $INSTALL_DIR && $DOCKER_COMPOSE_CMD ps"
    
    log_success "更新完成！"
}

# ==================== 配置更新 ====================
do_config() {
    log_step "更新配置"
    
    # 检查配置文件是否存在
    if [ "$LOCAL_MODE" = true ]; then
        if [ ! -f "$CONFIG_FILE" ]; then
            log_error "配置文件不存在"
            exit 1
        fi
    else
        # 远程模式：检查服务器上是否存在
        if ! ssh_exec "$SERVER_IP" "$SERVER_USER" "$SSH_PORT" \
            "[ -f $CONFIG_FILE ] && echo 'exists'" 2>/dev/null | grep -q 'exists'; then
            log_error "远程服务器配置文件不存在"
            exit 1
        fi
        log_success "远程配置文件存在"
    fi
    
    # 备份当前配置
    cp "$CONFIG_FILE" "$CONFIG_FILE.backup.$(date +%Y%m%d-%H%M%S)"
    log_success "原配置已备份"
    
    # 交互式配置
    echo -e "\n${BOLD}╔════════════════════════════════════════════════╗${NC}"
    echo -e "${BOLD}║         配置更新                                ║${NC}"
    echo -e "${BOLD}╚════════════════════════════════════════════════╝${NC}\n"
    
    # 读取当前配置
    CURRENT_PORT=$(grep "port:" "$CONFIG_FILE" | head -1 | awk '{print $2}')
    CURRENT_HTTPS=$(grep "enable_https:" "$CONFIG_FILE" | awk '{print $2}')
    
    read -p "请输入服务端口 (当前：$CURRENT_PORT): " SERVICE_PORT
    SERVICE_PORT=${SERVICE_PORT:-$CURRENT_PORT}
    
    read -p "是否启用 HTTPS? (y/n, 当前：$CURRENT_HTTPS): " ENABLE_HTTPS
    ENABLE_HTTPS=${ENABLE_HTTPS:-$CURRENT_HTTPS}
    
    # 重新生成配置
    API_KEY=$(grep "api_key:" "$CONFIG_FILE" | awk '{print $2}' | tr -d '"')
    JWT_SECRET=$(grep "jwt_secret:" "$CONFIG_FILE" | awk '{print $2}' | tr -d '"')
    
    generate_config "$SERVICE_PORT" "$ENABLE_HTTPS" "$API_KEY" "$JWT_SECRET"
    
    # 上传新配置
    ssh_upload "$SERVER_IP" "$SERVER_USER" "$SSH_PORT" \
        "$CONFIG_FILE" "$INSTALL_DIR/"
    
    # 如果需要 HTTPS，重新生成证书
    if [[ "$ENABLE_HTTPS" =~ ^[Yy]$ ]] && [[ "$CURRENT_HTTPS" != "true" ]]; then
        generate_ssl_cert "$SERVER_IP"
        ssh_upload "$SERVER_IP" "$SERVER_USER" "$SSH_PORT" \
            "$CERTS_DIR" "$INSTALL_DIR/"
    fi
    
    # 重启服务
    read -p "是否立即重启服务以应用配置？(y/n): " restart_confirm
    if [[ "$restart_confirm" =~ ^[Yy]$ ]]; then
        log_info "重启服务..."
        ssh_exec "$SERVER_IP" "$SERVER_USER" "$SSH_PORT" \
            "cd $INSTALL_DIR && $DOCKER_COMPOSE_CMD restart"
        log_success "服务已重启"
    fi
    
    log_success "配置已更新"
}

# ==================== 主程序 ====================
main() {
    while true; do
        show_menu
        
        read -p "请输入选项 (0-12): " choice
        
        case $choice in
            1)
                # 本地安装
                LOCAL_MODE=true
                SERVER_IP="localhost"
                SERVER_USER="root"
                SSH_PORT=22
                do_install
                ;;
            2)
                # 远程安装
                LOCAL_MODE=false
                echo -e "\n${BOLD}╔══════════════════════════════════════════════════╗${NC}"
                echo -e "${BOLD}║         远程服务器配置                            ║${NC}"
                echo -e "${BOLD}╚══════════════════════════════════════════════════╝${NC}\n"
                
                read -p "请输入服务器 IP: " SERVER_IP
                read -p "SSH 用户名 (默认：root): " SERVER_USER
                SERVER_USER=${SERVER_USER:-root}
                read -p "SSH 端口 (默认：22): " SSH_PORT
                SSH_PORT=${SSH_PORT:-22}
                
                do_install
                ;;
            3)
                # 本地更新
                LOCAL_MODE=true
                SERVER_IP="localhost"
                SERVER_USER="root"
                SSH_PORT=22
                AUTO_BACKUP=true
                do_update
                ;;
            4)
                # 远程更新
                LOCAL_MODE=false
                echo -e "\n${BOLD}╔══════════════════════════════════════════════════╗${NC}"
                echo -e "${BOLD}║         远程服务器配置                            ║${NC}"
                echo -e "${BOLD}╚══════════════════════════════════════════════════╝${NC}\n"
                
                read -p "请输入服务器 IP: " SERVER_IP
                read -p "SSH 用户名 (默认：root): " SERVER_USER
                SERVER_USER=${SERVER_USER:-root}
                read -p "SSH 端口 (默认：22): " SSH_PORT
                SSH_PORT=${SSH_PORT:-22}
                
                AUTO_BACKUP=true
                do_update
                ;;
            5)
                # 更新配置
                echo -e "\n${BOLD}╔══════════════════════════════════════════════════╗${NC}"
                echo -e "${BOLD}║         选择配置目标                              ║${NC}"
                echo -e "${BOLD}╚══════════════════════════════════════════════════╝${NC}\n"
                
                echo "  1) 本地配置"
                echo "  2) 远程服务器配置"
                read -p "请选择 (1-2): " config_target
                
                if [ "$config_target" = "1" ]; then
                    LOCAL_MODE=true
                    SERVER_IP="localhost"
                else
                    LOCAL_MODE=false
                    read -p "请输入服务器 IP: " SERVER_IP
                    read -p "SSH 用户名 (默认：root): " SERVER_USER
                    SERVER_USER=${SERVER_USER:-root}
                    read -p "SSH 端口 (默认：22): " SSH_PORT
                    SSH_PORT=${SSH_PORT:-22}
                fi
                
                do_config
                ;;
            6)
                # 备份数据
                echo -e "\n${BOLD}╔══════════════════════════════════════════════════╗${NC}"
                echo -e "${BOLD}║         选择备份目标                              ║${NC}"
                echo -e "${BOLD}╚══════════════════════════════════════════════════╝${NC}\n"
                
                echo "  1) 本地备份"
                echo "  2) 远程服务器备份"
                read -p "请选择 (1-2): " backup_target
                
                if [ "$backup_target" = "1" ]; then
                    LOCAL_MODE=true
                    SERVER_IP="localhost"
                else
                    LOCAL_MODE=false
                    read -p "请输入服务器 IP: " SERVER_IP
                    read -p "SSH 用户名 (默认：root): " SERVER_USER
                    SERVER_USER=${SERVER_USER:-root}
                    read -p "SSH 端口 (默认：22): " SSH_PORT
                    SSH_PORT=${SSH_PORT:-22}
                fi
                
                do_backup
                ;;
            7)
                # 从备份恢复
                echo -e "\n${BOLD}╔══════════════════════════════════════════════════╗${NC}"
                echo -e "${BOLD}║         选择恢复源                                ║${NC}"
                echo -e "${BOLD}╚══════════════════════════════════════════════════╝${NC}\n"
                
                echo "  1) 本地备份"
                echo "  2) 远程服务器备份"
                read -p "请选择 (1-2): " restore_target
                
                if [ "$restore_target" = "1" ]; then
                    LOCAL_MODE=true
                    SERVER_IP="localhost"
                else
                    LOCAL_MODE=false
                    read -p "请输入服务器 IP: " SERVER_IP
                    read -p "SSH 用户名 (默认：root): " SERVER_USER
                    SERVER_USER=${SERVER_USER:-root}
                    read -p "SSH 端口 (默认：22): " SSH_PORT
                    SSH_PORT=${SSH_PORT:-22}
                fi
                
                echo ""
                log_info "可用的备份:"
                ssh_exec "$SERVER_IP" "$SERVER_USER" "$SSH_PORT" \
                    "ls -lt $BACKUP_DIR | head -10"
                echo ""
                read -p "请输入备份路径: " backup_path
                do_restore "$backup_path"
                ;;
            8)
                # 查看状态
                echo -e "\n${BOLD}╔══════════════════════════════════════════════════╗${NC}"
                echo -e "${BOLD}║         选择查看目标                              ║${NC}"
                echo -e "${BOLD}╚══════════════════════════════════════════════════╝${NC}\n"
                
                echo "  1) 本地状态"
                echo "  2) 远程服务器状态"
                read -p "请选择 (1-2): " status_target
                
                if [ "$status_target" = "1" ]; then
                    LOCAL_MODE=true
                    SERVER_IP="localhost"
                else
                    LOCAL_MODE=false
                    read -p "请输入服务器 IP: " SERVER_IP
                    read -p "SSH 用户名 (默认：root): " SERVER_USER
                    SERVER_USER=${SERVER_USER:-root}
                    read -p "SSH 端口 (默认：22): " SSH_PORT
                    SSH_PORT=${SSH_PORT:-22}
                fi
                
                ssh_exec "$SERVER_IP" "$SERVER_USER" "$SSH_PORT" \
                    "cd $INSTALL_DIR && $DOCKER_COMPOSE_CMD ps"
                ;;
            9)
                # 查看日志
                echo -e "\n${BOLD}╔══════════════════════════════════════════════════╗${NC}"
                echo -e "${BOLD}║         选择查看目标                              ║${NC}"
                echo -e "${BOLD}╚══════════════════════════════════════════════════╝${NC}\n"
                
                echo "  1) 本地日志"
                echo "  2) 远程服务器日志"
                read -p "请选择 (1-2): " logs_target
                
                if [ "$logs_target" = "1" ]; then
                    LOCAL_MODE=true
                    SERVER_IP="localhost"
                else
                    LOCAL_MODE=false
                    read -p "请输入服务器 IP: " SERVER_IP
                    read -p "SSH 用户名 (默认：root): " SERVER_USER
                    SERVER_USER=${SERVER_USER:-root}
                    read -p "SSH 端口 (默认：22): " SSH_PORT
                    SSH_PORT=${SSH_PORT:-22}
                fi
                
                ssh_exec "$SERVER_IP" "$SERVER_USER" "$SSH_PORT" \
                    "cd $INSTALL_DIR && $DOCKER_COMPOSE_CMD logs -f"
                ;;
            10)
                # 重启服务
                echo -e "\n${BOLD}╔══════════════════════════════════════════════════╗${NC}"
                echo -e "${BOLD}║         选择重启目标                              ║${NC}"
                echo -e "${BOLD}╚══════════════════════════════════════════════════╝${NC}\n"
                
                echo "  1) 本地服务"
                echo "  2) 远程服务器服务"
                read -p "请选择 (1-2): " restart_target
                
                if [ "$restart_target" = "1" ]; then
                    LOCAL_MODE=true
                    SERVER_IP="localhost"
                else
                    LOCAL_MODE=false
                    read -p "请输入服务器 IP: " SERVER_IP
                    read -p "SSH 用户名 (默认：root): " SERVER_USER
                    SERVER_USER=${SERVER_USER:-root}
                    read -p "SSH 端口 (默认：22): " SSH_PORT
                    SSH_PORT=${SSH_PORT:-22}
                fi
                
                ssh_exec "$SERVER_IP" "$SERVER_USER" "$SSH_PORT" \
                    "cd $INSTALL_DIR && $DOCKER_COMPOSE_CMD restart"
                log_success "服务已重启"
                ;;
            11)
                # 卸载服务
                echo -e "\n${BOLD}╔══════════════════════════════════════════════════╗${NC}"
                echo -e "${BOLD}║         选择卸载目标                              ║${NC}"
                echo -e "${BOLD}╚══════════════════════════════════════════════════╝${NC}\n"
                
                echo "  1) 本地服务"
                echo "  2) 远程服务器服务"
                read -p "请选择 (1-2): " uninstall_target
                
                if [ "$uninstall_target" = "1" ]; then
                    LOCAL_MODE=true
                    SERVER_IP="localhost"
                else
                    LOCAL_MODE=false
                    read -p "请输入服务器 IP: " SERVER_IP
                    read -p "SSH 用户名 (默认：root): " SERVER_USER
                    SERVER_USER=${SERVER_USER:-root}
                    read -p "SSH 端口 (默认：22): " SSH_PORT
                    SSH_PORT=${SSH_PORT:-22}
                fi
                
                log_warning "卸载将删除所有数据，此操作不可逆！"
                read -p "确认卸载？(y/n): " confirm
                if [[ "$confirm" =~ ^[Yy]$ ]]; then
                    ssh_exec "$SERVER_IP" "$SERVER_USER" "$SSH_PORT" \
                        "cd $INSTALL_DIR && $DOCKER_COMPOSE_CMD down -v"
                    log_success "服务已卸载"
                else
                    log_info "卸载已取消"
                fi
                ;;
            0)
                log_info "退出"
                exit 0
                ;;
            *)
                log_error "无效选项，请重新选择"
                ;;
        esac
        
        echo ""
        read -p "按回车键继续..."
    done
}

# 运行主程序
main
