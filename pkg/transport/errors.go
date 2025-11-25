package transport

import "errors"

// 传输层错误定义
var (
	// ErrInvalidConfig 配置无效
	ErrInvalidConfig = errors.New("invalid transport configuration")

	// ErrNotImplemented 功能未实现
	ErrNotImplemented = errors.New("feature not implemented yet")

	// ErrUnsupportedTransportType 不支持的传输类型
	ErrUnsupportedTransportType = errors.New("unsupported transport type")

	// ErrUnsupportedCodecType 不支持的编解码器类型
	ErrUnsupportedCodecType = errors.New("unsupported codec type")

	// ErrCodecNotFound 编解码器未找到
	ErrCodecNotFound = errors.New("codec not found")

	// ErrAlreadyStarted 已经启动
	ErrAlreadyStarted = errors.New("transport already started")

	// ErrNotStarted 未启动
	ErrNotStarted = errors.New("transport not started")

	// ErrAlreadyConnected 已经连接
	ErrAlreadyConnected = errors.New("already connected")

	// ErrNotConnected 未连接
	ErrNotConnected = errors.New("not connected")

	// ErrConnectionClosed 连接已关闭
	ErrConnectionClosed = errors.New("connection closed")

	// ErrTimeout 操作超时
	ErrTimeout = errors.New("operation timeout")

	// ErrInvalidMessage 无效的消息
	ErrInvalidMessage = errors.New("invalid message")

	// ErrInvalidMessageHeader 无效的消息头
	ErrInvalidMessageHeader = errors.New("invalid message header")

	// ErrMissingMessageID 缺少消息ID
	ErrMissingMessageID = errors.New("missing message ID")

	// ErrMissingMessageType 缺少消息类型
	ErrMissingMessageType = errors.New("missing message type")

	// ErrIncompatibleVersion 协议版本不兼容
	ErrIncompatibleVersion = errors.New("incompatible protocol version")

	// ErrMessageExpired 消息已过期
	ErrMessageExpired = errors.New("message expired")

	// ErrMessageTooLarge 消息过大
	ErrMessageTooLarge = errors.New("message too large")

	// ErrBufferFull 缓冲区满
	ErrBufferFull = errors.New("buffer full")

	// ErrNoHandler 没有处理器
	ErrNoHandler = errors.New("no handler registered")

	// ErrHandlerAlreadyRegistered 处理器已注册
	ErrHandlerAlreadyRegistered = errors.New("handler already registered")

	// ErrEncodeFailed 编码失败
	ErrEncodeFailed = errors.New("failed to encode message")

	// ErrDecodeFailed 解码失败
	ErrDecodeFailed = errors.New("failed to decode message")

	// ErrCompressFailed 压缩失败
	ErrCompressFailed = errors.New("failed to compress data")

	// ErrDecompressFailed 解压失败
	ErrDecompressFailed = errors.New("failed to decompress data")

	// ErrEncryptFailed 加密失败
	ErrEncryptFailed = errors.New("failed to encrypt data")

	// ErrDecryptFailed 解密失败
	ErrDecryptFailed = errors.New("failed to decrypt data")

	// ErrSendFailed 发送失败
	ErrSendFailed = errors.New("failed to send message")

	// ErrReceiveFailed 接收失败
	ErrReceiveFailed = errors.New("failed to receive message")

	// ErrStreamClosed 流已关闭
	ErrStreamClosed = errors.New("stream closed")

	// ErrInvalidAddress 无效的地址
	ErrInvalidAddress = errors.New("invalid address")

	// ErrBindFailed 绑定失败
	ErrBindFailed = errors.New("failed to bind address")

	// ErrListenFailed 监听失败
	ErrListenFailed = errors.New("failed to listen")

	// ErrAcceptFailed 接受连接失败
	ErrAcceptFailed = errors.New("failed to accept connection")

	// ErrDialFailed 连接失败
	ErrDialFailed = errors.New("failed to dial")

	// ErrTLSConfigInvalid TLS配置无效
	ErrTLSConfigInvalid = errors.New("invalid TLS configuration")

	// ErrCertificateInvalid 证书无效
	ErrCertificateInvalid = errors.New("invalid certificate")

	// ErrSharedMemoryCreateFailed 共享内存创建失败
	ErrSharedMemoryCreateFailed = errors.New("failed to create shared memory")

	// ErrSharedMemoryOpenFailed 共享内存打开失败
	ErrSharedMemoryOpenFailed = errors.New("failed to open shared memory")

	// ErrSharedMemoryMapFailed 共享内存映射失败
	ErrSharedMemoryMapFailed = errors.New("failed to map shared memory")

	// ErrSharedMemoryUnmapFailed 共享内存解除映射失败
	ErrSharedMemoryUnmapFailed = errors.New("failed to unmap shared memory")

	// ErrSharedMemoryWriteFailed 共享内存写入失败
	ErrSharedMemoryWriteFailed = errors.New("failed to write to shared memory")

	// ErrSharedMemoryReadFailed 共享内存读取失败
	ErrSharedMemoryReadFailed = errors.New("failed to read from shared memory")

	// ErrSemaphoreCreateFailed 信号量创建失败
	ErrSemaphoreCreateFailed = errors.New("failed to create semaphore")

	// ErrSemaphoreWaitFailed 信号量等待失败
	ErrSemaphoreWaitFailed = errors.New("failed to wait on semaphore")

	// ErrSemaphorePostFailed 信号量释放失败
	ErrSemaphorePostFailed = errors.New("failed to post semaphore")

	// ErrTopicNotFound 主题未找到
	ErrTopicNotFound = errors.New("topic not found")

	// ErrSubscriptionFailed 订阅失败
	ErrSubscriptionFailed = errors.New("subscription failed")

	// ErrPublishFailed 发布失败
	ErrPublishFailed = errors.New("publish failed")

	// ErrNoResponse 没有响应
	ErrNoResponse = errors.New("no response received")

	// ErrResponseMismatch 响应不匹配
	ErrResponseMismatch = errors.New("response correlation ID mismatch")
)

// TransportError 传输错误（包含详细信息）
type TransportError struct {
	Op      string // 操作名称
	Type    string // 传输类型
	Address string // 地址
	Err     error  // 底层错误
	Message string // 错误消息
}

// Error 实现error接口
func (e *TransportError) Error() string {
	msg := "transport error"
	if e.Op != "" {
		msg += " [op=" + e.Op + "]"
	}
	if e.Type != "" {
		msg += " [type=" + e.Type + "]"
	}
	if e.Address != "" {
		msg += " [addr=" + e.Address + "]"
	}
	if e.Message != "" {
		msg += ": " + e.Message
	}
	if e.Err != nil {
		msg += ": " + e.Err.Error()
	}
	return msg
}

// Unwrap 返回底层错误
func (e *TransportError) Unwrap() error {
	return e.Err
}

// NewTransportError 创建传输错误
func NewTransportError(op, transportType, address string, err error, message string) *TransportError {
	return &TransportError{
		Op:      op,
		Type:    transportType,
		Address: address,
		Err:     err,
		Message: message,
	}
}

// IsTimeout 检查是否为超时错误
func IsTimeout(err error) bool {
	return errors.Is(err, ErrTimeout)
}

// IsConnectionError 检查是否为连接错误
func IsConnectionError(err error) bool {
	return errors.Is(err, ErrNotConnected) ||
		errors.Is(err, ErrConnectionClosed) ||
		errors.Is(err, ErrDialFailed)
}

// IsMessageError 检查是否为消息错误
func IsMessageError(err error) bool {
	return errors.Is(err, ErrInvalidMessage) ||
		errors.Is(err, ErrInvalidMessageHeader) ||
		errors.Is(err, ErrMessageExpired) ||
		errors.Is(err, ErrMessageTooLarge)
}

// IsCodecError 检查是否为编解码错误
func IsCodecError(err error) bool {
	return errors.Is(err, ErrEncodeFailed) ||
		errors.Is(err, ErrDecodeFailed) ||
		errors.Is(err, ErrUnsupportedCodecType)
}

// IsSharedMemoryError 检查是否为共享内存错误
func IsSharedMemoryError(err error) bool {
	return errors.Is(err, ErrSharedMemoryCreateFailed) ||
		errors.Is(err, ErrSharedMemoryOpenFailed) ||
		errors.Is(err, ErrSharedMemoryMapFailed) ||
		errors.Is(err, ErrSharedMemoryUnmapFailed) ||
		errors.Is(err, ErrSharedMemoryWriteFailed) ||
		errors.Is(err, ErrSharedMemoryReadFailed)
}

// IsRecoverableError 检查是否为可恢复错误
func IsRecoverableError(err error) bool {
	return errors.Is(err, ErrTimeout) ||
		errors.Is(err, ErrBufferFull) ||
		errors.Is(err, ErrNoResponse)
}
