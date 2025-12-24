package monitor

import (
	"fmt"
	"net"
	"sync"
)

// Matcher IP/CIDR 匹配器
type Matcher struct {
	// 单 IP 哈希表
	ipMap map[string]struct{}
	ipMu  sync.RWMutex

	// CIDR 列表（用于匹配）
	cidrList []*net.IPNet
	cidrMu   sync.RWMutex
}

// NewMatcher 创建新的匹配器
func NewMatcher() *Matcher {
	return &Matcher{
		ipMap:   make(map[string]struct{}),
		cidrList: make([]*net.IPNet, 0),
	}
}

// Update 更新匹配规则（从配置加载）
func (m *Matcher) Update(config *Config) error {
	if config == nil {
		return fmt.Errorf("配置不能为空")
	}

	// 如果未启用，清空所有规则
	if !config.Enabled {
		m.ipMu.Lock()
		m.ipMap = make(map[string]struct{})
		m.ipMu.Unlock()

		m.cidrMu.Lock()
		m.cidrList = make([]*net.IPNet, 0)
		m.cidrMu.Unlock()
		return nil
	}

	// 构建新的 IP 集合和 CIDR 列表
	newIPMap := make(map[string]struct{})
	newCIDRList := make([]*net.IPNet, 0, len(config.Targets))

	for _, target := range config.Targets {
		switch target.Type {
		case "ip":
			// 验证 IP 格式
			ip := net.ParseIP(target.Value)
			if ip == nil {
				return fmt.Errorf("无效的 IP 地址: %s", target.Value)
			}
			// 只支持 IPv4
			if ip.To4() == nil {
				return fmt.Errorf("只支持 IPv4，不支持: %s", target.Value)
			}
			newIPMap[target.Value] = struct{}{}

		case "cidr":
			// 解析 CIDR
			_, ipNet, err := net.ParseCIDR(target.Value)
			if err != nil {
				return fmt.Errorf("无效的 CIDR: %s, 错误: %w", target.Value, err)
			}
			// 只支持 IPv4
			if ipNet.IP.To4() == nil {
				return fmt.Errorf("只支持 IPv4 CIDR，不支持: %s", target.Value)
			}
			newCIDRList = append(newCIDRList, ipNet)

		default:
			return fmt.Errorf("不支持的目标类型: %s (只支持 'ip' 或 'cidr')", target.Type)
		}
	}

	// 原子性更新
	m.ipMu.Lock()
	m.ipMap = newIPMap
	m.ipMu.Unlock()

	m.cidrMu.Lock()
	m.cidrList = newCIDRList
	m.cidrMu.Unlock()

	return nil
}

// Match 匹配 IP 是否在管控列表中
// 返回 true 表示匹配（需要监控）
func (m *Matcher) Match(ip net.IP) bool {
	if ip == nil {
		return false
	}

	// 转换为 IPv4
	ipv4 := ip.To4()
	if ipv4 == nil {
		return false // 只支持 IPv4
	}

	ipStr := ipv4.String()

	// 1. 先检查单 IP 哈希表（O(1)）
	m.ipMu.RLock()
	_, exists := m.ipMap[ipStr]
	m.ipMu.RUnlock()
	if exists {
		return true
	}

	// 2. 检查 CIDR 列表（O(n)，但通常 n 很小）
	m.cidrMu.RLock()
	defer m.cidrMu.RUnlock()
	for _, cidr := range m.cidrList {
		if cidr.Contains(ipv4) {
			return true
		}
	}

	return false
}

// Count 返回当前规则数量（用于统计）
func (m *Matcher) Count() (ipCount, cidrCount int) {
	m.ipMu.RLock()
	ipCount = len(m.ipMap)
	m.ipMu.RUnlock()

	m.cidrMu.RLock()
	cidrCount = len(m.cidrList)
	m.cidrMu.RUnlock()

	return ipCount, cidrCount
}

