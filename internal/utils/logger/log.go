// Package logger 提供便捷的日志记录功能
package logger

import "sync"

var (
	// 预定义的模块日志器
	serverLogger     *ModuleLogger
	storageLogger    *ModuleLogger
	proxyLogger      *ModuleLogger
	detectorLogger   *ModuleLogger
	schedulerLogger  *ModuleLogger
	apiLogger        *ModuleLogger
	authLogger       *ModuleLogger
	discoveryLogger  *ModuleLogger
	hunterLogger     *ModuleLogger
	shodanLogger     *ModuleLogger
	fofaLogger       *ModuleLogger
	taskLogger       *ModuleLogger
	middlewareLogger *ModuleLogger

	initOnce sync.Once
)

// initLoggers 初始化所有预定义日志器
func initLoggers() {
	serverLogger = GetLogger(PrefixServer, ColorCyan)
	storageLogger = GetLogger(PrefixStorage, ColorGreen)
	proxyLogger = GetLogger(PrefixProxy, ColorBlue)
	detectorLogger = GetLogger(PrefixDetector, ColorYellow)
	schedulerLogger = GetLogger(PrefixScheduler, ColorPurple)
	apiLogger = GetLogger(PrefixAPI, ColorCyan)
	authLogger = GetLogger(PrefixAuth, ColorRed)
	discoveryLogger = GetLogger(PrefixDiscovery, ColorGreen)
	hunterLogger = GetLogger(PrefixHunter, ColorBlue)
	shodanLogger = GetLogger(PrefixShodan, ColorPurple)
	fofaLogger = GetLogger(PrefixFOFA, ColorCyan)
	taskLogger = GetLogger(PrefixTask, ColorYellow)
	middlewareLogger = GetLogger(PrefixMiddleware, ColorGray)
}

// Server 返回服务器日志器
func Server() *ModuleLogger {
	initOnce.Do(initLoggers)
	return serverLogger
}

// Storage 返回存储日志器
func Storage() *ModuleLogger {
	initOnce.Do(initLoggers)
	return storageLogger
}

// Proxy 返回代理日志器
func Proxy() *ModuleLogger {
	initOnce.Do(initLoggers)
	return proxyLogger
}

// Detector 返回检测器日志器
func Detector() *ModuleLogger {
	initOnce.Do(initLoggers)
	return detectorLogger
}

// Scheduler 返回调度器日志器
func Scheduler() *ModuleLogger {
	initOnce.Do(initLoggers)
	return schedulerLogger
}

// API 返回 API 日志器
func API() *ModuleLogger {
	initOnce.Do(initLoggers)
	return apiLogger
}

// Auth 返回认证日志器
func Auth() *ModuleLogger {
	initOnce.Do(initLoggers)
	return authLogger
}

// Discovery 返回发现服务日志器
func Discovery() *ModuleLogger {
	initOnce.Do(initLoggers)
	return discoveryLogger
}

// Hunter 返回 Hunter 日志器
func Hunter() *ModuleLogger {
	initOnce.Do(initLoggers)
	return hunterLogger
}

// Shodan 返回 Shodan 日志器
func Shodan() *ModuleLogger {
	initOnce.Do(initLoggers)
	return shodanLogger
}

// FOFA 返回 FOFA 日志器
func FOFA() *ModuleLogger {
	initOnce.Do(initLoggers)
	return fofaLogger
}

// Task 返回任务日志器
func Task() *ModuleLogger {
	initOnce.Do(initLoggers)
	return taskLogger
}

// Middleware 返回中间件日志器
func Middleware() *ModuleLogger {
	initOnce.Do(initLoggers)
	return middlewareLogger
}
