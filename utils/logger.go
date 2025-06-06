package utils

import (
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/ganlian2020AI/biupdata/config"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	logger     *log.Logger
	logBuffer  []string
	bufferSize int
	mu         sync.Mutex
)

// InitLogger 初始化日志系统
func InitLogger(cfg *config.LogConfig) error {
	// 确保日志目录存在
	dir := filepath.Dir(cfg.File)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	// 设置日志输出
	lumberjackLogger := &lumberjack.Logger{
		Filename:   cfg.File,
		MaxSize:    cfg.MaxSize,    // 以MB为单位
		MaxBackups: cfg.MaxBackups, // 保留的旧日志文件数量
		MaxAge:     cfg.MaxAge,     // 保留日志文件的天数
		Compress:   cfg.Compress,   // 是否压缩旧日志文件
	}

	// 同时输出到控制台和文件
	multiWriter := log.New(lumberjackLogger, "", log.LstdFlags)

	logger = multiWriter
	bufferSize = cfg.MaxRecords
	logBuffer = make([]string, 0, bufferSize)

	return nil
}

// LogInfo 记录信息日志
func LogInfo(format string, v ...interface{}) {
	if logger == nil {
		return
	}

	mu.Lock()
	defer mu.Unlock()

	logger.Printf("[INFO] "+format, v...)

	// 将日志添加到缓冲区
	logMsg := "[INFO] " + format
	addToBuffer(logMsg, v...)
}

// LogError 记录错误日志
func LogError(format string, v ...interface{}) {
	if logger == nil {
		return
	}

	mu.Lock()
	defer mu.Unlock()

	logger.Printf("[ERROR] "+format, v...)

	// 将日志添加到缓冲区
	logMsg := "[ERROR] " + format
	addToBuffer(logMsg, v...)
}

// LogWarning 记录警告日志
func LogWarning(format string, v ...interface{}) {
	if logger == nil {
		return
	}

	mu.Lock()
	defer mu.Unlock()

	logger.Printf("[WARNING] "+format, v...)

	// 将日志添加到缓冲区
	logMsg := "[WARNING] " + format
	addToBuffer(logMsg, v...)
}

// 添加日志到缓冲区，保持最大记录数限制
func addToBuffer(format string, v ...interface{}) {
	// 如果缓冲区已满，移除最旧的日志
	if len(logBuffer) >= bufferSize {
		logBuffer = logBuffer[1:]
	}

	// 添加新日志
	logBuffer = append(logBuffer, format)
}

// GetLogBuffer 获取日志缓冲区
func GetLogBuffer() []string {
	mu.Lock()
	defer mu.Unlock()

	// 返回日志缓冲区的副本
	result := make([]string, len(logBuffer))
	copy(result, logBuffer)

	return result
}
