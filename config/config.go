package config

import (
	"errors"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config 应用程序配置结构
type Config struct {
	Database DatabaseConfig
	API      APIConfig
	Binance  BinanceConfig
	Timezone TimezoneConfig
	Log      LogConfig
	Cron     CronConfig
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	User     string
	Password string
	Host     string
	Port     string
	Name     string
}

// APIConfig API服务配置
type APIConfig struct {
	Port           string
	AllowedOrigins []string
}

// BinanceConfig 币安API配置
type BinanceConfig struct {
	Symbols    []string
	Intervals  []string
	ProxyURL   string
	UseProxy   bool
	BaseURL    string
	TestSymbol string
}

// TimezoneConfig 时区配置
type TimezoneConfig struct {
	Name   string // 时区名称，如 "Asia/Shanghai"
	Offset int    // 与UTC的时差（小时），如东八区为8
}

// LogConfig 日志配置
type LogConfig struct {
	File       string
	MaxSize    int
	MaxBackups int
	MaxAge     int
	Compress   bool
	MaxRecords int
}

// CronConfig 定时任务配置
type CronConfig struct {
	UpdateSchedule string
}

// GetDSN 获取数据库连接字符串
func (c *DatabaseConfig) GetDSN() string {
	return c.User + ":" + c.Password + "@tcp(" + c.Host + ":" + c.Port + ")/" + c.Name + "?charset=utf8mb4&parseTime=True"
}

// LoadConfig 加载配置
func LoadConfig(envFile string) (*Config, error) {
	// 尝试加载环境变量文件
	if envFile != "" {
		// 如果指定了环境变量文件，则加载指定的文件
		if err := godotenv.Load(envFile); err != nil {
			return nil, err
		}
	} else {
		// 如果未指定环境变量文件，按优先级尝试加载
		// 1. config.env
		// 2. .env
		// 3. env.example
		if _, err := os.Stat("config.env"); err == nil {
			godotenv.Load("config.env")
		} else if _, err := os.Stat(".env"); err == nil {
			godotenv.Load(".env")
		} else if _, err := os.Stat("env.example"); err == nil {
			godotenv.Load("env.example")
		}
		// 如果都不存在，使用系统环境变量
	}

	config := &Config{
		Database: DatabaseConfig{
			User:     getEnv("DB_USER", "root"),
			Password: getEnv("DB_PASSWORD", ""),
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "3306"),
			Name:     getEnv("DB_NAME", "crypto_data"),
		},
		API: APIConfig{
			Port:           getEnv("API_PORT", "8080"),
			AllowedOrigins: strings.Split(getEnv("API_ALLOWED_ORIGINS", "*"), ","),
		},
		Binance: BinanceConfig{
			Symbols:    strings.Split(getEnv("BINANCE_SYMBOLS", "BTCUSDT,ETHUSDT,BNBUSDT"), ","),
			Intervals:  strings.Split(getEnv("BINANCE_INTERVALS", "5m,30m,1h,4h"), ","),
			ProxyURL:   getEnv("BINANCE_PROXY_URL", "https://your-proxy-url/"),
			UseProxy:   getEnvAsBool("BINANCE_USE_PROXY", false),
			BaseURL:    getEnv("BINANCE_BASE_URL", "https://api.binance.com"),
			TestSymbol: getEnv("BINANCE_TEST_SYMBOL", "BTCUSDT"),
		},
		Timezone: TimezoneConfig{
			Name:   getEnv("TIMEZONE", "Asia/Shanghai"),
			Offset: getEnvAsInt("TIMEZONE_OFFSET", 8),
		},
		Log: LogConfig{
			File:       getEnv("LOG_FILE", "logs/biupdata.log"),
			MaxSize:    getEnvAsInt("LOG_MAX_SIZE", 10),
			MaxBackups: getEnvAsInt("LOG_MAX_BACKUPS", 5),
			MaxAge:     getEnvAsInt("LOG_MAX_AGE", 30),
			Compress:   getEnvAsBool("LOG_COMPRESS", true),
			MaxRecords: getEnvAsInt("LOG_MAX_RECORDS", 1000),
		},
		Cron: CronConfig{
			UpdateSchedule: getEnv("CRON_UPDATE_SCHEDULE", "0 * * * * *"),
		},
	}

	// 验证配置
	if err := validateConfig(config); err != nil {
		return nil, err
	}

	return config, nil
}

// 获取环境变量，如果不存在则返回默认值
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// 获取环境变量并转换为整数，如果不存在或转换失败则返回默认值
func getEnvAsInt(key string, defaultValue int) int {
	valueStr := getEnv(key, "")
	if valueStr == "" {
		return defaultValue
	}

	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}

	return value
}

// 获取环境变量并转换为布尔值，如果不存在或转换失败则返回默认值
func getEnvAsBool(key string, defaultValue bool) bool {
	valueStr := getEnv(key, "")
	if valueStr == "" {
		return defaultValue
	}

	value, err := strconv.ParseBool(valueStr)
	if err != nil {
		return defaultValue
	}

	return value
}

// 验证配置
func validateConfig(config *Config) error {
	// 验证数据库配置
	if config.Database.Name == "" {
		return errors.New("数据库名称不能为空")
	}

	// 验证币安配置
	if len(config.Binance.Symbols) == 0 {
		return errors.New("币安交易对不能为空")
	}
	if len(config.Binance.Intervals) == 0 {
		return errors.New("币安时间间隔不能为空")
	}

	return nil
}
