package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"oppama/internal/api/middleware"
	v1 "oppama/internal/api/v1"
	"oppama/internal/config"
	"oppama/internal/detector"
	"oppama/internal/proxy"
	storagepkg "oppama/internal/storage"
	"oppama/internal/task"

	"github.com/gin-gonic/gin"
)

// Storage 类型别名，方便使用
type Storage = storagepkg.Storage
type BlacklistStorage = storagepkg.BlacklistStorage

// Server API 服务器
type Server struct {
	engine       *gin.Engine
	config       *config.Config
	configPath   string
	storage      storagepkg.Storage
	taskMgr      *task.Manager
	proxyService *proxy.ProxyService
	rateLimiter  *middleware.LoginRateLimiter
	blacklist    storagepkg.BlacklistStorage
	scheduler    interface{} // 使用 interface 避免循环导入
}

// NewServer 创建 API 服务器
func NewServer(cfg *config.Config, storage storagepkg.Storage, configPath string) *Server {
	gin.SetMode(cfg.Server.Mode)

	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(gin.Logger())

	// 配置 CORS
	setupCORS(engine, cfg)

	// 创建登录限流器
	rateLimiter := middleware.NewLoginRateLimiter(middleware.LoginRateLimitConfig{
		MaxAttempts:    cfg.Auth.LoginRateLimit.MaxAttempts,
		WindowMinutes:  cfg.Auth.LoginRateLimit.WindowMinutes,
		LockoutMinutes: cfg.Auth.LoginRateLimit.LockoutMinutes,
	})

	// 创建 Token 黑名单
	var blacklist storagepkg.BlacklistStorage
	// 尝试将 Storage 转换为 BlacklistStorage
	type blacklistChecker interface {
		AddToken(ctx context.Context, token string, expiresAt time.Time) error
		IsTokenBlacklisted(ctx context.Context, token string) (bool, error)
		DeleteToken(ctx context.Context, token string) error
		CleanExpiredTokens(ctx context.Context) error
	}
	if bl, ok := storage.(blacklistChecker); ok {
		blacklist = bl
	} else {
		blacklist = storagepkg.NewBlacklistMemory()
	}

	// 创建任务管理器
	taskMgr := task.NewManager(nil)

	// 创建代理服务
	proxyCfg := &proxy.ProxyConfig{
		EnableAuth:      cfg.Proxy.EnableAuth,
		APIKey:          cfg.Proxy.APIKey,
		DefaultModel:    cfg.Proxy.DefaultModel,
		FallbackEnabled: cfg.Proxy.FallbackEnabled,
		MaxRetries:      cfg.Proxy.MaxRetries,
		Timeout:         time.Duration(cfg.Proxy.Timeout) * time.Second,
		RateLimitRPM:    cfg.Proxy.RateLimit.RequestsPerMinute,
		// HTTP 代理配置
		HTTPProxy:  cfg.Proxy.HTTPProxy,
		HTTPSProxy: cfg.Proxy.HTTPSProxy,
		NoProxy:    cfg.Proxy.NoProxy,
	}
	proxySvc := proxy.NewProxyService(proxyCfg, storage)

	server := &Server{
		engine:       engine,
		config:       cfg,
		configPath:   configPath,
		storage:      storage,
		taskMgr:      taskMgr,
		proxyService: proxySvc,
		rateLimiter:  rateLimiter,
		blacklist:    blacklist,
	}

	// 初始化默认管理员账户
	if err := initDefaultAdmin(storage); err != nil {
		fmt.Printf("警告：初始化默认管理员失败：%v\n", err)
	}

	// 注册路由
	server.registerRoutes()

	// 启动后台清理任务
	go server.cleanupLoop()

	return server
}

// setupCORS 配置 CORS
func setupCORS(engine *gin.Engine, cfg *config.Config) {
	engine.Use(func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// 检查是否在允许列表中
		allowed := false
		for _, allowedOrigin := range cfg.CORS.AllowedOrigins {
			if allowedOrigin == "*" || allowedOrigin == origin {
				allowed = true
				break
			}
		}

		if allowed {
			c.Header("Access-Control-Allow-Origin", origin)
		}

		c.Header("Access-Control-Allow-Methods", strings.Join(cfg.CORS.AllowedMethods, ", "))
		c.Header("Access-Control-Allow-Headers", strings.Join(cfg.CORS.AllowedHeaders, ", "))
		if cfg.CORS.AllowCredentials {
			c.Header("Access-Control-Allow-Credentials", "true")
		}
		c.Header("Access-Control-Max-Age", fmt.Sprintf("%d", cfg.CORS.MaxAge))

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})
}

// cleanupLoop 定期清理任务
func (s *Server) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		ctx := context.Background()
		// 清理过期的 Token
		s.blacklist.CleanExpiredTokens(ctx)
	}
}

// GetProxyService 获取 Proxy 服务实例
func (s *Server) GetProxyService() *proxy.ProxyService {
	return s.proxyService
}

// SetScheduler 设置调度器
func (s *Server) SetScheduler(sched interface{}) {
	s.scheduler = sched
}

// UpdateSchedulerDetectorConfig 更新调度器的检测器配置
func (s *Server) UpdateSchedulerDetectorConfig() {
	// 使用类型断言来调用 UpdateDetectorConfig
	type schedulerInterface interface {
		UpdateDetectorConfig(cfg *detector.DetectorConfig)
	}

	if sched, ok := s.scheduler.(schedulerInterface); ok {
		// 构建检测器配置
		fakeVersions := make(map[string]bool)
		for _, v := range s.config.Detector.HoneypotDetection.FakeVersions {
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
			Timeout:         time.Duration(s.config.Detector.Timeout) * time.Second,
			Concurrency:     s.config.Detector.Concurrency,
			CheckHoneypot:   s.config.Detector.HoneypotDetection.Enabled,
			CheckModels:     true,
			SuspiciousPorts: s.config.Detector.HoneypotDetection.SuspiciousPorts,
			FakeVersions:    fakeVersions,
		}

		sched.UpdateDetectorConfig(detectorCfg)
		fmt.Printf("[Server] 已更新 Scheduler 检测器配置\n")
	}
}

// PrintDefaultAdminInfo 打印默认管理员账户信息
func PrintDefaultAdminInfo(store storagepkg.Storage) {
	ctx := context.Background()

	// 获取管理员账户
	admin, err := store.GetUserByUsername(ctx, "admin")
	if err != nil {
		return
	}

	fmt.Println("")
	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║                     Oppama 管理员账户                        ║")
	fmt.Println("╠════════════════════════════════════════════════════════════╣")
	fmt.Println("║  用户名: admin                                             ║")

	if admin.Status == storagepkg.UserStatusRequirePasswordChange {
		fmt.Println("║  密码: admin                                               ║")
		fmt.Println("║                                                            ║")
		fmt.Println("║  ⚠️  首次登录后请立即修改密码！                            ║")
	} else {
		fmt.Println("║  密码: [使用您设置的密码]                                   ║")
		fmt.Println("║                                                            ║")
		fmt.Println("║  账户已激活                                                ║")
	}
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Println("")
}

// initDefaultAdmin 初始化默认管理员账户
func initDefaultAdmin(store storagepkg.Storage) error {
	ctx := context.Background()

	// 检查是否已存在管理员
	admin, err := store.GetUserByUsername(ctx, "admin")
	if err == nil && admin != nil {
		// 已存在
		return nil
	}

	// 创建默认管理员（获取生成的密码）
	defaultAdmin, _ := storagepkg.DefaultAdminUserWithPassword()

	if err := store.SaveUser(ctx, defaultAdmin); err != nil {
		return fmt.Errorf("保存管理员失败：%w", err)
	}

	return nil
}

// registerRoutes 注册路由
func (s *Server) registerRoutes() {
	// 公开路由（不需要认证）
	publicGroup := s.engine.Group("/")
	{
		// 健康检查
		publicGroup.GET("/health", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})

		// 前端静态资源
		publicGroup.Static("/static", "./web/dist/static")
		publicGroup.Static("/admin/static", "./web/dist/static")

		// 根路径重定向到 /admin
		publicGroup.GET("/", func(c *gin.Context) {
			c.Redirect(http.StatusMovedPermanently, "/admin")
		})

		// 认证相关（公开访问）
		authHandler := v1.NewAuthHandler(s.storage, s.config.Auth, s.rateLimiter, s.blacklist)
		publicGroup.POST("/v1/api/auth/login", middleware.LoginRateLimit(s.rateLimiter), authHandler.Login)
		publicGroup.POST("/v1/api/auth/logout", authHandler.Logout)
	}

	// 需要认证的路由
	jwtCfg := middleware.JWTConfig{
		Secret:     s.config.Auth.JWTSecret,
		ExpireTime: 24 * time.Hour, // 默认值，实际从配置读取
	}
	if s.config.Auth.JWTExpire != "" {
		if duration, err := time.ParseDuration(s.config.Auth.JWTExpire); err == nil {
			jwtCfg.ExpireTime = duration
		}
	}

	// JWT 认证中间件，带黑名单检查
	jwtAuthWithBlacklist := func(c *gin.Context) {
		// 先进行 JWT 验证
		middleware.JWTAuth(jwtCfg)(c)

		// 如果通过验证，检查黑名单
		if !c.IsAborted() && s.config.Auth.EnableBlacklist {
			token, _ := middleware.GetToken(c)
			if token != "" {
				blacklisted, _ := s.blacklist.IsTokenBlacklisted(c.Request.Context(), token)
				if blacklisted {
					c.JSON(http.StatusUnauthorized, gin.H{
						"error": "Token 已失效，请重新登录",
					})
					c.Abort()
					return
				}
			}
		}
	}

	// 创建 Handler
	authHandler := v1.NewAuthHandler(s.storage, s.config.Auth, s.rateLimiter, s.blacklist)
	userHandler := v1.NewUserHandler(s.storage, s.config.Auth)
	serviceHandler := v1.NewServiceHandler(s.storage, s.config, s.configPath, s.taskMgr)
	modelHandler := v1.NewModelHandler(s.storage)
	discoveryHandler := v1.NewDiscoveryHandler(s.storage, s.config, s.configPath, s.taskMgr)
	proxyHandler := v1.NewProxyHandler(s.storage, s.config, s.configPath)
	// 设置 ProxyService 引用（用于配置热重载）
	proxyHandler.SetProxyService(s.proxyService)

	// 设置配置保存后的回调：重新加载搜索引擎配置和检测器配置
	proxyHandler.SetConfigSavedCallback(func() {
		if err := discoveryHandler.ReloadEngines(); err != nil {
			fmt.Printf("警告：重新加载搜索引擎配置失败：%v\n", err)
		}
		if err := serviceHandler.ReloadDetector(); err != nil {
			fmt.Printf("警告：重新加载检测器配置失败：%v\n", err)
		}
		s.UpdateSchedulerDetectorConfig()
	})

	// API v1 路由组（需要认证）
	v1Group := s.engine.Group("/v1/api")
	v1Group.Use(jwtAuthWithBlacklist)
	{
		// 认证相关
		v1Group.GET("/auth/me", authHandler.GetCurrentUser)
		v1Group.POST("/auth/change-password", authHandler.ChangePassword)

		// 服务管理（普通用户可读，管理员可写）
		v1Group.GET("/services", serviceHandler.ListServices)
		v1Group.GET("/services/stats", serviceHandler.GetStats)
		v1Group.GET("/services/:id", serviceHandler.GetService)
		v1Group.GET("/services/:id/check", serviceHandler.CheckService)
		v1Group.GET("/services/:id/models", serviceHandler.GetModels)
		v1Group.GET("/services/tasks/:taskId", serviceHandler.GetServiceTask)

		// 管理员专用服务管理
		adminServiceGroup := v1Group.Group("/services")
		adminServiceGroup.Use(middleware.RequireAdmin())
		{
			adminServiceGroup.POST("", serviceHandler.CreateService)
			adminServiceGroup.PUT("/:id", serviceHandler.UpdateService)
			adminServiceGroup.DELETE("/:id", serviceHandler.DeleteService)
			adminServiceGroup.POST("/:id/check", serviceHandler.CheckService)
			adminServiceGroup.POST("/batch-check", serviceHandler.BatchCheck)
			adminServiceGroup.POST("/check-all", serviceHandler.CheckAllServices)
		}

		// 模型管理
		v1Group.GET("/models", modelHandler.ListModels)
		v1Group.GET("/models/recommend", modelHandler.RecommendModels)

		// 服务发现（管理员）
		discoveryGroup := v1Group.Group("/discovery")
		discoveryGroup.Use(middleware.RequireAdmin())
		{
			discoveryGroup.POST("/search", discoveryHandler.Search)
			discoveryGroup.GET("/tasks/:id", discoveryHandler.GetTask)
			discoveryGroup.POST("/import", discoveryHandler.ImportURLs)
		}

		// 用户管理（管理员）
		userGroup := v1Group.Group("/users")
		userGroup.Use(middleware.RequireAdmin())
		{
			userGroup.GET("", userHandler.ListUsers)
			userGroup.POST("", userHandler.CreateUser)
			userGroup.GET("/:id", userHandler.GetUser)
			userGroup.PUT("/:id", userHandler.UpdateUser)
			userGroup.DELETE("/:id", userHandler.DeleteUser)
			userGroup.POST("/:id/reset-password", userHandler.ResetPassword)
		}

		// 代理配置（管理员）
		proxyGroup := v1Group.Group("/proxy")
		proxyGroup.Use(middleware.RequireAdmin())
		{
			proxyGroup.GET("/config", proxyHandler.GetConfig)
			proxyGroup.PUT("/config", proxyHandler.UpdateConfig)
			proxyGroup.GET("/status", proxyHandler.GetStatus)
			proxyGroup.POST("/test-engines", proxyHandler.TestEngines)
		}

		// 任务管理（管理员）
		taskGroup := v1Group.Group("/tasks")
		taskGroup.Use(middleware.RequireAdmin())
		{
			taskGroup.GET("", s.getTasks)
			taskGroup.GET("/:id", s.getTask)
		}
	}

	// OpenAI 兼容接口（需要认证）
	openaiGroup := s.engine.Group("/v1")
	openaiGroup.Use(jwtAuthWithBlacklist)
	{
		openaiHandler := v1.NewOpenAIHandler(s.storage, s.proxyService)
		openaiGroup.POST("/chat/completions", openaiHandler.ChatCompletions)
		openaiGroup.GET("/models", openaiHandler.ListModels)
		openaiGroup.POST("/chat", openaiHandler.Chat)
	}

	// NoRoute 处理未匹配的路由 - 必须放在最后，用于 SPA 前端路由
	s.engine.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		// 如果是 /admin 开头的路径（但排除静态资源），返回 index.html
		if strings.HasPrefix(path, "/admin") && !strings.HasPrefix(path, "/admin/static") {
			c.File("./web/dist/index.html")
			return
		}
		// 其他路径返回 404
		c.JSON(404, gin.H{"error": "Not Found"})
	})
}

// Run 启动服务器
func (s *Server) Run() error {
	addr := fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.Port)
	return s.engine.Run(addr)
}

// getTasks 获取所有任务
func (s *Server) getTasks(c *gin.Context) {
	taskType := c.Query("type")
	status := c.Query("status")
	limit := 50

	var tasks []*task.Task
	if taskType != "" {
		tasks = s.taskMgr.GetTasksByType(task.TaskType(taskType), limit)
	} else {
		tasks = s.taskMgr.GetActiveTasks()
	}

	// 过滤状态
	if status != "" {
		filtered := make([]*task.Task, 0)
		for _, t := range tasks {
			if string(t.Status) == status {
				filtered = append(filtered, t)
			}
		}
		tasks = filtered
	}

	c.JSON(200, gin.H{
		"data":  tasks,
		"total": len(tasks),
	})
}

// getTask 获取单个任务
func (s *Server) getTask(c *gin.Context) {
	id := c.Param("id")

	task := s.taskMgr.GetTask(id)
	if task == nil {
		c.JSON(404, gin.H{"error": "任务不存在"})
		return
	}

	c.JSON(200, gin.H{"data": task})
}
