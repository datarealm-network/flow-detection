# Flow Detection - 包结构说明

## 📦 包概览

```
pkg/
├── packet/          # 数据包定义
│   └── packet.go    # Packet 结构体和相关方法
├── capture/         # 网络捕获模块
│   ├── capture.go   # Capturer 接口定义
│   ├── config.go    # 捕获配置
│   ├── errors.go    # 错误定义
│   └── pcap_capturer.go  # 基于 gopacket/pcap 的实现
├── pipeline/        # 数据管道模块
│   ├── pipeline.go  # Pipeline 接口定义
│   ├── config.go    # 管道配置
│   ├── errors.go    # 错误定义
│   ├── channel_pipeline.go  # 基于 channel 的实现
│   └── middleware.go # 中间件实现
└── engine/          # 集成引擎
    └── engine.go    # CaptureEngine 整合捕获和管道
```

## 🎯 核心接口

### 1. Capturer 接口 (`capture/capture.go`)

```go
type Capturer interface {
    Start(ctx context.Context) error
    Stop() error
    Packets() <-chan *packet.Packet
    Stats() *CaptureStats
    IsRunning() bool
    SetBPFFilter(filter string) error
}
```

### 2. Pipeline 接口 (`pipeline/pipeline.go`)

```go
type Pipeline interface {
    Start(ctx context.Context) error
    Stop() error
    Input() chan<- *packet.Packet
    Output() <-chan *packet.Packet
    Stats() *PipelineStats
    IsRunning() bool
}
```

### 3. Consumer 接口 (`pipeline/pipeline.go`)

```go
type Consumer interface {
    Consume(ctx context.Context, pkt *packet.Packet) error
    ConsumeBatch(ctx context.Context, pkts []*packet.Packet) error
    OnStart(ctx context.Context) error
    OnStop() error
}
```

## 🚀 快速使用

### 方式一：使用 CaptureEngine（推荐）

```go
package main

import (
    "context"
    "datarealm.cn/network/pkg/engine"
    "datarealm.cn/network/pkg/packet"
)

// 实现 Consumer 接口
type MyConsumer struct{}

func (c *MyConsumer) OnStart(ctx context.Context) error { return nil }
func (c *MyConsumer) OnStop() error { return nil }
func (c *MyConsumer) Consume(ctx context.Context, pkt *packet.Packet) error {
    // 处理数据包
    info := pkt.NetworkInfo()
    fmt.Printf("%s -> %s\n", info.SrcIP, info.DstIP)
    return nil
}
func (c *MyConsumer) ConsumeBatch(ctx context.Context, pkts []*packet.Packet) error {
    for _, pkt := range pkts {
        c.Consume(ctx, pkt)
    }
    return nil
}

func main() {
    // 创建引擎
    config := engine.DefaultEngineConfig("eth0")
    config.CaptureConfig.BPFFilter = "tcp port 80"
    
    eng, _ := engine.NewCaptureEngine(config)
    eng.RegisterConsumer(&MyConsumer{})
    
    // 启动
    ctx := context.Background()
    eng.Start(ctx)
    defer eng.Stop()
    
    // 等待...
    select {}
}
```

### 方式二：分别使用组件

```go
package main

import (
    "context"
    "datarealm.cn/network/pkg/capture"
    "datarealm.cn/network/pkg/pipeline"
)

func main() {
    ctx := context.Background()
    
    // 1. 创建捕获器
    captureConfig := capture.DefaultConfig("eth0")
    capturer, _ := capture.NewPcapCapturer(captureConfig)
    
    // 2. 创建管道
    pipeConfig := pipeline.DefaultConfig()
    pipe, _ := pipeline.NewChannelPipeline(pipeConfig)
    
    // 3. 启动
    pipe.Start(ctx)
    capturer.Start(ctx)
    
    // 4. 转发数据
    go func() {
        for pkt := range capturer.Packets() {
            pipe.Input() <- pkt
        }
    }()
    
    // 5. 消费数据
    for pkt := range pipe.Output() {
        // 处理数据包
        _ = pkt
    }
}
```

## ⚙️ 配置选项

### 捕获配置 (`capture.Config`)

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| InterfaceName | string | - | 网络接口名 |
| SnapLen | int32 | 65535 | 捕获长度 |
| Promiscuous | bool | true | 混杂模式 |
| BPFFilter | string | "" | BPF 过滤器 |
| BufferSize | int | 4MB | 内核缓冲区 |
| ChannelSize | int | 10000 | 输出通道大小 |

### 管道配置 (`pipeline.Config`)

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| BufferSize | int | 10000 | 缓冲区大小 |
| BatchSize | int | 1 | 批量大小（1=禁用）|
| BatchTimeout | Duration | 10ms | 批量超时 |
| Workers | int | 1 | 工作协程数 |
| DropPolicy | DropPolicy | Newest | 丢弃策略 |

## 🔧 中间件

### 内置中间件

```go
// IP 范围过滤
filter, _ := pipeline.NewIPRangeFilter("monitor", 
    []string{"192.168.1.0/24", "10.0.0.0/8"}, 
    pipeline.IPFilterModeInclude,
)

// 端口过滤
portFilter := pipeline.NewPortFilter("db-ports", 
    []uint16{3306, 5432, 6379}, 
    pipeline.IPFilterModeInclude,
)

// 协议过滤
protoFilter := pipeline.NewProtocolFilter("tcp-only", 
    []string{"TCP"}, 
    pipeline.IPFilterModeInclude,
)

// 采样
sampler := pipeline.NewSamplingMiddleware("sample-1-100", 100) // 1/100 采样

// 添加到配置
config.Middlewares = []pipeline.Middleware{filter, portFilter}
```

### 自定义中间件

```go
type MyMiddleware struct{}

func (m *MyMiddleware) Process(pkt *packet.Packet) *packet.Packet {
    // 返回 nil 表示丢弃
    // 返回 pkt 表示保留
    return pkt
}

func (m *MyMiddleware) Name() string {
    return "my-middleware"
}
```

## 📊 数据流图

```
┌─────────────────────────────────────────────────────────────────┐
│                       CaptureEngine                             │
│                                                                 │
│  ┌──────────────┐       ┌───────────────┐       ┌──────────┐   │
│  │  PcapCapturer │ ───► │ ChannelPipeline│ ───► │ Consumer │   │
│  │              │       │               │       │          │   │
│  │ • pcap handle│       │ • Middlewares │       │ • Parser │   │
│  │ • BPF filter │       │ • Batching    │       │ • Storage│   │
│  │ • Stats      │       │ • Workers     │       │ • Alert  │   │
│  └──────────────┘       └───────────────┘       └──────────┘   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘

数据流：
  网卡 → libpcap → gopacket → Packet → Pipeline → Consumer (Parser)
```

## 🔬 性能优化建议

1. **调整内核缓冲区**: 高流量场景增大 `BufferSize`
2. **使用 BPF 过滤器**: 在内核层面过滤不需要的流量
3. **批量处理**: 设置 `BatchSize > 1` 减少上下文切换
4. **多 Worker**: 设置 `Workers > 1` 并行处理
5. **减少 SnapLen**: 只需协议头时设置较小的 `SnapLen`

## 📝 示例程序

```bash
# 编译示例
cd examples/capture
go build -o capture-demo

# 列出网络接口
./capture-demo -list

# 捕获流量
./capture-demo -interface eth0 -filter "tcp port 80"
```

---

**版本**: v1.0  
**更新日期**: 2024-11-27

