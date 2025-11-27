package engine

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"datarealm.cn/network/pkg/capture"
	"datarealm.cn/network/pkg/packet"
)

// PrintConsumer 打印数据包信息的消费者
type PrintConsumer struct {
	name        string
	packetCount uint64
	verbose     bool
}

func NewPrintConsumer(name string, verbose bool) *PrintConsumer {
	return &PrintConsumer{
		name:    name,
		verbose: verbose,
	}
}

func (c *PrintConsumer) OnStart(ctx context.Context) error {
	fmt.Printf("\n[%s] 消费者已启动，开始接收数据包...\n", c.name)
	fmt.Println("=" + string(make([]byte, 79)))
	return nil
}

func (c *PrintConsumer) OnStop() error {
	fmt.Println("=" + string(make([]byte, 79)))
	fmt.Printf("[%s] 消费者已停止，共处理 %d 个数据包\n", c.name, c.packetCount)
	return nil
}

func (c *PrintConsumer) Consume(ctx context.Context, pkt *packet.Packet) error {
	c.packetCount++
	info := pkt.NetworkInfo()

	if c.verbose {
		// 详细模式：打印每个数据包
		fmt.Printf("[#%05d] %s | %s:%d -> %s:%d | %s | %d bytes\n",
			c.packetCount,
			pkt.Timestamp.Format("15:04:05.000"),
			info.SrcIP, info.SrcPort,
			info.DstIP, info.DstPort,
			info.Protocol,
			pkt.Size(),
		)
	} else {
		// 简洁模式：每100个包打印一次
		if c.packetCount%100 == 0 {
			fmt.Printf("[#%05d] %s | %s:%d -> %s:%d | %s | %d bytes\n",
				c.packetCount,
				pkt.Timestamp.Format("15:04:05.000"),
				info.SrcIP, info.SrcPort,
				info.DstIP, info.DstPort,
				info.Protocol,
				pkt.Size(),
			)
		}
	}

	return nil
}

func (c *PrintConsumer) ConsumeBatch(ctx context.Context, pkts []*packet.Packet) error {
	for _, pkt := range pkts {
		if err := c.Consume(ctx, pkt); err != nil {
			return err
		}
	}
	return nil
}

// TestListInterfaces 测试列出所有网络接口
func TestListInterfaces(t *testing.T) {
	interfaces, err := capture.ListInterfaces()
	if err != nil {
		t.Fatalf("获取网络接口失败: %v", err)
	}

	fmt.Println("\n可用的网络接口:")
	fmt.Println("================")
	for _, iface := range interfaces {
		fmt.Printf("\n接口名称: %s\n", iface.Name)
		if iface.Description != "" {
			fmt.Printf("  描述: %s\n", iface.Description)
		}
		fmt.Printf("  地址:\n")
		for _, addr := range iface.Addresses {
			fmt.Printf("    - IP: %s, 掩码: %s\n", addr.IP, addr.Netmask)
		}
	}
	fmt.Println()
}

// TestCapturePackets 测试捕获数据包（需要 root 权限）
// 运行方式: sudo go test -v -run TestCapturePackets -timeout 30s
func TestCapturePackets(t *testing.T) {
	// 尝试自动检测网络接口
	interfaceName := detectInterface(t)
	if interfaceName == "" {
		t.Skip("未找到合适的网络接口，跳过测试")
	}

	fmt.Printf("\n使用接口: %s\n", interfaceName)

	// 创建引擎配置
	config := DefaultEngineConfig(interfaceName)
	config.CaptureConfig.SnapLen = 256     // 只捕获协议头
	config.CaptureConfig.ChannelSize = 1000
	config.CaptureConfig.BPFFilter = ""    // 捕获所有流量，可改为 "tcp" 或 "tcp port 80"
	
	config.PipelineConfig.BufferSize = 1000
	config.EnableStats = true
	config.StatsInterval = 5 * time.Second
	config.StatsCallback = func(stats *EngineStats) {
		fmt.Printf("\n[统计] 接收: %d, 发送: %d, 丢弃: %d, 字节: %d\n",
			stats.CaptureStats.PacketsReceived,
			stats.CaptureStats.PacketsSent,
			stats.CaptureStats.PacketsDropped,
			stats.CaptureStats.BytesReceived,
		)
	}

	// 创建引擎
	eng, err := NewCaptureEngine(config)
	if err != nil {
		t.Fatalf("创建捕获引擎失败: %v", err)
	}

	// 注册打印消费者（verbose=true 打印每个包）
	consumer := NewPrintConsumer("printer", true)
	eng.RegisterConsumer(consumer)

	// 创建上下文（10秒后超时）
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 监听中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case <-sigChan:
			fmt.Println("\n收到中断信号，正在停止...")
			cancel()
		case <-ctx.Done():
		}
	}()

	// 启动引擎
	fmt.Println("\n启动捕获引擎...")
	if err := eng.Start(ctx); err != nil {
		t.Fatalf("启动引擎失败: %v", err)
	}

	fmt.Println("捕获中... (10秒后自动停止，或按 Ctrl+C 手动停止)")
	fmt.Printf("提示: 尝试 ping 其他主机或浏览网页来生成流量\n")

	// 等待上下文结束
	<-ctx.Done()

	// 停止引擎
	fmt.Println("\n停止捕获引擎...")
	if err := eng.Stop(); err != nil {
		t.Logf("停止引擎时出错: %v", err)
	}

	// 打印最终统计
	stats := eng.Stats()
	fmt.Println("\n最终统计:")
	fmt.Printf("  接收数据包: %d\n", stats.CaptureStats.PacketsReceived)
	fmt.Printf("  发送数据包: %d\n", stats.CaptureStats.PacketsSent)
	fmt.Printf("  丢弃数据包: %d\n", stats.CaptureStats.PacketsDropped)
	fmt.Printf("  接收字节数: %d\n", stats.CaptureStats.BytesReceived)
	fmt.Printf("  运行时长: %s\n", stats.Uptime.Round(time.Millisecond))
}

// TestCaptureTCPOnly 测试只捕获 TCP 数据包
// 运行方式: sudo go test -v -run TestCaptureTCPOnly -timeout 30s
func TestCaptureTCPOnly(t *testing.T) {
	interfaceName := detectInterface(t)
	if interfaceName == "" {
		t.Skip("未找到合适的网络接口，跳过测试")
	}

	fmt.Printf("\n使用接口: %s, 过滤: TCP\n", interfaceName)

	config := DefaultEngineConfig(interfaceName)
	config.CaptureConfig.BPFFilter = "tcp"
	config.CaptureConfig.SnapLen = 128

	eng, err := NewCaptureEngine(config)
	if err != nil {
		t.Fatalf("创建捕获引擎失败: %v", err)
	}

	consumer := NewPrintConsumer("tcp-printer", true)
	eng.RegisterConsumer(consumer)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := eng.Start(ctx); err != nil {
		t.Fatalf("启动引擎失败: %v", err)
	}

	fmt.Println("只捕获 TCP 流量... (10秒)")
	fmt.Println("提示: 尝试 curl http://example.com 来生成 TCP 流量")

	<-ctx.Done()
	eng.Stop()
}

// TestCaptureHTTP 测试捕获 HTTP 流量（端口80）
// 运行方式: sudo go test -v -run TestCaptureHTTP -timeout 30s
func TestCaptureHTTP(t *testing.T) {
	interfaceName := detectInterface(t)
	if interfaceName == "" {
		t.Skip("未找到合适的网络接口，跳过测试")
	}

	fmt.Printf("\n使用接口: %s, 过滤: HTTP (端口80)\n", interfaceName)

	config := DefaultEngineConfig(interfaceName)
	config.CaptureConfig.BPFFilter = "tcp port 80"

	eng, err := NewCaptureEngine(config)
	if err != nil {
		t.Fatalf("创建捕获引擎失败: %v", err)
	}

	consumer := NewPrintConsumer("http-printer", true)
	eng.RegisterConsumer(consumer)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := eng.Start(ctx); err != nil {
		t.Fatalf("启动引擎失败: %v", err)
	}

	fmt.Println("只捕获 HTTP 流量 (端口80)... (15秒)")
	fmt.Println("提示: 在另一个终端运行 curl http://example.com")

	<-ctx.Done()
	eng.Stop()
}

// TestCaptureDNS 测试捕获 DNS 流量
// 运行方式: sudo go test -v -run TestCaptureDNS -timeout 30s
func TestCaptureDNS(t *testing.T) {
	interfaceName := detectInterface(t)
	if interfaceName == "" {
		t.Skip("未找到合适的网络接口，跳过测试")
	}

	fmt.Printf("\n使用接口: %s, 过滤: DNS (端口53)\n", interfaceName)

	config := DefaultEngineConfig(interfaceName)
	config.CaptureConfig.BPFFilter = "udp port 53"

	eng, err := NewCaptureEngine(config)
	if err != nil {
		t.Fatalf("创建捕获引擎失败: %v", err)
	}

	consumer := NewPrintConsumer("dns-printer", true)
	eng.RegisterConsumer(consumer)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := eng.Start(ctx); err != nil {
		t.Fatalf("启动引擎失败: %v", err)
	}

	fmt.Println("只捕获 DNS 流量 (端口53)... (10秒)")
	fmt.Println("提示: 在另一个终端运行 nslookup example.com")

	<-ctx.Done()
	eng.Stop()
}

// detectInterface 自动检测合适的网络接口
func detectInterface(t *testing.T) string {
	interfaces, err := capture.ListInterfaces()
	if err != nil {
		t.Logf("获取网络接口失败: %v", err)
		return ""
	}

	// 优先选择有 IP 地址的接口
	preferredNames := []string{"eth0", "ens33", "enp0s3", "enp0s8", "wlan0", "wlp2s0"}
	
	// 先尝试匹配常见接口名
	for _, prefName := range preferredNames {
		for _, iface := range interfaces {
			if iface.Name == prefName && len(iface.Addresses) > 0 {
				return iface.Name
			}
		}
	}

	// 如果没找到，选择第一个有 IP 的非 loopback 接口
	for _, iface := range interfaces {
		if iface.Name != "lo" && iface.Name != "any" && len(iface.Addresses) > 0 {
			return iface.Name
		}
	}

	// 最后尝试 "any" 接口（捕获所有接口）
	for _, iface := range interfaces {
		if iface.Name == "any" {
			return iface.Name
		}
	}

	return ""
}

// BenchmarkCapture 基准测试捕获性能
func BenchmarkCapture(b *testing.B) {
	interfaceName := "any" // 或者指定具体接口

	config := DefaultEngineConfig(interfaceName)
	config.CaptureConfig.SnapLen = 128
	config.EnableStats = false

	eng, err := NewCaptureEngine(config)
	if err != nil {
		b.Skipf("创建引擎失败: %v", err)
	}

	// 创建一个只计数的消费者
	counter := &CounterConsumer{}
	eng.RegisterConsumer(counter)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := eng.Start(ctx); err != nil {
		b.Skipf("启动引擎失败: %v", err)
	}

	b.ResetTimer()

	// 等待捕获 N 个包或超时
	for counter.count < uint64(b.N) {
		time.Sleep(time.Millisecond)
	}

	b.StopTimer()
	eng.Stop()

	b.ReportMetric(float64(counter.count), "packets")
}

// CounterConsumer 只计数的消费者
type CounterConsumer struct {
	count uint64
}

func (c *CounterConsumer) OnStart(ctx context.Context) error { return nil }
func (c *CounterConsumer) OnStop() error                     { return nil }
func (c *CounterConsumer) Consume(ctx context.Context, pkt *packet.Packet) error {
	c.count++
	return nil
}
func (c *CounterConsumer) ConsumeBatch(ctx context.Context, pkts []*packet.Packet) error {
	c.count += uint64(len(pkts))
	return nil
}

