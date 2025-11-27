// Package capture 提供网络数据包捕获的抽象接口和实现
package capture

import (
	"context"

	"datarealm.cn/network/pkg/packet"
)

// Capturer 网络数据包捕获器接口
// 定义了捕获网络数据包的核心行为抽象
type Capturer interface {
	// Start 启动捕获器
	// ctx 用于控制捕获器的生命周期
	Start(ctx context.Context) error

	// Stop 停止捕获器
	Stop() error

	// Packets 返回数据包的只读通道
	// 捕获到的数据包会通过此通道发送给消费者
	Packets() <-chan *packet.Packet

	// Stats 获取捕获统计信息
	Stats() *CaptureStats

	// IsRunning 检查捕获器是否正在运行
	IsRunning() bool

	// SetBPFFilter 设置 BPF 过滤器
	SetBPFFilter(filter string) error
}

// CaptureStats 捕获统计信息
type CaptureStats struct {
	// PacketsReceived 接收到的数据包数量
	PacketsReceived uint64

	// PacketsDropped 丢弃的数据包数量（内核层面）
	PacketsDropped uint64

	// PacketsIfDropped 接口层面丢弃的数据包数量
	PacketsIfDropped uint64

	// BytesReceived 接收到的字节数
	BytesReceived uint64

	// PacketsSent 发送到下游的数据包数量
	PacketsSent uint64

	// ErrorCount 错误计数
	ErrorCount uint64
}

// PacketHandler 数据包处理函数类型
// 用于回调式的数据包处理
type PacketHandler func(pkt *packet.Packet) error

// BatchPacketHandler 批量数据包处理函数类型
// 用于高性能场景的批量处理
type BatchPacketHandler func(pkts []*packet.Packet) error
