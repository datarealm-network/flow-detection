//go:build linux

// 网络流量捕获示例程序
// 演示如何使用 AF_PACKET + PACKET_MMAP 零拷贝捕获网络流量
// 并通过共享内存传输到下游处理模块
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"datarealm.cn/network/pkg/capture"
	"datarealm.cn/network/pkg/transport"
)

var (
	ifaceName = flag.String("interface", "eth0", "网卡名称")
	shmPath   = flag.String("shm", "network-capture", "共享内存路径")
	bpfFilter = flag.String("filter", "", "BPF 过滤器 (例如: 'tcp port 80')")
	mode      = flag.String("mode", "producer", "运行模式: producer (捕获) 或 consumer (接收)")
	snapLen   = flag.Uint("snaplen", 128, "捕获长度（字节）")
	ringSize  = flag.Int("ringsize", 256, "Ring Buffer 大小 (MB)")
	duration  = flag.Duration("duration", 0, "运行时长（0 表示永久运行）")
	statsInt  = flag.Duration("stats", 5*time.Second, "统计输出间隔")
)

func main() {
	flag.Parse()

	if *mode == "producer" {
		runProducer()
	} else if *mode == "consumer" {
		runConsumer()
	} else {
		log.Fatalf("无效的模式: %s (应该是 producer 或 consumer)", *mode)
	}
}

// runProducer 运行生产者（捕获器）
func runProducer() {
	log.Printf("启动流量捕获...")
	log.Printf("  网卡: %s", *ifaceName)
	log.Printf("  共享内存: /dev/shm/%s", *shmPath)
	log.Printf("  BPF 过滤器: %s", *bpfFilter)
	log.Printf("  捕获长度: %d 字节", *snapLen)
	log.Printf("  Ring Buffer: %d MB", *ringSize)

	// 创建管道配置
	config := capture.DefaultPipelineConfig(*ifaceName, *shmPath)
	config.CaptureConfig.SnapLen = uint32(*snapLen)
	config.CaptureConfig.RingBufferSize = (*ringSize) * 1024 * 1024
	config.CaptureConfig.BPFFilter = *bpfFilter
	config.CaptureConfig.ZeroCopy = true
	config.CaptureConfig.EnableBPFJIT = true
	config.ZeroCopyMode = true
	config.BatchSize = 100 // 批量发送 100 个包
	config.TransportConfig.Role = transport.RoleProducer

	// 自定义数据包转换（只传输协议头部）
	config.PacketTransform = func(pkt *capture.Packet) []byte {
		// 只传输前 snapLen 字节
		maxLen := int(*snapLen)
		if len(pkt.Data) < maxLen {
			maxLen = len(pkt.Data)
		}

		// 添加元数据: [8字节时间戳][4字节长度][数据]
		result := make([]byte, 12+maxLen)

		// 时间戳
		ts := uint64(pkt.Timestamp)
		for i := 0; i < 8; i++ {
			result[i] = byte(ts >> (56 - i*8))
		}

		// 原始长度
		for i := 0; i < 4; i++ {
			result[8+i] = byte(pkt.OriginalLength >> (24 - i*8))
		}

		// 数据
		copy(result[12:], pkt.Data[:maxLen])

		return result
	}

	// 创建管道
	pipeline, err := capture.NewPipeline(config)
	if err != nil {
		log.Fatalf("创建管道失败: %v", err)
	}

	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 如果设置了运行时长，使用超时上下文
	if *duration > 0 {
		ctx, cancel = context.WithTimeout(ctx, *duration)
		defer cancel()
	}

	// 启动管道
	if err := pipeline.Start(ctx); err != nil {
		log.Fatalf("启动管道失败: %v", err)
	}
	defer pipeline.Stop()

	log.Println("捕获已启动，按 Ctrl+C 停止...")

	// 定时打印统计
	statsTicker := time.NewTicker(*statsInt)
	defer statsTicker.Stop()

	// 监听信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	lastStats := &capture.PipelineStats{}

	for {
		select {
		case <-ctx.Done():
			log.Println("运行时长到达，正在停止...")
			printFinalStats(pipeline)
			return

		case <-sigChan:
			log.Println("收到中断信号，正在停止...")
			printFinalStats(pipeline)
			return

		case <-statsTicker.C:
			stats := pipeline.GetStats()
			captureStats := pipeline.GetCaptureStats()
			transportStats := pipeline.GetTransportStats()

			// 计算速率
			duration := time.Since(lastStats.LastPacketTime)
			if duration == 0 {
				duration = *statsInt
			}

			pps := float64(stats.PacketsSent-lastStats.PacketsSent) / duration.Seconds()
			mbps := float64(stats.BytesSent-lastStats.BytesSent) * 8 / duration.Seconds() / 1e6

			log.Printf("统计信息:")
			log.Printf("  捕获: %d 包 (%d 过滤, %d 丢弃)",
				stats.PacketsCaptured, stats.PacketsFiltered, captureStats.PacketsDropped)
			log.Printf("  发送: %d 包, %.2f MB",
				stats.PacketsSent, float64(stats.BytesSent)/1e6)
			log.Printf("  速率: %.0f pps, %.2f Mbps", pps, mbps)
			log.Printf("  零拷贝: %d 次", captureStats.ZeroCopyCount)
			log.Printf("  传输: %d 消息, %.2f MB",
				transportStats.MessagesSent, float64(transportStats.BytesSent)/1e6)
			log.Printf("  错误: %d (捕获), %d (传输)",
				stats.Errors, transportStats.ErrorCount)
			log.Println()

			*lastStats = *stats
		}
	}
}

// runConsumer 运行消费者（接收器）
func runConsumer() {
	log.Printf("启动流量接收...")
	log.Printf("  共享内存: /dev/shm/%s", *shmPath)

	// 创建传输配置
	config := transport.NewSPSCConfig(*shmPath)
	config.Role = transport.RoleConsumer
	config.CodecType = transport.CodecTypeBinary

	// 创建传输实例
	trans, err := transport.NewTransport(config)
	if err != nil {
		log.Fatalf("创建传输失败: %v", err)
	}

	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动传输
	if err := trans.Start(ctx); err != nil {
		log.Fatalf("启动传输失败: %v", err)
	}
	defer trans.Stop()

	log.Println("接收已启动，按 Ctrl+C 停止...")

	// 统计
	var packetsReceived uint64
	var bytesReceived uint64
	startTime := time.Now()
	lastPrintTime := time.Now()

	// 监听信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// 定时打印统计
	statsTicker := time.NewTicker(*statsInt)
	defer statsTicker.Stop()

	// 接收循环
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				// 接收消息
				msg, err := trans.Receive(ctx, 100*time.Millisecond)
				if err != nil {
					if err == transport.ErrTimeout {
						continue
					}
					if ctx.Err() != nil {
						return
					}
					log.Printf("接收错误: %v", err)
					continue
				}

				// 解析数据包
				if len(msg.Payload) < 12 {
					log.Printf("数据包过小: %d 字节", len(msg.Payload))
					continue
				}

				// 提取时间戳
				var timestamp uint64
				for i := 0; i < 8; i++ {
					timestamp = (timestamp << 8) | uint64(msg.Payload[i])
				}

				// 提取原始长度
				var origLen uint32
				for i := 0; i < 4; i++ {
					origLen = (origLen << 8) | uint32(msg.Payload[8+i])
				}

				// 更新统计
				packetsReceived++
				bytesReceived += uint64(len(msg.Payload))

				// 可选：解析协议头部
				if len(msg.Payload) >= 26 {
					// 以太网目标MAC (偏移12)
					// 以太网类型 (偏移24)
					ethType := uint16(msg.Payload[24])<<8 | uint16(msg.Payload[25])

					// 输出示例（每 1000 个包输出一次）
					if packetsReceived%1000 == 0 {
						log.Printf("接收数据包 #%d: 时间戳=%d, 长度=%d, 以太网类型=0x%04x",
							packetsReceived, timestamp, origLen, ethType)
					}
				}
			}
		}
	}()

	// 主循环：打印统计
	for {
		select {
		case <-ctx.Done():
			printConsumerStats(packetsReceived, bytesReceived, startTime)
			return

		case <-sigChan:
			log.Println("收到中断信号，正在停止...")
			cancel()
			time.Sleep(100 * time.Millisecond)
			printConsumerStats(packetsReceived, bytesReceived, startTime)
			return

		case <-statsTicker.C:
			now := time.Now()
			duration := now.Sub(lastPrintTime).Seconds()
			totalDuration := now.Sub(startTime).Seconds()

			pps := float64(packetsReceived) / duration
			mbps := float64(bytesReceived) * 8 / duration / 1e6
			avgPps := float64(packetsReceived) / totalDuration

			log.Printf("接收统计:")
			log.Printf("  接收: %d 包, %.2f MB", packetsReceived, float64(bytesReceived)/1e6)
			log.Printf("  速率: %.0f pps, %.2f Mbps", pps, mbps)
			log.Printf("  平均: %.0f pps", avgPps)
			log.Println()

			lastPrintTime = now
		}
	}
}

// printFinalStats 打印最终统计
func printFinalStats(pipeline *capture.Pipeline) {
	stats := pipeline.GetStats()
	captureStats := pipeline.GetCaptureStats()
	transportStats := pipeline.GetTransportStats()

	duration := time.Since(stats.StartTime).Seconds()

	log.Println()
	log.Println("========== 最终统计 ==========")
	log.Printf("运行时长: %.2f 秒", duration)
	log.Printf("捕获数据包: %d", stats.PacketsCaptured)
	log.Printf("发送数据包: %d", stats.PacketsSent)
	log.Printf("过滤数据包: %d", stats.PacketsFiltered)
	log.Printf("丢弃数据包: %d (捕获: %d, 传输: %d)",
		stats.PacketsDropped+captureStats.PacketsDropped,
		captureStats.PacketsDropped,
		stats.PacketsDropped)
	log.Printf("发送字节数: %.2f MB", float64(stats.BytesSent)/1e6)
	log.Printf("零拷贝次数: %d", captureStats.ZeroCopyCount)
	log.Printf("平均速率: %.0f pps, %.2f Mbps",
		float64(stats.PacketsSent)/duration,
		float64(stats.BytesSent)*8/duration/1e6)
	log.Printf("错误数: %d", stats.Errors+transportStats.ErrorCount)
	log.Println("=============================")
}

// printConsumerStats 打印消费者统计
func printConsumerStats(packets, bytes uint64, startTime time.Time) {
	duration := time.Since(startTime).Seconds()

	log.Println()
	log.Println("========== 接收统计 ==========")
	log.Printf("运行时长: %.2f 秒", duration)
	log.Printf("接收数据包: %d", packets)
	log.Printf("接收字节数: %.2f MB", float64(bytes)/1e6)
	log.Printf("平均速率: %.0f pps, %.2f Mbps",
		float64(packets)/duration,
		float64(bytes)*8/duration/1e6)
	log.Println("=============================")
}
