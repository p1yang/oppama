package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"oppama/internal/api"
	"oppama/internal/config"
	"oppama/internal/detector"
	"oppama/internal/scheduler"
	"oppama/internal/storage"
	"oppama/internal/utils/logger"
)

var version = "0.1.0"

func main() {
	// 解析命令行参数
	configPath := flag.String("config", "config.yaml", "配置文件路径")
	showVersion := flag.Bool("version", false, "显示版本号")
	flag.Parse()

	if *showVersion {
		fmt.Printf("Oppama v%s\n", version)
		os.Exit(0)
	}

	// 加载配置
	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Server().Fatalf("加载配置失败：%v", err)
	}

	// 验证配置
	if err := cfg.Validate(); err != nil {
		logger.Server().Fatalf("配置验证失败：%v", err)
	}

	// 初始化文件日志
	if cfg.Log.Output != "" && cfg.Log.Output != "stdout" {
		if err := logger.InitFileLogger(cfg.Log.Output); err != nil {
			fmt.Printf("警告：初始化文件日志失败：%v, 将使用控制台输出\n", err)
		}
	}

	// 设置日志级别
	if cfg.Log.Level != "" {
		logLevel := logger.ParseLogLevel(cfg.Log.Level)
		logger.SetLogLevel(logLevel)
		fmt.Printf("日志级别已设置为：%s\n", logLevel.String())
	}

	// 初始化存储
	var store storage.Storage
	switch cfg.Storage.Type {
	case "sqlite":
		pool := cfg.Storage.Pool
		store, err = storage.NewSQLiteStorageWithPool(
			cfg.Storage.SQLite.Path,
			pool.MaxOpenConns,
			pool.MaxIdleConns,
			time.Duration(pool.ConnMaxLifetime)*time.Second,
			time.Duration(pool.ConnMaxIdleTime)*time.Second,
		)
		if err != nil {
			logger.Storage().Fatalf("初始化 SQLite 存储失败：%v", err)
		}
	case "memory":
		store, err = storage.NewMemoryStorage()
		if err != nil {
			logger.Storage().Fatalf("初始化内存存储失败：%v", err)
		}
		logger.Storage().Printf("使用内存存储（仅用于开发/测试环境）")
	default:
		logger.Server().Fatalf("不支持的存储类型：%s", cfg.Storage.Type)
	}

	defer store.Close()

	// 检查数据库连接
	if err := store.Ping(nil); err != nil {
		logger.Storage().Fatalf("数据库连接失败：%v", err)
	}

	logger.Storage().Info("存储初始化成功")

	// 创建 API 服务器
	server := api.NewServer(cfg, store, *configPath)

	// 立即刷新一次 Proxy 缓存（加载所有有模型的服务）
	if proxySvc := server.GetProxyService(); proxySvc != nil {
		logger.Proxy().Info("初始化 Proxy 服务缓存...")
		if err := proxySvc.RefreshServices(); err != nil {
			logger.Proxy().Warnf("警告：刷新 Proxy 缓存失败：%v", err)
		}
	}

	// 打印默认管理员账户信息（在启动 scheduler 之前）
	api.PrintDefaultAdminInfo(store)

	// 创建检测器配置
	fakeVersions := make(map[string]bool)
	for _, v := range cfg.Detector.HoneypotDetection.FakeVersions {
		fakeVersions[v] = true
	}
	// 添加默认的虚假版本
	if fakeVersions["0.0.0"] == false {
		fakeVersions["0.0.0"] = true
	}
	if fakeVersions["unknown"] == false {
		fakeVersions["unknown"] = true
	}

	detectorCfg := &detector.DetectorConfig{
		Timeout:         time.Duration(cfg.Detector.Timeout) * time.Second,
		Concurrency:     cfg.Detector.Concurrency,
		CheckHoneypot:   cfg.Detector.HoneypotDetection.Enabled,
		CheckModels:     true,
		SuspiciousPorts: cfg.Detector.HoneypotDetection.SuspiciousPorts,
		FakeVersions:    fakeVersions,
	}

	// 创建并启动定时任务调度器，传入配置对象以读取间隔设置
	sched := scheduler.NewScheduler(nil, store, detectorCfg, &cfg.Detector)
	sched.Start()
	defer sched.Stop()

	// 设置调度器到服务器（用于配置热重载）
	server.SetScheduler(sched)

	// 优雅关闭
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		logger.Server().Info("正在关闭服务...")
		os.Exit(0)
	}()

	// 启动服务器
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	if cfg.Server.EnableHTTPS {
		logger.Server().Info("启动 HTTPS API 服务器：%s", addr)
		logger.Server().Info("管理界面：https://%s:%d/admin", cfg.Server.Host, cfg.Server.Port)
		logger.Server().Info("API 文档：https://%s:%d/v1/api", cfg.Server.Host, cfg.Server.Port)
	} else {
		logger.Server().Info("启动 HTTP API 服务器：%s", addr)
		logger.Server().Info("管理界面：http://%s:%d/admin", cfg.Server.Host, cfg.Server.Port)
		logger.Server().Info("API 文档：http://%s:%d/v1/api", cfg.Server.Host, cfg.Server.Port)
	}

	if err := server.Run(); err != nil {
		logger.Server().Fatalf("服务器启动失败：%v", err)
	}
}
