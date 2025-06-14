package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/ganlian2020AI/biupdata/config"
	"github.com/ganlian2020AI/biupdata/utils"
	_ "github.com/go-sql-driver/mysql"
)

// DB 数据库连接实例
var DB *sql.DB

// InitDB 初始化数据库连接
func InitDB(cfg *config.DatabaseConfig) error {
	var err error

	// 连接数据库
	DB, err = sql.Open("mysql", cfg.GetDSN())
	if err != nil {
		return err
	}

	// 测试连接
	if err = DB.Ping(); err != nil {
		return err
	}

	utils.LogInfo("数据库连接成功")
	return nil
}

// CloseDB 关闭数据库连接
func CloseDB() {
	if DB != nil {
		DB.Close()
	}
}

// InitAllTables 初始化所有需要的表
func InitAllTables(symbols []string, intervals []string) error {
	for _, symbol := range symbols {
		for _, interval := range intervals {
			if err := CreateTableIfNotExists(symbol, interval); err != nil {
				return err
			}
		}
	}
	utils.LogInfo("所有表初始化完成")
	return nil
}

// CreateTableIfNotExists 如果表不存在则创建表
func CreateTableIfNotExists(symbol, interval string) error {
	tableName := GetTableName(symbol, interval)

	query := fmt.Sprintf(`
	CREATE TABLE IF NOT EXISTS %s (
		timestamp DATETIME NOT NULL COMMENT '上海时间',
		open_price DECIMAL(30,8) NOT NULL,
		close_price DECIMAL(30,8) NOT NULL,
		high_price DECIMAL(30,8) NOT NULL,
		low_price DECIMAL(30,8) NOT NULL,
		volume DECIMAL(30,8) NOT NULL,
		note TEXT,
		PRIMARY KEY (timestamp)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
	`, tableName)

	_, err := DB.Exec(query)
	if err != nil {
		utils.LogError("创建表 %s 失败: %v", tableName, err)
		return err
	}

	utils.LogInfo("表 %s 已就绪", tableName)
	return nil
}

// SaveKlineData 保存K线数据到数据库
func SaveKlineData(symbol, interval string, timestamp int64, openPrice, closePrice, highPrice, lowPrice, volume, note string) error {
	tableName := GetTableName(symbol, interval)

	// 将时间戳转换为上海时间
	dateTime := utils.TimestampToShanghai(timestamp)
	formattedTime := dateTime.Format("2006-01-02 15:04:05")

	query := fmt.Sprintf(`
	INSERT INTO %s (timestamp, open_price, close_price, high_price, low_price, volume, note)
	VALUES (?, ?, ?, ?, ?, ?, ?)
	ON DUPLICATE KEY UPDATE
		open_price = VALUES(open_price),
		close_price = VALUES(close_price),
		high_price = VALUES(high_price),
		low_price = VALUES(low_price),
		volume = VALUES(volume),
		note = VALUES(note)
	`, tableName)

	_, err := DB.Exec(query, formattedTime, openPrice, closePrice, highPrice, lowPrice, volume, note)
	if err != nil {
		utils.LogError("保存K线数据到表 %s 失败: %v", tableName, err)
		return err
	}

	return nil
}

// GetKlineData 获取K线数据
func GetKlineData(symbol, interval string, startTime, endTime int64, limit int) ([]map[string]interface{}, error) {
	tableName := GetTableName(symbol, interval)

	var query string
	var rows *sql.Rows
	var err error

	// 转换时间戳为日期时间格式
	var startTimeStr, endTimeStr string
	if startTime > 0 {
		startTimeStr = utils.TimestampToShanghai(startTime).Format("2006-01-02 15:04:05")
	}
	if endTime > 0 {
		endTimeStr = utils.TimestampToShanghai(endTime).Format("2006-01-02 15:04:05")
	}

	if startTime > 0 && endTime > 0 {
		query = fmt.Sprintf(`
		SELECT timestamp, open_price, close_price, high_price, low_price, volume, note
		FROM %s
		WHERE timestamp >= ? AND timestamp <= ?
		ORDER BY timestamp DESC
		LIMIT ?
		`, tableName)
		rows, err = DB.Query(query, startTimeStr, endTimeStr, limit)
	} else if startTime > 0 {
		query = fmt.Sprintf(`
		SELECT timestamp, open_price, close_price, high_price, low_price, volume, note
		FROM %s
		WHERE timestamp >= ?
		ORDER BY timestamp DESC
		LIMIT ?
		`, tableName)
		rows, err = DB.Query(query, startTimeStr, limit)
	} else if endTime > 0 {
		query = fmt.Sprintf(`
		SELECT timestamp, open_price, close_price, high_price, low_price, volume, note
		FROM %s
		WHERE timestamp <= ?
		ORDER BY timestamp DESC
		LIMIT ?
		`, tableName)
		rows, err = DB.Query(query, endTimeStr, limit)
	} else {
		query = fmt.Sprintf(`
		SELECT timestamp, open_price, close_price, high_price, low_price, volume, note
		FROM %s
		ORDER BY timestamp DESC
		LIMIT ?
		`, tableName)
		rows, err = DB.Query(query, limit)
	}

	if err != nil {
		utils.LogError("查询表 %s 数据失败: %v", tableName, err)
		return nil, err
	}
	defer rows.Close()

	var result []map[string]interface{}

	for rows.Next() {
		var timestamp time.Time
		var openPrice, closePrice, highPrice, lowPrice, volume sql.NullString
		var note sql.NullString

		if err := rows.Scan(&timestamp, &openPrice, &closePrice, &highPrice, &lowPrice, &volume, &note); err != nil {
			utils.LogError("扫描表 %s 数据失败: %v", tableName, err)
			return nil, err
		}

		// 格式化时间
		formattedTime := timestamp.Format("2006-01-02 15:04")

		// 转回时间戳以保持API兼容性
		unixTimestamp := timestamp.Unix() * 1000

		data := map[string]interface{}{
			"timestamp":   unixTimestamp,
			"datetime":    formattedTime,
			"open_price":  openPrice.String,
			"close_price": closePrice.String,
			"high_price":  highPrice.String,
			"low_price":   lowPrice.String,
			"volume":      volume.String,
			"note":        note.String,
		}

		result = append(result, data)
	}

	return result, nil
}

// GetTableName 获取表名
func GetTableName(symbol, interval string) string {
	// 统一转换为小写并移除特殊字符
	symbol = strings.ToLower(symbol)
	interval = strings.ToLower(interval)

	return fmt.Sprintf("%s_%s", symbol, interval)
}
