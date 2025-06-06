package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	_ "github.com/go-sql-driver/mysql"
	"github.com/robfig/cron/v3"
)

// 配置结构
type Config struct {
	DBUser     string   `json:"db_user"`
	DBPassword string   `json:"db_password"`
	DBHost     string   `json:"db_host"`
	DBPort     string   `json:"db_port"`
	DBName     string   `json:"db_name"`
	Symbols    []string `json:"symbols"`
	Intervals  []string `json:"intervals"`
}

// 币安K线数据结构
type KlineData []interface{}

// 加载配置
func loadConfig() (*Config, error) {
	file, err := os.Open("config.json")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	bytes, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(bytes, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// 初始化数据库连接
func initDB(config *Config) (*sql.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True",
		config.DBUser, config.DBPassword, config.DBHost, config.DBPort, config.DBName)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	err = db.Ping()
	if err != nil {
		return nil, err
	}

	return db, nil
}

// 确保表存在
func ensureTableExists(db *sql.DB, symbol string, interval string) error {
	tableName := fmt.Sprintf("%s_%s", symbol, interval)

	query := fmt.Sprintf(`
	CREATE TABLE IF NOT EXISTS %s (
		timestamp BIGINT NOT NULL,
		open_price DECIMAL(30,8) NOT NULL,
		close_price DECIMAL(30,8) NOT NULL,
		high_price DECIMAL(30,8) NOT NULL,
		low_price DECIMAL(30,8) NOT NULL,
		volume DECIMAL(30,8) NOT NULL,
		note TEXT,
		PRIMARY KEY (timestamp)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
	`, tableName)

	_, err := db.Exec(query)
	return err
}

// 从币安获取K线数据
func fetchKlineData(symbol string, interval string, limit int) ([]KlineData, error) {
	url := fmt.Sprintf("https://api.binance.com/api/v3/klines?symbol=%s&interval=%s&limit=%d",
		symbol, interval, limit)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var klines []KlineData
	if err := json.Unmarshal(body, &klines); err != nil {
		return nil, err
	}

	return klines, nil
}

// 保存K线数据到数据库
func saveKlineData(db *sql.DB, symbol string, interval string, klines []KlineData) error {
	tableName := fmt.Sprintf("%s_%s", symbol, interval)

	for _, kline := range klines {
		// 币安K线数据格式: [开盘时间, 开盘价, 最高价, 最低价, 收盘价, 成交量, 收盘时间, 成交额, 成交笔数, 主动买入成交量, 主动买入成交额, 忽略]
		if len(kline) < 6 {
			continue
		}

		timestamp := int64(kline[0].(float64))
		openPrice := kline[1].(string)
		highPrice := kline[2].(string)
		lowPrice := kline[3].(string)
		closePrice := kline[4].(string)
		volume := kline[5].(string)

		query := fmt.Sprintf(`
		INSERT INTO %s (timestamp, open_price, close_price, high_price, low_price, volume, note)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			open_price = VALUES(open_price),
			close_price = VALUES(close_price),
			high_price = VALUES(high_price),
			low_price = VALUES(low_price),
			volume = VALUES(volume)
		`, tableName)

		_, err := db.Exec(query, timestamp, openPrice, closePrice, highPrice, lowPrice, volume, "")
		if err != nil {
			return err
		}
	}

	return nil
}

// 更新单个交易对和时间间隔的数据
func updateData(db *sql.DB, symbol string, interval string) {
	log.Printf("更新 %s %s 数据", symbol, interval)

	// 确保表存在
	if err := ensureTableExists(db, symbol, interval); err != nil {
		log.Printf("确保表存在失败: %v", err)
		return
	}

	// 获取最新的1000条K线数据
	klines, err := fetchKlineData(symbol, interval, 1000)
	if err != nil {
		log.Printf("获取K线数据失败: %v", err)
		return
	}

	// 保存数据到数据库
	if err := saveKlineData(db, symbol, interval, klines); err != nil {
		log.Printf("保存K线数据失败: %v", err)
		return
	}

	log.Printf("%s %s 数据更新完成，共 %d 条记录", symbol, interval, len(klines))
}

func main() {
	log.Println("币安数据更新服务启动")

	// 加载配置
	config, err := loadConfig()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 初始化数据库
	db, err := initDB(config)
	if err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}
	defer db.Close()

	// 创建定时任务
	c := cron.New()

	// 每分钟执行一次数据更新
	_, err = c.AddFunc("* * * * *", func() {
		for _, symbol := range config.Symbols {
			for _, interval := range config.Intervals {
				updateData(db, symbol, interval)
			}
		}
	})

	if err != nil {
		log.Fatalf("创建定时任务失败: %v", err)
	}

	// 启动定时任务
	c.Start()

	// 首次运行立即更新一次数据
	for _, symbol := range config.Symbols {
		for _, interval := range config.Intervals {
			updateData(db, symbol, interval)
		}
	}

	// 保持程序运行
	select {}
}
