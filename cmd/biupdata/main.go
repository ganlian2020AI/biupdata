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
	fmt.Println("正在加载配置...")
	cfg, err := config.LoadConfig(*envFile)
	if err != nil {
		fmt.Printf("加载配置失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("配置加载成功")

	// 初始化时区
	fmt.Println("正在初始化时区...")
	utils.InitTimezone(&cfg.Timezone)
	fmt.Printf("时区已设置为: %s (UTC%+d)\n", cfg.Timezone.Name, cfg.Timezone.Offset)

	// 初始化日志系统
	fmt.Println("正在初始化日志系统...")
	if err := utils.InitLogger(&cfg.Log); err != nil {
		fmt.Printf("初始化日志系统失败: %v\n", err)
		os.Exit(1)
	}
	utils.LogInfo("日志系统初始化成功")
	fmt.Println("日志系统初始化成功")

	// 初始化数据库
	fmt.Println("正在初始化数据库...")
	if err := db.InitDB(&cfg.Database); err != nil {
		fmt.Printf("初始化数据库失败: %v\n", err)
		utils.LogError("初始化数据库失败: %v", err)
		os.Exit(1)
	}
	defer db.CloseDB()
	utils.LogInfo("数据库初始化成功")
	fmt.Println("数据库初始化成功")

	// 设置API配置
	fmt.Println("正在设置API配置...")
	api.SetConfig(cfg)

	// 检查币安API连接状态
	fmt.Println("正在检查币安API连接状态...")
	isConnected := api.CheckBinanceConnection()
	if isConnected {
		utils.LogInfo("币安API连接正常，使用直接连接")
		fmt.Println("币安API连接正常，使用直接连接")
	} else {
		utils.LogWarning("币安API连接异常，将使用代理: %s", cfg.Binance.ProxyURL)
		fmt.Printf("币安API连接异常，将使用代理: %s\n", cfg.Binance.ProxyURL)
	}

	// 初始化定时任务
	fmt.Println("正在初始化定时任务...")
	api.InitScheduler()
	if err := api.AddUpdateTask(cfg); err != nil {
		fmt.Printf("添加定时任务失败: %v\n", err)
		utils.LogError("添加定时任务失败: %v", err)
		os.Exit(1)
	}
	api.StartScheduler()
	defer api.StopScheduler()
	fmt.Println("定时任务初始化成功")

	// 初始化HTTP服务器
	fmt.Println("正在初始化HTTP服务器...")
	api.InitServer(&cfg.API)

	// 启动HTTP服务器（非阻塞）
	fmt.Println("正在启动HTTP服务器...")
	go func() {
		if err := api.StartServer(&cfg.API); err != nil {
			fmt.Printf("启动HTTP服务器失败: %v\n", err)
			utils.LogError("启动HTTP服务器失败: %v", err)
			os.Exit(1)
		}
	}()

	utils.LogInfo("BiUpData 服务已启动")
	fmt.Println("BiUpData 服务已启动")
	fmt.Printf("监听端口: %s\n", cfg.API.Port)
	fmt.Printf("支持的交易对: %v\n", cfg.Binance.Symbols)
	fmt.Printf("支持的时间间隔: %v\n", cfg.Binance.Intervals)
	if cfg.Binance.UseProxy {
		fmt.Printf("使用代理URL: %s\n", cfg.Binance.ProxyURL)
	}

	// 等待中断信号
	fmt.Println("服务运行中，按Ctrl+C退出...")
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	utils.LogInfo("正在关闭服务...")
	fmt.Println("正在关闭服务...")
}
