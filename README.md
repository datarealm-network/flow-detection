# Flow Detection

基于 Go 的网络流量捕获与分析项目，重点支持高速零拷贝抓包、灵活管道和多种传输方式，方便在本地快速搭建流量检测或安全审计能力。

## 功能概览

- **capture**：AF_PACKET 抓包、BPF 过滤、Fan-out、统计等能力。
- **pipeline / engine**：把抓包、处理、消费者串起来，可单包或批量处理。
- **parser**：对流量记录做窗口化统计（总包/总字节、TopK IP、协议分布）。
- **transport**：抽象出共享内存、gRPC、REST 等多种传输实现与编码。
- **packet**：对原始报文做协议字段解析。

## 目录速览

```
pkg/
├── capture/     # 抓包接口与实现
├── engine/      # CaptureEngine 组合模块
├── packet/      # 数据包结构
├── parser/      # 流量记录与统计
├── pipeline/    # 处理/分发管道
└── transport/   # 传输抽象与实现
```

## 快速开始

> 需要 Go 1.23+、Linux、具备 root 或 `CAP_NET_RAW` 权限。

```bash
git clone https://github.com/datarealm-network/flow-detection.git
cd flow-detection
go mod download

# 演示：实时打印捕获到的数据包
sudo go test ./pkg/engine -run TestCapturePackets -v
```

更改 `engine_test.go` 中的 `config.CaptureConfig.BPFFilter` 即可只捕获 TCP/HTTP/DNS 等不同流量。

## 测试

```bash
# 普通单元测试（无 root）
go test ./...

# 验证抓包与性能（需 root）
sudo go test ./pkg/capture ./pkg/engine -v
```

## 许可证

本项目使用 [MIT License](LICENSE)。

