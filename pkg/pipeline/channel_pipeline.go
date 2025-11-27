package pipeline

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"datarealm.cn/network/pkg/packet"
)

// ChannelPipeline 基于 Go channel 的高效数据管道实现
// 支持单包和批量两种传输模式
type ChannelPipeline struct {
	config *Config

	// 通道
	inputChan  chan *packet.Packet
	outputChan chan *packet.Packet
	batchChan  chan []*packet.Packet

	// 状态
	running atomic.Bool
	mu      sync.RWMutex

	// 统计
	stats *PipelineStats

	// 取消控制
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewChannelPipeline 创建新的 ChannelPipeline
func NewChannelPipeline(config *Config) (*ChannelPipeline, error) {
	if config == nil {
		config = DefaultConfig()
	}

	if err := config.Validate(); err != nil {
		return nil, err
	}

	p := &ChannelPipeline{
		config:     config,
		inputChan:  make(chan *packet.Packet, config.BufferSize),
		outputChan: make(chan *packet.Packet, config.BufferSize),
		stats: &PipelineStats{
			QueueCapacity: config.BufferSize,
		},
	}

	// 如果启用批量模式，创建批量通道
	if config.BatchSize > 1 {
		p.batchChan = make(chan []*packet.Packet, config.BufferSize/config.BatchSize)
	}

	return p, nil
}

// Start 启动管道
func (p *ChannelPipeline) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running.Load() {
		return ErrPipelineAlreadyRunning
	}

	pipelineCtx, cancel := context.WithCancel(ctx)
	p.cancel = cancel
	p.running.Store(true)

	// 启动工作协程
	for i := 0; i < p.config.Workers; i++ {
		p.wg.Add(1)
		if p.config.BatchSize > 1 {
			go p.batchWorker(pipelineCtx)
		} else {
			go p.worker(pipelineCtx)
		}
	}

	return nil
}

// worker 单包处理工作协程
func (p *ChannelPipeline) worker(ctx context.Context) {
	defer p.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case pkt, ok := <-p.inputChan:
			if !ok {
				return
			}

			// 更新统计
			atomic.AddUint64(&p.stats.PacketsIn, 1)
			atomic.AddUint64(&p.stats.BytesIn, uint64(pkt.Size()))

			// 应用中间件
			pkt = p.applyMiddlewares(pkt)
			if pkt == nil {
				atomic.AddUint64(&p.stats.PacketsDropped, 1)
				continue
			}

			// 发送到输出通道
			select {
			case p.outputChan <- pkt:
				atomic.AddUint64(&p.stats.PacketsOut, 1)
				atomic.AddUint64(&p.stats.BytesOut, uint64(pkt.Size()))
			case <-ctx.Done():
				return
			default:
				// 根据丢弃策略处理
				p.handleDropPolicy(pkt)
			}
		}
	}
}

// batchWorker 批量处理工作协程
func (p *ChannelPipeline) batchWorker(ctx context.Context) {
	defer p.wg.Done()

	batch := make([]*packet.Packet, 0, p.config.BatchSize)
	var timer *time.Timer
	var timerC <-chan time.Time

	if p.config.BatchTimeout > 0 {
		timer = time.NewTimer(p.config.BatchTimeout)
		timerC = timer.C
		defer timer.Stop()
	}

	flushBatch := func() {
		if len(batch) == 0 {
			return
		}

		// 复制批次数据
		batchCopy := make([]*packet.Packet, len(batch))
		copy(batchCopy, batch)

		// 尝试发送到批量通道
		select {
		case p.batchChan <- batchCopy:
			atomic.AddUint64(&p.stats.BatchesSent, 1)
			for _, pkt := range batchCopy {
				atomic.AddUint64(&p.stats.PacketsOut, 1)
				atomic.AddUint64(&p.stats.BytesOut, uint64(pkt.Size()))
			}
		default:
			atomic.AddUint64(&p.stats.PacketsDropped, uint64(len(batchCopy)))
		}

		// 清空批次
		batch = batch[:0]

		// 重置定时器
		if timer != nil {
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(p.config.BatchTimeout)
		}
	}

	for {
		select {
		case <-ctx.Done():
			flushBatch() // 刷新剩余数据
			return

		case pkt, ok := <-p.inputChan:
			if !ok {
				flushBatch()
				return
			}

			// 更新统计
			atomic.AddUint64(&p.stats.PacketsIn, 1)
			atomic.AddUint64(&p.stats.BytesIn, uint64(pkt.Size()))

			// 应用中间件
			pkt = p.applyMiddlewares(pkt)
			if pkt == nil {
				atomic.AddUint64(&p.stats.PacketsDropped, 1)
				continue
			}

			// 添加到批次
			batch = append(batch, pkt)

			// 检查批次是否已满
			if len(batch) >= p.config.BatchSize {
				flushBatch()
			}

		case <-timerC:
			flushBatch()
		}
	}
}

// applyMiddlewares 应用所有中间件
func (p *ChannelPipeline) applyMiddlewares(pkt *packet.Packet) *packet.Packet {
	for _, m := range p.config.Middlewares {
		pkt = m.Process(pkt)
		if pkt == nil {
			return nil
		}
	}
	return pkt
}

// handleDropPolicy 根据丢弃策略处理
func (p *ChannelPipeline) handleDropPolicy(pkt *packet.Packet) {
	switch p.config.DropPolicy {
	case DropPolicyBlock:
		// 阻塞等待
		p.outputChan <- pkt
		atomic.AddUint64(&p.stats.PacketsOut, 1)
		atomic.AddUint64(&p.stats.BytesOut, uint64(pkt.Size()))

	case DropPolicyNewest:
		// 丢弃最新的（当前这个）
		atomic.AddUint64(&p.stats.PacketsDropped, 1)

	case DropPolicyOldest:
		// 尝试丢弃最旧的
		select {
		case <-p.outputChan:
			atomic.AddUint64(&p.stats.PacketsDropped, 1)
		default:
		}
		// 然后尝试发送当前的
		select {
		case p.outputChan <- pkt:
			atomic.AddUint64(&p.stats.PacketsOut, 1)
			atomic.AddUint64(&p.stats.BytesOut, uint64(pkt.Size()))
		default:
			atomic.AddUint64(&p.stats.PacketsDropped, 1)
		}
	}
}

// Stop 停止管道
func (p *ChannelPipeline) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running.Load() {
		return ErrPipelineNotRunning
	}

	// 取消上下文
	if p.cancel != nil {
		p.cancel()
	}

	// 关闭输入通道
	close(p.inputChan)

	// 等待工作协程结束
	p.wg.Wait()

	// 关闭输出通道
	close(p.outputChan)
	if p.batchChan != nil {
		close(p.batchChan)
	}

	p.running.Store(false)
	return nil
}

// Input 返回输入通道
func (p *ChannelPipeline) Input() chan<- *packet.Packet {
	return p.inputChan
}

// Output 返回输出通道
func (p *ChannelPipeline) Output() <-chan *packet.Packet {
	return p.outputChan
}

// BatchOutput 返回批量输出通道
func (p *ChannelPipeline) BatchOutput() <-chan []*packet.Packet {
	return p.batchChan
}

// Stats 获取统计信息
func (p *ChannelPipeline) Stats() *PipelineStats {
	return &PipelineStats{
		PacketsIn:      atomic.LoadUint64(&p.stats.PacketsIn),
		PacketsOut:     atomic.LoadUint64(&p.stats.PacketsOut),
		PacketsDropped: atomic.LoadUint64(&p.stats.PacketsDropped),
		BytesIn:        atomic.LoadUint64(&p.stats.BytesIn),
		BytesOut:       atomic.LoadUint64(&p.stats.BytesOut),
		BatchesSent:    atomic.LoadUint64(&p.stats.BatchesSent),
		QueueLength:    len(p.inputChan),
		QueueCapacity:  p.config.BufferSize,
	}
}

// IsRunning 检查管道是否运行
func (p *ChannelPipeline) IsRunning() bool {
	return p.running.Load()
}

// SetBatchSize 设置批量大小
func (p *ChannelPipeline) SetBatchSize(size int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.config.BatchSize = size
}

// SetBatchTimeout 设置批量超时
func (p *ChannelPipeline) SetBatchTimeout(timeoutMs int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.config.BatchTimeout = time.Duration(timeoutMs) * time.Millisecond
}

// Send 发送单个数据包到管道（实现 Producer 接口）
func (p *ChannelPipeline) Send(pkt *packet.Packet) error {
	if !p.running.Load() {
		return ErrPipelineNotRunning
	}

	select {
	case p.inputChan <- pkt:
		return nil
	default:
		return ErrBufferFull
	}
}

// SendBlocking 阻塞发送单个数据包
func (p *ChannelPipeline) SendBlocking(ctx context.Context, pkt *packet.Packet) error {
	if !p.running.Load() {
		return ErrPipelineNotRunning
	}

	select {
	case p.inputChan <- pkt:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// SendBatch 发送一批数据包
func (p *ChannelPipeline) SendBatch(pkts []*packet.Packet) error {
	if !p.running.Load() {
		return ErrPipelineNotRunning
	}

	for _, pkt := range pkts {
		select {
		case p.inputChan <- pkt:
		default:
			return ErrBufferFull
		}
	}
	return nil
}

// 确保实现了接口
var _ Pipeline = (*ChannelPipeline)(nil)
var _ BatchPipeline = (*ChannelPipeline)(nil)
