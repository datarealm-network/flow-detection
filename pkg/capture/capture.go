// Package capture 提供网络流量捕获功能
// 支持零拷贝技术（AF_PACKET + PACKET_MMAP）和 eBPF 过滤器
package capture

import (
	"context"
	"time"
)

// Capture 流量捕获接口
type Capture interface {
	// Start 启动捕获
	Start(ctx context.Context) error

	// Stop 停止捕获
	Stop() error

	// ReadPacket 读取单个数据包（阻塞）
	// 返回数据包和错误
	ReadPacket() (*Packet, error)

	// ReadPacketZeroCopy 零拷贝读取数据包
	// 注意：返回的数据只在下次ReadPacket之前有效
	ReadPacketZeroCopy() (*Packet, error)

	// SetBPFFilter 设置 BPF 过滤器
	// filter: BPF 过滤表达式，例如 "tcp port 80"
	SetBPFFilter(filter string) error

	// SetPromiscuous 设置混杂模式
	SetPromiscuous(enable bool) error

	// GetStats 获取统计信息
	GetStats() *CaptureStats

	// IsRunning 是否正在运行
	IsRunning() bool

	// Close 关闭捕获
	Close() error
}

// Packet 网络数据包
type Packet struct {
	// Timestamp 捕获时间戳（纳秒）
	Timestamp int64

	// CaptureLength 实际捕获长度
	CaptureLength uint32

	// OriginalLength 原始数据包长度
	OriginalLength uint32

	// InterfaceIndex 网卡索引
	InterfaceIndex int

	// Data 数据包内容（零拷贝模式下指向 mmap 区域）
	Data []byte

	// Metadata 元数据
	Metadata *PacketMetadata
}

// PacketMetadata 数据包元数据
type PacketMetadata struct {
	// VLANTag VLAN 标签
	VLANTag uint16

	// RXHash RSS 哈希值
	RXHash uint32

	// Status 状态标志
	Status uint32

	// IsZeroCopy 是否零拷贝
	IsZeroCopy bool
}

// CaptureStats 捕获统计信息
type CaptureStats struct {
	// PacketsReceived 接收的数据包数
	PacketsReceived uint64

	// PacketsDropped 丢弃的数据包数（内核层）
	PacketsDropped uint64

	// PacketsIfDropped 网卡层丢包数
	PacketsIfDropped uint64

	// BytesReceived 接收的字节数
	BytesReceived uint64

	// ZeroCopyCount 零拷贝次数
	ZeroCopyCount uint64

	// FilteredCount 过滤器过滤的数据包数
	FilteredCount uint64

	// StartTime 开始时间
	StartTime time.Time

	// LastPacketTime 最后一个数据包时间
	LastPacketTime time.Time
}

// CaptureConfig 捕获配置
type CaptureConfig struct {
	// InterfaceName 网卡名称（如 eth0）
	InterfaceName string

	// SnapLen 捕获长度（字节）
	// 0 表示捕获完整数据包，建议 65535
	SnapLen uint32

	// Promiscuous 是否混杂模式
	Promiscuous bool

	// RingBufferSize Ring Buffer 大小（字节）
	// 建议 8MB - 256MB
	RingBufferSize int

	// FrameSize 单个帧大小（字节）
	// 建议 2048
	FrameSize int

	// BlockSize 块大小（字节）
	// 必须是 pagesize 的倍数，建议 4096 * N
	BlockSize int

	// Timeout 读取超时时间
	Timeout time.Duration

	// BPFFilter BPF 过滤表达式
	// 示例: "tcp port 80", "host 192.168.1.1"
	BPFFilter string

	// EnableBPFJIT 是否启用 BPF JIT 编译
	EnableBPFJIT bool

	// ZeroCopy 是否启用零拷贝模式
	ZeroCopy bool

	// FanoutGroup Fanout 组 ID（多进程捕获，0 表示禁用）
	FanoutGroup int

	// FanoutType Fanout 类型
	FanoutType FanoutType
}

// FanoutType Fanout 类型
type FanoutType int

const (
	// FanoutHash 基于流哈希分发
	FanoutHash FanoutType = 0

	// FanoutLB 负载均衡（轮询）
	FanoutLB FanoutType = 1

	// FanoutCPU 基于 CPU 分发
	FanoutCPU FanoutType = 2

	// FanoutRollover 滚动分发
	FanoutRollover FanoutType = 3

	// FanoutRandom 随机分发
	FanoutRandom FanoutType = 4

	// FanoutQM 队列映射
	FanoutQM FanoutType = 5
)

// DefaultCaptureConfig 返回默认配置
func DefaultCaptureConfig(interfaceName string) *CaptureConfig {
	return &CaptureConfig{
		InterfaceName:  interfaceName,
		SnapLen:        65535,                  // 捕获完整包
		Promiscuous:    true,                   // 混杂模式
		RingBufferSize: 256 * 1024 * 1024,      // 256MB
		FrameSize:      2048,                   // 2KB per frame
		BlockSize:      4096 * 32,              // 128KB per block
		Timeout:        100 * time.Millisecond, // 100ms 超时
		BPFFilter:      "",                     // 无过滤
		EnableBPFJIT:   true,                   // 启用 JIT
		ZeroCopy:       true,                   // 启用零拷贝
		FanoutGroup:    0,                      // 禁用 Fanout
		FanoutType:     FanoutHash,
	}
}

// Validate 验证配置
func (c *CaptureConfig) Validate() error {
	if c.InterfaceName == "" {
		return ErrInvalidInterface
	}
	if c.SnapLen == 0 {
		c.SnapLen = 65535
	}
	if c.RingBufferSize <= 0 {
		c.RingBufferSize = 256 * 1024 * 1024
	}
	if c.FrameSize <= 0 {
		c.FrameSize = 2048
	}
	if c.BlockSize <= 0 {
		c.BlockSize = 4096 * 32
	}
	return nil
}

// NewCapture 创建捕获实例
func NewCapture(config *CaptureConfig) (Capture, error) {
	if config == nil {
		return nil, ErrInvalidConfig
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}
	// 目前只支持 Linux 平台
	// 在非 Linux 平台会在编译时报错
	return newAFPacketCaptureImpl(config)
}
