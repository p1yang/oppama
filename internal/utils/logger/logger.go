// Package logger 提供统一的日志记录功能
package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
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

// LogLevel 日志级别类型
type LogLevel int

const (
	// DebugLevel 显示所有日志
	DebugLevel LogLevel = iota
	// InfoLevel 显示 Info、Warn、Error 日志
	InfoLevel
	// WarnLevel 显示 Warn、Error 日志
	WarnLevel
	// ErrorLevel 只显示 Error 日志
	ErrorLevel
	// FatalLevel 只显示 Fatal 日志
	FatalLevel
)

// String 返回日志级别名称
func (l LogLevel) String() string {
	switch l {
	case DebugLevel:
		return "DEBUG"
	case InfoLevel:
		return "INFO"
	case WarnLevel:
		return "WARN"
	case ErrorLevel:
		return "ERROR"
	case FatalLevel:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// ParseLogLevel 解析日志级别字符串
func ParseLogLevel(level string) LogLevel {
	switch level {
	case "DEBUG", "debug":
		return DebugLevel
	case "INFO", "info":
		return InfoLevel
	case "WARN", "warn", "WARNING", "warning":
		return WarnLevel
	case "ERROR", "error":
		return ErrorLevel
	case "FATAL", "fatal":
		return FatalLevel
	default:
		return InfoLevel // 默认 Info 级别
	}
}

// ModuleLogger 模块日志记录器
type ModuleLogger struct {
	prefix string
	color  string
	mu     sync.Mutex
	logger *log.Logger // 实际写入的 logger
}

// 全局日志实例
var (
	globalMu      sync.RWMutex
	moduleLoggers = make(map[string]*ModuleLogger)
	useColor      = true
	logWriter     io.Writer // 日志写入目标（文件或控制台）
	logFile       *os.File  // 日志文件句柄
	logLevel      LogLevel  // 全局日志级别
)

// InitFileLogger 初始化文件日志输出
func InitFileLogger(logPath string) error {
	globalMu.Lock()
	defer globalMu.Unlock()

	// 如果已有文件，先关闭
	if logFile != nil {
		logFile.Close()
	}

	// 创建日志目录
	logDir := filepath.Dir(logPath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("创建日志目录失败：%w", err)
	}

	// 打开日志文件（追加模式）
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("打开日志文件失败：%w", err)
	}

	logFile = f
	logWriter = f

	// 更新所有已存在的 logger
	for _, logger := range moduleLoggers {
		logger.logger = log.New(logWriter, "", log.LstdFlags|log.Lmsgprefix)
	}

	fmt.Printf("[Logger] 日志文件已初始化：%s\n", logPath)
	return nil
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
		logger: log.New(logWriter, "", log.LstdFlags|log.Lmsgprefix),
	}
	moduleLoggers[prefix] = logger

	return logger
}

// SetLogLevel 设置全局日志级别
func SetLogLevel(level LogLevel) {
	globalMu.Lock()
	defer globalMu.Unlock()
	logLevel = level
}

// GetLogLevel 获取当前全局日志级别
func GetLogLevel() LogLevel {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return logLevel
}

// shouldLog 判断是否应该输出该级别的日志
func shouldLog(level LogLevel) bool {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return level >= logLevel
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
	if !shouldLog(InfoLevel) {
		return
	}
	if l.logger != nil {
		l.logger.Output(2, l.formatMessage(format, a...))
	} else {
		log.Println(l.formatMessage(format, a...))
	}
}

// Error 打印错误级别日志
func (l *ModuleLogger) Error(format string, a ...interface{}) {
	if !shouldLog(ErrorLevel) {
		return
	}
	if l.logger != nil {
		l.logger.Output(2, l.formatMessage(format, a...))
	} else {
		log.Println(l.formatMessage(format, a...))
	}
}

// Warn 打印警告级别日志
func (l *ModuleLogger) Warn(format string, a ...interface{}) {
	if !shouldLog(WarnLevel) {
		return
	}
	if l.logger != nil {
		l.logger.Output(2, l.formatMessage(format, a...))
	} else {
		log.Println(l.formatMessage(format, a...))
	}
}

// Warnf 格式化打印警告级别日志
func (l *ModuleLogger) Warnf(format string, a ...interface{}) {
	if !shouldLog(WarnLevel) {
		return
	}
	if l.logger != nil {
		l.logger.Output(2, l.formatMessage(format, a...))
	} else {
		log.Println(l.formatMessage(format, a...))
	}
}

// Debug 打印调试级别日志
func (l *ModuleLogger) Debug(format string, a ...interface{}) {
	if !shouldLog(DebugLevel) {
		return
	}
	if l.logger != nil {
		l.logger.Output(2, l.formatMessage(format, a...))
	} else {
		log.Println(l.formatMessage(format, a...))
	}
}

// Fatal 打印致命错误并退出
func (l *ModuleLogger) Fatal(format string, a ...interface{}) {
	if l.logger != nil {
		l.logger.Output(2, l.formatMessage(format, a...))
	} else {
		log.Fatal(l.formatMessage(format, a...))
	}
	os.Exit(1)
}

// Fatalf 格式化打印致命错误并退出
func (l *ModuleLogger) Fatalf(format string, a ...interface{}) {
	if l.logger != nil {
		l.logger.Output(2, l.formatMessage(format, a...))
	} else {
		log.Fatalf(l.formatMessage(format, a...))
	}
	os.Exit(1)
}

// Panic 打印致命错误并 panic
func (l *ModuleLogger) Panic(format string, a ...interface{}) {
	if l.logger != nil {
		l.logger.Output(2, l.formatMessage(format, a...))
	} else {
		log.Panic(l.formatMessage(format, a...))
	}
}

// Print 打印日志 (无格式)
func (l *ModuleLogger) Print(a ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	msg := fmt.Sprint(a...)
	if l.logger != nil {
		l.logger.Output(2, msg)
	} else {
		if useColor && l.color != "" {
			fmt.Printf("%s%s %v%s\n", l.color, l.prefix, fmt.Sprint(a...), ColorReset)
		} else {
			fmt.Printf("%s %v\n", l.prefix, fmt.Sprint(a...))
		}
	}
}

// Printf 格式化打印日志
func (l *ModuleLogger) Printf(format string, a ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	msg := fmt.Sprintf(format, a...)
	if l.logger != nil {
		l.logger.Output(2, msg)
	} else {
		if useColor && l.color != "" {
			fmt.Printf("%s%s %s%s\n", l.color, l.prefix, msg, ColorReset)
		} else {
			fmt.Printf("%s %s\n", l.prefix, msg)
		}
	}
}

// Println 打印日志并换行
func (l *ModuleLogger) Println(a ...interface{}) {
	l.Print(a...)
}

// Init 初始化日志（默认输出到 stdout）
func Init() {
	globalMu.Lock()
	defer globalMu.Unlock()

	if logWriter == nil {
		logWriter = os.Stdout
	}
}

func init() {
	// 在包初始化时设置默认输出
	Init()
}

