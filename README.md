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


## 测试

```bash

cd pkg/engine


配置国内镜像
go env -w GOPROXY=https://goproxy.cn,direct
go mod tidy
sudo yum install libpcap-devel

go test 


```

## 许可证

本项目使用 [MIT License](LICENSE)。

