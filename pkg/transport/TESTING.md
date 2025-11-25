# Transport SPSC 测试指南

## 概述

本文档说明如何在Linux系统上测试SPSC（单生产者单消费者）共享内存传输实现。

**注意：共享内存传输仅支持Linux系统！**

---

## 环境要求

- **操作系统**: Linux (内核 3.0+)
- **Go版本**: 1.19+
- **权限**: 能够访问 `/dev/shm` 目录

---

## 单元测试

### 1. 共享内存和Ring Buffer测试

```bash
cd pkg/transport/shm
go test -v
```

**测试内容：**
- 共享内存创建/打开/关闭
- Ring Buffer写入/读取
- 缓冲区满/空处理
- 环绕（Wrap Around）
- 性能基准测试

**预期结果：**
```
=== RUN   TestSharedMemory
--- PASS: TestSharedMemory (0.00s)
=== RUN   TestRingBuffer
--- PASS: TestRingBuffer (0.00s)
=== RUN   TestRingBufferFull
--- PASS: TestRingBufferFull (0.01s)
=== RUN   TestRingBufferStatistics
--- PASS: TestRingBufferStatistics (0.00s)
=== RUN   TestRingBufferWrapAround
--- PASS: TestRingBufferWrapAround (0.00s)
PASS
ok      datarealm.cn/network/pkg/transport/shm  0.023s
```

### 2. SPSC传输测试

```bash
cd pkg/transport
go test -v -run TestSPSCTransport
```

**测试内容：**
- 基本消息收发
- 多条消息传输
- 超时处理
- 跨进程通信
- 吞吐量基准测试

**预期结果：**
```
=== RUN   TestSPSCTransportBasic
--- PASS: TestSPSCTransportBasic (0.05s)
=== RUN   TestSPSCTransportMultipleMessages
--- PASS: TestSPSCTransportMultipleMessages (0.10s)
=== RUN   TestSPSCTransportTimeout
--- PASS: TestSPSCTransportTimeout (1.01s)
=== RUN   TestSPSCTransportCrossProcess
--- PASS: TestSPSCTransportCrossProcess (3.00s)
PASS
ok      datarealm.cn/network/pkg/transport      4.168s
```

### 3. 性能基准测试

```bash
# Ring Buffer性能
cd pkg/transport/shm
go test -bench=BenchmarkRingBuffer -benchmem

# SPSC传输吞吐量
cd pkg/transport
go test -bench=BenchmarkSPSCTransportThroughput -benchtime=10s
```

**预期性能指标：**
- **Ring Buffer写入**: > 10M ops/sec
- **Ring Buffer读取**: > 10M ops/sec  
- **SPSC吞吐量**: > 1 GB/s (1KB消息)
- **延迟**: < 1μs

---

## 手动测试

### 示例1: 生产者-消费者（同进程）

创建文件 `examples/transport/spsc_test.go`：

```go
//go:build linux

package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "datarealm.cn/network/pkg/transport"
)

func main() {
    shmPath := "manual-test-spsc"
    
    // 创建生产者
    producerConfig := transport.NewSPSCConfig(shmPath)
    producerConfig.Role = transport.RoleProducer
    
    producer, err := transport.NewSPSCTransport(producerConfig)
    if err != nil {
        log.Fatalf("Create producer failed: %v", err)
    }
    
    ctx := context.Background()
    if err := producer.Start(ctx); err != nil {
        log.Fatalf("Start producer failed: %v", err)
    }
    defer producer.Stop()
    
    // 创建消费者
    consumerConfig := transport.NewSPSCConfig(shmPath)
    consumerConfig.Role = transport.RoleConsumer
    
    consumer, err := transport.NewSPSCTransport(consumerConfig)
    if err != nil {
        log.Fatalf("Create consumer failed: %v", err)
    }
    
    if err := consumer.Start(ctx); err != nil {
        log.Fatalf("Start consumer failed: %v", err)
    }
    defer consumer.Stop()
    
    // 启动消费者goroutine
    go func() {
        for i := 0; i < 10; i++ {
            msg, err := consumer.Receive(ctx, 5*time.Second)
            if err != nil {
                log.Printf("Receive error: %v", err)
                return
            }
            fmt.Printf("Received: %s\n", string(msg.Payload))
        }
    }()
    
    // 发送消息
    for i := 0; i < 10; i++ {
        msg := transport.NewMessage("test", 
            []byte(fmt.Sprintf("Message %d", i)))
        if err := producer.Send(ctx, msg); err != nil {
            log.Printf("Send error: %v", err)
            return
        }
        fmt.Printf("Sent: Message %d\n", i)
        time.Sleep(500 * time.Millisecond)
    }
    
    time.Sleep(2 * time.Second)
    fmt.Println("Test completed!")
}
```

运行：
```bash
cd examples/transport
go run spsc_test.go
```

### 示例2: 跨进程通信

**生产者进程：**
```go
//go:build linux

package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "datarealm.cn/network/pkg/transport"
)

func main() {
    config := transport.NewSPSCConfig("cross-process-test")
    config.Role = transport.RoleProducer
    
    producer, err := transport.NewSPSCTransport(config)
    if err != nil {
        log.Fatal(err)
    }
    
    ctx := context.Background()
    producer.Start(ctx)
    defer producer.Stop()
    
    for i := 0; ; i++ {
        msg := transport.NewMessage("data", 
            []byte(fmt.Sprintf("Message %d", i)))
        producer.Send(ctx, msg)
        fmt.Printf("Sent: %d\n", i)
        time.Sleep(1 * time.Second)
    }
}
```

**消费者进程：**
```go
//go:build linux

package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "datarealm.cn/network/pkg/transport"
)

func main() {
    config := transport.NewSPSCConfig("cross-process-test")
    config.Role = transport.RoleConsumer
    
    consumer, err := transport.NewSPSCTransport(config)
    if err != nil {
        log.Fatal(err)
    }
    
    ctx := context.Background()
    consumer.Start(ctx)
    defer consumer.Stop()
    
    for {
        msg, err := consumer.Receive(ctx, 5*time.Second)
        if err != nil {
            log.Printf("Receive error: %v", err)
            continue
        }
        fmt.Printf("Received: %s\n", string(msg.Payload))
    }
}
```

运行：
```bash
# 终端1
go run producer.go

# 终端2
go run consumer.go
```

---

## 故障排查

### 问题1: 权限错误

**错误信息：**
```
create shared memory failed: open /dev/shm/xxx: permission denied
```

**解决方案：**
```bash
# 检查权限
ls -la /dev/shm/

# 清理旧文件
rm -f /dev/shm/test-*
rm -f /dev/shm/bench-*
```

### 问题2: 文件已存在

**错误信息：**
```
create shared memory failed: file exists
```

**解决方案：**
```bash
# 清理共享内存文件
rm -f /dev/shm/你的文件名

# 或使用不同的名称
```

### 问题3: 消费者超时

**错误信息：**
```
Receive error: timeout
```

**原因：**
- 生产者未启动
- 生产者已停止发送
- 共享内存路径不匹配

**解决方案：**
- 确保生产者先启动
- 检查共享内存路径是否一致
- 检查 `/dev/shm` 中的文件

### 问题4: 缓冲区满

**现象：**
生产者发送阻塞

**解决方案：**
- 增加共享内存大小 (`ShmSize`)
- 加快消费者处理速度
- 检查消费者是否正常运行

---

## 性能优化建议

### 1. 共享内存大小

```go
config.ShmSize = 256 * 1024 * 1024  // 256MB (默认)
```

**建议：**
- 低流量: 64MB
- 中流量: 256MB
- 高流量: 512MB - 1GB
- 超高流量: 2GB+

### 2. 编解码器选择

```go
config.CodecType = transport.CodecTypeBinary  // 零拷贝，最快
// config.CodecType = transport.CodecTypeJSON  // 可读性好，较慢
```

**性能对比：**
- Binary: 最快，零拷贝
- Protobuf: 较快，适合跨语言
- JSON: 最慢，但易于调试

### 3. 消息大小

```go
// 避免过大的消息
maxMessageSize := 1 * 1024 * 1024  // 1MB
```

**建议：**
- 小消息 (< 1KB): 最佳性能
- 中消息 (1KB - 64KB): 良好性能
- 大消息 (> 1MB): 考虑分片

---

## 监控指标

### 获取统计信息

```go
stats := transport.GetStats()
fmt.Printf("Messages Sent: %d\n", stats.MessagesSent)
fmt.Printf("Messages Received: %d\n", stats.MessagesReceived)
fmt.Printf("Bytes Sent: %d\n", stats.BytesSent)
fmt.Printf("Bytes Received: %d\n", stats.BytesReceived)
fmt.Printf("Errors: %d\n", stats.ErrorCount)
```

### 关键指标

- **吞吐量**: BytesSent/BytesReceived per second
- **消息速率**: MessagesSent/MessagesReceived per second
- **错误率**: ErrorCount / MessagesSent
- **延迟**: 发送到接收的时间差

---

## 最佳实践

### 1. 资源清理

```go
// 始终使用defer清理
producer, _ := transport.NewSPSCTransport(config)
producer.Start(ctx)
defer producer.Stop()  // 重要！
```

### 2. 错误处理

```go
msg, err := consumer.Receive(ctx, timeout)
if err != nil {
    if err == transport.ErrTimeout {
        // 超时是正常情况
        continue
    }
    // 其他错误需要处理
    log.Printf("Error: %v", err)
}
```

### 3. 优雅关闭

```go
// 使用context控制生命周期
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

// 监听信号
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

go func() {
    <-sigChan
    cancel()
}()
```

---

## 总结

SPSC共享内存传输提供：

✅ **极致性能**: < 1μs 延迟，> 10 Gbps 吞吐  
✅ **零拷贝**: 数据直接在共享内存中传递  
✅ **进程间通信**: 支持跨进程高效通信  
✅ **简单易用**: 统一的Transport接口  

**适用场景：**
- 单机多进程架构
- 高性能数据传输
- 流量采集→解析
- 实时数据处理

**不适用场景：**
- 跨节点通信（使用gRPC）
- 多生产者多消费者（使用MPMC模式，待实现）
- Windows系统（不支持）

---

**测试完成日期:** 2024-01-15  
**版本:** v1.0  
**状态:** 基础功能完成 ✅

