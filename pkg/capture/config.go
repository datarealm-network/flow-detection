package capture

import "time"

// Config 捕获器配置
type Config struct {
	// InterfaceName 网络接口名称（如 eth0, en0）
	InterfaceName string

	// SnapLen 每个数据包捕获的最大字节数
	// 默认值: 65535 (完整数据包)
	// 性能优化时可设置为 128 或 256（仅捕获协议头）
	SnapLen int32

	// Promiscuous 是否开启混杂模式
	// 混杂模式下可以捕获经过网卡的所有数据包
	Promiscuous bool

	// Timeout 读取超时时间
	// 设置为 0 表示阻塞读取
	Timeout time.Duration

	// BPFFilter BPF 过滤器表达式
	// 例如: "tcp port 80", "host 192.168.1.1"
	BPFFilter string

	// BufferSize 内核缓冲区大小（字节）
	// 增大此值可以减少高流量下的丢包
	BufferSize int

	// ChannelSize 输出通道缓冲大小
	// 用于控制捕获goroutine和消费者之间的缓冲
	ChannelSize int

	// ImmediateMode 是否启用即时模式
	// 启用后数据包会立即返回，不等待缓冲区填满
	ImmediateMode bool

	// EnableZeroCopy 是否启用零拷贝模式
	// 启用后不会复制数据包数据，调用者需要注意数据生命周期
	EnableZeroCopy bool

	// DeviceID 设备标识（多设备场景）
	DeviceID int
}

// DefaultConfig 返回默认配置
func DefaultConfig(interfaceName string) *Config {
	return &Config{
		InterfaceName:  interfaceName,
		SnapLen:        65535,           // 完整数据包
		Promiscuous:    true,            // 开启混杂模式
		Timeout:        0,               // 阻塞读取
		BPFFilter:      "",              // 无过滤
		BufferSize:     4 * 1024 * 1024, // 4MB 内核缓冲
		ChannelSize:    10000,           // 10K 数据包缓冲
		ImmediateMode:  true,            // 即时模式
		EnableZeroCopy: false,           // 默认关闭零拷贝
		DeviceID:       0,
	}
}

// HighPerformanceConfig 返回高性能配置
// 适用于高流量场景
func HighPerformanceConfig(interfaceName string) *Config {
	return &Config{
		InterfaceName:  interfaceName,
		SnapLen:        256, // 仅捕获协议头
		Promiscuous:    true,
		Timeout:        0,
		BPFFilter:      "",
		BufferSize:     64 * 1024 * 1024, // 64MB 内核缓冲
		ChannelSize:    100000,           // 100K 数据包缓冲
		ImmediateMode:  true,
		EnableZeroCopy: false,
		DeviceID:       0,
	}
}

// Validate 验证配置有效性
func (c *Config) Validate() error {
	if c.InterfaceName == "" {
		return ErrInvalidInterface
	}
	if c.SnapLen <= 0 {
		c.SnapLen = 65535
	}
	if c.ChannelSize <= 0 {
		c.ChannelSize = 10000
	}
	if c.BufferSize <= 0 {
		c.BufferSize = 4 * 1024 * 1024
	}
	return nil
}
