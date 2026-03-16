.PHONY: build run test clean docker-build docker-run help

# 变量
BINARY_NAME=oppama
VERSION=0.1.0
BUILD_DIR=./bin

# 默认目标
all: build

# 构建后端
build:
	@echo "构建后端..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=1 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/server

# 跨平台编译
build-all: build-linux build-darwin build-windows

build-linux:
	@echo "构建 Linux 版本..."
	@mkdir -p $(BUILD_DIR)/linux
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_DIR)/linux/$(BINARY_NAME) ./cmd/server

build-darwin:
	@echo "构建 macOS 版本..."
	@mkdir -p $(BUILD_DIR)/darwin
	CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_DIR)/darwin/$(BINARY_NAME) ./cmd/server

build-windows:
	@echo "构建 Windows 版本..."
	@mkdir -p $(BUILD_DIR)/windows
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_DIR)/windows/$(BINARY_NAME).exe ./cmd/server

# 运行服务（前台）
run:
	@echo "启动服务...（按 Ctrl+C 停止）"
	@echo "API 地址：http://localhost:8080"
	@echo "管理界面：http://localhost:8080/admin"
	go run ./cmd/server/main.go -config config.yaml

# 后台运行服务
run-bg:
	@echo "启动服务（后台）..."
	nohup go run ./cmd/server/main.go -config config.yaml > server.log 2>&1 &
	sleep 3
	@echo "服务已启动，日志文件：server.log"
	@echo "API 地址：http://localhost:8080"
	@echo "管理界面：http://localhost:8080/admin"

# 查看后台服务日志
logs:
	tail -f server.log

# 停止后台服务
stop:
	@echo "停止服务..."
	-@pkill -f oppama || true
	@echo "服务已停止"

# 运行测试
test:
	@echo "运行测试..."
	go test -v ./...

# 代码检查
lint:
	@echo "代码检查..."
	golangci-lint run

# 清理构建产物
clean:
	@echo "清理..."
	rm -rf $(BUILD_DIR)
	rm -f $(BINARY_NAME)

# Docker 相关
docker-build:
	@echo "构建 Docker 镜像..."
	docker build -t $(BINARY_NAME):$(VERSION) -f deploy/Dockerfile .

docker-run:
	@echo "启动 Docker 容器..."
	docker-compose -f deploy/docker-compose.yml up -d

docker-stop:
	@echo "停止 Docker 容器..."
	docker-compose -f deploy/docker-compose.yml down

docker-logs:
	docker-compose -f deploy/docker-compose.yml logs -f

# 前端构建
web-install:
	@echo "安装前端依赖..."
	cd web && npm install

web-build:
	@echo "构建前端..."
	cd web && npm run build

web-dev:
	@echo "启动前端开发服务器..."
	cd web && npm run dev

# 生成配置文件
init-config:
	@echo "生成配置文件..."
	cp deploy/config.example.yaml config.yaml
	cp .env.example .env
	@echo "配置文件已生成，请编辑 config.yaml 和 .env 文件"

# 帮助信息
help:
	@echo "Oppama v$(VERSION)"
	@echo ""
	@echo "可用命令:"
	@echo "  make build        - 构建后端"
	@echo "  make build-all    - 跨平台编译 (Linux, macOS, Windows)"
	@echo "  make run          - 运行服务"
	@echo "  make test         - 运行测试"
	@echo "  make lint         - 代码检查"
	@echo "  make clean        - 清理构建产物"
	@echo "  make docker-build - 构建 Docker 镜像"
	@echo "  make docker-run   - 启动 Docker 容器"
	@echo "  make docker-stop  - 停止 Docker 容器"
	@echo "  make web-install  - 安装前端依赖"
	@echo "  make web-build    - 构建前端"
	@echo "  make web-dev      - 启动前端开发服务器"
	@echo "  make init-config  - 生成配置文件"
