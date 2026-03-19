# 多阶段构建 Dockerfile
# Stage 1: 构建前端
FROM node:20-alpine AS frontend-builder

WORKDIR /web

# 复制前端依赖文件
COPY web/package*.json ./
RUN npm ci

# 复制前端源码并构建
COPY web/ ./
RUN npm run build

# Stage 2: 构建后端
FROM golang:1.25-alpine AS backend-builder

WORKDIR /build

# 安装构建依赖
RUN apk add --no-cache git gcc musl-dev sqlite-dev

# 复制 go mod 文件并下载依赖
COPY go.mod go.sum ./
RUN go mod download

# 复制源码并构建
COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -a -ldflags="-s -w" -o oppama ./cmd/server

# Stage 3: 最终镜像
FROM alpine:3.20

WORKDIR /app

# 安装运行时依赖
RUN apk add --no-cache ca-certificates tzdata

# 创建必要的目录
RUN mkdir -p /app/data /app/logs /app/web

# 从后端构建阶段复制二进制文件
COPY --from=backend-builder /build/oppama /app/oppama

# 从前端构建阶段复制静态文件（保持 web/dist 目录结构）
COPY --from=frontend-builder /web/dist /app/web/dist

# 复制配置文件
COPY config.yaml /app/config.yaml

# 暴露端口
EXPOSE 9001

# 设置数据目录为卷
VOLUME ["/app/data", "/app/logs"]

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:9001/health || exit 1

# 运行应用
ENTRYPOINT ["/app/oppama"]
CMD ["-config", "config.yaml"]
