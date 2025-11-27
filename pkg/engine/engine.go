// Package engine 提供网络数据包捕获和传输的集成引擎
package engine

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"datarealm.cn/network/pkg/capture"
	"datarealm.cn/network/pkg/packet"
	"datarealm.cn/network/pkg/pipeline"
)

// CaptureEngine 捕获引擎
// 集成 Capturer 和 Pipeline，提供端到端的数据包捕获和传输功能
type CaptureEngine struct {
	capturer capture.Capturer
	pipeline pipeline.Pipeline

	config *EngineConfig

	// 状态
	running atomic.Bool
	mu      sync.RWMutex

	// 取消控制
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// 消费者回调
	consumers []pipeline.Consumer

	// 统计
	stats *EngineStats
}

// EngineConfig 引擎配置
type EngineConfig struct {
	// CaptureConfig 捕获配置
	CaptureConfig *capture.Config

	// PipelineConfig 管道配置
	PipelineConfig *pipeline.Config

	// EnableStats 是否启用统计
	EnableStats bool

	// StatsInterval 统计输出间隔
	StatsInterval time.Duration

	// StatsCallback 统计回调函数
	StatsCallback func(*EngineStats)
}

// EngineStats 引擎统计信息
type EngineStats struct {
	// CaptureStats 捕获统计
	CaptureStats *capture.CaptureStats

	// PipelineStats 管道统计
	PipelineStats *pipeline.PipelineStats

	// StartTime 启动时间
	StartTime time.Time

	// Uptime 运行时长
	Uptime time.Duration
}

// DefaultEngineConfig 返回默认引擎配置
func DefaultEngineConfig(interfaceName string) *EngineConfig {
	return &EngineConfig{
		CaptureConfig:  capture.DefaultConfig(interfaceName),
		PipelineConfig: pipeline.DefaultConfig(),
		EnableStats:    true,
		StatsInterval:  5 * time.Second,
	}
}

// HighPerformanceEngineConfig 返回高性能引擎配置
func HighPerformanceEngineConfig(interfaceName string) *EngineConfig {
	return &EngineConfig{
		CaptureConfig:  capture.HighPerformanceConfig(interfaceName),
		PipelineConfig: pipeline.HighThroughputConfig(),
		EnableStats:    true,
		StatsInterval:  1 * time.Second,
	}
}

// NewCaptureEngine 创建捕获引擎
func NewCaptureEngine(config *EngineConfig) (*CaptureEngine, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	// 创建捕获器
	capturer, err := capture.NewPcapCapturer(config.CaptureConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create capturer: %w", err)
	}

	// 创建管道
	pipe, err := pipeline.NewChannelPipeline(config.PipelineConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create pipeline: %w", err)
	}

	return &CaptureEngine{
		capturer:  capturer,
		pipeline:  pipe,
		config:    config,
		consumers: make([]pipeline.Consumer, 0),
		stats: &EngineStats{
			CaptureStats:  &capture.CaptureStats{},
			PipelineStats: &pipeline.PipelineStats{},
		},
	}, nil
}

// NewCaptureEngineWithComponents 使用自定义组件创建引擎
func NewCaptureEngineWithComponents(
	capturer capture.Capturer,
	pipe pipeline.Pipeline,
	config *EngineConfig,
) *CaptureEngine {
	return &CaptureEngine{
		capturer:  capturer,
		pipeline:  pipe,
		config:    config,
		consumers: make([]pipeline.Consumer, 0),
		stats: &EngineStats{
			CaptureStats:  &capture.CaptureStats{},
			PipelineStats: &pipeline.PipelineStats{},
		},
	}
}

// Start 启动引擎
func (e *CaptureEngine) Start(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running.Load() {
		return fmt.Errorf("engine is already running")
	}

	engineCtx, cancel := context.WithCancel(ctx)
	e.cancel = cancel
	e.stats.StartTime = time.Now()

	// 启动管道
	if err := e.pipeline.Start(engineCtx); err != nil {
		return fmt.Errorf("failed to start pipeline: %w", err)
	}

	// 启动捕获器
	if err := e.capturer.Start(engineCtx); err != nil {
		e.pipeline.Stop()
		return fmt.Errorf("failed to start capturer: %w", err)
	}

	// 启动转发 goroutine（将捕获的包转发到管道）
	e.wg.Add(1)
	go e.forwardLoop(engineCtx)

	// 启动消费者
	for _, consumer := range e.consumers {
		if err := consumer.OnStart(engineCtx); err != nil {
			e.Stop()
			return fmt.Errorf("failed to start consumer: %w", err)
		}
		e.wg.Add(1)
		go e.consumeLoop(engineCtx, consumer)
	}

	// 启动统计输出
	if e.config.EnableStats && e.config.StatsCallback != nil {
		e.wg.Add(1)
		go e.statsLoop(engineCtx)
	}

	e.running.Store(true)
	return nil
}

// forwardLoop 将捕获的数据包转发到管道
func (e *CaptureEngine) forwardLoop(ctx context.Context) {
	defer e.wg.Done()

	inputChan := e.pipeline.Input()
	for {
		select {
		case <-ctx.Done():
			return
		case pkt, ok := <-e.capturer.Packets():
			if !ok {
				return
			}
			select {
			case inputChan <- pkt:
			case <-ctx.Done():
				return
			}
		}
	}
}

// consumeLoop 消费循环
func (e *CaptureEngine) consumeLoop(ctx context.Context, consumer pipeline.Consumer) {
	defer e.wg.Done()
	defer consumer.OnStop()

	// 检查是否是批量管道
	if batchPipe, ok := e.pipeline.(pipeline.BatchPipeline); ok && batchPipe.BatchOutput() != nil {
		for {
			select {
			case <-ctx.Done():
				return
			case batch, ok := <-batchPipe.BatchOutput():
				if !ok {
					return
				}
				if err := consumer.ConsumeBatch(ctx, batch); err != nil {
					// 记录错误但继续处理
					continue
				}
			}
		}
	}

	// 单包消费
	for {
		select {
		case <-ctx.Done():
			return
		case pkt, ok := <-e.pipeline.Output():
			if !ok {
				return
			}
			if err := consumer.Consume(ctx, pkt); err != nil {
				// 记录错误但继续处理
				continue
			}
		}
	}
}

// statsLoop 统计输出循环
func (e *CaptureEngine) statsLoop(ctx context.Context) {
	defer e.wg.Done()

	ticker := time.NewTicker(e.config.StatsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			stats := e.Stats()
			if e.config.StatsCallback != nil {
				e.config.StatsCallback(stats)
			}
		}
	}
}

// Stop 停止引擎
func (e *CaptureEngine) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running.Load() {
		return fmt.Errorf("engine is not running")
	}

	// 取消上下文
	if e.cancel != nil {
		e.cancel()
	}

	// 停止捕获器
	if err := e.capturer.Stop(); err != nil {
		// 继续停止其他组件
	}

	// 停止管道
	if err := e.pipeline.Stop(); err != nil {
		// 继续
	}

	// 等待所有 goroutine 结束
	e.wg.Wait()

	e.running.Store(false)
	return nil
}

// RegisterConsumer 注册消费者
func (e *CaptureEngine) RegisterConsumer(consumer pipeline.Consumer) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.consumers = append(e.consumers, consumer)
}

// SetBPFFilter 设置 BPF 过滤器
func (e *CaptureEngine) SetBPFFilter(filter string) error {
	return e.capturer.SetBPFFilter(filter)
}

// Stats 获取统计信息
func (e *CaptureEngine) Stats() *EngineStats {
	return &EngineStats{
		CaptureStats:  e.capturer.Stats(),
		PipelineStats: e.pipeline.Stats(),
		StartTime:     e.stats.StartTime,
		Uptime:        time.Since(e.stats.StartTime),
	}
}

// IsRunning 检查引擎是否运行
func (e *CaptureEngine) IsRunning() bool {
	return e.running.Load()
}

// Capturer 获取捕获器实例
func (e *CaptureEngine) Capturer() capture.Capturer {
	return e.capturer
}

// Pipeline 获取管道实例
func (e *CaptureEngine) Pipeline() pipeline.Pipeline {
	return e.pipeline
}

// Output 返回输出通道（供外部消费者使用）
func (e *CaptureEngine) Output() <-chan *packet.Packet {
	return e.pipeline.Output()
}
