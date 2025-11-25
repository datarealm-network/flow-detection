//go:build linux

package transport

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"datarealm.cn/network/pkg/transport/shm"
)

// SPSCTransport SPSC共享内存传输实现
type SPSCTransport struct {
	config *TransportConfig
	shm    *shm.SharedMemory
	rb     *shm.RingBuffer
	codec  Codec

	// 信号量（简化版，使用轮询）
	notEmpty *shm.Semaphore
	notFull  *shm.Semaphore

	// 状态
	running atomic.Bool
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup

	// 统计
	stats      *TransportStats
	statsMutex sync.RWMutex
}

// NewSPSCTransport 创建SPSC传输
func NewSPSCTransport(config *TransportConfig) (*SPSCTransport, error) {
	if config == nil {
		return nil, ErrInvalidConfig
	}

	// 验证配置
	if err := config.Validate(); err != nil {
		return nil, err
	}

	// 创建编解码器
	codec, err := GetCodec(config.CodecType)
	if err != nil {
		return nil, err
	}

	t := &SPSCTransport{
		config: config,
		codec:  codec,
		stats:  &TransportStats{},
	}

	return t, nil
}

// Start 启动传输
func (t *SPSCTransport) Start(ctx context.Context) error {
	if t.running.Load() {
		return fmt.Errorf("transport already running")
	}

	// 根据角色创建或打开共享内存
	var err error
	if t.config.Role == RoleProducer || t.config.Role == RoleBoth {
		// 生产者创建共享内存
		t.shm, err = shm.CreateSharedMemory(t.config.ShmPath, t.config.ShmSize)
		if err != nil {
			return fmt.Errorf("create shared memory failed: %w", err)
		}

		// 创建信号量
		t.notEmpty, err = shm.CreateSemaphore(t.config.ShmPath+"-notempty", 0)
		if err != nil {
			t.shm.Close()
			return fmt.Errorf("create semaphore failed: %w", err)
		}

		t.notFull, err = shm.CreateSemaphore(t.config.ShmPath+"-notfull", 1)
		if err != nil {
			t.shm.Close()
			t.notEmpty.Close()
			return fmt.Errorf("create semaphore failed: %w", err)
		}
	} else {
		// 消费者打开共享内存
		// 等待生产者创建共享内存
		timeout := time.After(30 * time.Second)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			t.shm, err = shm.OpenSharedMemory(t.config.ShmPath, t.config.ShmSize)
			if err == nil {
				break
			}

			select {
			case <-timeout:
				return fmt.Errorf("wait for shared memory timeout")
			case <-ticker.C:
				continue
			}
		}

		// 打开信号量
		t.notEmpty, err = shm.OpenSemaphore(t.config.ShmPath + "-notempty")
		if err != nil {
			t.shm.Close()
			return fmt.Errorf("open semaphore failed: %w", err)
		}

		t.notFull, err = shm.OpenSemaphore(t.config.ShmPath + "-notfull")
		if err != nil {
			t.shm.Close()
			t.notEmpty.Close()
			return fmt.Errorf("open semaphore failed: %w", err)
		}
	}

	// 创建Ring Buffer
	t.rb = shm.NewRingBuffer(t.shm)

	// 设置PID
	ctrl := t.shm.GetControlArea()
	if t.config.Role == RoleProducer || t.config.Role == RoleBoth {
		ctrl.ProducerPID = uint32(os.Getpid())
	} else {
		ctrl.ConsumerPID = uint32(os.Getpid())
	}

	// 创建上下文
	t.ctx, t.cancel = context.WithCancel(ctx)

	// 标记为运行中
	t.running.Store(true)

	return nil
}

// Stop 停止传输
func (t *SPSCTransport) Stop() error {
	if !t.running.Load() {
		return nil
	}

	// 取消上下文
	if t.cancel != nil {
		t.cancel()
	}

	// 等待所有goroutine退出
	t.wg.Wait()

	// 关闭信号量
	if t.notEmpty != nil {
		t.notEmpty.Close()
	}
	if t.notFull != nil {
		t.notFull.Close()
	}

	// 关闭共享内存
	if t.shm != nil {
		t.shm.Close()
	}

	// 标记为已停止
	t.running.Store(false)

	return nil
}

// Send 发送消息
func (t *SPSCTransport) Send(ctx context.Context, msg *Message) error {
	if !t.running.Load() {
		return fmt.Errorf("transport not running")
	}

	// 编码消息
	data, err := t.codec.Encode(msg)
	if err != nil {
		t.incrementErrorCount()
		return fmt.Errorf("encode failed: %w", err)
	}

	// 写入Ring Buffer
	for {
		err = t.rb.Write(data)
		if err == nil {
			break
		}

		if err == shm.ErrBufferFull {
			// 缓冲区满，等待
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(1 * time.Millisecond):
				continue
			}
		}

		t.incrementErrorCount()
		return fmt.Errorf("write failed: %w", err)
	}

	// 更新统计
	t.statsMutex.Lock()
	t.stats.MessagesSent++
	t.stats.BytesSent += uint64(len(data))
	t.statsMutex.Unlock()

	return nil
}

// SendAsync 异步发送消息
func (t *SPSCTransport) SendAsync(ctx context.Context, msg *Message) error {
	return t.Send(ctx, msg)
}

// Receive 接收消息
func (t *SPSCTransport) Receive(ctx context.Context, timeout time.Duration) (*Message, error) {
	if !t.running.Load() {
		return nil, fmt.Errorf("transport not running")
	}

	deadline := time.Now().Add(timeout)
	if timeout == 0 {
		deadline = time.Now().Add(365 * 24 * time.Hour) // 几乎永久
	}

	// 从Ring Buffer读取
	for {
		data, err := t.rb.Read()
		if err == nil {
			// 解码消息
			msg, err := t.codec.Decode(data)
			if err != nil {
				t.incrementErrorCount()
				return nil, fmt.Errorf("decode failed: %w", err)
			}

			// 更新统计
			t.statsMutex.Lock()
			t.stats.MessagesReceived++
			t.stats.BytesReceived += uint64(len(data))
			t.statsMutex.Unlock()

			return msg, nil
		}

		if err == shm.ErrBufferEmpty {
			// 缓冲区空，等待
			if time.Now().After(deadline) {
				return nil, ErrTimeout
			}

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(100 * time.Microsecond):
				continue
			}
		}

		t.incrementErrorCount()
		return nil, fmt.Errorf("read failed: %w", err)
	}
}

// Request 请求-响应（不支持）
func (t *SPSCTransport) Request(ctx context.Context, req *Message, timeout time.Duration) (*Message, error) {
	return nil, fmt.Errorf("request-response not supported in SPSC mode")
}

// Subscribe 订阅（不支持）
func (t *SPSCTransport) Subscribe(ctx context.Context, topic string, handler MessageHandler) error {
	return fmt.Errorf("subscribe not supported in SPSC mode")
}

// Unsubscribe 取消订阅（不支持）
func (t *SPSCTransport) Unsubscribe(topic string) error {
	return fmt.Errorf("unsubscribe not supported in SPSC mode")
}

// Publish 发布（不支持）
func (t *SPSCTransport) Publish(ctx context.Context, topic string, msg *Message) error {
	return fmt.Errorf("publish not supported in SPSC mode")
}

// IsRunning 是否运行中
func (t *SPSCTransport) IsRunning() bool {
	return t.running.Load()
}

// GetStats 获取统计信息
func (t *SPSCTransport) GetStats() *TransportStats {
	t.statsMutex.RLock()
	defer t.statsMutex.RUnlock()

	stats := *t.stats
	return &stats
}

// GetType 获取类型
func (t *SPSCTransport) GetType() TransportType {
	return TransportTypeSharedMemory
}

// SetCodec 设置编解码器
func (t *SPSCTransport) SetCodec(codec Codec) {
	t.codec = codec
}

// Close 关闭
func (t *SPSCTransport) Close() error {
	return t.Stop()
}

// incrementErrorCount 增加错误计数
func (t *SPSCTransport) incrementErrorCount() {
	t.statsMutex.Lock()
	t.stats.ErrorCount++
	t.statsMutex.Unlock()
}
