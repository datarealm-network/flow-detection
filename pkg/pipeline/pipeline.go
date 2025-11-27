// Package pipeline 提供捕获数据到下游处理的高效传输通道
package pipeline

import (
	"context"

	"datarealm.cn/network/pkg/packet"
)

// Pipeline 数据管道接口
// 定义了从上游（捕获器）到下游（解析器）的数据传输抽象
type Pipeline interface {
	// Start 启动管道
	Start(ctx context.Context) error

	// Stop 停止管道
	Stop() error

	// Input 返回输入通道（用于上游写入）
	Input() chan<- *packet.Packet

	// Output 返回输出通道（用于下游消费）
	Output() <-chan *packet.Packet

	// Stats 获取管道统计信息
	Stats() *PipelineStats

	// IsRunning 检查管道是否正在运行
	IsRunning() bool
}

// BatchPipeline 批量数据管道接口
// 支持批量传输以提升吞吐量
type BatchPipeline interface {
	Pipeline

	// BatchOutput 返回批量输出通道
	BatchOutput() <-chan []*packet.Packet

	// SetBatchSize 设置批量大小
	SetBatchSize(size int)

	// SetBatchTimeout 设置批量超时时间（强制刷新不完整批次）
	SetBatchTimeout(timeout int64)
}

// PipelineStats 管道统计信息
type PipelineStats struct {
	// PacketsIn 接收的数据包数量
	PacketsIn uint64

	// PacketsOut 发送的数据包数量
	PacketsOut uint64

	// PacketsDropped 丢弃的数据包数量（缓冲区满）
	PacketsDropped uint64

	// BytesIn 接收的字节数
	BytesIn uint64

	// BytesOut 发送的字节数
	BytesOut uint64

	// BatchesSent 发送的批次数量（批量模式）
	BatchesSent uint64

	// QueueLength 当前队列长度
	QueueLength int

	// QueueCapacity 队列容量
	QueueCapacity int
}

// Consumer 数据包消费者接口
// 下游处理模块需要实现此接口
type Consumer interface {
	// Consume 消费单个数据包
	Consume(ctx context.Context, pkt *packet.Packet) error

	// ConsumeBatch 消费一批数据包
	ConsumeBatch(ctx context.Context, pkts []*packet.Packet) error

	// OnStart 消费者启动时调用
	OnStart(ctx context.Context) error

	// OnStop 消费者停止时调用
	OnStop() error
}

// Producer 数据包生产者接口
// 上游捕获模块可以实现此接口
type Producer interface {
	// Produce 生产单个数据包
	Produce(ctx context.Context, pkt *packet.Packet) error

	// ProduceBatch 生产一批数据包
	ProduceBatch(ctx context.Context, pkts []*packet.Packet) error
}

// Middleware 中间件接口
// 用于在管道中插入处理逻辑（如过滤、转换等）
type Middleware interface {
	// Process 处理数据包，返回处理后的数据包或 nil（丢弃）
	Process(pkt *packet.Packet) *packet.Packet

	// Name 中间件名称
	Name() string
}

// FilterFunc 过滤函数类型
// 返回 true 表示保留数据包，false 表示丢弃
type FilterFunc func(pkt *packet.Packet) bool

// TransformFunc 转换函数类型
// 对数据包进行转换处理
type TransformFunc func(pkt *packet.Packet) *packet.Packet
