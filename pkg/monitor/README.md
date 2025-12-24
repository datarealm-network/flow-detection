# Monitor 包 - IP/CIDR 管控与热更新

本包实现了高性能的 IP/CIDR 匹配和配置热更新功能，用于网络流量监控场景。

## 功能特性

- ✅ 支持 YAML/JSON 配置文件
- ✅ 单 IP 匹配（哈希表，O(1)）
- ✅ CIDR 匹配（前缀匹配，O(n) 但 n 通常很小）
- ✅ 配置热更新（fsnotify + atomic.Value，无锁切换）
- ✅ 高并发安全（支持每秒 10 万+ 次匹配）
- ✅ 变更日志记录

## 快速开始

### 1. 创建配置文件

YAML 格式 (`configs/monitor.yaml`):
```yaml
enabled: true
targets:
  - type: ip
    value: "192.168.1.1"
  - type: cidr
    value: "10.0.0.0/24"
```

JSON 格式 (`configs/monitor.json`):
```json
{
  "enabled": true,
  "targets": [
    { "type": "ip", "value": "192.168.1.1" },
    { "type": "cidr", "value": "10.0.0.0/24" }
  ]
}
```

### 2. 使用示例

```go
package main

import (
    "context"
    "net"
    "datarealm.cn/network/pkg/monitor"
)

func main() {
    // 创建监控集合
    ms, err := monitor.NewMonitorSet("configs/monitor.yaml")
    if err != nil {
        panic(err)
    }

    // 启动热更新监听
    ctx := context.Background()
    if err := ms.Start(ctx); err != nil {
        panic(err)
    }
    defer ms.Stop()

    // 匹配 IP
    ip := net.ParseIP("192.168.1.1")
    if ms.Match(ip) {
        println("IP 在管控列表中")
    }

    // 修改配置文件后，系统会自动热更新，无需重启
}
```

## API 说明

### MonitorSet

主要接口：

- `NewMonitorSet(configPath string) (*MonitorSet, error)` - 创建监控集合
- `Start(ctx context.Context) error` - 启动热更新监听
- `Stop() error` - 停止监听
- `Match(ip net.IP) bool` - 匹配 IP（线程安全，无锁读取）
- `GetStats() (ipCount, cidrCount int)` - 获取统计信息

### Matcher

底层匹配器（通常不需要直接使用）：

- `NewMatcher() *Matcher` - 创建匹配器
- `Update(config *Config) error` - 更新规则
- `Match(ip net.IP) bool` - 匹配 IP
- `Count() (ipCount, cidrCount int)` - 获取规则数量

## 性能特性

- **单 IP 匹配**：使用 `map[string]struct{}`，O(1) 时间复杂度
- **CIDR 匹配**：遍历 CIDR 列表，O(n)，但通常 n < 100
- **并发安全**：使用 `sync.RWMutex` 保护读写
- **热更新**：使用 `atomic.Value` 实现无锁切换，不影响正在进行的匹配操作

## 测试

运行测试：
```bash
go test ./pkg/monitor -v
```

运行基准测试：
```bash
go test ./pkg/monitor -bench=.
```

## 注意事项

1. 目前只支持 **IPv4**，IPv6 会被忽略
2. 配置文件路径必须存在且可读
3. 热更新有 100ms 防抖，避免频繁触发
4. 配置变更会记录日志，包含变更前后的统计信息

