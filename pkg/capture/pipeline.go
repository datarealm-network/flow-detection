package capture

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"datarealm.cn/network/pkg/transport"
)

// Pipeline 捕获管道 - 连接捕获层和传输层
type Pipeline struct {
	capture   Capture
	transport transport.Transport
	config    *PipelineConfig

	// 状态
	running atomic.Bool
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup

	// 统计
	stats      *PipelineStats
	statsMutex sync.RWMutex
}

// PipelineConfig 管道配置
type PipelineConfig struct {
	// CaptureConfig 捕获配置
	CaptureConfig *CaptureConfig

	// TransportConfig 传输配置
	TransportConfig *transport.TransportConfig

	// BatchSize 批量发送大小（0 表示单包发送）
	BatchSize int

	// BatchTimeout 批量发送超时
	BatchTimeout time.Duration

	// WorkerCount 工作线程数量（0 表示单线程）
	WorkerCount int

	// ChannelSize 通道缓冲区大小
	ChannelSize int

	// ZeroCopyMode 是否启用零拷贝模式
	// true: 使用 ReadPacketZeroCopy (更快，但需要立即处理)
	// false: 使用 ReadPacket (较慢，但数据独立)
	ZeroCopyMode bool

	// PacketFilter 数据包过滤函数（应用层过滤）
	PacketFilter func(*Packet) bool

	// PacketTransform 数据包转换函数
	// 可以在这里提取关键字段，减少传输数据量
	PacketTransform func(*Packet) []byte
}

// PipelineStats 管道统计信息
type PipelineStats struct {
	// PacketsCaptured 捕获的数据包数
	PacketsCaptured uint64

	// PacketsSent 发送的数据包数
	PacketsSent uint64

	// PacketsFiltered 过滤的数据包数
	PacketsFiltered uint64

	// PacketsDropped 丢弃的数据包数
	PacketsDropped uint64

	// BytesSent 发送的字节数
	BytesSent uint64

	// Errors 错误数
	Errors uint64

	// StartTime 启动时间
	StartTime time.Time

	// LastPacketTime 最后一个数据包时间
	LastPacketTime time.Time
}

// DefaultPipelineConfig 返回默认管道配置
func DefaultPipelineConfig(interfaceName, shmPath string) *PipelineConfig {
	return &PipelineConfig{
		CaptureConfig:   DefaultCaptureConfig(interfaceName),
		TransportConfig: transport.NewSPSCConfig(shmPath),
		BatchSize:       100,                   // 批量发送 100 个包
		BatchTimeout:    10 * time.Millisecond, // 10ms 超时
		WorkerCount:     1,                     // 单线程（SPSC）
		ChannelSize:     10000,                 // 1万个包缓冲
		ZeroCopyMode:    true,                  // 启用零拷贝
		PacketFilter:    nil,                   // 无应用层过滤
		PacketTransform: defaultPacketTransform,
	}
}

// NewPipeline 创建捕获管道
func NewPipeline(config *PipelineConfig) (*Pipeline, error) {
	if config == nil {
		return nil, ErrInvalidConfig
	}

	// 创建捕获实例
	capture, err := NewCapture(config.CaptureConfig)
	if err != nil {
		return nil, err
	}

	// 创建传输实例
	trans, err := transport.NewTransport(config.TransportConfig)
	if err != nil {
		return nil, err
	}

	p := &Pipeline{
		capture:   capture,
		transport: trans,
		config:    config,
		stats: &PipelineStats{
			StartTime: time.Now(),
		},
	}

	return p, nil
}

// Start 启动管道
func (p *Pipeline) Start(ctx context.Context) error {
	if p.running.Load() {
		return ErrAlreadyRunning
	}

	// 启动捕获
	if err := p.capture.Start(ctx); err != nil {
		return fmt.Errorf("start capture failed: %w", err)
	}

	// 启动传输
	if err := p.transport.Start(ctx); err != nil {
		p.capture.Stop()
		return fmt.Errorf("start transport failed: %w", err)
	}

	p.ctx, p.cancel = context.WithCancel(ctx)
	p.running.Store(true)
	p.stats.StartTime = time.Now()

	// 启动工作线程
	if p.config.WorkerCount > 0 {
		// 多线程模式（需要 MPMC）
		p.startMultiWorker()
	} else {
		// 单线程模式（SPSC，最高性能）
		p.startSingleWorker()
	}

	return nil
}

// startSingleWorker 启动单线程工作模式（SPSC 零拷贝）
func (p *Pipeline) startSingleWorker() {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		p.runSingleWorker()
	}()
}

// runSingleWorker 单线程工作循环（零拷贝优化）
func (p *Pipeline) runSingleWorker() {
	batch := make([]*Packet, 0, p.config.BatchSize)
	batchTimer := time.NewTimer(p.config.BatchTimeout)
	defer batchTimer.Stop()

	for {
		select {
		case <-p.ctx.Done():
			// 发送剩余批次
			if len(batch) > 0 {
				p.sendBatch(batch)
			}
			return

		default:
			// 零拷贝读取
			var packet *Packet
			var err error

			if p.config.ZeroCopyMode {
				packet, err = p.capture.ReadPacketZeroCopy()
			} else {
				packet, err = p.capture.ReadPacket()
			}

			if err != nil {
				if err == ErrTimeout {
					// 超时是正常的，检查批次
					if len(batch) > 0 {
						select {
						case <-batchTimer.C:
							p.sendBatch(batch)
							batch = batch[:0]
							batchTimer.Reset(p.config.BatchTimeout)
						default:
						}
					}
					continue
				}
				p.incrementError()
				continue
			}

			// 更新统计
			atomic.AddUint64(&p.stats.PacketsCaptured, 1)

			// 应用层过滤
			if p.config.PacketFilter != nil && !p.config.PacketFilter(packet) {
				atomic.AddUint64(&p.stats.PacketsFiltered, 1)
				continue
			}

			// 批量模式
			if p.config.BatchSize > 1 {
				// 零拷贝模式下，需要立即拷贝数据
				if p.config.ZeroCopyMode {
					packetCopy := *packet
					packetCopy.Data = make([]byte, len(packet.Data))
					copy(packetCopy.Data, packet.Data)
					batch = append(batch, &packetCopy)
				} else {
					batch = append(batch, packet)
				}

				// 批次满，发送
				if len(batch) >= p.config.BatchSize {
					p.sendBatch(batch)
					batch = batch[:0]
					batchTimer.Reset(p.config.BatchTimeout)
				}
			} else {
				// 单包模式（零拷贝友好）
				p.sendPacket(packet)
			}
		}
	}
}

// sendPacket 发送单个数据包
func (p *Pipeline) sendPacket(packet *Packet) {
	// 转换数据
	data := p.config.PacketTransform(packet)

	// 创建消息
	msg := transport.NewMessage(string(transport.MessageTypePacket), data)
	msg.Header.Source = "capture"
	msg.Header.Destination = "parser"
	msg.Header.Timestamp = packet.Timestamp

	// 发送
	ctx, cancel := context.WithTimeout(p.ctx, 1*time.Second)
	defer cancel()

	if err := p.transport.Send(ctx, msg); err != nil {
		p.incrementError()
		atomic.AddUint64(&p.stats.PacketsDropped, 1)
		return
	}

	// 更新统计
	atomic.AddUint64(&p.stats.PacketsSent, 1)
	atomic.AddUint64(&p.stats.BytesSent, uint64(len(data)))
	p.statsMutex.Lock()
	p.stats.LastPacketTime = time.Now()
	p.statsMutex.Unlock()
}

// sendBatch 批量发送数据包
func (p *Pipeline) sendBatch(batch []*Packet) {
	for _, packet := range batch {
		p.sendPacket(packet)
	}
}

// startMultiWorker 启动多线程工作模式
func (p *Pipeline) startMultiWorker() {
	// 创建数据包通道
	packetChan := make(chan *Packet, p.config.ChannelSize)

	// 启动捕获goroutine
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		defer close(packetChan)

		for {
			select {
			case <-p.ctx.Done():
				return
			default:
				packet, err := p.capture.ReadPacket()
				if err != nil {
					if err != ErrTimeout {
						p.incrementError()
					}
					continue
				}

				atomic.AddUint64(&p.stats.PacketsCaptured, 1)

				// 应用层过滤
				if p.config.PacketFilter != nil && !p.config.PacketFilter(packet) {
					atomic.AddUint64(&p.stats.PacketsFiltered, 1)
					continue
				}

				select {
				case packetChan <- packet:
				case <-p.ctx.Done():
					return
				default:
					// 通道满，丢包
					atomic.AddUint64(&p.stats.PacketsDropped, 1)
				}
			}
		}
	}()

	// 启动发送worker
	for i := 0; i < p.config.WorkerCount; i++ {
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			for packet := range packetChan {
				p.sendPacket(packet)
			}
		}()
	}
}

// Stop 停止管道
func (p *Pipeline) Stop() error {
	if !p.running.Load() {
		return nil
	}

	p.running.Store(false)
	if p.cancel != nil {
		p.cancel()
	}

	// 等待所有worker退出
	p.wg.Wait()

	// 停止传输
	if err := p.transport.Stop(); err != nil {
		return err
	}

	// 停止捕获
	if err := p.capture.Stop(); err != nil {
		return err
	}

	return nil
}

// GetStats 获取统计信息
func (p *Pipeline) GetStats() *PipelineStats {
	p.statsMutex.RLock()
	defer p.statsMutex.RUnlock()

	stats := *p.stats
	return &stats
}

// GetCaptureStats 获取捕获统计
func (p *Pipeline) GetCaptureStats() *CaptureStats {
	return p.capture.GetStats()
}

// GetTransportStats 获取传输统计
func (p *Pipeline) GetTransportStats() *transport.TransportStats {
	return p.transport.GetStats()
}

// IsRunning 是否正在运行
func (p *Pipeline) IsRunning() bool {
	return p.running.Load()
}

// Close 关闭管道
func (p *Pipeline) Close() error {
	return p.Stop()
}

// incrementError 增加错误计数
func (p *Pipeline) incrementError() {
	atomic.AddUint64(&p.stats.Errors, 1)
}

// defaultPacketTransform 默认数据包转换
// 提取数据包关键信息，减少传输数据量
func defaultPacketTransform(packet *Packet) []byte {
	// 简单实现：只传输前 128 字节（包含以太网+IP+TCP/UDP头）
	// 实际使用中可以解析协议，只传输关键字段
	maxLen := 128
	if len(packet.Data) < maxLen {
		maxLen = len(packet.Data)
	}

	// 添加元数据头部
	// 格式: [8字节时间戳][4字节原始长度][4字节捕获长度][N字节数据]
	headerSize := 16
	result := make([]byte, headerSize+maxLen)

	// 写入时间戳
	timestamp := uint64(packet.Timestamp)
	result[0] = byte(timestamp >> 56)
	result[1] = byte(timestamp >> 48)
	result[2] = byte(timestamp >> 40)
	result[3] = byte(timestamp >> 32)
	result[4] = byte(timestamp >> 24)
	result[5] = byte(timestamp >> 16)
	result[6] = byte(timestamp >> 8)
	result[7] = byte(timestamp)

	// 写入原始长度
	result[8] = byte(packet.OriginalLength >> 24)
	result[9] = byte(packet.OriginalLength >> 16)
	result[10] = byte(packet.OriginalLength >> 8)
	result[11] = byte(packet.OriginalLength)

	// 写入捕获长度
	result[12] = byte(packet.CaptureLength >> 24)
	result[13] = byte(packet.CaptureLength >> 16)
	result[14] = byte(packet.CaptureLength >> 8)
	result[15] = byte(packet.CaptureLength)

	// 拷贝数据
	copy(result[headerSize:], packet.Data[:maxLen])

	return result
}
