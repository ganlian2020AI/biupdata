package api

import (
	"net/http"
	"strconv"
	"strings"

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

	// 添加HTML日志页面
	router.GET("/logs/view", viewLogs)

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

// viewLogs 显示日志HTML页面
func viewLogs(c *gin.Context) {
	logs := utils.GetLogBuffer()
	htmlContent := generateLogsHTML(logs)
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, htmlContent)
}

// generateLogsHTML 生成日志HTML内容
func generateLogsHTML(logs []string) string {
	html := `
<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>BiUpData 系统日志</title>
    <style>
        body {
            font-family: Arial, sans-serif;
            margin: 0;
            padding: 20px;
            background-color: #f5f5f5;
        }
        h1 {
            color: #333;
            border-bottom: 1px solid #ddd;
            padding-bottom: 10px;
        }
        .container {
            max-width: 1200px;
            margin: 0 auto;
            background-color: white;
            padding: 20px;
            border-radius: 5px;
            box-shadow: 0 1px 3px rgba(0,0,0,0.1);
        }
        .logs {
            height: 600px;
            overflow-y: auto;
            background-color: #f8f8f8;
            padding: 10px;
            border: 1px solid #ddd;
            border-radius: 3px;
            font-family: monospace;
            white-space: pre-wrap;
        }
        .log-entry {
            margin: 5px 0;
            padding: 5px;
            border-bottom: 1px solid #eee;
        }
        .info {
            color: #31708f;
        }
        .error {
            color: #a94442;
            font-weight: bold;
        }
        .warning {
            color: #8a6d3b;
        }
        .controls {
            margin-top: 20px;
            display: flex;
            justify-content: space-between;
        }
        button {
            padding: 8px 16px;
            background-color: #337ab7;
            color: white;
            border: none;
            border-radius: 3px;
            cursor: pointer;
        }
        button:hover {
            background-color: #286090;
        }
        .status {
            margin-top: 20px;
            padding: 10px;
            background-color: #dff0d8;
            border: 1px solid #d6e9c6;
            border-radius: 3px;
            color: #3c763d;
        }
        #autoRefresh {
            margin-right: 10px;
        }
        .refresh-indicator {
            font-size: 12px;
            margin-left: 10px;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>BiUpData 系统日志</h1>
        
        <div class="controls">
            <div>
                <input type="checkbox" id="autoRefresh" checked>
                <label for="autoRefresh">自动刷新 (10秒)</label>
                <span class="refresh-indicator" id="refreshIndicator"></span>
            </div>
            <button onclick="refreshLogs()">立即刷新</button>
        </div>
        
        <div class="logs" id="logsContainer">
`

	// 添加日志条目
	if len(logs) == 0 {
		html += "            <div class='log-entry'>暂无日志记录</div>\n"
	} else {
		// 倒序显示日志，最新的在顶部
		for i := len(logs) - 1; i >= 0; i-- {
			logClass := "info"
			if strings.Contains(logs[i], "[ERROR]") {
				logClass = "error"
			} else if strings.Contains(logs[i], "[WARNING]") {
				logClass = "warning"
			}
			html += "            <div class='log-entry " + logClass + "'>" + logs[i] + "</div>\n"
		}
	}

	html += `
        </div>
        
        <div class="status" id="statusContainer">
            正在从服务器获取状态...
        </div>
    </div>

    <script>
        let autoRefreshEnabled = true;
        let refreshTimer;
        let countdown = 10;
        
        // 页面加载完成后自动刷新
        document.addEventListener('DOMContentLoaded', function() {
            refreshLogs();
            getStatus();
            startRefreshTimer();
            
            // 自动刷新开关
            document.getElementById('autoRefresh').addEventListener('change', function() {
                autoRefreshEnabled = this.checked;
                if (autoRefreshEnabled) {
                    startRefreshTimer();
                } else {
                    clearTimeout(refreshTimer);
                    document.getElementById('refreshIndicator').textContent = '';
                }
            });
        });
        
        // 刷新日志
        function refreshLogs() {
            fetch('/logs')
                .then(response => response.json())
                .then(data => {
                    const logsContainer = document.getElementById('logsContainer');
                    logsContainer.innerHTML = '';
                    
                    if (data.logs.length === 0) {
                        logsContainer.innerHTML = "<div class='log-entry'>暂无日志记录</div>";
                    } else {
                        // 倒序显示日志，最新的在顶部
                        for (let i = data.logs.length - 1; i >= 0; i--) {
                            const log = data.logs[i];
                            let logClass = 'info';
                            
                            if (log.includes('[ERROR]')) {
                                logClass = 'error';
                            } else if (log.includes('[WARNING]')) {
                                logClass = 'warning';
                            }
                            
                            const logEntry = document.createElement('div');
                            logEntry.className = 'log-entry ' + logClass;
                            logEntry.textContent = log;
                            logsContainer.appendChild(logEntry);
                        }
                    }
                })
                .catch(error => {
                    console.error('获取日志失败:', error);
                });
        }
        
        // 获取系统状态
        function getStatus() {
            Promise.all([
                fetch('/api/v1/scheduler').then(response => response.json()),
                fetch('/api/v1/network').then(response => response.json())
            ])
            .then(([schedulerData, networkData]) => {
                const status = document.getElementById('statusContainer');
                const schedulerStatus = schedulerData.running ? '运行中' : '已停止';
                const networkMode = networkData.use_proxy ? '代理模式' : '直接连接';
                
                status.innerHTML = 
                    '<strong>系统状态:</strong> 正常运行<br>' +
                    '<strong>定时任务:</strong> ' + schedulerStatus + '<br>' +
                    '<strong>网络模式:</strong> ' + networkMode + '<br>' +
                    '<strong>更新时间:</strong> ' + new Date().toLocaleString();
            })
            .catch(error => {
                console.error('获取状态失败:', error);
                document.getElementById('statusContainer').innerHTML = 
                    '<strong>系统状态:</strong> 无法获取状态信息<br>' +
                    '<strong>错误信息:</strong> ' + error.message;
            });
        }
        
        // 开始自动刷新计时器
        function startRefreshTimer() {
            clearTimeout(refreshTimer);
            countdown = 10;
            updateCountdown();
            
            function updateCountdown() {
                if (countdown > 0) {
                    document.getElementById('refreshIndicator').textContent = '(' + countdown + '秒后刷新)';
                    countdown--;
                    refreshTimer = setTimeout(updateCountdown, 1000);
                } else {
                    refreshLogs();
                    getStatus();
                    countdown = 10;
                    if (autoRefreshEnabled) {
                        startRefreshTimer();
                    }
                }
            }
        }
    </script>
</body>
</html>
`

	return html
}
