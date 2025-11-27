package pipeline

import "time"

// Config 管道配置
type Config struct {
	// BufferSize 缓冲区大小（数据包数量）
	BufferSize int

	// BatchSize 批量处理大小
	// 设置为 1 表示禁用批量处理
	BatchSize int

	// BatchTimeout 批量超时时间
	// 超过此时间未满一批也会强制发送
	BatchTimeout time.Duration

	// Workers 工作协程数量
	// 用于并行处理
	Workers int

	// EnableMetrics 是否启用指标收集
	EnableMetrics bool

	// DropPolicy 丢弃策略
	DropPolicy DropPolicy

	// Middlewares 中间件列表
	Middlewares []Middleware
}

// DropPolicy 丢弃策略
type DropPolicy int

const (
	// DropPolicyBlock 阻塞等待（不丢弃）
	DropPolicyBlock DropPolicy = iota

	// DropPolicyNewest 丢弃最新的数据包
	DropPolicyNewest

	// DropPolicyOldest 丢弃最旧的数据包
	DropPolicyOldest
)

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		BufferSize:    10000,
		BatchSize:     1, // 默认不启用批量
		BatchTimeout:  10 * time.Millisecond,
		Workers:       1,
		EnableMetrics: true,
		DropPolicy:    DropPolicyNewest,
		Middlewares:   nil,
	}
}

// HighThroughputConfig 返回高吞吐量配置
func HighThroughputConfig() *Config {
	return &Config{
		BufferSize:    100000,
		BatchSize:     100,
		BatchTimeout:  5 * time.Millisecond,
		Workers:       4,
		EnableMetrics: true,
		DropPolicy:    DropPolicyNewest,
		Middlewares:   nil,
	}
}

// LowLatencyConfig 返回低延迟配置
func LowLatencyConfig() *Config {
	return &Config{
		BufferSize:    1000,
		BatchSize:     1, // 禁用批量以降低延迟
		BatchTimeout:  0,
		Workers:       1,
		EnableMetrics: false,
		DropPolicy:    DropPolicyNewest,
		Middlewares:   nil,
	}
}

// WithBufferSize 设置缓冲区大小
func (c *Config) WithBufferSize(size int) *Config {
	c.BufferSize = size
	return c
}

// WithBatchSize 设置批量大小
func (c *Config) WithBatchSize(size int) *Config {
	c.BatchSize = size
	return c
}

// WithBatchTimeout 设置批量超时
func (c *Config) WithBatchTimeout(timeout time.Duration) *Config {
	c.BatchTimeout = timeout
	return c
}

// WithWorkers 设置工作协程数
func (c *Config) WithWorkers(workers int) *Config {
	c.Workers = workers
	return c
}

// WithMiddleware 添加中间件
func (c *Config) WithMiddleware(m Middleware) *Config {
	c.Middlewares = append(c.Middlewares, m)
	return c
}

// Validate 验证配置
func (c *Config) Validate() error {
	if c.BufferSize <= 0 {
		c.BufferSize = 10000
	}
	if c.BatchSize <= 0 {
		c.BatchSize = 1
	}
	if c.Workers <= 0 {
		c.Workers = 1
	}
	return nil
}
