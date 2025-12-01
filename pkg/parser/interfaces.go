package parser

import (
	"context"
	"net"
	"time"

	"datarealm.cn/network/pkg/packet"
)

// IPMatcher IP 匹配器接口（用于判断 IP 是否在管控列表中）
type IPMatcher interface {
	// Match 判断 IP 是否匹配
	Match(ip net.IP) bool

	// MatchString 判断 IP 字符串是否匹配
	MatchString(ipStr string) bool
}

// FlowAggregator 流聚合器接口
// 负责将数据包聚合为流，管理流的生命周期
// ⚠️ 重要：只有涉及管控IP的流才会被创建和跟踪
type FlowAggregator interface {
	// AddPacket 添加数据包到对应流
	// 注意：调用者需要先判断包的源IP或目标IP是否在管控列表中
	// 只有涉及管控IP的包才会被处理，其他包会被忽略
	AddPacket(pkt *packet.Packet) error

	// ShouldProcess 判断数据包是否应该被处理（是否涉及管控IP）
	// 如果返回 false，则不需要调用 AddPacket
	ShouldProcess(pkt *packet.Packet, matcher IPMatcher) bool

	// GetFlow 获取/查找流
	GetFlow(key FlowKey) (*FlowContext, bool)

	// GetActiveFlowCount 获取活跃流数量
	GetActiveFlowCount() int

	// ExpireFlows 获取并移除过期流
	ExpireFlows() []*FlowRecord

	// SetTimeout 设置超时时间
	SetTimeout(tcp, udp, icmp, other time.Duration)

	// FlushAll 强制输出所有流
	FlushAll() []*FlowRecord

	// Stats 获取统计信息
	Stats() *AggregatorStats
}

// FlowTable 流表接口（活跃流存储）
type FlowTable interface {
	// Get 获取流
	Get(key FlowKey) (*FlowContext, bool)

	// Put 添加/更新流
	Put(key FlowKey, ctx *FlowContext)

	// Delete 删除流
	Delete(key FlowKey)

	// Size 获取流表大小
	Size() int

	// Clear 清空流表
	Clear()

	// Iterate 遍历所有流（用于过期检查）
	Iterate(fn func(key FlowKey, ctx *FlowContext) bool)
}

// TimeWheel 时间轮接口（超时管理）
type TimeWheel interface {
	// Add 添加流到时间轮（指定超时时间）
	Add(key FlowKey, timeout time.Duration)

	// Remove 从时间轮移除流
	Remove(key FlowKey)

	// Advance 推进时间轮，返回过期的流 Key 列表
	Advance(now time.Time) []FlowKey

	// SetTimeout 设置默认超时时间
	SetTimeout(protocol string, timeout time.Duration)
}

// FlowOutputter 流记录输出器接口
type FlowOutputter interface {
	// Output 输出流记录
	Output(record *FlowRecord) error

	// OutputBatch 批量输出流记录
	OutputBatch(records []*FlowRecord) error

	// Start 启动输出器
	Start(ctx context.Context) error

	// Stop 停止输出器
	Stop() error

	// Stats 获取统计信息
	Stats() *OutputterStats
}

// ProtocolParser 协议解析器接口
type ProtocolParser interface {
	// CanParse 判断是否可以解析该流量
	CanParse(ctx *FlowContext, payload []byte) bool

	// Parse 解析应用层数据，提取元数据
	Parse(ctx *FlowContext, payload []byte) (*AppMetadata, error)

	// Name 协议名称
	Name() string

	// DefaultPorts 关联端口
	DefaultPorts() []uint16
}

// FlowParser 流解析器主接口（实现 pipeline.Consumer）
// 这是整个流重组处理模块的主入口
// ⚠️ 重要：只有涉及管控IP的流才会被创建和跟踪
type FlowParser interface {
	// 实现 pipeline.Consumer 接口
	Consume(ctx context.Context, pkt *packet.Packet) error
	ConsumeBatch(ctx context.Context, pkts []*packet.Packet) error
	OnStart(ctx context.Context) error
	OnStop() error

	// 配置
	SetConfig(config *ParserConfig) error
	GetConfig() *ParserConfig

	// 流记录输出
	FlowRecords() <-chan *FlowRecord

	// 注册输出器
	RegisterOutputter(outputter FlowOutputter) error

	// 设置 IP 匹配器（用于判断是否处理该包）
	// ⚠️ 重要：只有源IP或目标IP在管控列表中的包才会被处理
	SetIPMatcher(matcher IPMatcher)

	// 统计
	Stats() *ParserStats

	// 手动触发
	FlushAll() []*FlowRecord // 强制输出所有流
}

// AggregatorStats 聚合器统计信息
type AggregatorStats struct {
	TotalFlowsCreated uint64
	TotalFlowsExpired uint64
	ActiveFlows       int
	TotalPackets      uint64
	TotalBytes        uint64
	ImportantFlows    int // 重要流数量
}

// OutputterStats 输出器统计信息
type OutputterStats struct {
	RecordsOutputted uint64
	BytesOutputted   uint64
	ErrorCount       uint64
}

// ParserStats 解析器统计信息
type ParserStats struct {
	AggregatorStats  *AggregatorStats
	OutputterStats   *OutputterStats
	PacketsProcessed uint64
	PacketsDropped   uint64
}
