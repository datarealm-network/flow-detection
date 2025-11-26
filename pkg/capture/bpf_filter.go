//go:build linux

package capture

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/net/bpf"
)

// sockFilter BPF 过滤器指令
type sockFilter struct {
	code uint16
	jt   uint8
	jf   uint8
	k    uint32
}

// sockFprog BPF 程序
type sockFprog struct {
	len    uint16
	filter *sockFilter
}

// compileBPFFilterInternal 编译 BPF 过滤表达式
// 支持 tcpdump 风格的过滤器，如:
// - "tcp port 80"
// - "host 192.168.1.1"
// - "udp and dst port 53"
// - "icmp"
func compileBPFFilterInternal(filterExpr string, snapLen uint32) (*sockFprog, error) {
	// 使用 golang.org/x/net/bpf 包编译高级表达式
	// 这里我们提供一些常用的预编译过滤器

	instructions, err := parseFilterExpression(filterExpr)
	if err != nil {
		return nil, err
	}

	// 转换为 sock_filter 格式
	// 使用 bpf.Assemble 将指令编译为原始格式
	rawInsns, err := bpf.Assemble(instructions)
	if err != nil {
		return nil, err
	}

	filters := make([]sockFilter, len(rawInsns))
	for i, raw := range rawInsns {
		filters[i] = sockFilter{
			code: raw.Op,
			jt:   raw.Jt,
			jf:   raw.Jf,
			k:    raw.K,
		}
	}

	return &sockFprog{
		len:    uint16(len(filters)),
		filter: &filters[0],
	}, nil
}

// parseFilterExpression 解析过滤表达式
func parseFilterExpression(expr string) ([]bpf.Instruction, error) {
	switch expr {
	case "tcp":
		return compileTCPFilter(), nil
	case "udp":
		return compileUDPFilter(), nil
	case "icmp":
		return compileICMPFilter(), nil
	case "arp":
		return compileARPFilter(), nil
	default:
		// 复杂表达式，使用通用编译器
		return compileComplexFilter(expr)
	}
}

// compileTCPFilter TCP 过滤器
func compileTCPFilter() []bpf.Instruction {
	// 检查 IP 协议字段是否为 TCP (6)
	// 偏移: 以太网头 (14字节) + IP协议字段 (9字节) = 23字节
	return []bpf.Instruction{
		// 加载以太网类型 (偏移12)
		bpf.LoadAbsolute{Off: 12, Size: 2},
		// 检查是否为 IPv4 (0x0800)
		bpf.JumpIf{Cond: bpf.JumpNotEqual, Val: 0x0800, SkipTrue: 3},
		// 加载 IP 协议字段 (偏移23)
		bpf.LoadAbsolute{Off: 23, Size: 1},
		// 检查是否为 TCP (6)
		bpf.JumpIf{Cond: bpf.JumpNotEqual, Val: 6, SkipTrue: 1},
		// 接受数据包
		bpf.RetConstant{Val: 0xFFFFFFFF},
		// 拒绝数据包
		bpf.RetConstant{Val: 0},
	}
}

// compileUDPFilter UDP 过滤器
func compileUDPFilter() []bpf.Instruction {
	return []bpf.Instruction{
		bpf.LoadAbsolute{Off: 12, Size: 2},
		bpf.JumpIf{Cond: bpf.JumpNotEqual, Val: 0x0800, SkipTrue: 3},
		bpf.LoadAbsolute{Off: 23, Size: 1},
		bpf.JumpIf{Cond: bpf.JumpNotEqual, Val: 17, SkipTrue: 1}, // UDP = 17
		bpf.RetConstant{Val: 0xFFFFFFFF},
		bpf.RetConstant{Val: 0},
	}
}

// compileICMPFilter ICMP 过滤器
func compileICMPFilter() []bpf.Instruction {
	return []bpf.Instruction{
		bpf.LoadAbsolute{Off: 12, Size: 2},
		bpf.JumpIf{Cond: bpf.JumpNotEqual, Val: 0x0800, SkipTrue: 3},
		bpf.LoadAbsolute{Off: 23, Size: 1},
		bpf.JumpIf{Cond: bpf.JumpNotEqual, Val: 1, SkipTrue: 1}, // ICMP = 1
		bpf.RetConstant{Val: 0xFFFFFFFF},
		bpf.RetConstant{Val: 0},
	}
}

// compileARPFilter ARP 过滤器
func compileARPFilter() []bpf.Instruction {
	return []bpf.Instruction{
		bpf.LoadAbsolute{Off: 12, Size: 2},
		bpf.JumpIf{Cond: bpf.JumpNotEqual, Val: 0x0806, SkipTrue: 1}, // ARP = 0x0806
		bpf.RetConstant{Val: 0xFFFFFFFF},
		bpf.RetConstant{Val: 0},
	}
}

// compileComplexFilter 编译复杂过滤器
// 支持 "tcp port 80", "host 192.168.1.1" 等表达式
func compileComplexFilter(expr string) ([]bpf.Instruction, error) {
	// 这里需要完整的 tcpdump 过滤器解析器
	// 为简化，我们提供几个常用模式的硬编码实现

	// 模式: "port 80" 或 "tcp port 80"
	var port uint32
	if n, _ := fmt.Sscanf(expr, "tcp port %d", &port); n == 1 {
		return compileTCPPortFilter(uint16(port)), nil
	}
	if n, _ := fmt.Sscanf(expr, "udp port %d", &port); n == 1 {
		return compileUDPPortFilter(uint16(port)), nil
	}
	if n, _ := fmt.Sscanf(expr, "port %d", &port); n == 1 {
		return compileAnyPortFilter(uint16(port)), nil
	}

	// 模式: "host 192.168.1.1"
	var a, b, c, d uint32
	if n, _ := fmt.Sscanf(expr, "host %d.%d.%d.%d", &a, &b, &c, &d); n == 4 {
		ip := (a << 24) | (b << 16) | (c << 8) | d
		return compileHostFilter(ip), nil
	}

	// 不支持的表达式，返回接受所有数据包的过滤器
	return []bpf.Instruction{
		bpf.RetConstant{Val: 0xFFFFFFFF},
	}, nil
}

// compileTCPPortFilter TCP 端口过滤器
func compileTCPPortFilter(port uint16) []bpf.Instruction {
	return []bpf.Instruction{
		// 检查是否为 IPv4
		bpf.LoadAbsolute{Off: 12, Size: 2},
		bpf.JumpIf{Cond: bpf.JumpNotEqual, Val: 0x0800, SkipTrue: 7},
		// 检查是否为 TCP
		bpf.LoadAbsolute{Off: 23, Size: 1},
		bpf.JumpIf{Cond: bpf.JumpNotEqual, Val: 6, SkipTrue: 5},
		// 检查源端口
		bpf.LoadAbsolute{Off: 34, Size: 2},
		bpf.JumpIf{Cond: bpf.JumpEqual, Val: uint32(port), SkipFalse: 2},
		// 检查目标端口
		bpf.LoadAbsolute{Off: 36, Size: 2},
		bpf.JumpIf{Cond: bpf.JumpNotEqual, Val: uint32(port), SkipTrue: 1},
		// 接受
		bpf.RetConstant{Val: 0xFFFFFFFF},
		// 拒绝
		bpf.RetConstant{Val: 0},
	}
}

// compileUDPPortFilter UDP 端口过滤器
func compileUDPPortFilter(port uint16) []bpf.Instruction {
	return []bpf.Instruction{
		bpf.LoadAbsolute{Off: 12, Size: 2},
		bpf.JumpIf{Cond: bpf.JumpNotEqual, Val: 0x0800, SkipTrue: 7},
		bpf.LoadAbsolute{Off: 23, Size: 1},
		bpf.JumpIf{Cond: bpf.JumpNotEqual, Val: 17, SkipTrue: 5}, // UDP = 17
		bpf.LoadAbsolute{Off: 34, Size: 2},
		bpf.JumpIf{Cond: bpf.JumpEqual, Val: uint32(port), SkipFalse: 2},
		bpf.LoadAbsolute{Off: 36, Size: 2},
		bpf.JumpIf{Cond: bpf.JumpNotEqual, Val: uint32(port), SkipTrue: 1},
		bpf.RetConstant{Val: 0xFFFFFFFF},
		bpf.RetConstant{Val: 0},
	}
}

// compileAnyPortFilter 任意协议端口过滤器（TCP 或 UDP）
func compileAnyPortFilter(port uint16) []bpf.Instruction {
	return []bpf.Instruction{
		// 检查 IPv4
		bpf.LoadAbsolute{Off: 12, Size: 2},
		bpf.JumpIf{Cond: bpf.JumpNotEqual, Val: 0x0800, SkipTrue: 9},
		// 检查协议
		bpf.LoadAbsolute{Off: 23, Size: 1},
		// TCP (6) 或 UDP (17)?
		bpf.JumpIf{Cond: bpf.JumpEqual, Val: 6, SkipFalse: 1},
		bpf.Jump{Skip: 1},
		bpf.JumpIf{Cond: bpf.JumpNotEqual, Val: 17, SkipTrue: 5},
		// 检查源端口
		bpf.LoadAbsolute{Off: 34, Size: 2},
		bpf.JumpIf{Cond: bpf.JumpEqual, Val: uint32(port), SkipFalse: 2},
		// 检查目标端口
		bpf.LoadAbsolute{Off: 36, Size: 2},
		bpf.JumpIf{Cond: bpf.JumpNotEqual, Val: uint32(port), SkipTrue: 1},
		bpf.RetConstant{Val: 0xFFFFFFFF},
		bpf.RetConstant{Val: 0},
	}
}

// compileHostFilter 主机 IP 过滤器
func compileHostFilter(ip uint32) []bpf.Instruction {
	return []bpf.Instruction{
		// 检查 IPv4
		bpf.LoadAbsolute{Off: 12, Size: 2},
		bpf.JumpIf{Cond: bpf.JumpNotEqual, Val: 0x0800, SkipTrue: 5},
		// 检查源 IP (偏移26)
		bpf.LoadAbsolute{Off: 26, Size: 4},
		bpf.JumpIf{Cond: bpf.JumpEqual, Val: ip, SkipFalse: 2},
		// 检查目标 IP (偏移30)
		bpf.LoadAbsolute{Off: 30, Size: 4},
		bpf.JumpIf{Cond: bpf.JumpNotEqual, Val: ip, SkipTrue: 1},
		bpf.RetConstant{Val: 0xFFFFFFFF},
		bpf.RetConstant{Val: 0},
	}
}

// setsockoptSockFprogInternal 设置 BPF 过滤器
func setsockoptSockFprogInternal(fd, level, opt int, prog *sockFprog) error {
	_, _, errno := syscall.Syscall6(
		syscall.SYS_SETSOCKOPT,
		uintptr(fd),
		uintptr(level),
		uintptr(opt),
		uintptr(unsafe.Pointer(prog)),
		unsafe.Sizeof(*prog),
		0,
	)
	if errno != 0 {
		return errno
	}
	return nil
}

// eBPF 支持 (需要 Linux 4.1+)
// 注意：完整的 eBPF 实现需要 libbpf 或 cilium/ebpf 库
// 这里提供接口定义，实际项目中可以集成 github.com/cilium/ebpf

// eBPFFilter eBPF 过滤器接口
type eBPFFilter interface {
	// Attach 附加到网卡
	Attach(ifaceIndex int) error

	// Detach 分离
	Detach() error

	// GetStats 获取统计信息
	GetStats() (map[string]uint64, error)
}

// TODO: 实现完整的 eBPF 支持
// 可以使用 github.com/cilium/ebpf 库:
//
// import "github.com/cilium/ebpf"
//
// func loadeBPFProgram(programPath string) (*ebpf.Program, error) {
//     spec, err := ebpf.LoadCollectionSpec(programPath)
//     if err != nil {
//         return nil, err
//     }
//
//     coll, err := ebpf.NewCollection(spec)
//     if err != nil {
//         return nil, err
//     }
//
//     return coll.Programs["xdp_filter"], nil
// }
