package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"github.com/ganlian2020AI/biupdata/config"
	"github.com/ganlian2020AI/biupdata/db"
	"github.com/ganlian2020AI/biupdata/utils"
)

// KlineData 币安K线数据结构
type KlineData []interface{}

// 每种时间间隔对应的更新频率（秒）
var intervalUpdateFrequency = map[string]int{
	"5m":  5 * 60,      // 5分钟
	"30m": 30 * 60,     // 30分钟
	"1h":  60 * 60,     // 1小时
	"4h":  4 * 60 * 60, // 4小时
}

// 全局配置
var appConfig *config.Config

// 设置配置
func SetConfig(cfg *config.Config) {
	appConfig = cfg
}

// 获取时间间隔对应的毫秒数
func getIntervalMilliseconds(interval string) int64 {
	switch interval {
	case "5m":
		return 5 * 60 * 1000
	case "30m":
		return 30 * 60 * 1000
	case "1h":
		return 60 * 60 * 1000
	case "4h":
		return 4 * 60 * 60 * 1000
	default:
		return 60 * 60 * 1000 // 默认1小时
	}
}

// CheckBinanceConnection 检查币安API连接状态
func CheckBinanceConnection() bool {
	if appConfig == nil {
		utils.LogError("配置未初始化")
		return false
	}

	// 使用获取BTC现价的API测试连接
	url := fmt.Sprintf("%s/api/v3/ticker/price?symbol=%s", appConfig.Binance.BaseURL, appConfig.Binance.TestSymbol)
	utils.LogInfo("测试币安API连接: %s", url)

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		utils.LogWarning("币安API连接失败: %v，将使用代理", err)
		appConfig.Binance.UseProxy = true
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		utils.LogWarning("币安API返回非200状态码: %d，将使用代理", resp.StatusCode)
		appConfig.Binance.UseProxy = true
		return false
	}

	utils.LogInfo("币安API连接正常")
	appConfig.Binance.UseProxy = false
	return true
}

// GetBinanceURL 根据连接状态返回适当的URL
func GetBinanceURL(path string) string {
	if appConfig == nil {
		return path // 如果配置未初始化，直接返回路径
	}

	if appConfig.Binance.UseProxy {
		return appConfig.Binance.ProxyURL + path
	}
	return path
}

// FetchKlineData 从币安获取K线数据
func FetchKlineData(symbol string, interval string, startTime, endTime int64, limit int) ([]KlineData, error) {
	// 构建URL
	baseURL := "https://api.binance.com"
	if appConfig != nil {
		baseURL = appConfig.Binance.BaseURL
	}

	url := fmt.Sprintf("%s/api/v3/klines?symbol=%s&interval=%s", baseURL, symbol, interval)

	// 添加开始时间（如果有）
	if startTime > 0 {
		url += fmt.Sprintf("&startTime=%d", startTime)
	}

	// 添加结束时间（如果有）
	if endTime > 0 {
		url += fmt.Sprintf("&endTime=%d", endTime)
	}

	// 添加限制数量
	if limit > 0 {
		url += fmt.Sprintf("&limit=%d", limit)
	}

	// 根据连接状态决定是否使用代理
	finalURL := url
	if appConfig != nil && appConfig.Binance.UseProxy {
		finalURL = appConfig.Binance.ProxyURL + url
		utils.LogInfo("使用代理请求币安API: %s", finalURL)
	} else {
		utils.LogInfo("请求币安API: %s", url)
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Get(finalURL)
	if err != nil {
		utils.LogError("请求币安API失败: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		utils.LogError("读取币安API响应失败: %v", err)
		return nil, err
	}

	var klines []KlineData
	if err := json.Unmarshal(body, &klines); err != nil {
		utils.LogError("解析币安API响应失败: %v", err)
		return nil, err
	}

	utils.LogInfo("成功获取 %s %s 数据，共 %d 条记录", symbol, interval, len(klines))
	return klines, nil
}

// ProcessKlineData 处理并保存K线数据
func ProcessKlineData(symbol string, interval string, klines []KlineData) (int, error) {
	// 确保表存在
	if err := db.CreateTableIfNotExists(symbol, interval); err != nil {
		return 0, err
	}

	successCount := 0

	for _, kline := range klines {
		// 币安K线数据格式: [开盘时间, 开盘价, 最高价, 最低价, 收盘价, 成交量, 收盘时间, 成交额, 成交笔数, 主动买入成交量, 主动买入成交额, 忽略]
		if len(kline) < 6 {
			utils.LogWarning("K线数据格式不正确: %v", kline)
			continue
		}

		// 转换数据类型
		timestamp := int64(kline[0].(float64))

		// 将UTC时间戳转换为上海时间戳（加8小时）
		shanghaiTime := utils.TimestampToShanghai(timestamp)
		shanghaiTimestamp := utils.ShanghaiToTimestamp(shanghaiTime)

		openPrice := kline[1].(string)
		highPrice := kline[2].(string)
		lowPrice := kline[3].(string)
		closePrice := kline[4].(string)
		volume := kline[5].(string)

		// 保存到数据库（使用上海时间戳）
		if err := db.SaveKlineData(symbol, interval, shanghaiTimestamp, openPrice, closePrice, highPrice, lowPrice, volume, ""); err != nil {
			utils.LogError("保存K线数据失败: %v", err)
			continue
		}

		successCount++
	}

	return successCount, nil
}

// GetLastKlineTimestamp 获取最后一条K线数据的时间戳
func GetLastKlineTimestamp(symbol, interval string) (int64, error) {
	// 从数据库获取最后一条记录
	data, err := db.GetKlineData(symbol, interval, 0, 0, 1)
	if err != nil {
		return 0, err
	}

	// 如果没有记录，返回默认起始时间
	if len(data) == 0 {
		defaultTime := utils.GetDefaultStartTime(interval)
		return utils.ShanghaiToTimestamp(defaultTime), nil
	}

	// 返回最后一条记录的时间戳
	return data[0]["timestamp"].(int64), nil
}

// ShouldUpdateInterval 判断是否应该更新指定的时间间隔
func ShouldUpdateInterval(interval string, lastUpdateTime time.Time) bool {
	now := time.Now().UTC()
	frequency, exists := intervalUpdateFrequency[interval]

	if !exists {
		// 默认10分钟更新一次
		frequency = 10 * 60
	}

	// 如果上次更新时间距离现在超过了更新频率，则需要更新
	return now.Sub(lastUpdateTime).Seconds() >= float64(frequency)
}

// UpdateSymbolData 更新单个交易对的所有时间间隔数据
func UpdateSymbolData(symbol string, intervals []string) (map[string]int, error) {
	result := make(map[string]int)

	for _, interval := range intervals {
		// 获取最后一条K线数据的时间戳
		lastTimestamp, err := GetLastKlineTimestamp(symbol, interval)
		if err != nil {
			utils.LogError("获取 %s %s 最后时间戳失败: %v", symbol, interval, err)
			result[interval] = 0
			continue
		}

		// 将上海时间戳转换回UTC时间戳（减8小时）
		shanghaiTime := utils.TimestampToShanghai(lastTimestamp)
		utcTime := utils.ShanghaiToUTC(shanghaiTime)
		utcTimestamp := utcTime.UnixNano() / int64(time.Millisecond)

		// 获取当前UTC时间戳
		nowUTC := time.Now().UTC().UnixNano() / int64(time.Millisecond)

		// 计算需要更新的数据量
		intervalMs := getIntervalMilliseconds(interval)
		neededBars := (nowUTC - utcTimestamp) / intervalMs

		// 如果需要更新的数据量超过1000条，则分批更新
		totalUpdated := 0
		if neededBars > 1000 {
			// 分批更新，每批1000条
			for startTime := utcTimestamp; startTime < nowUTC; startTime += 1000 * intervalMs {
				endTime := startTime + 1000*intervalMs
				if endTime > nowUTC {
					endTime = nowUTC
				}

				// 获取K线数据
				klines, err := FetchKlineData(symbol, interval, startTime, endTime, 1000)
				if err != nil {
					utils.LogError("获取 %s %s K线数据失败: %v", symbol, interval, err)
					continue
				}

				// 处理并保存数据
				count, err := ProcessKlineData(symbol, interval, klines)
				if err != nil {
					utils.LogError("处理 %s %s K线数据失败: %v", symbol, interval, err)
					continue
				}

				totalUpdated += count

				// 避免API请求过于频繁
				time.Sleep(100 * time.Millisecond)
			}

			// 更新频率调整为10分钟
			intervalUpdateFrequency[interval] = 10 * 60
			utils.LogInfo("由于 %s %s 数据量较大，更新频率已调整为10分钟", symbol, interval)
		} else {
			// 直接获取所有数据
			klines, err := FetchKlineData(symbol, interval, utcTimestamp, 0, 1000)
			if err != nil {
				utils.LogError("获取 %s %s K线数据失败: %v", symbol, interval, err)
				result[interval] = 0
				continue
			}

			// 处理并保存数据
			totalUpdated, err = ProcessKlineData(symbol, interval, klines)
			if err != nil {
				utils.LogError("处理 %s %s K线数据失败: %v", symbol, interval, err)
				result[interval] = 0
				continue
			}
		}

		result[interval] = totalUpdated
		utils.LogInfo("成功更新 %s %s 数据，共 %d 条记录", symbol, interval, totalUpdated)
	}

	return result, nil
}

// GetKlineDataFromDB 从数据库获取K线数据
func GetKlineDataFromDB(symbol, interval string, startTime, endTime string, limit int) ([]map[string]interface{}, error) {
	var startTimestamp, endTimestamp int64
	var err error

	// 转换时间戳
	if startTime != "" {
		startTimestamp, err = strconv.ParseInt(startTime, 10, 64)
		if err != nil {
			utils.LogError("解析开始时间戳失败: %v", err)
			return nil, err
		}
	}

	if endTime != "" {
		endTimestamp, err = strconv.ParseInt(endTime, 10, 64)
		if err != nil {
			utils.LogError("解析结束时间戳失败: %v", err)
			return nil, err
		}
	}

	// 限制查询记录数量
	if limit <= 0 || limit > 1000 {
		limit = 1000
	}

	// 从数据库获取数据
	return db.GetKlineData(symbol, interval, startTimestamp, endTimestamp, limit)
}
