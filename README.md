# Oppama - Ollama 服务发现与代理网关

## 项目简介

Oppama 是一个专为 Ollama 设计的轻量级服务发现和 API 代理网关。它能够自动发现网络中公开的 Ollama 服务实例，通过蜜罐检测识别真实服务，并提供统一的 API 接口和负载均衡功能。

**适用场景**：内部部署、私有化分发、企业内网使用

## 核心功能

### 🔍 服务发现

- **多源搜索集成**：支持 FOFA、Hunter、ZoomEye、Shodan 等主流网络空间搜索引擎
- **自动发现**：定时自动扫描新增的 Ollama 服务实例
- **灵活配置**：可自定义搜索语法和结果数量

### 🛡️ 蜜罐检测

- **智能识别**：自动检测并过滤蜜罐和虚假服务
- **多维度验证**：基于端口特征、版本信息等进行综合判断
- **定期巡检**：周期性检查已发现服务的可用性

### 🔄 代理网关

- **统一入口**：提供标准化的 API 接口，兼容 OpenAI 格式
- **负载均衡**：支持故障转移和多实例轮询
- **认证鉴权**：可选的 API Key 认证机制
- **限流保护**：防止滥用和过载

### 📊 可视化管理

- **Web 控制台**：直观的服务管理和监控界面
- **实时状态**：查看服务健康度和响应性能
- **任务追踪**：异步任务进度实时通知

## 快速开始

### 方式一：Docker 部署（推荐）

#### 1. 使用 Docker Compose

```bash
# 克隆项目
git clone https://github.com/p1yang/oppama.git
cd oppama

# 启动服务
docker-compose up -d

# 查看日志
docker-compose logs -f

# 停止服务
docker-compose down
```

#### 2. 访问服务

- **Web 控制台**: http://localhost:9001
- **健康检查**: http://localhost:9001/health
- **API 地址**: http://localhost:9001/api

#### 3. 数据持久化

Docker Compose 已自动配置卷挂载：
- `./data` → `/app/data` (数据库)
- `./logs` → `/app/logs` (日志文件)
- `./config.yaml` → `/app/config.yaml` (配置文件)

### 方式二：使用部署脚本（支持远程部署）

项目提供统一的管理脚本 `scripts/manage.sh`，支持本地和远程服务器的一键安装、更新、备份。

```bash
# 进入项目目录
cd oppama

# 运行管理脚本
chmod +x scripts/manage.sh
./scripts/manage.sh
```

**功能菜单：**
- ✅ 本地安装（Docker）
- ✅ 远程服务器安装（SSH）
- ✅ 版本更新
- ✅ 配置管理
- ✅ 数据备份/恢复
- ✅ 状态监控
- ✅ 日志查看

**远程部署示例：**

1. 选择 "安装（远程服务器）"
2. 输入服务器 IP、SSH 用户名和端口
3. 配置服务端口和 HTTPS 选项
4. 自动完成环境检测、镜像构建、文件上传和服务部署

**常用命令：**

```bash
# 查看服务状态
./scripts/manage.sh status

# 查看实时日志
./scripts/manage.sh logs

# 重启服务
./scripts/manage.sh restart

# 备份数据
./scripts/manage.sh backup
```

### 方式三：源码编译运行

#### 1. 环境要求

- Go 1.25+
- Node.js 20+ (用于构建前端)
- GCC 和 SQLite 开发库

#### 2. 克隆项目

```bash
git clone https://github.com/p1yang/oppama.git
cd oppama
```

#### 3. 构建前端

```bash
cd web
npm install
npm run build
cd ..
```

#### 4. 构建后端

```bash
# 安装依赖
go mod download

# 编译（需要 CGO 支持）
CGO_ENABLED=1 go build -o oppama ./cmd/server
```

#### 5. 运行服务

```bash
# 使用默认配置运行
./oppama

# 指定配置文件
./oppama --config /path/to/config.yaml
```

#### 6. 使用 Makefile（可选）

项目提供了 Makefile，可以简化构建流程：

```bash
# 安装前端依赖
make web-install

# 构建前端
make web-build

# 构建后端
make build

# 直接运行（开发模式）
make run

# 后台运行
make run-bg

# 查看日志
make logs

# 停止服务
make stop
```

## 配置说明

### 环境变量配置

所有配置项都可以通过环境变量设置（优先级高于配置文件）：

```bash
# 服务器配置
export OLLAMA_SERVER_HOST=0.0.0.0
export OLLAMA_SERVER_PORT=8080

# FOFA API
export OLLAMA_FOFA_EMAIL="your-email@example.com"
export OLLAMA_FOFA_KEY="your-api-key"

# Hunter API
export OLLAMA_HUNTER_KEY="your-api-key"

# ZoomEye API
export OLLAMA_ZOOMEEYE_USERNAME="your-username"
export OLLAMA_ZOOMEEYE_PASSWORD="your-password"

# Shodan API
export OLLAMA_SHODAN_KEY="your-api-key"

# 日志配置
export OLLAMA_LOG_LEVEL=info
export OLLAMA_LOG_FORMAT=text
```

### 配置文件详解

完整配置示例请参考 `deploy/config.example.yaml`。

### Docker 部署配置

使用 Docker 部署时，可以通过以下方式配置：

**1. 修改 docker-compose.yml**

```yaml
services:
  oppama:
    ports:
      - "9001:9001"  # 修改宿主机端口
    volumes:
      - ./data:/app/data
      - ./logs:/app/logs
      - ./config.yaml:/app/config.yaml
```

**2. 使用环境变量**

```bash
# 在 docker-compose.yml 中添加
environment:
  - OLLAMA_SERVER_PORT=9001
  - OLLAMA_LOG_LEVEL=info
```

**3. 挂载自定义配置文件**

```bash
# 准备 config.yaml
docker run -v $(pwd)/config.yaml:/app/config.yaml oppama:latest
```

## API 使用指南

### 1. 获取模型列表

```bash
curl http://localhost:8080/api/tags
```

### 2. 调用生成接口

```bash
curl http://localhost:8080/api/generate \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama2",
    "prompt": "你好，请介绍一下你自己",
    "stream": false
  }'
```

### 3. OpenAI 兼容接口

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-api-key" \
  -d '{
    "model": "llama2",
    "messages": [
      {"role": "user", "content": "你好"}
    ]
  }'
```

### 4. Anthropic Claude 兼容接口

```bash
# 使用 x-api-key header (推荐)
curl http://localhost:8080/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: your-api-key" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-sonnet-4-20250514",
    "max_tokens": 1024,
    "messages": [
      {"role": "user", "content": "Hello!"}
    ]
  }'
```

详细使用说明请参考 [ANTHROPIC_API.md](./ANTHROPIC_API.md)。

### 5. 服务发现接口

```bash
# 手动触发服务发现
curl -X POST http://localhost:8080/api/v1/discovery/scan

# 查询发现的服務
curl http://localhost:8080/api/v1/services
```

## 数据持久化

Oppama 使用 SQLite 数据库存储服务发现记录和任务历史：

- **默认路径**：`./data/oppama.db`
- **数据保留期**：30 天（可配置）
- **备份建议**：定期备份 `data/` 目录

## 日志管理

### 日志文件位置

```
./logs/server.log
```

### 日志轮转配置

```yaml
log:
  level: info           # debug, info, warn, error
  format: json          # json, console
  output: ./logs/server.log
  max_size: 100         # MB
  max_backups: 5        # 保留的旧日志文件数
  max_age: 30           # 天
```

### 查看实时日志

```bash
docker-compose logs -f oppama
```

## 健康检查

Oppama 提供健康检查端点：

```bash
curl http://localhost:8080/health
```

返回示例：

```json
{
  "status": "healthy",
  "timestamp": "2026-03-16T10:30:00Z",
  "version": "1.0.0"
}
```

## 生产环境部署

### Docker 生产部署

**1. 启用 HTTPS**

修改 `config.yaml`：

```yaml
server:
  enable_https: true
  cert_file: "./certs/server.crt"
  key_file: "./certs/server.key"
  min_tls_version: "1.2"
```

**2. 配置反向代理（Nginx）**

```nginx
server {
    listen 80;
    server_name your-domain.com;
    
    location / {
        proxy_pass http://localhost:9001;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }
}
```

**3. 配置系统服务（Systemd）**

创建 `/etc/systemd/system/oppama.service`：

```ini
[Unit]
Description=Oppama Service
After=docker.service
Requires=docker.service

[Service]
Restart=always
WorkingDirectory=/opt/oppama
ExecStart=/usr/local/bin/docker-compose up -d
ExecStop=/usr/local/bin/docker-compose down

[Install]
WantedBy=multi-user.target
```

启用服务：

```bash
sudo systemctl daemon-reload
sudo systemctl enable oppama
sudo systemctl start oppama
```

### 使用管理脚本部署到生产环境

项目提供统一的交互式管理脚本 `scripts/manage.sh`，支持本地和远程服务器的完整生命周期管理。

**基本用法：**

```bash
# 进入项目目录
cd oppama

# 运行管理脚本
chmod +x scripts/manage.sh
./scripts/manage.sh
```

**功能菜单：**

1. ✅ **安装（本地）** - 在当前服务器通过 Docker 安装
2. ✅ **安装（远程服务器）** - 通过 SSH 部署到远程服务器
3. ✅ **更新（本地）** - 更新当前服务器的版本
4. ✅ **更新（远程服务器）** - 更新远程服务器的版本
5. ✅ **更新配置** - 修改服务端口、HTTPS 等配置
6. ✅ **备份数据** - 备份数据、配置和证书
7. ✅ **从备份恢复** - 从历史备份中恢复
8. ✅ **查看状态** - 查看服务运行状态
9. ✅ **查看日志** - 实时查看日志输出
10. ✅ **重启服务** - 重启服务
11. ✅ **卸载服务** - 完全卸载服务

**远程部署示例：**

```bash
# 运行脚本
./scripts/manage.sh

# 选择 "2) 安装（远程服务器）"
# 输入服务器 IP: 192.168.1.100
# SSH 用户名 (默认：root): root
# SSH 端口 (默认：22): 22

# 然后按照提示配置：
# - 服务端口 (默认：9001)
# - 是否启用 HTTPS
# - 自动生成 API Key、管理员密码、JWT Secret

# 脚本会自动完成：
# 1. 检测远程服务器 Docker 环境
# 2. 构建 Docker 镜像并打包
# 3. 上传镜像到远程服务器
# 4. 生成配置文件和证书
# 5. 启动服务并进行健康检查
# 6. 显示访问地址和凭证信息
```

**自动化运维命令：**

```bash
# 查看服务状态
./scripts/manage.sh status          # 选择 8 -> 1 (本地) 或 2 (远程)

# 查看实时日志
./scripts/manage.sh logs            # 选择 9 -> 1 (本地) 或 2 (远程)

# 重启服务
./scripts/manage.sh restart         # 选择 10 -> 1 (本地) 或 2 (远程)

# 备份数据
./scripts/manage.sh backup          # 选择 6 -> 1 (本地) 或 2 (远程)

# 从备份恢复
./scripts/manage.sh restore         # 选择 7 -> 选择备份文件
```

**特点：**

- 🔐 **自动生成安全凭证**：API Key、管理员密码、JWT Secret
- 📦 **Docker 镜像打包**：自动构建并传输优化后的镜像
- 🔒 **HTTPS 支持**：可选的自签名证书生成
- 💾 **数据持久化**：自动配置卷挂载，确保数据安全
- ❤️ **健康检查**：部署后自动进行健康验证
- 🎯 **交互式界面**：无需记忆参数，引导式操作


## 维护与故障排查

### 查看服务状态

```bash
# Docker 方式
docker-compose ps

# 使用管理脚本
./scripts/manage.sh status
```

### 查看日志

```bash
# 实时日志
docker-compose logs -f

# 最近 100 行
docker-compose logs --tail=100

# 导出日志
docker-compose logs > oppama.log
```

### 备份数据

```bash
# 使用管理脚本备份
./scripts/manage.sh backup

# 手动备份
cp -r data/ backup-$(date +%Y%m%d)
cp config.yaml backup-config-$(date +%Y%m%d).yaml
```

### 恢复数据

```bash
# 停止服务
docker-compose down

# 恢复数据
cp -r backup-20260320/ data/
cp backup-config-20260320.yaml config.yaml

# 重启服务
docker-compose up -d
```

### 常见问题

### Q: 如何启用 API 认证？

A: 在配置文件中设置：

```yaml
proxy:
  enable_auth: true
  api_key: "your-secret-key"
```

调用接口时需添加请求头：

```bash
curl -H "Authorization: Bearer your-secret-key" http://localhost:8080/api/...
```

### Q: 如何配置多个服务发现引擎？

A: 在配置文件中启用多个引擎：

```yaml
discovery:
  engines:
    fofa:
      enabled: true
      email: "..."
      key: "..."
    hunter:
      enabled: true
      key: "..."
```

### Q: 服务发现失败怎么办？

A: 检查以下事项：

1. API Key 是否正确配置
2. 网络连接是否正常
3. 查看日志中的详细错误信息
4. 确认搜索引擎账户配额是否充足

### Q: 如何修改服务端口？

A: 通过环境变量或配置文件修改：

```yaml
server:
  port: 8080  # 改为其他端口
```

或：

```bash
export OLLAMA_SERVER_PORT=9000
```

## 技术栈

- **后端**：Go 1.25+
- **Web 框架**：Gin
- **数据库**：SQLite / PostgreSQL
- **前端**：Vue 3 + TypeScript + Vite
- **UI 组件**：Element Plus

## 目录结构

```
oppama/
├── bin/                    # 编译产物
├── cmd/server/            # 应用入口
├── internal/              # 内部包
│   ├── api/              # HTTP API
│   ├── config/           # 配置管理
│   ├── detector/         # 蜜罐检测器
│   ├── discovery/        # 服务发现
│   ├── proxy/            # 代理网关
│   ├── scheduler/        # 定时任务
│   ├── storage/          # 数据存储
│   └── utils/            # 工具函数
├── web/                   # 前端源码
├── deploy/                # 部署配置
├── data/                  # 数据文件
├── logs/                  # 日志文件
└── config.yaml            # 配置文件
```

## 版本说明

**当前版本**：1.0.0
**构建时间**：2026-03-16
**适用 Go 版本**：1.25.6
