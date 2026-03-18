// Package logger 提供统一的日志记录功能
package logger

import (
	"fmt"
	"log"
	"os"
	"sync"
)

// 定义不同模块的日志前缀
const (
	PrefixServer     = "[Server]"
	PrefixStorage    = "[Storage]"
	PrefixProxy      = "[Proxy]"
	PrefixDetector   = "[Detector]"
	PrefixScheduler  = "[Scheduler]"
	PrefixAPI        = "[API]"
	PrefixAuth       = "[Auth]"
	PrefixDiscovery  = "[Discovery]"
	PrefixHunter     = "[Hunter]"
	PrefixShodan     = "[Shodan]"
	PrefixFOFA       = "[FOFA]"
	PrefixTask       = "[Task]"
	PrefixMiddleware = "[Middleware]"
)

// 颜色代码 (ANSI)
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorPurple = "\033[35m"
	ColorCyan   = "\033[36m"
	ColorGray   = "\033[37m"
	ColorWhite  = "\033[97m"
)

// ModuleLogger 模块日志记录器
type ModuleLogger struct {
	prefix string
	color  string
	mu     sync.Mutex
}

// 全局日志实例
var (
	globalMu      sync.RWMutex
	moduleLoggers = make(map[string]*ModuleLogger)
	useColor      = true
)

// init 初始化默认日志配置
func init() {
	// 检测是否支持颜色
	if os.Getenv("NO_COLOR") != "" {
		useColor = false
	}
}

// GetLogger 获取模块日志记录器
func GetLogger(prefix string, color string) *ModuleLogger {
	globalMu.RLock()
	logger, exists := moduleLoggers[prefix]
	globalMu.RUnlock()

	if exists {
		return logger
	}

	globalMu.Lock()
	defer globalMu.Unlock()

	// 双重检查
	if logger, exists = moduleLoggers[prefix]; exists {
		return logger
	}

	logger = &ModuleLogger{
		prefix: prefix,
		color:  color,
	}
	moduleLoggers[prefix] = logger

	return logger
}

// formatMessage 格式化日志消息
func (l *ModuleLogger) formatMessage(format string, a ...interface{}) string {
	l.mu.Lock()
	defer l.mu.Unlock()

	msg := fmt.Sprintf(format, a...)

	if useColor && l.color != "" {
		return fmt.Sprintf("%s%s %s%s", l.color, l.prefix, msg, ColorReset)
	}
	return fmt.Sprintf("%s %s", l.prefix, msg)
}

// Info 打印信息级别日志
func (l *ModuleLogger) Info(format string, a ...interface{}) {
	log.Println(l.formatMessage(format, a...))
}

// Error 打印错误级别日志
func (l *ModuleLogger) Error(format string, a ...interface{}) {
	log.Println(l.formatMessage(format, a...))
}

// Warn 打印警告级别日志
func (l *ModuleLogger) Warn(format string, a ...interface{}) {
	log.Println(l.formatMessage(format, a...))
}

// Warnf 格式化打印警告级别日志
func (l *ModuleLogger) Warnf(format string, a ...interface{}) {
	log.Println(l.formatMessage(format, a...))
}

// Error 打印错误级别日志
func (l *ModuleLogger) Debug(format string, a ...interface{}) {
	log.Println(l.formatMessage(format, a...))
}

// Fatal 打印致命错误并退出
func (l *ModuleLogger) Fatal(format string, a ...interface{}) {
	log.Fatal(l.formatMessage(format, a...))
}

// Fatalf 格式化打印致命错误并退出
func (l *ModuleLogger) Fatalf(format string, a ...interface{}) {
	log.Fatalf(l.formatMessage(format, a...))
}

// Panic 打印致命错误并 panic
func (l *ModuleLogger) Panic(format string, a ...interface{}) {
	log.Panic(l.formatMessage(format, a...))
}

// Print 打印日志 (无格式)
func (l *ModuleLogger) Print(a ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if useColor && l.color != "" {
		fmt.Printf("%s%s %v%s\n", l.color, l.prefix, fmt.Sprint(a...), ColorReset)
	} else {
		fmt.Printf("%s %v\n", l.prefix, fmt.Sprint(a...))
	}
}

// Printf 格式化打印日志
func (l *ModuleLogger) Printf(format string, a ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	msg := fmt.Sprintf(format, a...)

	if useColor && l.color != "" {
		fmt.Printf("%s%s %s%s\n", l.color, l.prefix, msg, ColorReset)
	} else {
		fmt.Printf("%s %s\n", l.prefix, msg)
	}
}

// Println 打印日志并换行
func (l *ModuleLogger) Println(a ...interface{}) {
	l.Print(a...)
}

// SetColorEnabled 启用/禁用颜色
func SetColorEnabled(enabled bool) {
	globalMu.Lock()
	defer globalMu.Unlock()
	useColor = enabled
}

// IsColorEnabled 检查是否启用了颜色
func IsColorEnabled() bool {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return useColor
}
