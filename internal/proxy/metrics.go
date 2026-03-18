package proxy

import (
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics 收集器
type Metrics struct {
	// 请求相关
	requestsTotal     *prometheus.CounterVec
	requestDuration   *prometheus.HistogramVec
	activeRequests    prometheus.Gauge

	// 响应相关
	responseSize      *prometheus.HistogramVec
	responseErrors    *prometheus.CounterVec

	// 服务相关
	serviceHealth     *prometheus.GaugeVec
	serviceLatency    *prometheus.HistogramVec
	serviceRequests   *prometheus.CounterVec

	// 缓存相关
	cacheHits         *prometheus.CounterVec
	cacheMisses       *prometheus.CounterVec

	// 会话相关
	activeSessions    prometheus.Gauge
	sessionDuration   *prometheus.HistogramVec

	// 工具调用相关
	toolCallsTotal    *prometheus.CounterVec
	toolCallDuration  *prometheus.HistogramVec
	toolCallErrors    *prometheus.CounterVec

	registry          *prometheus.Registry
	mu                sync.RWMutex
}

// NewMetrics 创建新的指标收集器
func NewMetrics() *Metrics {
	m := &Metrics{
		registry: prometheus.NewRegistry(),
	}

	m.initMetrics()
	m.registerMetrics()

	return m
}

// initMetrics 初始化指标
func (m *Metrics) initMetrics() {
	// 请求总数
	m.requestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "oppama_proxy_requests_total",
			Help: "Total number of requests proxied",
		},
		[]string{"model", "service_id", "status"},
	)

	// 请求延迟
	m.requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "oppama_proxy_request_duration_seconds",
			Help:    "Request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"model", "service_id"},
	)

	// 活跃请求数
	m.activeRequests = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "oppama_proxy_active_requests",
			Help: "Number of active requests",
		},
	)

	// 响应大小
	m.responseSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "oppama_proxy_response_size_bytes",
			Help:    "Response size in bytes",
			Buckets: []float64{100, 1000, 10000, 100000, 1000000, 10000000},
		},
		[]string{"model"},
	)

	// 响应错误
	m.responseErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "oppama_proxy_response_errors_total",
			Help: "Total number of response errors",
		},
		[]string{"model", "error_type"},
	)

	// 服务健康状态
	m.serviceHealth = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "oppama_service_health",
			Help: "Service health status (1=online, 0=offline)",
		},
		[]string{"service_id", "service_url"},
	)

	// 服务延迟
	m.serviceLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "oppama_service_latency_seconds",
			Help:    "Service response latency",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"service_id", "model"},
	)

	// 服务请求计数
	m.serviceRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "oppama_service_requests_total",
			Help: "Total requests per service",
		},
		[]string{"service_id", "model"},
	)

	// 缓存命中
	m.cacheHits = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "oppama_cache_hits_total",
			Help: "Total cache hits",
		},
		[]string{"cache_type"}, // service, response
	)

	// 缓存未命中
	m.cacheMisses = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "oppama_cache_misses_total",
			Help: "Total cache misses",
		},
		[]string{"cache_type"}, // service, response
	)

	// 活跃会话数
	m.activeSessions = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "oppama_active_sessions",
			Help: "Number of active sessions",
		},
	)

	// 会话持续时间
	m.sessionDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "oppama_session_duration_seconds",
			Help:    "Session duration in seconds",
			Buckets: []float64{60, 300, 600, 1800, 3600, 7200},
		},
		[]string{"service_id"},
	)

	// 工具调用总数
	m.toolCallsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "oppama_tool_calls_total",
			Help: "Total number of tool calls",
		},
		[]string{"tool_name", "status"},
	)

	// 工具调用延迟
	m.toolCallDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "oppama_tool_call_duration_seconds",
			Help:    "Tool call duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"tool_name"},
	)

	// 工具调用错误
	m.toolCallErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "oppama_tool_call_errors_total",
			Help: "Total number of tool call errors",
		},
		[]string{"tool_name", "error_type"},
	)
}

// registerMetrics 注册指标
func (m *Metrics) registerMetrics() {
	m.registry.MustRegister(m.requestsTotal)
	m.registry.MustRegister(m.requestDuration)
	m.registry.MustRegister(m.activeRequests)
	m.registry.MustRegister(m.responseSize)
	m.registry.MustRegister(m.responseErrors)
	m.registry.MustRegister(m.serviceHealth)
	m.registry.MustRegister(m.serviceLatency)
	m.registry.MustRegister(m.serviceRequests)
	m.registry.MustRegister(m.cacheHits)
	m.registry.MustRegister(m.cacheMisses)
	m.registry.MustRegister(m.activeSessions)
	m.registry.MustRegister(m.sessionDuration)
	m.registry.MustRegister(m.toolCallsTotal)
	m.registry.MustRegister(m.toolCallDuration)
	m.registry.MustRegister(m.toolCallErrors)
}

// RecordRequest 记录请求
func (m *Metrics) RecordRequest(model, serviceID, status string, duration time.Duration) {
	m.requestsTotal.WithLabelValues(model, serviceID, status).Inc()
	m.requestDuration.WithLabelValues(model, serviceID).Observe(duration.Seconds())
}

// IncActiveRequests 增加活跃请求
func (m *Metrics) IncActiveRequests() {
	m.activeRequests.Inc()
}

// DecActiveRequests 减少活跃请求
func (m *Metrics) DecActiveRequests() {
	m.activeRequests.Dec()
}

// RecordResponseSize 记录响应大小
func (m *Metrics) RecordResponseSize(model string, size int) {
	m.responseSize.WithLabelValues(model).Observe(float64(size))
}

// RecordError 记录错误
func (m *Metrics) RecordError(model, errorType string) {
	m.responseErrors.WithLabelValues(model, errorType).Inc()
}

// UpdateServiceHealth 更新服务健康状态
func (m *Metrics) UpdateServiceHealth(serviceID, serviceURL string, isOnline bool) {
	value := 0.0
	if isOnline {
		value = 1.0
	}
	m.serviceHealth.WithLabelValues(serviceID, serviceURL).Set(value)
}

// RecordServiceLatency 记录服务延迟
func (m *Metrics) RecordServiceLatency(serviceID, model string, latency time.Duration) {
	m.serviceLatency.WithLabelValues(serviceID, model).Observe(latency.Seconds())
	m.serviceRequests.WithLabelValues(serviceID, model).Inc()
}

// RecordCacheHit 记录缓存命中
func (m *Metrics) RecordCacheHit(cacheType string) {
	m.cacheHits.WithLabelValues(cacheType).Inc()
}

// RecordCacheMiss 记录缓存未命中
func (m *Metrics) RecordCacheMiss(cacheType string) {
	m.cacheMisses.WithLabelValues(cacheType).Inc()
}

// SetActiveSessions 设置活跃会话数
func (m *Metrics) SetActiveSessions(count int) {
	m.activeSessions.Set(float64(count))
}

// RecordSessionDuration 记录会话持续时间
func (m *Metrics) RecordSessionDuration(serviceID string, duration time.Duration) {
	m.sessionDuration.WithLabelValues(serviceID).Observe(duration.Seconds())
}

// RecordToolCall 记录工具调用
func (m *Metrics) RecordToolCall(toolName, status string, duration time.Duration) {
	m.toolCallsTotal.WithLabelValues(toolName, status).Inc()
	m.toolCallDuration.WithLabelValues(toolName).Observe(duration.Seconds())
}

// RecordToolCallError 记录工具调用错误
func (m *Metrics) RecordToolCallError(toolName, errorType string) {
	m.toolCallErrors.WithLabelValues(toolName, errorType).Inc()
}

// Handler 返回 Prometheus HTTP handler
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	})
}

// GetMetricsAsMap 获取指标数据（用于调试）
func (m *Metrics) GetMetricsAsMap() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"active_requests":   m.activeRequests.Desc().String(),
		"active_sessions":   m.activeSessions.Desc().String(),
	}
}

// StartMetricsServer 启动指标服务器
