//go:build !linux

package capture

import (
	"context"
	"fmt"
)

// newAFPacketCaptureImpl 非 Linux 平台不支持
func newAFPacketCaptureImpl(config *CaptureConfig) (Capture, error) {
	return nil, fmt.Errorf("AF_PACKET capture is only supported on Linux")
}

// afPacketCaptureStub stub implementation for non-Linux platforms
type afPacketCaptureStub struct{}

func (c *afPacketCaptureStub) Start(ctx context.Context) error {
	return fmt.Errorf("not supported on this platform")
}

func (c *afPacketCaptureStub) Stop() error {
	return nil
}

func (c *afPacketCaptureStub) ReadPacket() (*Packet, error) {
	return nil, fmt.Errorf("not supported on this platform")
}

func (c *afPacketCaptureStub) ReadPacketZeroCopy() (*Packet, error) {
	return nil, fmt.Errorf("not supported on this platform")
}

func (c *afPacketCaptureStub) SetBPFFilter(filter string) error {
	return fmt.Errorf("not supported on this platform")
}

func (c *afPacketCaptureStub) SetPromiscuous(enable bool) error {
	return fmt.Errorf("not supported on this platform")
}

func (c *afPacketCaptureStub) GetStats() *CaptureStats {
	return &CaptureStats{}
}

func (c *afPacketCaptureStub) IsRunning() bool {
	return false
}

func (c *afPacketCaptureStub) Close() error {
	return nil
}
