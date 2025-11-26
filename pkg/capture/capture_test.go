//go:build linux

package capture

import (
	"context"
	"os"
	"testing"
	"time"
)

// 注意：这些测试需要 root 权限和真实网卡

// TestNewCapture 测试创建捕获实例
func TestNewCapture(t *testing.T) {
	// 跳过 CI 环境
	if os.Getenv("CI") != "" {
		t.Skip("跳过需要 root 权限的测试")
	}

	config := DefaultCaptureConfig("lo")     // 使用 lo 回环接口
	config.RingBufferSize = 64 * 1024 * 1024 // 64MB

	capture, err := NewCapture(config)
	if err != nil {
		t.Skipf("创建捕获失败（可能需要 root 权限）: %v", err)
	}

	if capture == nil {
		t.Fatal("capture 为 nil")
	}
}

// TestCaptureStartStop 测试启动和停止
func TestCaptureStartStop(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("跳过需要 root 权限的测试")
	}

	config := DefaultCaptureConfig("lo")
	config.RingBufferSize = 64 * 1024 * 1024

	capture, err := NewCapture(config)
	if err != nil {
		t.Skipf("创建捕获失败: %v", err)
	}

	ctx := context.Background()
	if err := capture.Start(ctx); err != nil {
		t.Skipf("启动捕获失败（可能需要 root 权限）: %v", err)
	}

	if !capture.IsRunning() {
		t.Error("捕获应该正在运行")
	}

	// 等待一小段时间
	time.Sleep(100 * time.Millisecond)

	if err := capture.Stop(); err != nil {
		t.Errorf("停止捕获失败: %v", err)
	}

	if capture.IsRunning() {
		t.Error("捕获应该已停止")
	}
}

// TestCaptureBPFFilter 测试 BPF 过滤器
func TestCaptureBPFFilter(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("跳过需要 root 权限的测试")
	}

	config := DefaultCaptureConfig("lo")
	config.BPFFilter = "tcp" // 只捕获 TCP
	config.RingBufferSize = 64 * 1024 * 1024

	capture, err := NewCapture(config)
	if err != nil {
		t.Skipf("创建捕获失败: %v", err)
	}

	ctx := context.Background()
	if err := capture.Start(ctx); err != nil {
		t.Skipf("启动捕获失败: %v", err)
	}
	defer capture.Stop()

	// 测试动态设置过滤器
	if err := capture.SetBPFFilter("udp"); err != nil {
		t.Logf("设置 BPF 过滤器失败（可能不支持）: %v", err)
	}
}

// TestCaptureStats 测试统计信息
func TestCaptureStats(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("跳过需要 root 权限的测试")
	}

	config := DefaultCaptureConfig("lo")
	config.RingBufferSize = 64 * 1024 * 1024

	capture, err := NewCapture(config)
	if err != nil {
		t.Skipf("创建捕获失败: %v", err)
	}

	ctx := context.Background()
	if err := capture.Start(ctx); err != nil {
		t.Skipf("启动捕获失败: %v", err)
	}
	defer capture.Stop()

	stats := capture.GetStats()
	if stats == nil {
		t.Error("统计信息不应该为 nil")
	}

	t.Logf("统计信息: 接收=%d, 丢包=%d", stats.PacketsReceived, stats.PacketsDropped)
}

// BenchmarkCapture 性能基准测试
func BenchmarkCapture(b *testing.B) {
	if os.Getenv("CI") != "" {
		b.Skip("跳过需要 root 权限的测试")
	}

	config := DefaultCaptureConfig("lo")
	config.RingBufferSize = 256 * 1024 * 1024
	config.ZeroCopy = true

	capture, err := NewCapture(config)
	if err != nil {
		b.Skipf("创建捕获失败: %v", err)
	}

	ctx := context.Background()
	if err := capture.Start(ctx); err != nil {
		b.Skipf("启动捕获失败: %v", err)
	}
	defer capture.Stop()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := capture.ReadPacketZeroCopy()
		if err != nil {
			if err == ErrTimeout {
				continue
			}
			b.Fatalf("读取数据包失败: %v", err)
		}
	}
}
