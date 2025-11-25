//go:build !linux

package transport

import "fmt"

// newShmTransport 共享内存传输（非Linux平台不支持）
func newShmTransport(config *TransportConfig) (Transport, error) {
	return nil, fmt.Errorf("shared memory transport is only supported on Linux")
}
