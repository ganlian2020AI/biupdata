package api

import (
	"net/http"
	"strconv"

	"github.com/ganlian2020AI/biupdata/config"
	"github.com/ganlian2020AI/biupdata/utils"
	"github.com/gin-gonic/gin"
)

var router *gin.Engine

// InitServer 初始化HTTP服务器
func InitServer(cfg *config.APIConfig) *gin.Engine {
	// 设置为发布模式
	gin.SetMode(gin.ReleaseMode)

	// 创建路由
	router = gin.Default()

	// 允许跨域
	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// 注册路由
	registerRoutes()

	return router
}

// StartServer 启动HTTP服务器
func StartServer(cfg *config.APIConfig) error {
	utils.LogInfo("启动HTTP服务器，监听端口: %s", cfg.Port)
	return router.Run(":" + cfg.Port)
}

// 注册API路由
func registerRoutes() {
	// 健康检查
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
		})
	})

	// 获取日志
	router.GET("/logs", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"logs": utils.GetLogBuffer(),
		})
	})

	// 币安数据API
	v1 := router.Group("/api/v1")
	{
		// 获取K线数据
		v1.GET("/kline", getKlineData)

		// 手动触发数据更新
		v1.POST("/update", triggerUpdate)

		// 获取网络连接状态
		v1.GET("/network", getNetworkStatus)

		// 手动切换网络模式
		v1.POST("/network", setNetworkMode)

		// 测试网络连接
		v1.POST("/network/test", testNetworkConnection)

		// 定时任务控制
		v1.GET("/scheduler", getSchedulerStatus)
		v1.POST("/scheduler/start", startScheduler)
		v1.POST("/scheduler/stop", stopScheduler)
	}
}

// getKlineData 获取K线数据处理函数
func getKlineData(c *gin.Context) {
	symbol := c.Query("symbol")
	interval := c.Query("interval")
	startTime := c.Query("start_time")
	endTime := c.Query("end_time")
	limitStr := c.DefaultQuery("limit", "1000")

	// 参数验证
	if symbol == "" || interval == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "缺少必要参数: symbol, interval",
		})
		return
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的limit参数",
		})
		return
	}

	// 获取数据
	data, err := GetKlineDataFromDB(symbol, interval, startTime, endTime, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"symbol":   symbol,
		"interval": interval,
		"data":     data,
		"count":    len(data),
	})
}

// triggerUpdate 手动触发数据更新处理函数
func triggerUpdate(c *gin.Context) {
	var req struct {
		Symbol    string   `json:"symbol"`
		Intervals []string `json:"intervals"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的请求参数",
		})
		return
	}

	// 参数验证
	if req.Symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "缺少必要参数: symbol",
		})
		return
	}

	if len(req.Intervals) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "缺少必要参数: intervals",
		})
		return
	}

	// 异步更新数据
	go func() {
		UpdateSymbolData(req.Symbol, req.Intervals)
	}()

	c.JSON(http.StatusOK, gin.H{
		"message": "数据更新已触发",
		"symbol":  req.Symbol,
	})
}

// getNetworkStatus 获取网络连接状态
func getNetworkStatus(c *gin.Context) {
	if appConfig == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "配置未初始化",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"use_proxy":   appConfig.Binance.UseProxy,
		"base_url":    appConfig.Binance.BaseURL,
		"proxy_url":   appConfig.Binance.ProxyURL,
		"test_symbol": appConfig.Binance.TestSymbol,
	})
}

// setNetworkMode 手动设置网络模式
func setNetworkMode(c *gin.Context) {
	if appConfig == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "配置未初始化",
		})
		return
	}

	var req struct {
		UseProxy bool `json:"use_proxy"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "无效的请求参数",
		})
		return
	}

	// 设置代理模式
	appConfig.Binance.UseProxy = req.UseProxy

	mode := "直接连接"
	if req.UseProxy {
		mode = "代理模式"
	}

	utils.LogInfo("手动切换网络连接模式为: %s", mode)

	c.JSON(http.StatusOK, gin.H{
		"message":   "网络模式已切换",
		"use_proxy": appConfig.Binance.UseProxy,
	})
}

// testNetworkConnection 测试网络连接
func testNetworkConnection(c *gin.Context) {
	isConnected := CheckBinanceConnection()

	mode := "直接连接"
	if appConfig.Binance.UseProxy {
		mode = "代理模式"
	}

	c.JSON(http.StatusOK, gin.H{
		"connected": isConnected,
		"use_proxy": appConfig.Binance.UseProxy,
		"mode":      mode,
	})
}

// getSchedulerStatus 获取定时任务状态
func getSchedulerStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"running": IsSchedulerRunning(),
	})
}

// startScheduler 启动定时任务
func startScheduler(c *gin.Context) {
	if IsSchedulerRunning() {
		c.JSON(http.StatusOK, gin.H{
			"message": "定时任务已经在运行中",
			"running": true,
		})
		return
	}

	StartScheduler()

	c.JSON(http.StatusOK, gin.H{
		"message": "定时任务已启动",
		"running": true,
	})
}

// stopScheduler 停止定时任务
func stopScheduler(c *gin.Context) {
	if !IsSchedulerRunning() {
		c.JSON(http.StatusOK, gin.H{
			"message": "定时任务已经停止",
			"running": false,
		})
		return
	}

	StopScheduler()

	c.JSON(http.StatusOK, gin.H{
		"message": "定时任务已停止",
		"running": false,
	})
}
