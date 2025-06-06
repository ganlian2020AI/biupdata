package utils

import (
	"time"

	"github.com/ganlian2020AI/biupdata/config"
)

// 上海时区（东八区）
var shanghaiLocation *time.Location

// InitTimezone 初始化时区
func InitTimezone(cfg *config.TimezoneConfig) {
	var err error

	// 尝试加载配置的时区
	shanghaiLocation, err = time.LoadLocation(cfg.Name)
	if err != nil {
		// 如果无法加载配置的时区，则使用配置的偏移量创建固定时区
		shanghaiLocation = time.FixedZone(cfg.Name, cfg.Offset*60*60)
	}
}

// UTCToShanghai 将UTC时间转换为配置的时区时间
func UTCToShanghai(utcTime time.Time) time.Time {
	if shanghaiLocation == nil {
		// 默认使用东八区
		shanghaiLocation = time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	return utcTime.In(shanghaiLocation)
}

// ShanghaiToUTC 将配置的时区时间转换为UTC时间
func ShanghaiToUTC(shanghaiTime time.Time) time.Time {
	if shanghaiLocation == nil {
		// 默认使用东八区
		shanghaiLocation = time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	// 先确保时间是配置的时区
	inShanghai := shanghaiTime.In(shanghaiLocation)
	// 然后转换为UTC
	return inShanghai.UTC()
}

// GetShanghaiNow 获取当前的配置时区时间
func GetShanghaiNow() time.Time {
	if shanghaiLocation == nil {
		// 默认使用东八区
		shanghaiLocation = time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	return time.Now().In(shanghaiLocation)
}

// TimestampToShanghai 将UTC时间戳（毫秒）转换为配置的时区时间
func TimestampToShanghai(timestamp int64) time.Time {
	// 币安的时间戳是毫秒级的
	utcTime := time.Unix(timestamp/1000, (timestamp%1000)*int64(time.Millisecond))
	return UTCToShanghai(utcTime)
}

// ShanghaiToTimestamp 将配置的时区时间转换为UTC时间戳（毫秒）
func ShanghaiToTimestamp(shanghaiTime time.Time) int64 {
	utcTime := ShanghaiToUTC(shanghaiTime)
	return utcTime.UnixNano() / int64(time.Millisecond)
}

// GetDefaultStartTime 根据时间间隔获取默认的起始时间
func GetDefaultStartTime(interval string) time.Time {
	if shanghaiLocation == nil {
		// 默认使用东八区
		shanghaiLocation = time.FixedZone("Asia/Shanghai", 8*60*60)
	}

	switch interval {
	case "5m":
		// 2025-01-01 00:00:00 上海时间
		return time.Date(2025, 1, 1, 0, 0, 0, 0, shanghaiLocation)
	case "30m":
		// 2022-01-01 00:00:00 上海时间
		return time.Date(2022, 1, 1, 0, 0, 0, 0, shanghaiLocation)
	default:
		// 2020-01-01 00:00:00 上海时间
		return time.Date(2020, 1, 1, 0, 0, 0, 0, shanghaiLocation)
	}
}
