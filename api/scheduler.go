package api

import (
	"sync"
	"time"

	"github.com/ganlian2020AI/biupdata/config"
	"github.com/ganlian2020AI/biupdata/utils"
	"github.com/robfig/cron/v3"
)

var (
	scheduler          *cron.Cron
	jobIDs             map[string]cron.EntryID
	updateMutex        sync.Mutex
	lastUpdateTime     map[string]map[string]time.Time // 记录每个交易对和时间间隔的最后更新时间
	lastConnCheck      time.Time                       // 上次连接检查时间
	isSchedulerRunning bool                            // 定时器是否正在运行
)

// InitScheduler 初始化定时任务调度器
func InitScheduler() {
	scheduler = cron.New(cron.WithSeconds())
	jobIDs = make(map[string]cron.EntryID)
	lastUpdateTime = make(map[string]map[string]time.Time)
	lastConnCheck = time.Time{} // 初始化为零值，确保首次运行时会检查连接
	isSchedulerRunning = true   // 默认为启动状态
}

// StartScheduler 启动定时任务调度器
func StartScheduler() {
	if scheduler != nil {
		scheduler.Start()
		isSchedulerRunning = true
		utils.LogInfo("定时任务调度器已启动")
	}
}

// StopScheduler 停止定时任务调度器
func StopScheduler() {
	if scheduler != nil {
		scheduler.Stop()
		isSchedulerRunning = false
		utils.LogInfo("定时任务调度器已停止")
	}
}

// IsSchedulerRunning 获取定时任务调度器运行状态
func IsSchedulerRunning() bool {
	return isSchedulerRunning
}

// AddUpdateTask 添加数据更新定时任务
func AddUpdateTask(cfg *config.Config) error {
	if scheduler == nil {
		InitScheduler()
	}

	// 添加每分钟检查任务
	_, err := scheduler.AddFunc("* * * * *", func() {
		checkAndUpdateData(cfg)
	})

	if err != nil {
		utils.LogError("添加定时任务失败: %v", err)
		return err
	}

	utils.LogInfo("已添加数据更新定时任务，将根据时间间隔自动调整更新频率")
	return nil
}

// checkAndUpdateData 检查并更新数据
func checkAndUpdateData(cfg *config.Config) {
	updateMutex.Lock()
	defer updateMutex.Unlock()

	// 每10分钟检查一次网络连接状态
	if time.Since(lastConnCheck) > 10*time.Minute {
		utils.LogInfo("定期检查币安API连接状态...")
		CheckBinanceConnection()
		lastConnCheck = time.Now()
	}

	// 遍历所有交易对
	for _, symbol := range cfg.Binance.Symbols {
		// 确保该交易对的时间记录存在
		if _, exists := lastUpdateTime[symbol]; !exists {
			lastUpdateTime[symbol] = make(map[string]time.Time)
		}

		// 需要更新的时间间隔
		var intervalsToUpdate []string

		// 检查每个时间间隔是否需要更新
		for _, interval := range cfg.Binance.Intervals {
			lastUpdate, exists := lastUpdateTime[symbol][interval]

			// 如果没有更新记录或者已经到了更新时间
			if !exists || ShouldUpdateInterval(interval, lastUpdate) {
				intervalsToUpdate = append(intervalsToUpdate, interval)
			}
		}

		// 如果有需要更新的时间间隔
		if len(intervalsToUpdate) > 0 {
			utils.LogInfo("开始更新 %s 的数据，时间间隔: %v", symbol, intervalsToUpdate)

			// 异步更新数据
			go func(s string, intervals []string) {
				results, err := UpdateSymbolData(s, intervals)
				if err != nil {
					utils.LogError("更新 %s 数据失败: %v", s, err)
					return
				}

				// 更新最后更新时间
				updateMutex.Lock()
				defer updateMutex.Unlock()

				for interval, count := range results {
					lastUpdateTime[s][interval] = time.Now().UTC()
					utils.LogInfo("定时任务: %s %s 数据更新完成，共 %d 条记录", s, interval, count)
				}
			}(symbol, intervalsToUpdate)
		}
	}
}
