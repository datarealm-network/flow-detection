package parser

import "time"

// FlowRecord 流量记录
type FlowRecord struct {
	Timestamp   time.Time
	SrcIP       string
	DstIP       string
	Protocol    string // "TCP", "UDP", "ICMP"
	BytesSent   uint64
	PacketsSent uint64
}
