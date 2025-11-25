//go:build linux

package transport

// newShmTransport 创建共享内存传输
func newShmTransport(config *TransportConfig) (Transport, error) {
	// 根据并发模式选择实现
	switch config.ConcurrencyMode {
	case ConcurrencyModeSPSC:
		// SPSC模式（单生产者单消费者）
		return NewSPSCTransport(config)

	case ConcurrencyModeMPSC, ConcurrencyModeSPMC, ConcurrencyModeMPMC:
		// MPMC模式（TODO）
		return nil, ErrNotImplemented

	default:
		// 默认使用SPSC
		return NewSPSCTransport(config)
	}
}
