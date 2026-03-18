package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// 版本信息
const Version = "1.0.0"

// Config 应用配置
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Proxy     ProxyConfig     `yaml:"proxy"`
	Auth      AuthConfig      `yaml:"auth"`
	CORS      CORSConfig      `yaml:"cors"`
	Detector  DetectorConfig  `yaml:"detector"`
	Discovery DiscoveryConfig `yaml:"discovery"`
	Storage   StorageConfig   `yaml:"storage"`
	Log       LogConfig       `yaml:"log"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
	Mode string `yaml:"mode"` // debug, release, test
}

// ProxyConfig 代理配置
type ProxyConfig struct {
	Enabled         bool            `yaml:"enabled"`
	EnableAuth      bool            `yaml:"enable_auth"`
	APIKey          string          `yaml:"api_key"`
	DefaultModel    string          `yaml:"default_model"`
	FallbackEnabled bool            `yaml:"fallback_enabled"`
	MaxRetries      int             `yaml:"max_retries"`
	Timeout         int             `yaml:"timeout"`
	RateLimit       RateLimitConfig `yaml:"rate_limit"`
	// HTTP 代理配置（用于访问后端 Ollama 服务）
	HTTPProxy  string `yaml:"http_proxy"`  // HTTP 代理地址，如 http://proxy.example.com:8080
	HTTPSProxy string `yaml:"https_proxy"` // HTTPS 代理地址
	NoProxy    string `yaml:"no_proxy"`    // 不使用代理的地址列表，逗号分隔
}

// RateLimitConfig 限流配置
type RateLimitConfig struct {
	Enabled           bool `yaml:"enabled"`
	RequestsPerMinute int  `yaml:"requests_per_minute"`
}

// DetectorConfig 检测器配置
type DetectorConfig struct {
	Concurrency       int            `yaml:"concurrency"`
	Timeout           int            `yaml:"timeout"`
	CheckInterval     int            `yaml:"check_interval"`      // 健康检查间隔（秒）
	ModelSyncInterval int            `yaml:"model_sync_interval"` // 模型同步间隔（秒）
	HoneypotDetection HoneypotConfig `yaml:"honeypot_detection"`
}

// HoneypotConfig 蜜罐检测配置
type HoneypotConfig struct {
	Enabled         bool     `yaml:"enabled"`
	SuspiciousPorts []int    `yaml:"suspicious_ports"`
	FakeVersions    []string `yaml:"fake_versions"`
	Threshold       int      `yaml:"threshold"` // 蜜罐判定阈值
}

// DiscoveryConfig 服务发现配置
type DiscoveryConfig struct {
	AutoDiscovery AutoDiscoveryConfig `yaml:"auto_discovery"`
	Engines       SearchEngineConfig  `yaml:"engines"`
}

// AutoDiscoveryConfig 自动发现配置
type AutoDiscoveryConfig struct {
	Enabled  bool `yaml:"enabled"`
	Interval int  `yaml:"interval"`
}

// SearchEngineConfig 搜索引擎配置
type SearchEngineConfig struct {
	FOFA   FOFAConfig   `yaml:"fofa"`
	Hunter HunterConfig `yaml:"hunter"`
	Shodan ShodanConfig `yaml:"shodan"`
}

// FOFAConfig FOFA API 配置
type FOFAConfig struct {
	Enabled    bool   `yaml:"enabled"`
	Email      string `yaml:"email"`
	Key        string `yaml:"key"`
	Query      string `yaml:"query"`
	MaxResults int    `yaml:"max_results"`
}

// HunterConfig Hunter API 配置
type HunterConfig struct {
	Enabled    bool   `yaml:"enabled"`
	Key        string `yaml:"key"`
	Query      string `yaml:"query"`
	MaxResults int    `yaml:"max_results"`
}

// ShodanConfig Shodan API 配置
type ShodanConfig struct {
	Enabled    bool   `yaml:"enabled"`
	Key        string `yaml:"key"`
	Query      string `yaml:"query"`
	MaxResults int    `yaml:"max_results"`
}

// StorageConfig 存储配置
type StorageConfig struct {
	Type          string          `yaml:"type"` // memory, sqlite, postgres
	SQLite        SQLiteConfig    `yaml:"sqlite"`
	Postgres      PostgresConfig  `yaml:"postgres"`
	RetentionDays int             `yaml:"retention_days"`
	Pool          PoolConfig      `yaml:"pool"` // 连接池配置
}

// SQLiteConfig SQLite 配置
type SQLiteConfig struct {
	Path string `yaml:"path"`
}

// PostgresConfig PostgreSQL 配置
type PostgresConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Database string `yaml:"database"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
}

// PoolConfig 数据库连接池配置
type PoolConfig struct {
	MaxOpenConns    int `yaml:"max_open_conns"`    // 最大打开连接数
	MaxIdleConns    int `yaml:"max_idle_conns"`    // 最大空闲连接数
	ConnMaxLifetime int `yaml:"conn_max_lifetime"` // 连接最大生命周期（秒）
	ConnMaxIdleTime int `yaml:"conn_max_idle_time"` // 连接最大空闲时间（秒）
}

// LogConfig 日志配置
type LogConfig struct {
	Level      string `yaml:"level"`  // debug, info, warn, error
	Format     string `yaml:"format"` // json, text
	Output     string `yaml:"output"`
	MaxSize    int    `yaml:"max_size"` // MB
	MaxBackups int    `yaml:"max_backups"`
	MaxAge     int    `yaml:"max_age"` // days
}

// AuthConfig 认证配置
type AuthConfig struct {
	Enabled         bool           `yaml:"enabled"`
	JWTSecret       string         `yaml:"jwt_secret"`
	JWTExpire       string         `yaml:"jwt_expire"`
	EnableBlacklist bool           `yaml:"enable_blacklist"`
	BlacklistTTL    string         `yaml:"blacklist_ttl"`
	PasswordPolicy  PasswordPolicy `yaml:"password_policy"`
	LoginRateLimit  LoginRateLimit `yaml:"login_rate_limit"`
	SessionTimeout  string         `yaml:"session_timeout"`
}

// PasswordPolicy 密码策略配置
type PasswordPolicy struct {
	MinLength      int  `yaml:"min_length"`
	RequireUpper   bool `yaml:"require_upper"`
	RequireLower   bool `yaml:"require_lower"`
	RequireNumbers bool `yaml:"require_numbers"`
	RequireSpecial bool `yaml:"require_special"`
}

// LoginRateLimit 登录限流配置
type LoginRateLimit struct {
	MaxAttempts    int `yaml:"max_attempts"`
	WindowMinutes  int `yaml:"window_minutes"`
	LockoutMinutes int `yaml:"lockout_minutes"`
}

// CORSConfig 跨域配置
type CORSConfig struct {
	AllowedOrigins   []string `yaml:"allowed_origins"`
	AllowedMethods   []string `yaml:"allowed_methods"`
	AllowedHeaders   []string `yaml:"allowed_headers"`
	AllowCredentials bool     `yaml:"allow_credentials"`
	MaxAge           int      `yaml:"max_age"`
}

// Load 加载配置
func Load(configPath string) (*Config, error) {
	cfg := &Config{}

	// 读取配置文件
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败：%w", err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败：%w", err)
	}

	// 设置默认值
	setDefaults(cfg)

	// 验证配置
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("配置验证失败：%w", err)
	}

	return cfg, nil
}

// setDefaults 设置默认值
func setDefaults(cfg *Config) {
	// Server 默认值
	if cfg.Server.Host == "" {
		cfg.Server.Host = "0.0.0.0"
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.Server.Mode == "" {
		cfg.Server.Mode = "release"
	}

	// Proxy 默认值
	if cfg.Proxy.MaxRetries == 0 {
		cfg.Proxy.MaxRetries = 3
	}
	if cfg.Proxy.Timeout == 0 {
		cfg.Proxy.Timeout = 120
	}

	// Auth 默认值
	if cfg.Auth.JWTSecret == "" {
		cfg.Auth.JWTSecret = "change-this-secret-in-production"
	}
	if cfg.Auth.JWTExpire == "" {
		cfg.Auth.JWTExpire = "24h"
	}
	if cfg.Auth.BlacklistTTL == "" {
		cfg.Auth.BlacklistTTL = "24h"
	}
	if cfg.Auth.SessionTimeout == "" {
		cfg.Auth.SessionTimeout = "168h" // 7 天
	}
	if cfg.Auth.PasswordPolicy.MinLength == 0 {
		cfg.Auth.PasswordPolicy.MinLength = 8
	}
	if cfg.Auth.LoginRateLimit.MaxAttempts == 0 {
		cfg.Auth.LoginRateLimit.MaxAttempts = 5
	}
	if cfg.Auth.LoginRateLimit.WindowMinutes == 0 {
		cfg.Auth.LoginRateLimit.WindowMinutes = 5
	}
	if cfg.Auth.LoginRateLimit.LockoutMinutes == 0 {
		cfg.Auth.LoginRateLimit.LockoutMinutes = 30
	}

	// CORS 默认值
	if len(cfg.CORS.AllowedOrigins) == 0 {
		cfg.CORS.AllowedOrigins = []string{"http://localhost:5173", "http://localhost:8080"}
	}
	if len(cfg.CORS.AllowedMethods) == 0 {
		cfg.CORS.AllowedMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	}
	if len(cfg.CORS.AllowedHeaders) == 0 {
		cfg.CORS.AllowedHeaders = []string{"Origin", "Content-Type", "Authorization"}
	}

	// Detector 默认值
	if cfg.Detector.Concurrency == 0 {
		cfg.Detector.Concurrency = 10
	}
	if cfg.Detector.Timeout == 0 {
		cfg.Detector.Timeout = 30
	}
	if cfg.Detector.CheckInterval == 0 {
		cfg.Detector.CheckInterval = 300 // 5 分钟
	}
	if cfg.Detector.ModelSyncInterval == 0 {
		cfg.Detector.ModelSyncInterval = 600 // 10 分钟
	}
	if cfg.Detector.HoneypotDetection.Threshold == 0 {
		cfg.Detector.HoneypotDetection.Threshold = 60
	}

	// Storage 默认值
	if cfg.Storage.Type == "" {
		cfg.Storage.Type = "sqlite"
	}
	if cfg.Storage.SQLite.Path == "" {
		cfg.Storage.SQLite.Path = "./data/oppama.db"
	}
	if cfg.Storage.RetentionDays == 0 {
		cfg.Storage.RetentionDays = 30
	}
	// 连接池默认值
	if cfg.Storage.Pool.MaxOpenConns == 0 {
		cfg.Storage.Pool.MaxOpenConns = 25
	}
	if cfg.Storage.Pool.MaxIdleConns == 0 {
		cfg.Storage.Pool.MaxIdleConns = 5
	}
	if cfg.Storage.Pool.ConnMaxLifetime == 0 {
		cfg.Storage.Pool.ConnMaxLifetime = 1800 // 30 分钟
	}
	if cfg.Storage.Pool.ConnMaxIdleTime == 0 {
		cfg.Storage.Pool.ConnMaxIdleTime = 300 // 5 分钟
	}

	// Log 默认值
	if cfg.Log.Level == "" {
		cfg.Log.Level = "info"
	}
	if cfg.Log.Format == "" {
		cfg.Log.Format = "json"
	}
	if cfg.Log.Output == "" {
		cfg.Log.Output = "./logs/server.log"
	}
	if cfg.Log.MaxSize == 0 {
		cfg.Log.MaxSize = 100
	}
	if cfg.Log.MaxBackups == 0 {
		cfg.Log.MaxBackups = 5
	}
	if cfg.Log.MaxAge == 0 {
		cfg.Log.MaxAge = 30
	}
}

// Validate 验证配置
func (c *Config) Validate() error {
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("无效的服务器端口：%d", c.Server.Port)
	}
	if c.Detector.Concurrency < 1 {
		return fmt.Errorf("无效的并发数：%d", c.Detector.Concurrency)
	}
	return nil
}

// Save 保存配置到文件
func (c *Config) Save(configPath string) error {
	// 创建备份（如果文件已存在）
	if _, err := os.Stat(configPath); err == nil {
		backupPath := configPath + ".bak"
		if err := copyFile(configPath, backupPath); err != nil {
			// 备份失败不影响保存，只记录警告
			fmt.Printf("警告：创建配置备份失败：%v\n", err)
		}
	}

	// 序列化配置为 YAML
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("序列化配置失败：%w", err)
	}

	// 确保目录存在
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建配置目录失败：%w", err)
	}

	// 写入文件
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("保存配置文件失败：%w", err)
	}

	return nil
}

// copyFile 复制文件
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

// GetVersionURL 获取版本检查 URL
func (c *Config) GetVersionURL(serviceURL string) string {
	return fmt.Sprintf("%s/api/version", serviceURL)
}

// GetModelsURL 获取模型列表 URL
func (c *Config) GetModelsURL(serviceURL string) string {
	return fmt.Sprintf("%s/api/tags", serviceURL)
}

// GetGenerateURL 获取生成接口 URL
func (c *Config) GetGenerateURL(serviceURL string) string {
	return fmt.Sprintf("%s/api/generate", serviceURL)
}

// GetChatURL 获取聊天接口 URL
func (c *Config) GetChatURL(serviceURL string) string {
	return fmt.Sprintf("%s/api/chat", serviceURL)
}

// TimeoutDuration 获取超时时间
func (c *DetectorConfig) TimeoutDuration() time.Duration {
	return time.Duration(c.Timeout) * time.Second
}

// TimeoutDuration 获取代理超时
func (c *ProxyConfig) TimeoutDuration() time.Duration {
	return time.Duration(c.Timeout) * time.Second
}
