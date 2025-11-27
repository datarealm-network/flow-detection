package capture

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"datarealm.cn/network/pkg/packet"
	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"
)

// PcapCapturer 基于 gopacket/pcap 的网络数据包捕获器实现
type PcapCapturer struct {
	config *Config
	handle *pcap.Handle

	// 输出通道
	packetChan chan *packet.Packet

	// 状态管理
	running atomic.Bool
	mu      sync.RWMutex

	// 统计信息
	stats *CaptureStats

	// 取消函数
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewPcapCapturer 创建一个新的 PcapCapturer 实例
func NewPcapCapturer(config *Config) (*PcapCapturer, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &PcapCapturer{
		config:     config,
		packetChan: make(chan *packet.Packet, config.ChannelSize),
		stats:      &CaptureStats{},
	}, nil
}

// Start 启动捕获器
func (c *PcapCapturer) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running.Load() {
		return ErrAlreadyRunning
	}

	// 打开网络接口
	handle, err := c.openHandle()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrOpenDevice, err)
	}
	c.handle = handle

	// 设置 BPF 过滤器
	if c.config.BPFFilter != "" {
		if err := c.handle.SetBPFFilter(c.config.BPFFilter); err != nil {
			c.handle.Close()
			return fmt.Errorf("%w: %v", ErrSetFilter, err)
		}
	}

	// 创建取消上下文
	captureCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	// 标记为运行中
	c.running.Store(true)

	// 启动捕获 goroutine
	c.wg.Add(1)
	go c.captureLoop(captureCtx)

	return nil
}

// openHandle 打开 pcap handle
func (c *PcapCapturer) openHandle() (*pcap.Handle, error) {
	// 创建非活动句柄以设置更多参数
	inactive, err := pcap.NewInactiveHandle(c.config.InterfaceName)
	if err != nil {
		return nil, fmt.Errorf("failed to create inactive handle: %w", err)
	}
	defer inactive.CleanUp()

	// 设置捕获长度
	if err := inactive.SetSnapLen(int(c.config.SnapLen)); err != nil {
		return nil, fmt.Errorf("failed to set snap length: %w", err)
	}

	// 设置混杂模式
	if err := inactive.SetPromisc(c.config.Promiscuous); err != nil {
		return nil, fmt.Errorf("failed to set promiscuous mode: %w", err)
	}

	// 设置超时
	timeout := c.config.Timeout
	if timeout == 0 {
		timeout = pcap.BlockForever
	}
	if err := inactive.SetTimeout(timeout); err != nil {
		return nil, fmt.Errorf("failed to set timeout: %w", err)
	}

	// 设置缓冲区大小
	if c.config.BufferSize > 0 {
		if err := inactive.SetBufferSize(c.config.BufferSize); err != nil {
			return nil, fmt.Errorf("failed to set buffer size: %w", err)
		}
	}

	// 设置即时模式
	if c.config.ImmediateMode {
		if err := inactive.SetImmediateMode(true); err != nil {
			return nil, fmt.Errorf("failed to set immediate mode: %w", err)
		}
	}

	// 激活句柄
	handle, err := inactive.Activate()
	if err != nil {
		return nil, fmt.Errorf("failed to activate handle: %w", err)
	}

	return handle, nil
}

// captureLoop 捕获循环（在独立 goroutine 中运行）
func (c *PcapCapturer) captureLoop(ctx context.Context) {
	defer c.wg.Done()
	defer close(c.packetChan)

	// 使用 gopacket 的包源
	packetSource := gopacket.NewPacketSource(c.handle, c.handle.LinkType())
	packetSource.DecodeOptions.Lazy = true // 延迟解码，提升性能
	packetSource.DecodeOptions.NoCopy = c.config.EnableZeroCopy

	packets := packetSource.Packets()

	for {
		select {
		case <-ctx.Done():
			return
		case gp, ok := <-packets:
			if !ok {
				return
			}

			// 更新统计
			atomic.AddUint64(&c.stats.PacketsReceived, 1)
			atomic.AddUint64(&c.stats.BytesReceived, uint64(len(gp.Data())))

			// 创建 Packet 对象
			pkt := c.createPacket(gp)

			// 发送到输出通道（非阻塞）
			select {
			case c.packetChan <- pkt:
				atomic.AddUint64(&c.stats.PacketsSent, 1)
			default:
				// 通道满，丢弃数据包
				atomic.AddUint64(&c.stats.PacketsDropped, 1)
			}
		}
	}
}

// createPacket 从 gopacket.Packet 创建 packet.Packet
func (c *PcapCapturer) createPacket(gp gopacket.Packet) *packet.Packet {
	metadata := gp.Metadata()
	captureLen := metadata.CaptureLength
	originalLen := metadata.Length

	var pkt *packet.Packet
	if c.config.EnableZeroCopy {
		pkt = packet.NewPacketZeroCopy(gp.Data(), metadata.Timestamp, captureLen, originalLen)
	} else {
		pkt = packet.NewPacket(gp.Data(), metadata.Timestamp, captureLen, originalLen)
	}

	// 设置元数据
	pkt.Metadata.InterfaceName = c.config.InterfaceName
	pkt.Metadata.DeviceID = c.config.DeviceID
	pkt.Metadata.Truncated = metadata.Truncated

	return pkt
}

// Stop 停止捕获器
func (c *PcapCapturer) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running.Load() {
		return ErrNotRunning
	}

	// 取消捕获上下文
	if c.cancel != nil {
		c.cancel()
	}

	// 关闭 pcap handle
	if c.handle != nil {
		c.handle.Close()
	}

	// 等待捕获 goroutine 结束
	c.wg.Wait()

	c.running.Store(false)
	return nil
}

// Packets 返回数据包的只读通道
func (c *PcapCapturer) Packets() <-chan *packet.Packet {
	return c.packetChan
}

// Stats 获取捕获统计信息
func (c *PcapCapturer) Stats() *CaptureStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 获取 pcap 统计
	if c.handle != nil {
		if pcapStats, err := c.handle.Stats(); err == nil {
			atomic.StoreUint64(&c.stats.PacketsDropped, uint64(pcapStats.PacketsDropped))
			atomic.StoreUint64(&c.stats.PacketsIfDropped, uint64(pcapStats.PacketsIfDropped))
		}
	}

	// 返回统计副本
	return &CaptureStats{
		PacketsReceived:  atomic.LoadUint64(&c.stats.PacketsReceived),
		PacketsDropped:   atomic.LoadUint64(&c.stats.PacketsDropped),
		PacketsIfDropped: atomic.LoadUint64(&c.stats.PacketsIfDropped),
		BytesReceived:    atomic.LoadUint64(&c.stats.BytesReceived),
		PacketsSent:      atomic.LoadUint64(&c.stats.PacketsSent),
		ErrorCount:       atomic.LoadUint64(&c.stats.ErrorCount),
	}
}

// IsRunning 检查捕获器是否正在运行
func (c *PcapCapturer) IsRunning() bool {
	return c.running.Load()
}

// SetBPFFilter 设置 BPF 过滤器
func (c *PcapCapturer) SetBPFFilter(filter string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.config.BPFFilter = filter

	if c.handle != nil && c.running.Load() {
		if err := c.handle.SetBPFFilter(filter); err != nil {
			return fmt.Errorf("%w: %v", ErrSetFilter, err)
		}
	}

	return nil
}

// GetConfig 获取当前配置（只读）
func (c *PcapCapturer) GetConfig() Config {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return *c.config
}

// 确保 PcapCapturer 实现了 Capturer 接口
var _ Capturer = (*PcapCapturer)(nil)

// ListInterfaces 列出所有可用的网络接口
func ListInterfaces() ([]pcap.Interface, error) {
	return pcap.FindAllDevs()
}

// GetInterfaceByName 根据名称获取网络接口信息
func GetInterfaceByName(name string) (*pcap.Interface, error) {
	devices, err := pcap.FindAllDevs()
	if err != nil {
		return nil, err
	}

	for _, device := range devices {
		if device.Name == name {
			return &device, nil
		}
	}

	return nil, fmt.Errorf("interface %s not found", name)
}

// OpenOffline 打开 pcap 文件进行离线分析
func OpenOffline(filename string, config *Config) (*PcapCapturer, error) {
	if config == nil {
		config = &Config{
			ChannelSize: 10000,
		}
	}

	handle, err := pcap.OpenOffline(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open pcap file: %w", err)
	}

	capturer := &PcapCapturer{
		config:     config,
		handle:     handle,
		packetChan: make(chan *packet.Packet, config.ChannelSize),
		stats:      &CaptureStats{},
	}

	return capturer, nil
}

// StartOfflineCapture 开始离线捕获（从 pcap 文件读取）
func (c *PcapCapturer) StartOfflineCapture(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running.Load() {
		return ErrAlreadyRunning
	}

	if c.handle == nil {
		return fmt.Errorf("handle is not initialized, use OpenOffline first")
	}

	// 设置 BPF 过滤器
	if c.config.BPFFilter != "" {
		if err := c.handle.SetBPFFilter(c.config.BPFFilter); err != nil {
			return fmt.Errorf("%w: %v", ErrSetFilter, err)
		}
	}

	// 创建取消上下文
	captureCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	// 标记为运行中
	c.running.Store(true)

	// 启动捕获 goroutine
	c.wg.Add(1)
	go c.captureLoop(captureCtx)

	return nil
}

// WaitUntilDone 等待捕获结束（用于离线分析）
func (c *PcapCapturer) WaitUntilDone() {
	c.wg.Wait()
}

// StatsString 返回格式化的统计字符串
func (c *PcapCapturer) StatsString() string {
	stats := c.Stats()
	return fmt.Sprintf(
		"Packets: received=%d, sent=%d, dropped=%d, if_dropped=%d | Bytes: %d | Errors: %d",
		stats.PacketsReceived,
		stats.PacketsSent,
		stats.PacketsDropped,
		stats.PacketsIfDropped,
		stats.BytesReceived,
		stats.ErrorCount,
	)
}

// StatsPeriodic 定期输出统计信息
func (c *PcapCapturer) StatsPeriodic(ctx context.Context, interval time.Duration, callback func(stats *CaptureStats)) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if callback != nil {
				callback(c.Stats())
			}
		}
	}
}
