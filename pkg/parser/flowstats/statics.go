package flowstats

import (
	"sort"
	"sync"
	"time"

	"datarealm.cn/network/pkg/parser"
)

// FlowStats 流量统计器
type FlowStats struct {
	windowSize    time.Duration                // 统计时间窗口
	records       []*parser.FlowRecord         // 时间窗口内的所有记录
	ipTraffic     map[string]uint64            // IP地址 -> 总字节数
	protocolDist  map[string]uint64            // 协议 -> 总字节数
	totalBytes    uint64                       // 时间窗口内总字节数
	totalPackets  uint64                       // 时间窗口内总包数
	mutex         sync.RWMutex                 // 读写锁，保证并发安全
}

// NewFlowStats 创建统计器
// windowSize: 统计时间窗口（如1分钟）
func NewFlowStats(windowSize time.Duration) *FlowStats {
	return &FlowStats{
		windowSize:   windowSize,
		records:      make([]*parser.FlowRecord, 0),
		ipTraffic:    make(map[string]uint64),
		protocolDist: make(map[string]uint64),
		totalBytes:   0,
		totalPackets: 0,
	}
}

// AddRecord 添加一条流量记录
func (fs *FlowStats) AddRecord(record *parser.FlowRecord) {
	if record == nil {
		return
	}

	fs.mutex.Lock()
	defer fs.mutex.Unlock()

	// 清理过期记录（超出时间窗口的记录）
	fs.cleanExpiredRecords()

	// 添加新记录
	fs.records = append(fs.records, record)

	// 更新统计信息
	fs.totalBytes += record.BytesSent
	fs.totalPackets += record.PacketsSent

	// 更新IP流量统计（源IP和目标IP都统计）
	fs.ipTraffic[record.SrcIP] += record.BytesSent
	fs.ipTraffic[record.DstIP] += record.BytesSent

	// 更新协议分布统计
	fs.protocolDist[record.Protocol] += record.BytesSent
}

// cleanExpiredRecords 清理过期的记录（超出时间窗口）
func (fs *FlowStats) cleanExpiredRecords() {
	if len(fs.records) == 0 {
		return
	}

	now := time.Now()
	cutoffTime := now.Add(-fs.windowSize)

	// 找到第一个未过期的记录索引
	validIndex := 0
	for i, record := range fs.records {
		if record.Timestamp.After(cutoffTime) {
			validIndex = i
			break
		}
		// 从统计中移除过期记录
		fs.totalBytes -= record.BytesSent
		fs.totalPackets -= record.PacketsSent

		// 从IP流量统计中减去
		fs.ipTraffic[record.SrcIP] -= record.BytesSent
		if fs.ipTraffic[record.SrcIP] == 0 {
			delete(fs.ipTraffic, record.SrcIP)
		}
		fs.ipTraffic[record.DstIP] -= record.BytesSent
		if fs.ipTraffic[record.DstIP] == 0 {
			delete(fs.ipTraffic, record.DstIP)
		}

		// 从协议分布中减去
		fs.protocolDist[record.Protocol] -= record.BytesSent
		if fs.protocolDist[record.Protocol] == 0 {
			delete(fs.protocolDist, record.Protocol)
		}
	}

	// 移除过期记录
	if validIndex > 0 {
		fs.records = fs.records[validIndex:]
	}
}

// GetTotalBytes 获取时间窗口内总字节数
func (fs *FlowStats) GetTotalBytes() uint64 {
	fs.mutex.RLock()
	defer fs.mutex.RUnlock()
	return fs.totalBytes
}

// GetTotalPackets 获取时间窗口内总包数
func (fs *FlowStats) GetTotalPackets() uint64 {
	fs.mutex.RLock()
	defer fs.mutex.RUnlock()
	return fs.totalPackets
}

// GetTopKIPs 获取流量最大的K个IP地址
func (fs *FlowStats) GetTopKIPs(k int) []string {
	fs.mutex.RLock()
	defer fs.mutex.RUnlock()

	if k <= 0 || len(fs.ipTraffic) == 0 {
		return []string{}
	}

	// 将map转换为切片，方便排序
	type ipTrafficPair struct {
		ip      string
		traffic uint64
	}

	pairs := make([]ipTrafficPair, 0, len(fs.ipTraffic))
	for ip, traffic := range fs.ipTraffic {
		pairs = append(pairs, ipTrafficPair{ip: ip, traffic: traffic})
	}

	// 按流量降序排序
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].traffic > pairs[j].traffic
	})

	// 取前K个
	if k > len(pairs) {
		k = len(pairs)
	}

	result := make([]string, k)
	for i := 0; i < k; i++ {
		result[i] = pairs[i].ip
	}

	return result
}

// GetProtocolDistribution 获取协议分布
// 返回: map[协议]字节数
func (fs *FlowStats) GetProtocolDistribution() map[string]uint64 {
	fs.mutex.RLock()
	defer fs.mutex.RUnlock()

	// 返回协议分布的副本，避免外部修改
	result := make(map[string]uint64, len(fs.protocolDist))
	for protocol, bytes := range fs.protocolDist {
		result[protocol] = bytes
	}

	return result
}
