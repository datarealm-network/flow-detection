# Transport Module - 跨进程通信模块

## 概述

Transport模块提供了一个统一的跨进程通信接口，支持多种传输方式：
- **共享内存（Shared Memory）** - 零拷贝，极致性能
- **gRPC** - 高性能RPC框架
- **RESTful** - HTTP/JSON，易于集成
- **Unix Socket** - 本地进程通信

## 核心特性

### ✅ 统一接口
- 一套API适配多种传输方式
- 可在运行时动态切换传输类型
- 支持多种编解码器（JSON/Protobuf/MessagePack/Binary）

### ✅ 高性能设计
- 零拷贝技术（共享内存）
- 批量传输支持
- 异步消息处理
- 连接池管理

### ✅ 灵活的通信模式
- **点对点（P2P）** - Send/Receive
- **请求-响应（Request-Response）** - Request/Response
- **发布-订阅（Pub-Sub）** - Publish/Subscribe
- **流式传输（Streaming）** - Stream双向流

### ✅ 生产就绪
- 完善的错误处理
- 消息超时控制
- 连接保活机制
- TLS加密支持
- 统计监控

---

## 核心接口

### 1. Transport 接口

所有传输实现的统一接口：

```go
type Transport interface {
    Start(ctx context.Context) error
    Stop() error
    
    Send(ctx context.Context, msg *Message) error
    SendAsync(ctx context.Context, msg *Message) error
    Receive(ctx context.Context, timeout time.Duration) (*Message, error)
    
    Request(ctx context.Context, req *Message, timeout time.Duration) (*Message, error)
    
    Subscribe(ctx context.Context, topic string, handler MessageHandler) error
    Unsubscribe(topic string) error
    Publish(ctx context.Context, topic string, msg *Message) error
    
    IsRunning() bool
    GetStats() *TransportStats
    GetType() TransportType
    SetCodec(codec Codec)
    Close() error
}
```

### 2. Server 接口

服务端接口：

```go
type Server interface {
    Start(ctx context.Context, addr string) error
    Stop() error
    
    RegisterHandler(msgType string, handler MessageHandler) error
    UnregisterHandler(msgType string) error
    
    IsRunning() bool
    GetAddr() string
    GetStats() *ServerStats
}
```

### 3. Client 接口

客户端接口：

```go
type Client interface {
    Connect(ctx context.Context, addr string) error
    Disconnect() error
    
    Send(ctx context.Context, msg *Message) error
    Call(ctx context.Context, method string, req *Message, resp *Message) error
    CallAsync(ctx context.Context, method string, req *Message, callback func(*Message, error)) error
    
    IsConnected() bool
    GetAddr() string
    Close() error
}
```

---

## 消息格式

### Message 结构

```go
type Message struct {
    Header   *MessageHeader        // 消息头
    Payload  []byte                // 消息载荷
    Metadata map[string]string     // 元数据
}

type MessageHeader struct {
    ID            string   // 消息唯一ID
    Type          string   // 消息类型
    Source        string   // 来源模块
    Destination   string   // 目标模块
    Timestamp     int64    // 时间戳（纳秒）
    CorrelationID string   // 关联ID（用于请求-响应）
    ReplyTo       string   // 回复地址
    Priority      int      // 优先级（0-9）
    TTL           int64    // 生存时间（毫秒）
    ContentType   string   // 内容类型
    Version       string   // 协议版本
}
```

### 预定义消息类型

```go
const (
    MessageTypePacket    = "packet"     // 数据包消息
    MessageTypeFlow      = "flow"       // 流记录消息
    MessageTypeAlert     = "alert"      // 告警消息
    MessageTypeStats     = "stats"      // 统计信息
    MessageTypeCommand   = "command"    // 命令消息
    MessageTypeResponse  = "response"   // 响应消息
    MessageTypeHeartbeat = "heartbeat"  // 心跳消息
    MessageTypeEvent     = "event"      // 事件消息
)
```

---

## 使用示例

### 1. 共享内存传输（零拷贝）

#### 发送端（采集模块）

```go
package main

import (
    "context"
    "datarealm.cn/network/pkg/transport"
)

func main() {
    // 创建共享内存配置
    config := transport.DefaultTransportConfig(transport.TransportTypeSharedMemory)
    config.ShmPath = "/dev/shm/network-capture"
    config.ShmSize = 256 * 1024 * 1024  // 256MB
    
    // 创建传输实例
    trans, err := transport.NewTransport(config)
    if err != nil {
        panic(err)
    }
    
    // 启动传输层
    ctx := context.Background()
    if err := trans.Start(ctx); err != nil {
        panic(err)
    }
    defer trans.Stop()
    
    // 发送数据包消息
    for packet := range getPackets() {
        msg := transport.NewMessage(string(transport.MessageTypePacket), packet.Data)
        msg.Header.Source = "capture"
        msg.Header.Destination = "parser"
        
        if err := trans.Send(ctx, msg); err != nil {
            log.Printf("send error: %v", err)
        }
    }
}
```

#### 接收端（解析模块）

```go
package main

import (
    "context"
    "datarealm.cn/network/pkg/transport"
)

func main() {
    // 创建共享内存配置（相同路径）
    config := transport.DefaultTransportConfig(transport.TransportTypeSharedMemory)
    config.ShmPath = "/dev/shm/network-capture"
    
    // 创建传输实例
    trans, err := transport.NewTransport(config)
    if err != nil {
        panic(err)
    }
    
    // 启动传输层
    ctx := context.Background()
    if err := trans.Start(ctx); err != nil {
        panic(err)
    }
    defer trans.Stop()
    
    // 接收消息
    for {
        msg, err := trans.Receive(ctx, 0) // 阻塞等待
        if err != nil {
            log.Printf("receive error: %v", err)
            continue
        }
        
        // 处理消息（零拷贝）
        processPacket(msg.Payload)
    }
}
```

### 2. gRPC传输（跨节点）

#### 服务端（解析模块）

```go
package main

import (
    "context"
    "datarealm.cn/network/pkg/transport"
)

func main() {
    // 创建gRPC配置
    config := transport.DefaultTransportConfig(transport.TransportTypeGRPC)
    config.Address = "0.0.0.0:50051"
    
    // 创建服务端
    server, err := transport.NewServer(config)
    if err != nil {
        panic(err)
    }
    
    // 注册消息处理器
    server.RegisterHandler(string(transport.MessageTypeFlow), func(ctx context.Context, msg *transport.Message) (*transport.Message, error) {
        // 处理流记录
        processFlow(msg.Payload)
        
        // 返回响应
        resp := transport.NewResponseMessage(msg, []byte("OK"))
        return resp, nil
    })
    
    // 启动服务端
    ctx := context.Background()
    if err := server.Start(ctx, config.Address); err != nil {
        panic(err)
    }
    defer server.Stop()
    
    // 阻塞等待
    select {}
}
```

#### 客户端（存储模块）

```go
package main

import (
    "context"
    "datarealm.cn/network/pkg/transport"
)

func main() {
    // 创建gRPC配置
    config := transport.DefaultTransportConfig(transport.TransportTypeGRPC)
    config.Address = "parser-node:50051"
    
    // 创建客户端
    client, err := transport.NewClient(config)
    if err != nil {
        panic(err)
    }
    
    // 连接服务端
    ctx := context.Background()
    if err := client.Connect(ctx, config.Address); err != nil {
        panic(err)
    }
    defer client.Close()
    
    // 调用远程方法
    req := transport.NewMessage(string(transport.MessageTypeFlow), flowData)
    resp := &transport.Message{}
    
    if err := client.Call(ctx, "ProcessFlow", req, resp); err != nil {
        log.Printf("call error: %v", err)
    }
}
```

### 3. 发布-订阅模式

#### 发布者

```go
package main

import (
    "context"
    "datarealm.cn/network/pkg/transport"
)

func main() {
    config := transport.DefaultTransportConfig(transport.TransportTypeGRPC)
    trans, _ := transport.NewTransport(config)
    trans.Start(context.Background())
    defer trans.Stop()
    
    // 发布告警消息
    alertMsg := transport.NewMessage(string(transport.MessageTypeAlert), alertData)
    trans.Publish(context.Background(), "alerts.high", alertMsg)
}
```

#### 订阅者

```go
package main

import (
    "context"
    "datarealm.cn/network/pkg/transport"
)

func main() {
    config := transport.DefaultTransportConfig(transport.TransportTypeGRPC)
    trans, _ := transport.NewTransport(config)
    trans.Start(context.Background())
    defer trans.Stop()
    
    // 订阅告警主题
    trans.Subscribe(context.Background(), "alerts.*", func(ctx context.Context, msg *transport.Message) (*transport.Message, error) {
        log.Printf("收到告警: %s", msg.Payload)
        return nil, nil
    })
    
    select {} // 阻塞
}
```

### 4. RESTful传输

```go
package main

import (
    "context"
    "datarealm.cn/network/pkg/transport"
)

func main() {
    // 创建RESTful配置
    config := transport.DefaultTransportConfig(transport.TransportTypeREST)
    config.Address = "http://localhost:8080"
    
    // 创建客户端
    client, _ := transport.NewClient(config)
    client.Connect(context.Background(), config.Address)
    defer client.Close()
    
    // 发送HTTP请求
    msg := transport.NewMessage(string(transport.MessageTypeStats), statsData)
    client.Send(context.Background(), msg)
}
```

---

## 编解码器

### 支持的编解码器

| 编解码器 | 性能 | 可读性 | 跨语言 | 适用场景 |
|---------|------|--------|--------|----------|
| **JSON** | 中 | 高 | 是 | 开发调试、跨语言 |
| **Protobuf** | 高 | 低 | 是 | 生产环境、高性能 |
| **MessagePack** | 高 | 低 | 是 | 生产环境 |
| **Gob** | 高 | 低 | 否 | Go内部通信 |
| **Binary** | 极高 | 低 | 否 | 零拷贝场景 |

### 使用自定义编解码器

```go
// 设置JSON编解码器
codec := transport.NewJSONCodec()
trans.SetCodec(codec)

// 启用压缩
compressor := NewGzipCompressor()
compressedCodec := transport.NewCompressionCodec(codec, compressor)
trans.SetCodec(compressedCodec)

// 启用加密
encryptor := NewAESEncryptor(key)
encryptedCodec := transport.NewEncryptionCodec(codec, encryptor)
trans.SetCodec(encryptedCodec)
```

---

## 配置选项

### TransportConfig

```go
config := &transport.TransportConfig{
    Type:              transport.TransportTypeGRPC,
    Address:           "localhost:50051",
    BufferSize:        10000,
    Timeout:           30 * time.Second,
    MaxMessageSize:    64 * 1024 * 1024,  // 64MB
    EnableCompression: true,
    EnableEncryption:  false,
    CodecType:         transport.CodecTypeJSON,
    PoolSize:          10,
    KeepAlive: &transport.KeepAliveConfig{
        Enabled: true,
        Time:    30 * time.Second,
        Timeout: 10 * time.Second,
    },
}
```

### TLS配置

```go
config.TLSConfig = &transport.TLSConfig{
    Enabled:            true,
    CertFile:           "/path/to/cert.pem",
    KeyFile:            "/path/to/key.pem",
    CAFile:             "/path/to/ca.pem",
    ServerName:         "example.com",
    InsecureSkipVerify: false,
}
```

---

## 性能对比

| 传输方式 | 延迟 | 吞吐量 | CPU占用 | 适用场景 |
|---------|------|--------|---------|----------|
| **共享内存** | < 1μs | 极高 | 极低 | 同机进程，零拷贝 |
| **Unix Socket** | < 10μs | 高 | 低 | 同机进程 |
| **gRPC** | < 1ms | 高 | 中 | 跨节点，高性能 |
| **RESTful** | 1-10ms | 中 | 中 | 跨节点，易集成 |

---

## 监控指标

### TransportStats

```go
stats := trans.GetStats()
fmt.Printf("发送消息数: %d\n", stats.MessagesSent)
fmt.Printf("接收消息数: %d\n", stats.MessagesReceived)
fmt.Printf("发送字节数: %d\n", stats.BytesSent)
fmt.Printf("接收字节数: %d\n", stats.BytesReceived)
fmt.Printf("错误数: %d\n", stats.ErrorCount)
fmt.Printf("平均延迟: %d ns\n", stats.AverageLatency)
fmt.Printf("活动连接数: %d\n", stats.ActiveConnections)
```

---

## 错误处理

### 常见错误

```go
// 检查超时错误
if transport.IsTimeout(err) {
    // 重试或其他处理
}

// 检查连接错误
if transport.IsConnectionError(err) {
    // 重连
}

// 检查消息错误
if transport.IsMessageError(err) {
    // 验证消息格式
}

// 检查可恢复错误
if transport.IsRecoverableError(err) {
    // 重试
}
```

---

## 最佳实践

### 1. 选择合适的传输方式

- **同机高性能场景** → 共享内存
- **同机一般场景** → Unix Socket
- **跨节点RPC调用** → gRPC
- **跨语言/简单集成** → RESTful

### 2. 消息大小控制

```go
// 对于大消息，考虑分片传输
if msg.Size() > config.MaxMessageSize {
    // 分片逻辑
}
```

### 3. 超时控制

```go
// 总是设置超时
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

msg, err := trans.Receive(ctx, 5*time.Second)
```

### 4. 资源清理

```go
// 使用defer确保资源释放
defer trans.Close()
defer client.Close()
defer server.Stop()
```

### 5. 错误处理

```go
// 区分可恢复和不可恢复错误
if transport.IsRecoverableError(err) {
    // 重试逻辑
    retry(func() error {
        return trans.Send(ctx, msg)
    })
} else {
    // 记录错误并返回
    log.Error("fatal error", err)
    return err
}
```

---

## 实现状态

### ✅ 已完成
- [x] 核心接口定义
- [x] 消息格式定义
- [x] 编解码器接口
- [x] 错误类型定义
- [x] 配置结构

### 🚧 进行中
- [ ] 共享内存实现
- [ ] gRPC实现
- [ ] RESTful实现
- [ ] Unix Socket实现

### 📅 计划中
- [ ] 编解码器实现（JSON/Protobuf/MessagePack）
- [ ] 压缩支持
- [ ] 加密支持
- [ ] 连接池
- [ ] 完整的单元测试
- [ ] 性能基准测试
- [ ] 示例程序

---

## 下一步

1. **实现共享内存传输** - 零拷贝，极致性能
2. **实现gRPC传输** - 跨节点高性能RPC
3. **实现RESTful传输** - HTTP/JSON易集成
4. **编写完整的测试** - 确保可靠性
5. **性能测试与优化** - 达到设计目标

---

**文档版本：** v1.0  
**创建日期：** 2024-01-15  
**状态：** 接口定义完成，实现进行中

