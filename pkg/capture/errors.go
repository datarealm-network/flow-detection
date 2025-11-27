package capture

import "errors"

// 捕获器错误定义
var (
	// ErrNotRunning 捕获器未运行
	ErrNotRunning = errors.New("capturer is not running")

	// ErrAlreadyRunning 捕获器已经在运行
	ErrAlreadyRunning = errors.New("capturer is already running")

	// ErrInvalidInterface 无效的网络接口
	ErrInvalidInterface = errors.New("invalid network interface")

	// ErrOpenDevice 打开设备失败
	ErrOpenDevice = errors.New("failed to open capture device")

	// ErrSetFilter 设置过滤器失败
	ErrSetFilter = errors.New("failed to set BPF filter")

	// ErrChannelClosed 通道已关闭
	ErrChannelClosed = errors.New("packet channel is closed")

	// ErrCaptureTimeout 捕获超时
	ErrCaptureTimeout = errors.New("capture timeout")
)
