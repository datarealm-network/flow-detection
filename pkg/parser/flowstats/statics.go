package flowstats

import (
	"time"

	"datarealm.cn/network/pkg/parser"
)

// FlowStats 流量统计器
type FlowStats struct {
	// TODO: 其他参与者实现数据结构
}

// NewFlowStats 创建统计器
// windowSize: 统计时间窗口（如1分钟）
func NewFlowStats(windowSize time.Duration) *FlowStats {
	// TODO: 其他参与者实现
	return &FlowStats{}
}

// AddRecord 添加一条流量记录
func (fs *FlowStats) AddRecord(record *parser.FlowRecord) {
	// TODO: 其他参与者实现
}

// GetTotalBytes 获取时间窗口内总字节数
func (fs *FlowStats) GetTotalBytes() uint64 {
	// TODO: 其他参与者实现
	return 0
}

// GetTotalPackets 获取时间窗口内总包数
func (fs *FlowStats) GetTotalPackets() uint64 {
	// TODO: 其他参与者实现
	return 0
}

// GetTopKIPs 获取流量最大的K个IP地址
func (fs *FlowStats) GetTopKIPs(k int) []string {
	// TODO: 其他参与者实现
	return []string{}
}

// GetProtocolDistribution 获取协议分布
// 返回: map[协议]字节数
func (fs *FlowStats) GetProtocolDistribution() map[string]uint64 {
	// TODO: 其他参与者实现
	return map[string]uint64{}
}
