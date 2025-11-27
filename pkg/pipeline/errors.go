package pipeline

import "errors"

// 管道错误定义
var (
	// ErrPipelineNotRunning 管道未运行
	ErrPipelineNotRunning = errors.New("pipeline is not running")

	// ErrPipelineAlreadyRunning 管道已在运行
	ErrPipelineAlreadyRunning = errors.New("pipeline is already running")

	// ErrBufferFull 缓冲区已满
	ErrBufferFull = errors.New("pipeline buffer is full")

	// ErrChannelClosed 通道已关闭
	ErrChannelClosed = errors.New("pipeline channel is closed")

	// ErrInvalidConfig 无效配置
	ErrInvalidConfig = errors.New("invalid pipeline configuration")

	// ErrConsumerFailed 消费者处理失败
	ErrConsumerFailed = errors.New("consumer processing failed")
)
