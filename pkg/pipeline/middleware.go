package pipeline

import (
	"net"

	"datarealm.cn/network/pkg/packet"
)

// FilterMiddleware 过滤中间件
type FilterMiddleware struct {
	name   string
	filter FilterFunc
}

// NewFilterMiddleware 创建过滤中间件
func NewFilterMiddleware(name string, filter FilterFunc) *FilterMiddleware {
	return &FilterMiddleware{
		name:   name,
		filter: filter,
	}
}

// Process 实现 Middleware 接口
func (m *FilterMiddleware) Process(pkt *packet.Packet) *packet.Packet {
	if m.filter(pkt) {
		return pkt
	}
	return nil
}

// Name 返回中间件名称
func (m *FilterMiddleware) Name() string {
	return m.name
}

// TransformMiddleware 转换中间件
type TransformMiddleware struct {
	name      string
	transform TransformFunc
}

// NewTransformMiddleware 创建转换中间件
func NewTransformMiddleware(name string, transform TransformFunc) *TransformMiddleware {
	return &TransformMiddleware{
		name:      name,
		transform: transform,
	}
}

// Process 实现 Middleware 接口
func (m *TransformMiddleware) Process(pkt *packet.Packet) *packet.Packet {
	return m.transform(pkt)
}

// Name 返回中间件名称
func (m *TransformMiddleware) Name() string {
	return m.name
}

// IPRangeFilter IP 范围过滤中间件
type IPRangeFilter struct {
	name     string
	networks []*net.IPNet
	mode     IPFilterMode
}

// IPFilterMode IP 过滤模式
type IPFilterMode int

const (
	// IPFilterModeInclude 包含模式（只保留匹配的）
	IPFilterModeInclude IPFilterMode = iota
	// IPFilterModeExclude 排除模式（丢弃匹配的）
	IPFilterModeExclude
)

// NewIPRangeFilter 创建 IP 范围过滤器
func NewIPRangeFilter(name string, cidrs []string, mode IPFilterMode) (*IPRangeFilter, error) {
	networks := make([]*net.IPNet, 0, len(cidrs))
	for _, cidr := range cidrs {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			// 尝试解析为单个 IP
			ip := net.ParseIP(cidr)
			if ip == nil {
				return nil, err
			}
			// 转换为 /32 或 /128 的 CIDR
			if ip.To4() != nil {
				_, network, _ = net.ParseCIDR(cidr + "/32")
			} else {
				_, network, _ = net.ParseCIDR(cidr + "/128")
			}
		}
		networks = append(networks, network)
	}

	return &IPRangeFilter{
		name:     name,
		networks: networks,
		mode:     mode,
	}, nil
}

// Process 实现 Middleware 接口
func (f *IPRangeFilter) Process(pkt *packet.Packet) *packet.Packet {
	info := pkt.NetworkInfo()
	srcIP := net.ParseIP(info.SrcIP)
	dstIP := net.ParseIP(info.DstIP)

	matched := false
	for _, network := range f.networks {
		if (srcIP != nil && network.Contains(srcIP)) ||
			(dstIP != nil && network.Contains(dstIP)) {
			matched = true
			break
		}
	}

	switch f.mode {
	case IPFilterModeInclude:
		if matched {
			return pkt
		}
		return nil
	case IPFilterModeExclude:
		if matched {
			return nil
		}
		return pkt
	}

	return pkt
}

// Name 返回中间件名称
func (f *IPRangeFilter) Name() string {
	return f.name
}

// PortFilter 端口过滤中间件
type PortFilter struct {
	name  string
	ports map[uint16]bool
	mode  IPFilterMode
}

// NewPortFilter 创建端口过滤器
func NewPortFilter(name string, ports []uint16, mode IPFilterMode) *PortFilter {
	portMap := make(map[uint16]bool, len(ports))
	for _, port := range ports {
		portMap[port] = true
	}
	return &PortFilter{
		name:  name,
		ports: portMap,
		mode:  mode,
	}
}

// Process 实现 Middleware 接口
func (f *PortFilter) Process(pkt *packet.Packet) *packet.Packet {
	info := pkt.NetworkInfo()
	matched := f.ports[info.SrcPort] || f.ports[info.DstPort]

	switch f.mode {
	case IPFilterModeInclude:
		if matched {
			return pkt
		}
		return nil
	case IPFilterModeExclude:
		if matched {
			return nil
		}
		return pkt
	}

	return pkt
}

// Name 返回中间件名称
func (f *PortFilter) Name() string {
	return f.name
}

// ProtocolFilter 协议过滤中间件
type ProtocolFilter struct {
	name      string
	protocols map[string]bool
	mode      IPFilterMode
}

// NewProtocolFilter 创建协议过滤器
func NewProtocolFilter(name string, protocols []string, mode IPFilterMode) *ProtocolFilter {
	protoMap := make(map[string]bool, len(protocols))
	for _, proto := range protocols {
		protoMap[proto] = true
	}
	return &ProtocolFilter{
		name:      name,
		protocols: protoMap,
		mode:      mode,
	}
}

// Process 实现 Middleware 接口
func (f *ProtocolFilter) Process(pkt *packet.Packet) *packet.Packet {
	info := pkt.NetworkInfo()
	matched := f.protocols[info.Protocol]

	switch f.mode {
	case IPFilterModeInclude:
		if matched {
			return pkt
		}
		return nil
	case IPFilterModeExclude:
		if matched {
			return nil
		}
		return pkt
	}

	return pkt
}

// Name 返回中间件名称
func (f *ProtocolFilter) Name() string {
	return f.name
}

// SamplingMiddleware 采样中间件
type SamplingMiddleware struct {
	name    string
	rate    int // 采样率（每 N 个包取 1 个）
	counter uint64
}

// NewSamplingMiddleware 创建采样中间件
func NewSamplingMiddleware(name string, rate int) *SamplingMiddleware {
	if rate <= 0 {
		rate = 1
	}
	return &SamplingMiddleware{
		name: name,
		rate: rate,
	}
}

// Process 实现 Middleware 接口
func (m *SamplingMiddleware) Process(pkt *packet.Packet) *packet.Packet {
	m.counter++
	if m.counter%uint64(m.rate) == 0 {
		return pkt
	}
	return nil
}

// Name 返回中间件名称
func (m *SamplingMiddleware) Name() string {
	return m.name
}

// ChainMiddleware 中间件链
type ChainMiddleware struct {
	name        string
	middlewares []Middleware
}

// NewChainMiddleware 创建中间件链
func NewChainMiddleware(name string, middlewares ...Middleware) *ChainMiddleware {
	return &ChainMiddleware{
		name:        name,
		middlewares: middlewares,
	}
}

// Process 实现 Middleware 接口
func (c *ChainMiddleware) Process(pkt *packet.Packet) *packet.Packet {
	for _, m := range c.middlewares {
		pkt = m.Process(pkt)
		if pkt == nil {
			return nil
		}
	}
	return pkt
}

// Name 返回中间件名称
func (c *ChainMiddleware) Name() string {
	return c.name
}

// Add 添加中间件到链
func (c *ChainMiddleware) Add(m Middleware) {
	c.middlewares = append(c.middlewares, m)
}
