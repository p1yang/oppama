package storage

import (
	"time"
)

// ServiceStatus 服务状态
type ServiceStatus string

const (
	StatusUnknown  ServiceStatus = "unknown"
	StatusOnline   ServiceStatus = "online"
	StatusOffline  ServiceStatus = "offline"
	StatusSlow     ServiceStatus = "slow"
	StatusHoneypot ServiceStatus = "honeypot"
	StatusError    ServiceStatus = "error"
)

// DiscoverySource 发现来源
type DiscoverySource string

const (
	SourceManual  DiscoverySource = "manual"
	SourceFOFA    DiscoverySource = "fofa"
	SourceHunter  DiscoverySource = "hunter"
	SourceZoomEye DiscoverySource = "zoomeye"
	SourceShodan  DiscoverySource = "shodan"
	SourceImport  DiscoverySource = "import"
)

// OllamaService Ollama 服务实例
type OllamaService struct {
	ID           string                 `json:"id"`
	URL          string                 `json:"url"`
	Name         string                 `json:"name"`
	Status       ServiceStatus          `json:"status"`
	Version      string                 `json:"version"`
	Models       []ModelInfo            `json:"models"`
	LastChecked  time.Time              `json:"last_checked"`
	ResponseTime time.Duration          `json:"response_time"`
	IsHoneypot   bool                   `json:"is_honeypot"`
	RequiresAuth bool                   `json:"requires_auth"`
	Country      string                 `json:"country"`
	Region       string                 `json:"region"`
	City         string                 `json:"city"`
	ISP          string                 `json:"isp"`
	Source       DiscoverySource        `json:"source"`
	Metadata     map[string]interface{} `json:"metadata"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

// ModelInfo 模型信息
type ModelInfo struct {
	ID            string    `json:"id"`
	ServiceID     string    `json:"service_id"`
	Name          string    `json:"name"`
	Size          int64     `json:"size"` // 字节
	Digest        string    `json:"digest"`
	Family        string    `json:"family"`
	Format        string    `json:"format"`
	ParameterSize string    `json:"parameter_size"`
	QuantLevel    string    `json:"quantization_level"`
	IsAvailable   bool      `json:"is_available"`
	LastTested    time.Time `json:"last_tested"`
}

// DetectionResult 检测结果
type DetectionResult struct {
	URL             string        `json:"url"`
	IsValid         bool          `json:"is_valid"`
	Error           string        `json:"error,omitempty"`
	Version         string        `json:"version"`
	Models          []ModelInfo   `json:"models"`
	ResponseTime    time.Duration `json:"response_time"`
	IsHoneypot      bool          `json:"is_honeypot"`
	HoneypotReasons []string      `json:"honeypot_reasons,omitempty"`
	RequiresAuth    bool          `json:"requires_auth"`
	CheckedAt       time.Time     `json:"checked_at"`
}

// BatchDetectionRequest 批量检测请求
type BatchDetectionRequest struct {
	URLs          []string `json:"urls"`
	Concurrency   int      `json:"concurrency"`
	Timeout       int      `json:"timeout"` // 秒
	CheckModels   bool     `json:"check_models"`
	CheckHoneypot bool     `json:"check_honeypot"`
}

// BatchDetectionResponse 批量检测响应
type BatchDetectionResponse struct {
	Total    int               `json:"total"`
	Success  int               `json:"success"`
	Failed   int               `json:"failed"`
	Results  []DetectionResult `json:"results"`
	Duration string            `json:"duration"` // 格式化后的持续时间
}

// ProxyConfig 代理配置
type ProxyConfigStruct struct {
	ID               string `json:"id"`
	ListenAddr       string `json:"listen_addr"`
	ListenPort       int    `json:"listen_port"`
	EnableAuth       bool   `json:"enable_auth"`
	APIKey           string `json:"api_key"`
	DefaultModel     string `json:"default_model"`
	FallbackEnabled  bool   `json:"fallback_enabled"`
	MaxRetries       int    `json:"max_retries"`
	Timeout          int    `json:"timeout"`
	RateLimitEnabled bool   `json:"rate_limit_enabled"`
	RateLimitRPM     int    `json:"rate_limit_rpm"`
}

// DiscoveryTask 发现任务
type DiscoveryTask struct {
	ID          string            `json:"id"`
	Engines     []DiscoverySource `json:"engines"`
	Query       string            `json:"query"`
	MaxResults  int               `json:"max_results"`
	Status      TaskStatus        `json:"status"`
	Progress    int               `json:"progress"`
	Total       int               `json:"total"`
	FoundCount  int               `json:"found_count"`
	StartedAt   time.Time         `json:"started_at"`
	CompletedAt time.Time         `json:"completed_at"`
	Results     []string          `json:"results"`
	CreatedAt   time.Time         `json:"created_at"`
}

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskPending   TaskStatus = "pending"
	TaskRunning   TaskStatus = "running"
	TaskCompleted TaskStatus = "completed"
	TaskFailed    TaskStatus = "failed"
)

// Stats 统计数据
type Stats struct {
	TotalServices    int       `json:"total_services"`
	OnlineServices   int       `json:"online_services"`
	OfflineServices  int       `json:"offline_services"`
	HoneypotServices int       `json:"honeypot_services"`
	TotalModels      int       `json:"total_models"`
	AvailableModels  int       `json:"available_models"`
	Timestamp        time.Time `json:"timestamp"`
}

// ServiceFilter 服务查询过滤条件
type ServiceFilter struct {
	Status     *ServiceStatus `form:"status"`
	Source     *DiscoverySource `form:"source"`
	HasModels  *bool          `form:"has_models"`
	IsHoneypot *bool          `form:"is_honeypot"`
	Search     string         `form:"search"`
	Page       int            `form:"page"`
	PageSize   int            `form:"limit"` // 前端发送的是 limit
}

// ModelFilter 模型查询过滤条件
type ModelFilter struct {
	Family        string `form:"family"`
	MinSize       int64  `form:"min_size"`
	AvailableOnly bool   `form:"available"`
	ServiceID     string `form:"service_id"`
}

// Task 通用异步任务
type Task struct {
	ID          string     `json:"id"`
	Type        string     `json:"type"`
	Title       string     `json:"title"`
	Status      TaskStatus `json:"status"`
	Progress    int        `json:"progress"`
	Total       int        `json:"total"`
	Result      string     `json:"result,omitempty"` // JSON string
	Error       string     `json:"error,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// TaskFilter 任务查询过滤条件
type TaskFilter struct {
	Type   string
	Status TaskStatus
	Limit  int
}
