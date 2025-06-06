package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/ganlian2020AI/biupdata/api"
	"github.com/ganlian2020AI/biupdata/config"
	"github.com/ganlian2020AI/biupdata/db"
	"github.com/ganlian2020AI/biupdata/utils"
)

var (
	envFile = flag.String("env", "", "环境变量文件路径")
)

func main() {
	// 解析命令行参数
	flag.Parse()

	// 加载配置
	cfg, err := config.LoadConfig(*envFile)
	if err != nil {
		fmt.Printf("加载配置失败: %v\n", err)
		os.Exit(1)
	}

	// 初始化时区
	utils.InitTimezone(&cfg.Timezone)
	fmt.Printf("时区已设置为: %s (UTC%+d)\n", cfg.Timezone.Name, cfg.Timezone.Offset)

	// 初始化日志系统
	if err := utils.InitLogger(&cfg.Log); err != nil {
		fmt.Printf("初始化日志系统失败: %v\n", err)
		os.Exit(1)
	}
	utils.LogInfo("日志系统初始化成功")

	// 初始化数据库
	if err := db.InitDB(&cfg.Database); err != nil {
		utils.LogError("初始化数据库失败: %v", err)
		os.Exit(1)
	}
	defer db.CloseDB()
	utils.LogInfo("数据库初始化成功")

	// 设置API配置
	api.SetConfig(cfg)

	// 检查币安API连接状态
	utils.LogInfo("正在检查币安API连接状态...")
	isConnected := api.CheckBinanceConnection()
	if isConnected {
		utils.LogInfo("币安API连接正常，使用直接连接")
	} else {
		utils.LogWarning("币安API连接异常，将使用代理: %s", cfg.Binance.ProxyURL)
	}

	// 初始化定时任务
	api.InitScheduler()
	if err := api.AddUpdateTask(cfg); err != nil {
		utils.LogError("添加定时任务失败: %v", err)
		os.Exit(1)
	}
	api.StartScheduler()
	defer api.StopScheduler()

	// 初始化HTTP服务器
	api.InitServer(&cfg.API)

	// 启动HTTP服务器（非阻塞）
	go func() {
		if err := api.StartServer(&cfg.API); err != nil {
			utils.LogError("启动HTTP服务器失败: %v", err)
			os.Exit(1)
		}
	}()

	utils.LogInfo("BiUpData 服务已启动")
	utils.LogInfo("监听端口: %s", cfg.API.Port)
	utils.LogInfo("支持的交易对: %v", cfg.Binance.Symbols)
	utils.LogInfo("支持的时间间隔: %v", cfg.Binance.Intervals)
	if cfg.Binance.UseProxy {
		utils.LogInfo("使用代理URL: %s", cfg.Binance.ProxyURL)
	}

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	utils.LogInfo("正在关闭服务...")
}
