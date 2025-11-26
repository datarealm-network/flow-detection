package capture

import "errors"

// 捕获层错误定义
var (
	// ErrInvalidConfig 配置无效
	ErrInvalidConfig = errors.New("invalid capture configuration")

	// ErrInvalidInterface 网卡无效
	ErrInvalidInterface = errors.New("invalid network interface")

	// ErrNotRunning 未运行
	ErrNotRunning = errors.New("capture not running")

	// ErrAlreadyRunning 已经运行
	ErrAlreadyRunning = errors.New("capture already running")

	// ErrTimeout 超时
	ErrTimeout = errors.New("capture timeout")

	// ErrNoPackets 没有数据包
	ErrNoPackets = errors.New("no packets available")

	// ErrBPFFilterInvalid BPF 过滤器无效
	ErrBPFFilterInvalid = errors.New("invalid BPF filter expression")

	// ErrSocketCreationFailed socket 创建失败
	ErrSocketCreationFailed = errors.New("failed to create raw socket")

	// ErrMmapFailed mmap 失败
	ErrMmapFailed = errors.New("failed to mmap ring buffer")

	// ErrBindFailed 绑定失败
	ErrBindFailed = errors.New("failed to bind socket to interface")

	// ErrSetSockOptFailed setsockopt 失败
	ErrSetSockOptFailed = errors.New("failed to set socket option")

	// ErrInterfaceNotFound 网卡未找到
	ErrInterfaceNotFound = errors.New("network interface not found")

	// ErrPermissionDenied 权限不足
	ErrPermissionDenied = errors.New("permission denied (需要 root 或 CAP_NET_RAW)")

	// ErrPacketTooLarge 数据包过大
	ErrPacketTooLarge = errors.New("packet too large")

	// ErrRingBufferFull Ring Buffer 满
	ErrRingBufferFull = errors.New("ring buffer full")
)

// CaptureError 捕获错误（包含详细信息）
type CaptureError struct {
	Op        string // 操作名称
	Interface string // 网卡名称
	Err       error  // 底层错误
	Message   string // 错误消息
}

// Error 实现 error 接口
func (e *CaptureError) Error() string {
	msg := "capture error"
	if e.Op != "" {
		msg += " [op=" + e.Op + "]"
	}
	if e.Interface != "" {
		msg += " [iface=" + e.Interface + "]"
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
func (e *CaptureError) Unwrap() error {
	return e.Err
}

// NewCaptureError 创建捕获错误
func NewCaptureError(op, iface string, err error, message string) *CaptureError {
	return &CaptureError{
		Op:        op,
		Interface: iface,
		Err:       err,
		Message:   message,
	}
}
