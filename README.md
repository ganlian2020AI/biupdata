# BiUpData

一个用Go语言编写的工具，用于从币安获取加密货币数据并存储到MariaDB数据库。

## 功能特点

- 定时从币安API获取加密货币K线数据
- 支持多种交易对（BTC、ETH、BNB等）
- 支持多种时间间隔（5分钟、30分钟、1小时、4小时等）
- 自动创建数据库表并存储数据
- 定时更新数据
- HTTP API接口，支持远程访问和数据查询
- 日志系统，限制日志记录数量
- 自动检测网络连接状态，支持代理模式

## 安装

1. 确保已安装Go环境（1.16或更高版本）
2. 克隆仓库
   ```
   git clone https://github.com/ganlian2020AI/biupdata.git
   cd biupdata
   ```
3. 安装依赖
   ```
   go mod tidy
   ```
4. 创建配置文件
   ```
   cp env.example config.env
   ```
   然后编辑`config.env`文件，设置您的数据库信息和代理URL

## 配置

项目使用环境变量进行配置，配置文件按以下优先级加载：
1. 命令行参数`-env`指定的文件
2. 项目根目录下的`config.env`文件
3. 项目根目录下的`.env`文件
4. 项目根目录下的`env.example`文件
5. 系统环境变量

### 配置项说明

```
# 数据库配置
DB_USER=root                # 数据库用户名
DB_PASSWORD=password        # 数据库密码
DB_HOST=localhost           # 数据库主机
DB_PORT=3306                # 数据库端口
DB_NAME=crypto_data         # 数据库名称

# API配置
API_PORT=8080               # API服务端口
API_ALLOWED_ORIGINS=*       # 允许的跨域来源

# 币安API配置
BINANCE_SYMBOLS=BTCUSDT,ETHUSDT,BNBUSDT    # 交易对，逗号分隔
BINANCE_INTERVALS=5m,30m,1h,4h              # 时间间隔，逗号分隔
BINANCE_BASE_URL=https://api.binance.com    # 币安API基础URL
BINANCE_PROXY_URL=https://your-proxy-url/   # 代理URL前缀，需要自行配置
BINANCE_USE_PROXY=false     # 是否默认使用代理
BINANCE_TEST_SYMBOL=BTCUSDT # 用于测试连接的交易对

# 时区配置
TIMEZONE=Asia/Shanghai      # 时区名称
TIMEZONE_OFFSET=8           # 与UTC的时差（小时）

# 日志配置
LOG_FILE=logs/biupdata.log  # 日志文件路径
LOG_MAX_SIZE=10             # 日志文件最大大小(MB)
LOG_MAX_BACKUPS=5           # 保留的旧日志文件数量
LOG_MAX_AGE=30              # 保留日志文件的天数
LOG_COMPRESS=true           # 是否压缩旧日志文件
LOG_MAX_RECORDS=1000        # 内存中保留的最大日志记录数

# 定时任务配置
CRON_UPDATE_SCHEDULE=* * * * *  # 检查更新的Cron表达式
```

### 代理配置

如果您需要通过代理访问币安API，请在`config.env`文件中设置：
```
BINANCE_PROXY_URL=https://your-proxy-url/   # 替换为您的代理URL
BINANCE_USE_PROXY=true                      # 启用代理
```

## 运行

```
go run cmd/biupdata/main.go
```

或者编译后运行：

```
go build -o biupdata cmd/biupdata/main.go
./biupdata
```

指定配置文件：

```
./biupdata -env /path/to/config.env
```

## 网络连接检查

程序启动时会自动检查与币安API的连接状态：
1. 首先尝试直接连接币安API获取BTC现价
2. 如果连接失败（超时或返回错误），自动切换到代理模式
3. 代理模式下，所有API请求将通过配置的代理URL进行转发

可以通过以下配置项控制代理行为：
- `BINANCE_BASE_URL`: 币安API的基础URL
- `BINANCE_PROXY_URL`: 代理服务器URL前缀
- `BINANCE_USE_PROXY`: 是否默认使用代理
- `BINANCE_TEST_SYMBOL`: 用于测试连接的交易对

## 时间间隔更新频率

- 5分钟K线数据：每5分钟更新一次
- 30分钟K线数据：每30分钟更新一次
- 1小时K线数据：每1小时更新一次
- 4小时K线数据：每4小时更新一次

如果数据量较大（超过1000条），更新频率会自动调整为10分钟一次。

## 时区处理

系统默认使用上海时区（UTC+8）。从币安获取的数据（UTC时间）会自动转换为上海时间后存储到数据库中。

## API接口

### 健康检查

```
GET /health
```

### 获取日志

```
GET /logs
```

### 获取K线数据

```
GET /api/v1/kline?symbol=BTCUSDT&interval=1h&start_time=1609459200000&end_time=1609545600000&limit=100
```

参数：
- symbol: 交易对（必填）
- interval: 时间间隔（必填）
- start_time: 开始时间戳（可选）
- end_time: 结束时间戳（可选）
- limit: 返回记录限制，默认1000（可选）

### 手动触发数据更新

```
POST /api/v1/update
```

请求体：
```json
{
  "symbol": "BTCUSDT",
  "intervals": ["1h", "4h"]
}
```

### 网络连接管理

#### 获取网络连接状态
```
GET /api/v1/network
```

返回：
```json
{
  "use_proxy": false,
  "base_url": "https://api.binance.com",
  "proxy_url": "https://your-proxy-url/",
  "test_symbol": "BTCUSDT"
}
```

#### 设置网络连接模式
```
POST /api/v1/network
```

请求体：
```json
{
  "use_proxy": true
}
```

#### 测试网络连接
```
POST /api/v1/network/test
```

返回：
```json
{
  "connected": true,
  "use_proxy": false,
  "mode": "直接连接"
}
```

### 定时任务管理

#### 获取定时任务状态
```
GET /api/v1/scheduler
```

返回：
```json
{
  "running": true
}
```

#### 启动定时任务
```
POST /api/v1/scheduler/start
```

#### 停止定时任务
```
POST /api/v1/scheduler/stop
```

## 数据库表结构

对于每个交易对和时间间隔组合，程序会自动创建一个表，表名格式为：`{交易对}_{时间间隔}`

表结构如下：

| 字段名 | 类型 | 说明 |
|--------|------|------|
| timestamp | BIGINT | 时间戳（主键） |
| open_price | DECIMAL(30,8) | 开盘价 |
| close_price | DECIMAL(30,8) | 收盘价 |
| high_price | DECIMAL(30,8) | 最高价 |
| low_price | DECIMAL(30,8) | 最低价 |
| volume | DECIMAL(30,8) | 成交量 |
| note | TEXT | 备注 |

## 项目结构

```
biupdata/
├── api/                # API相关代码
│   ├── binance.go      # 币安API交互
│   ├── scheduler.go    # 定时任务调度
│   └── server.go       # HTTP服务器
├── cmd/                # 命令行入口
│   └── biupdata/       
│       └── main.go     # 主程序入口
├── config/             # 配置相关
│   └── config.go       # 配置处理
├── db/                 # 数据库相关
│   └── database.go     # 数据库操作
├── utils/              # 工具函数
│   ├── logger.go       # 日志处理
│   └── timezone.go     # 时区处理
├── env.example         # 示例配置文件
├── go.mod              # Go模块定义
└── README.md           # 项目说明
```

## 许可证

MIT 