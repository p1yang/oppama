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

### 方式二：直接运行

#### 1. 下载二进制文件

```bash
# Linux
chmod +x oppama
./oppama

# macOS
chmod +x oppama-darwin
./oppama-darwin

# Windows
oppama.exe
```

#### 2. 指定配置文件（可选）

```bash
./oppama --config /path/to/config.yaml
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

## 常见问题

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
