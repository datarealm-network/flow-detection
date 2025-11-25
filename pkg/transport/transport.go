// Package transport 提供跨进程通信的统一接口
// 支持共享内存（零拷贝）、gRPC、RESTful三种传输方式
package transport

import (
	"context"
	"time"
)

// Transport 传输层接口
// 定义了进程间通信的核心方法，支持多种底层实现
type Transport interface {
	// Start 启动传输层
	// ctx: 上下文控制
	// 返回: error
	Start(ctx context.Context) error

	// Stop 停止传输层
	// 优雅关闭，释放所有资源
	// 返回: error
	Stop() error

	// Send 发送消息
	// ctx: 上下文控制
	// msg: 要发送的消息
	// 返回: error
	Send(ctx context.Context, msg *Message) error

	// SendAsync 异步发送消息（不等待响应）
	// ctx: 上下文控制
	// msg: 要发送的消息
	// 返回: error
	SendAsync(ctx context.Context, msg *Message) error

	// Receive 接收消息
	// ctx: 上下文控制
	// timeout: 超时时间，0表示阻塞等待
	// 返回: 接收到的消息和error
	// 注意: MPMC模式下，消息需要手动确认（调用Acknowledge）
	Receive(ctx context.Context, timeout time.Duration) (*Message, error)

	// Request 请求-响应模式（同步）
	// ctx: 上下文控制
	// req: 请求消息
	// timeout: 超时时间
	// 返回: 响应消息和error
	Request(ctx context.Context, req *Message, timeout time.Duration) (*Message, error)

	// Subscribe 订阅主题/频道
	// ctx: 上下文控制
	// topic: 主题名称
	// handler: 消息处理函数
	// 返回: error
	Subscribe(ctx context.Context, topic string, handler MessageHandler) error

	// Unsubscribe 取消订阅
	// topic: 主题名称
	// 返回: error
	Unsubscribe(topic string) error

	// Publish 发布消息到主题
	// ctx: 上下文控制
	// topic: 主题名称
	// msg: 消息
	// 返回: error
	Publish(ctx context.Context, topic string, msg *Message) error

	// IsRunning 检查是否正在运行
	// 返回: bool
	IsRunning() bool

	// GetStats 获取传输统计信息
	// 返回: 统计信息
	GetStats() *TransportStats

	// GetType 获取传输类型
	// 返回: 传输类型（SharedMemory/gRPC/RESTful）
	GetType() TransportType

	// SetCodec 设置编解码器
	// codec: 编解码器实例
	SetCodec(codec Codec)

	// Close 关闭传输层（实现io.Closer接口）
	Close() error
}

// TransportEx 扩展传输接口（可选，用于MPMC等高级特性）
// 实现此接口的传输层可提供额外的功能
type TransportEx interface {
	Transport // 继承基础接口

	// Acknowledge 确认消息已处理（MPMC竞争消费模式）
	// msg: 要确认的消息
	// 返回: error
	// 注意: 只有在MPMC模式下需要手动确认，SPSC模式自动确认
	Acknowledge(msg *Message) error

	// Nack 否认消息（处理失败，重新入队）
	// msg: 要否认的消息
	// requeue: 是否重新入队
	// 返回: error
	Nack(msg *Message, requeue bool) error

	// SendToPartition 发送到指定分区
	// ctx: 上下文控制
	// msg: 消息
	// partition: 分区ID
	// 返回: error
	SendToPartition(ctx context.Context, msg *Message, partition int) error

	// ReceiveFromPartition 从指定分区接收
	// ctx: 上下文控制
	// partition: 分区ID
	// timeout: 超时时间
	// 返回: 消息和error
	ReceiveFromPartition(ctx context.Context, partition int, timeout time.Duration) (*Message, error)

	// GetPartitionCount 获取分区数量
	// 返回: 分区数量
	GetPartitionCount() int

	// GetConsumerID 获取消费者ID
	// 返回: 消费者ID，如果是生产者返回-1
	GetConsumerID() int

	// GetConsumerGroup 获取消费者组名称
	// 返回: 消费者组名称
	GetConsumerGroup() string

	// GetPendingMessageCount 获取待处理消息数量
	// 返回: 待处理消息数量
	GetPendingMessageCount() uint64

	// GetProcessingMessageCount 获取正在处理的消息数量
	// 返回: 正在处理的消息数量
	GetProcessingMessageCount() uint64
}

// Server 服务端接口
// 用于接收和处理来自客户端的请求
type Server interface {
	// Start 启动服务端
	// ctx: 上下文控制
	// addr: 监听地址
	// 返回: error
	Start(ctx context.Context, addr string) error

	// Stop 停止服务端
	// 返回: error
	Stop() error

	// RegisterHandler 注册消息处理器
	// msgType: 消息类型
	// handler: 处理函数
	// 返回: error
	RegisterHandler(msgType string, handler MessageHandler) error

	// UnregisterHandler 注销消息处理器
	// msgType: 消息类型
	// 返回: error
	UnregisterHandler(msgType string) error

	// IsRunning 检查是否正在运行
	// 返回: bool
	IsRunning() bool

	// GetAddr 获取监听地址
	// 返回: 地址字符串
	GetAddr() string

	// GetStats 获取服务端统计信息
	// 返回: 统计信息
	GetStats() *ServerStats
}

// Client 客户端接口
// 用于向服务端发送请求
type Client interface {
	// Connect 连接到服务端
	// ctx: 上下文控制
	// addr: 服务端地址
	// 返回: error
	Connect(ctx context.Context, addr string) error

	// Disconnect 断开连接
	// 返回: error
	Disconnect() error

	// Send 发送消息
	// ctx: 上下文控制
	// msg: 消息
	// 返回: error
	Send(ctx context.Context, msg *Message) error

	// Call 调用远程方法（同步）
	// ctx: 上下文控制
	// method: 方法名
	// req: 请求消息
	// resp: 响应消息（输出参数）
	// 返回: error
	Call(ctx context.Context, method string, req *Message, resp *Message) error

	// CallAsync 调用远程方法（异步）
	// ctx: 上下文控制
	// method: 方法名
	// req: 请求消息
	// callback: 回调函数
	// 返回: error
	CallAsync(ctx context.Context, method string, req *Message, callback func(*Message, error)) error

	// IsConnected 检查是否已连接
	// 返回: bool
	IsConnected() bool

	// GetAddr 获取服务端地址
	// 返回: 地址字符串
	GetAddr() string

	// Close 关闭客户端连接
	Close() error
}

// MessageHandler 消息处理函数类型
// 用于处理接收到的消息
type MessageHandler func(ctx context.Context, msg *Message) (*Message, error)

// StreamHandler 流式处理函数类型
// 用于处理流式数据传输
type StreamHandler func(ctx context.Context, stream Stream) error

// Stream 流式传输接口
// 支持双向流式数据传输
type Stream interface {
	// Send 发送消息到流
	// msg: 消息
	// 返回: error
	Send(msg *Message) error

	// Recv 从流接收消息
	// 返回: 消息和error，io.EOF表示流结束
	Recv() (*Message, error)

	// Close 关闭流
	// 返回: error
	Close() error

	// Context 获取流的上下文
	// 返回: context.Context
	Context() context.Context
}

// Publisher 发布者接口
// 用于发布-订阅模式
type Publisher interface {
	// Publish 发布消息
	// topic: 主题
	// msg: 消息
	// 返回: error
	Publish(topic string, msg *Message) error

	// Close 关闭发布者
	Close() error
}

// Subscriber 订阅者接口
// 用于发布-订阅模式
type Subscriber interface {
	// Subscribe 订阅主题
	// topic: 主题
	// handler: 消息处理函数
	// 返回: error
	Subscribe(topic string, handler MessageHandler) error

	// Unsubscribe 取消订阅
	// topic: 主题
	// 返回: error
	Unsubscribe(topic string) error

	// Close 关闭订阅者
	Close() error
}

// TransportType 传输类型
type TransportType string

const (
	// TransportTypeSharedMemory 共享内存传输（零拷贝）
	TransportTypeSharedMemory TransportType = "sharedmemory"

	// TransportTypeGRPC gRPC传输
	TransportTypeGRPC TransportType = "grpc"

	// TransportTypeREST RESTful传输
	TransportTypeREST TransportType = "rest"

	// TransportTypeUnix Unix Domain Socket传输
	TransportTypeUnix TransportType = "unix"
)

// ConcurrencyMode 并发模式
type ConcurrencyMode string

const (
	// ConcurrencyModeSPSC 单生产者单消费者（默认，最高性能）
	ConcurrencyModeSPSC ConcurrencyMode = "spsc"

	// ConcurrencyModeSPMC 单生产者多消费者
	ConcurrencyModeSPMC ConcurrencyMode = "spmc"

	// ConcurrencyModeMPSC 多生产者单消费者
	ConcurrencyModeMPSC ConcurrencyMode = "mpsc"

	// ConcurrencyModeMPMC 多生产者多消费者
	ConcurrencyModeMPMC ConcurrencyMode = "mpmc"
)

// TransportRole 传输角色
type TransportRole string

const (
	// RoleProducer 生产者角色
	RoleProducer TransportRole = "producer"

	// RoleConsumer 消费者角色
	RoleConsumer TransportRole = "consumer"

	// RoleBoth 同时作为生产者和消费者
	RoleBoth TransportRole = "both"
)

// PartitionStrategy 分区策略
type PartitionStrategy string

const (
	// PartitionStrategyHash 基于消息哈希分区（保证相同key的消息到同一分区）
	PartitionStrategyHash PartitionStrategy = "hash"

	// PartitionStrategyRoundRobin 轮询分区（均匀分布）
	PartitionStrategyRoundRobin PartitionStrategy = "round_robin"

	// PartitionStrategyRandom 随机分区
	PartitionStrategyRandom PartitionStrategy = "random"

	// PartitionStrategySticky 粘性分区（同一生产者总是使用同一分区）
	PartitionStrategySticky PartitionStrategy = "sticky"

	// PartitionStrategyManual 手动指定分区（通过SendToPartition）
	PartitionStrategyManual PartitionStrategy = "manual"
)

// LoadBalancePolicy 负载均衡策略
type LoadBalancePolicy string

const (
	// LoadBalancePolicyRoundRobin 轮询（按顺序从各分区读取）
	LoadBalancePolicyRoundRobin LoadBalancePolicy = "round_robin"

	// LoadBalancePolicyRandom 随机选择分区
	LoadBalancePolicyRandom LoadBalancePolicy = "random"

	// LoadBalancePolicyLeastLoad 选择负载最低的分区
	LoadBalancePolicyLeastLoad LoadBalancePolicy = "least_load"

	// LoadBalancePolicyFanout 扇出（从所有分区读取，广播模式）
	LoadBalancePolicyFanout LoadBalancePolicy = "fanout"

	// LoadBalancePolicySticky 粘性（消费者绑定固定分区）
	LoadBalancePolicySticky LoadBalancePolicy = "sticky"
)

// TransportConfig 传输配置
type TransportConfig struct {
	// Type 传输类型
	Type TransportType `json:"type" yaml:"type"`

	// Address 地址（gRPC/REST使用）
	Address string `json:"address" yaml:"address"`

	// ShmPath 共享内存路径（SharedMemory使用）
	ShmPath string `json:"shm_path" yaml:"shm_path"`

	// ShmSize 共享内存大小（字节）
	ShmSize int64 `json:"shm_size" yaml:"shm_size"`

	// BufferSize 缓冲区大小
	BufferSize int `json:"buffer_size" yaml:"buffer_size"`

	// Timeout 默认超时时间
	Timeout time.Duration `json:"timeout" yaml:"timeout"`

	// MaxMessageSize 最大消息大小（字节）
	MaxMessageSize int64 `json:"max_message_size" yaml:"max_message_size"`

	// EnableCompression 是否启用压缩
	EnableCompression bool `json:"enable_compression" yaml:"enable_compression"`

	// EnableEncryption 是否启用加密
	EnableEncryption bool `json:"enable_encryption" yaml:"enable_encryption"`

	// Codec 编解码器类型
	CodecType CodecType `json:"codec_type" yaml:"codec_type"`

	// TLS TLS配置（gRPC/REST使用）
	TLSConfig *TLSConfig `json:"tls_config,omitempty" yaml:"tls_config,omitempty"`

	// PoolSize 连接池大小
	PoolSize int `json:"pool_size" yaml:"pool_size"`

	// KeepAlive 保活配置
	KeepAlive *KeepAliveConfig `json:"keep_alive,omitempty" yaml:"keep_alive,omitempty"`

	// ===== MPMC相关配置 =====

	// ConcurrencyMode 并发模式
	ConcurrencyMode ConcurrencyMode `json:"concurrency_mode" yaml:"concurrency_mode"`

	// Role 角色（生产者/消费者）
	Role TransportRole `json:"role" yaml:"role"`

	// ProducerID 生产者ID（多生产者场景）
	ProducerID int `json:"producer_id" yaml:"producer_id"`

	// ConsumerID 消费者ID（多消费者场景，-1表示自动分配）
	ConsumerID int `json:"consumer_id" yaml:"consumer_id"`

	// ConsumerGroup 消费者组名称（用于负载均衡）
	ConsumerGroup string `json:"consumer_group" yaml:"consumer_group"`

	// PartitionCount 分区数量（MPMC模式使用）
	PartitionCount int `json:"partition_count" yaml:"partition_count"`

	// PartitionStrategy 分区策略
	PartitionStrategy PartitionStrategy `json:"partition_strategy" yaml:"partition_strategy"`

	// LoadBalancePolicy 负载均衡策略
	LoadBalancePolicy LoadBalancePolicy `json:"load_balance_policy" yaml:"load_balance_policy"`

	// AutoAcknowledge 是否自动确认消息（false需要手动调用Acknowledge）
	AutoAcknowledge bool `json:"auto_acknowledge" yaml:"auto_acknowledge"`

	// AckTimeout 消息确认超时时间（超时未确认则重新入队）
	AckTimeout time.Duration `json:"ack_timeout" yaml:"ack_timeout"`

	// MaxRetries 最大重试次数（消息处理失败后重试）
	MaxRetries int `json:"max_retries" yaml:"max_retries"`

	// HeartbeatInterval 心跳间隔（消费者向系统报告存活）
	HeartbeatInterval time.Duration `json:"heartbeat_interval" yaml:"heartbeat_interval"`

	// ConsumerTimeout 消费者超时时间（超时未心跳则认为死亡）
	ConsumerTimeout time.Duration `json:"consumer_timeout" yaml:"consumer_timeout"`
}

// TLSConfig TLS配置
type TLSConfig struct {
	// Enabled 是否启用TLS
	Enabled bool `json:"enabled" yaml:"enabled"`

	// CertFile 证书文件路径
	CertFile string `json:"cert_file" yaml:"cert_file"`

	// KeyFile 密钥文件路径
	KeyFile string `json:"key_file" yaml:"key_file"`

	// CAFile CA证书文件路径
	CAFile string `json:"ca_file" yaml:"ca_file"`

	// ServerName 服务器名称
	ServerName string `json:"server_name" yaml:"server_name"`

	// InsecureSkipVerify 是否跳过证书验证
	InsecureSkipVerify bool `json:"insecure_skip_verify" yaml:"insecure_skip_verify"`
}

// KeepAliveConfig 保活配置
type KeepAliveConfig struct {
	// Enabled 是否启用保活
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Time 保活时间间隔
	Time time.Duration `json:"time" yaml:"time"`

	// Timeout 保活超时时间
	Timeout time.Duration `json:"timeout" yaml:"timeout"`
}

// DefaultTransportConfig 返回默认配置
func DefaultTransportConfig(transportType TransportType) *TransportConfig {
	config := &TransportConfig{
		Type:              transportType,
		BufferSize:        10000,
		Timeout:           30 * time.Second,
		MaxMessageSize:    64 * 1024 * 1024, // 64MB
		EnableCompression: false,
		EnableEncryption:  false,
		CodecType:         CodecTypeJSON,
		PoolSize:          10,

		// MPMC默认配置
		ConcurrencyMode:   ConcurrencyModeSPSC,         // 默认SPSC模式
		Role:              RoleBoth,                    // 默认双向
		ProducerID:        0,                           // 默认生产者ID
		ConsumerID:        -1,                          // 自动分配
		ConsumerGroup:     "default",                   // 默认消费者组
		PartitionCount:    1,                           // 默认1个分区（SPSC）
		PartitionStrategy: PartitionStrategyRoundRobin, // 默认轮询
		LoadBalancePolicy: LoadBalancePolicyRoundRobin, // 默认轮询
		AutoAcknowledge:   true,                        // 默认自动确认
		AckTimeout:        30 * time.Second,            // 30秒确认超时
		MaxRetries:        3,                           // 最多重试3次
		HeartbeatInterval: 5 * time.Second,             // 5秒心跳
		ConsumerTimeout:   15 * time.Second,            // 15秒超时
	}

	switch transportType {
	case TransportTypeSharedMemory:
		config.ShmPath = "/dev/shm/network-traffic"
		config.ShmSize = 256 * 1024 * 1024 // 256MB
		config.CodecType = CodecTypeBinary // 共享内存默认使用Binary编码（零拷贝）

	case TransportTypeGRPC:
		config.Address = "localhost:50051"
		config.CodecType = CodecTypeProtobuf // gRPC默认使用Protobuf
		config.KeepAlive = &KeepAliveConfig{
			Enabled: true,
			Time:    30 * time.Second,
			Timeout: 10 * time.Second,
		}

	case TransportTypeREST:
		config.Address = "http://localhost:8080"
		config.CodecType = CodecTypeJSON // REST默认使用JSON
		config.KeepAlive = &KeepAliveConfig{
			Enabled: true,
			Time:    60 * time.Second,
			Timeout: 10 * time.Second,
		}

	case TransportTypeUnix:
		config.Address = "/tmp/network-traffic.sock"
		config.CodecType = CodecTypeBinary
	}

	return config
}

// TransportStats 传输统计信息
type TransportStats struct {
	// MessagesSent 发送的消息数
	MessagesSent uint64 `json:"messages_sent"`

	// MessagesReceived 接收的消息数
	MessagesReceived uint64 `json:"messages_received"`

	// BytesSent 发送的字节数
	BytesSent uint64 `json:"bytes_sent"`

	// BytesReceived 接收的字节数
	BytesReceived uint64 `json:"bytes_received"`

	// ErrorCount 错误数
	ErrorCount uint64 `json:"error_count"`

	// AverageLatency 平均延迟（纳秒）
	AverageLatency int64 `json:"average_latency"`

	// ActiveConnections 活动连接数
	ActiveConnections int32 `json:"active_connections"`

	// StartTime 启动时间
	StartTime time.Time `json:"start_time"`

	// LastMessageTime 最后一条消息时间
	LastMessageTime time.Time `json:"last_message_time"`
}

// ServerStats 服务端统计信息
type ServerStats struct {
	// RequestsReceived 接收的请求数
	RequestsReceived uint64 `json:"requests_received"`

	// ResponsesSent 发送的响应数
	ResponsesSent uint64 `json:"responses_sent"`

	// ActiveClients 活动客户端数
	ActiveClients int32 `json:"active_clients"`

	// ErrorCount 错误数
	ErrorCount uint64 `json:"error_count"`

	// AverageProcessingTime 平均处理时间（纳秒）
	AverageProcessingTime int64 `json:"average_processing_time"`

	// StartTime 启动时间
	StartTime time.Time `json:"start_time"`
}

// NewTransport 创建传输实例
// config: 传输配置
// 返回: Transport实例和error
func NewTransport(config *TransportConfig) (Transport, error) {
	if config == nil {
		return nil, ErrInvalidConfig
	}

	switch config.Type {
	case TransportTypeSharedMemory:
		// 共享内存传输（Linux实现）
		return newShmTransport(config)

	case TransportTypeGRPC:
		// TODO: 实现gRPC传输
		return nil, ErrNotImplemented

	case TransportTypeREST:
		// TODO: 实现RESTful传输
		return nil, ErrNotImplemented

	case TransportTypeUnix:
		// TODO: 实现Unix Socket传输
		return nil, ErrNotImplemented

	default:
		return nil, ErrUnsupportedTransportType
	}
}

// NewServer 创建服务端实例
// config: 传输配置
// 返回: Server实例和error
func NewServer(config *TransportConfig) (Server, error) {
	if config == nil {
		return nil, ErrInvalidConfig
	}

	switch config.Type {
	case TransportTypeSharedMemory:
		return nil, ErrNotImplemented

	case TransportTypeGRPC:
		return nil, ErrNotImplemented

	case TransportTypeREST:
		return nil, ErrNotImplemented

	case TransportTypeUnix:
		return nil, ErrNotImplemented

	default:
		return nil, ErrUnsupportedTransportType
	}
}

// NewClient 创建客户端实例
// config: 传输配置
// 返回: Client实例和error
func NewClient(config *TransportConfig) (Client, error) {
	if config == nil {
		return nil, ErrInvalidConfig
	}

	switch config.Type {
	case TransportTypeSharedMemory:
		return nil, ErrNotImplemented

	case TransportTypeGRPC:
		return nil, ErrNotImplemented

	case TransportTypeREST:
		return nil, ErrNotImplemented

	case TransportTypeUnix:
		return nil, ErrNotImplemented

	default:
		return nil, ErrUnsupportedTransportType
	}
}

// 注意：Transport接口包含Close()方法，实现了io.Closer接口
// 各个具体实现需要保证实现Close()方法

// ===== 配置辅助函数 =====

// NewSPSCConfig 创建SPSC配置（单生产者单消费者，最高性能）
func NewSPSCConfig(shmPath string) *TransportConfig {
	config := DefaultTransportConfig(TransportTypeSharedMemory)
	config.ShmPath = shmPath
	config.ConcurrencyMode = ConcurrencyModeSPSC
	config.PartitionCount = 1
	config.AutoAcknowledge = true
	return config
}

// NewMPMCConfig 创建MPMC配置（多生产者多消费者，支持故障恢复）
func NewMPMCConfig(shmPath string, partitionCount int, consumerGroup string) *TransportConfig {
	config := DefaultTransportConfig(TransportTypeSharedMemory)
	config.ShmPath = shmPath
	config.ConcurrencyMode = ConcurrencyModeMPMC
	config.PartitionCount = partitionCount
	config.ConsumerGroup = consumerGroup
	config.AutoAcknowledge = false // MPMC需要手动确认
	config.PartitionStrategy = PartitionStrategySticky
	config.LoadBalancePolicy = LoadBalancePolicyRoundRobin
	return config
}

// NewProducerConfig 创建生产者配置
func NewProducerConfig(baseConfig *TransportConfig, producerID int) *TransportConfig {
	config := *baseConfig
	config.Role = RoleProducer
	config.ProducerID = producerID
	return &config
}

// NewConsumerConfig 创建消费者配置
func NewConsumerConfig(baseConfig *TransportConfig, consumerGroup string) *TransportConfig {
	config := *baseConfig
	config.Role = RoleConsumer
	config.ConsumerGroup = consumerGroup
	config.ConsumerID = -1 // 自动分配
	return &config
}

// Clone 克隆配置
func (c *TransportConfig) Clone() *TransportConfig {
	if c == nil {
		return nil
	}
	config := *c
	if c.TLSConfig != nil {
		tlsConfig := *c.TLSConfig
		config.TLSConfig = &tlsConfig
	}
	if c.KeepAlive != nil {
		keepAlive := *c.KeepAlive
		config.KeepAlive = &keepAlive
	}
	return &config
}

// Validate 验证配置
func (c *TransportConfig) Validate() error {
	if c == nil {
		return ErrInvalidConfig
	}

	// 验证传输类型
	switch c.Type {
	case TransportTypeSharedMemory:
		if c.ShmPath == "" {
			return ErrInvalidConfig
		}
		if c.ShmSize <= 0 {
			return ErrInvalidConfig
		}
	case TransportTypeGRPC, TransportTypeREST, TransportTypeUnix:
		if c.Address == "" {
			return ErrInvalidConfig
		}
	default:
		return ErrUnsupportedTransportType
	}

	// 验证MPMC配置
	if c.ConcurrencyMode == ConcurrencyModeMPMC {
		if c.PartitionCount <= 0 {
			return ErrInvalidConfig
		}
		if c.ConsumerGroup == "" {
			return ErrInvalidConfig
		}
	}

	// 验证超时配置
	if c.AckTimeout <= 0 {
		c.AckTimeout = 30 * time.Second
	}
	if c.HeartbeatInterval <= 0 {
		c.HeartbeatInterval = 5 * time.Second
	}
	if c.ConsumerTimeout <= 0 {
		c.ConsumerTimeout = 15 * time.Second
	}

	return nil
}
