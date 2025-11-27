// Package packet 定义网络数据包的核心数据结构
package packet

import (
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// Packet 表示一个捕获的网络数据包
type Packet struct {
	// Timestamp 数据包捕获时间戳
	Timestamp time.Time

	// CaptureLength 实际捕获的字节数
	CaptureLength int

	// Length 原始数据包的完整长度
	Length int

	// Data 数据包的原始字节数据
	Data []byte

	// Metadata 可选的元数据信息
	Metadata *PacketMetadata

	// gopacketData 缓存的 gopacket.Packet（延迟解析）
	gopacketData gopacket.Packet
}

// PacketMetadata 数据包的元数据信息
type PacketMetadata struct {
	// InterfaceName 捕获接口名称
	InterfaceName string

	// InterfaceIndex 捕获接口索引
	InterfaceIndex int

	// Truncated 是否被截断
	Truncated bool

	// DeviceID 设备标识（多设备场景）
	DeviceID int
}

// NetworkInfo 网络层信息（提取后的结构化数据）
type NetworkInfo struct {
	// SrcIP 源 IP 地址
	SrcIP string

	// DstIP 目标 IP 地址
	DstIP string

	// SrcPort 源端口
	SrcPort uint16

	// DstPort 目标端口
	DstPort uint16

	// Protocol 协议类型 (TCP/UDP/ICMP 等)
	Protocol string

	// EtherType 以太网类型
	EtherType uint16

	// SrcMAC 源 MAC 地址
	SrcMAC string

	// DstMAC 目标 MAC 地址
	DstMAC string
}

// NewPacket 创建一个新的 Packet 实例
func NewPacket(data []byte, timestamp time.Time, captureLen, originalLen int) *Packet {
	// 复制数据以避免底层 buffer 被重用
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)

	return &Packet{
		Timestamp:     timestamp,
		CaptureLength: captureLen,
		Length:        originalLen,
		Data:          dataCopy,
		Metadata:      &PacketMetadata{},
	}
}

// NewPacketZeroCopy 创建 Packet 实例（零拷贝，调用者负责数据生命周期）
func NewPacketZeroCopy(data []byte, timestamp time.Time, captureLen, originalLen int) *Packet {
	return &Packet{
		Timestamp:     timestamp,
		CaptureLength: captureLen,
		Length:        originalLen,
		Data:          data,
		Metadata:      &PacketMetadata{},
	}
}

// GoPacket 返回 gopacket.Packet 用于详细解析（延迟解析）
func (p *Packet) GoPacket() gopacket.Packet {
	if p.gopacketData == nil {
		p.gopacketData = gopacket.NewPacket(p.Data, layers.LayerTypeEthernet, gopacket.Default)
	}
	return p.gopacketData
}

// NetworkInfo 提取网络层信息
func (p *Packet) NetworkInfo() *NetworkInfo {
	gp := p.GoPacket()
	info := &NetworkInfo{}

	// 提取以太网层
	if ethLayer := gp.Layer(layers.LayerTypeEthernet); ethLayer != nil {
		eth := ethLayer.(*layers.Ethernet)
		info.SrcMAC = eth.SrcMAC.String()
		info.DstMAC = eth.DstMAC.String()
		info.EtherType = uint16(eth.EthernetType)
	}

	// 提取 IPv4 层
	if ipv4Layer := gp.Layer(layers.LayerTypeIPv4); ipv4Layer != nil {
		ipv4 := ipv4Layer.(*layers.IPv4)
		info.SrcIP = ipv4.SrcIP.String()
		info.DstIP = ipv4.DstIP.String()
		info.Protocol = ipv4.Protocol.String()
	}

	// 提取 IPv6 层
	if ipv6Layer := gp.Layer(layers.LayerTypeIPv6); ipv6Layer != nil {
		ipv6 := ipv6Layer.(*layers.IPv6)
		info.SrcIP = ipv6.SrcIP.String()
		info.DstIP = ipv6.DstIP.String()
		info.Protocol = ipv6.NextHeader.String()
	}

	// 提取 TCP 层
	if tcpLayer := gp.Layer(layers.LayerTypeTCP); tcpLayer != nil {
		tcp := tcpLayer.(*layers.TCP)
		info.SrcPort = uint16(tcp.SrcPort)
		info.DstPort = uint16(tcp.DstPort)
		info.Protocol = "TCP"
	}

	// 提取 UDP 层
	if udpLayer := gp.Layer(layers.LayerTypeUDP); udpLayer != nil {
		udp := udpLayer.(*layers.UDP)
		info.SrcPort = uint16(udp.SrcPort)
		info.DstPort = uint16(udp.DstPort)
		info.Protocol = "UDP"
	}

	return info
}

// ApplicationPayload 获取应用层载荷数据
func (p *Packet) ApplicationPayload() []byte {
	gp := p.GoPacket()
	if app := gp.ApplicationLayer(); app != nil {
		return app.Payload()
	}
	return nil
}

// IsTCP 判断是否为 TCP 数据包
func (p *Packet) IsTCP() bool {
	return p.GoPacket().Layer(layers.LayerTypeTCP) != nil
}

// IsUDP 判断是否为 UDP 数据包
func (p *Packet) IsUDP() bool {
	return p.GoPacket().Layer(layers.LayerTypeUDP) != nil
}

// Size 返回数据包大小（捕获长度）
func (p *Packet) Size() int {
	return p.CaptureLength
}

// Clone 深拷贝数据包
func (p *Packet) Clone() *Packet {
	dataCopy := make([]byte, len(p.Data))
	copy(dataCopy, p.Data)

	clone := &Packet{
		Timestamp:     p.Timestamp,
		CaptureLength: p.CaptureLength,
		Length:        p.Length,
		Data:          dataCopy,
	}

	if p.Metadata != nil {
		clone.Metadata = &PacketMetadata{
			InterfaceName:  p.Metadata.InterfaceName,
			InterfaceIndex: p.Metadata.InterfaceIndex,
			Truncated:      p.Metadata.Truncated,
			DeviceID:       p.Metadata.DeviceID,
		}
	}

	return clone
}
