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
	}

	switch transportType {
	case TransportTypeSharedMemory:
		config.ShmPath = "/dev/shm/network-traffic"
		config.ShmSize = 256 * 1024 * 1024 // 256MB

	case TransportTypeGRPC:
		config.Address = "localhost:50051"
		config.KeepAlive = &KeepAliveConfig{
			Enabled: true,
			Time:    30 * time.Second,
			Timeout: 10 * time.Second,
		}

	case TransportTypeREST:
		config.Address = "http://localhost:8080"
		config.KeepAlive = &KeepAliveConfig{
			Enabled: true,
			Time:    60 * time.Second,
			Timeout: 10 * time.Second,
		}

	case TransportTypeUnix:
		config.Address = "/tmp/network-traffic.sock"
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
		// TODO: 实现共享内存传输
		return nil, ErrNotImplemented

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
