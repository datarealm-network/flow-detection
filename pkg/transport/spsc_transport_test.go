//go:build linux

package transport

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestSPSCTransportBasic(t *testing.T) {
	shmPath := fmt.Sprintf("test-spsc-%d", time.Now().UnixNano())

	// 创建配置
	config := &TransportConfig{
		Type:            TransportTypeSharedMemory,
		ShmPath:         shmPath,
		ShmSize:         256 * 1024 * 1024, // 256MB
		CodecType:       CodecTypeBinary,
		ConcurrencyMode: ConcurrencyModeSPSC,
		Role:            RoleProducer,
	}

	// 创建生产者
	producer, err := NewSPSCTransport(config)
	if err != nil {
		t.Fatalf("NewSPSCTransport failed: %v", err)
	}

	ctx := context.Background()
	if err := producer.Start(ctx); err != nil {
		t.Fatalf("Start producer failed: %v", err)
	}
	defer producer.Stop()

	// 发送消息
	msg := NewMessage("test", []byte("Hello, SPSC!"))
	msg.Header.Source = "producer"
	msg.Header.Destination = "consumer"

	if err := producer.Send(ctx, msg); err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// 创建消费者
	consumerConfig := *config
	consumerConfig.Role = RoleConsumer

	consumer, err := NewSPSCTransport(&consumerConfig)
	if err != nil {
		t.Fatalf("NewSPSCTransport consumer failed: %v", err)
	}

	if err := consumer.Start(ctx); err != nil {
		t.Fatalf("Start consumer failed: %v", err)
	}
	defer consumer.Stop()

	// 接收消息
	receivedMsg, err := consumer.Receive(ctx, 5*time.Second)
	if err != nil {
		t.Fatalf("Receive failed: %v", err)
	}

	// 验证消息
	if string(receivedMsg.Payload) != string(msg.Payload) {
		t.Errorf("Payload mismatch: got %s, want %s", string(receivedMsg.Payload), string(msg.Payload))
	}
	if receivedMsg.Header.Source != msg.Header.Source {
		t.Errorf("Source mismatch: got %s, want %s", receivedMsg.Header.Source, msg.Header.Source)
	}
	if receivedMsg.Header.Destination != msg.Header.Destination {
		t.Errorf("Destination mismatch: got %s, want %s", receivedMsg.Header.Destination, msg.Header.Destination)
	}

	// 检查统计
	stats := producer.GetStats()
	if stats.MessagesSent != 1 {
		t.Errorf("Producer MessagesSent: got %d, want 1", stats.MessagesSent)
	}

	stats = consumer.GetStats()
	if stats.MessagesReceived != 1 {
		t.Errorf("Consumer MessagesReceived: got %d, want 1", stats.MessagesReceived)
	}
}

func TestSPSCTransportMultipleMessages(t *testing.T) {
	shmPath := fmt.Sprintf("test-spsc-multi-%d", time.Now().UnixNano())

	config := &TransportConfig{
		Type:            TransportTypeSharedMemory,
		ShmPath:         shmPath,
		ShmSize:         256 * 1024 * 1024,
		CodecType:       CodecTypeBinary,
		ConcurrencyMode: ConcurrencyModeSPSC,
		Role:            RoleProducer,
	}

	// 创建生产者
	producer, err := NewSPSCTransport(config)
	if err != nil {
		t.Fatalf("NewSPSCTransport failed: %v", err)
	}

	ctx := context.Background()
	if err := producer.Start(ctx); err != nil {
		t.Fatalf("Start producer failed: %v", err)
	}
	defer producer.Stop()

	// 发送多条消息
	messageCount := 1000
	for i := 0; i < messageCount; i++ {
		msg := NewMessage("test", []byte(fmt.Sprintf("Message %d", i)))
		if err := producer.Send(ctx, msg); err != nil {
			t.Fatalf("Send %d failed: %v", i, err)
		}
	}

	// 创建消费者
	consumerConfig := *config
	consumerConfig.Role = RoleConsumer

	consumer, err := NewSPSCTransport(&consumerConfig)
	if err != nil {
		t.Fatalf("NewSPSCTransport consumer failed: %v", err)
	}

	if err := consumer.Start(ctx); err != nil {
		t.Fatalf("Start consumer failed: %v", err)
	}
	defer consumer.Stop()

	// 接收并验证所有消息
	for i := 0; i < messageCount; i++ {
		receivedMsg, err := consumer.Receive(ctx, 5*time.Second)
		if err != nil {
			t.Fatalf("Receive %d failed: %v", i, err)
		}

		expected := fmt.Sprintf("Message %d", i)
		if string(receivedMsg.Payload) != expected {
			t.Errorf("Message %d mismatch: got %s, want %s", i, string(receivedMsg.Payload), expected)
		}
	}

	// 验证统计
	stats := producer.GetStats()
	if stats.MessagesSent != uint64(messageCount) {
		t.Errorf("MessagesSent: got %d, want %d", stats.MessagesSent, messageCount)
	}

	stats = consumer.GetStats()
	if stats.MessagesReceived != uint64(messageCount) {
		t.Errorf("MessagesReceived: got %d, want %d", stats.MessagesReceived, messageCount)
	}
}

func TestSPSCTransportTimeout(t *testing.T) {
	shmPath := fmt.Sprintf("test-spsc-timeout-%d", time.Now().UnixNano())

	config := &TransportConfig{
		Type:            TransportTypeSharedMemory,
		ShmPath:         shmPath,
		ShmSize:         64 * 1024 * 1024,
		CodecType:       CodecTypeBinary,
		ConcurrencyMode: ConcurrencyModeSPSC,
		Role:            RoleProducer,
	}

	// 创建生产者（但不发送消息）
	producer, err := NewSPSCTransport(config)
	if err != nil {
		t.Fatalf("NewSPSCTransport failed: %v", err)
	}

	ctx := context.Background()
	if err := producer.Start(ctx); err != nil {
		t.Fatalf("Start producer failed: %v", err)
	}
	defer producer.Stop()

	// 创建消费者
	consumerConfig := *config
	consumerConfig.Role = RoleConsumer

	consumer, err := NewSPSCTransport(&consumerConfig)
	if err != nil {
		t.Fatalf("NewSPSCTransport consumer failed: %v", err)
	}

	if err := consumer.Start(ctx); err != nil {
		t.Fatalf("Start consumer failed: %v", err)
	}
	defer consumer.Stop()

	// 尝试接收（应该超时）
	start := time.Now()
	_, err = consumer.Receive(ctx, 1*time.Second)
	elapsed := time.Since(start)

	if err != ErrTimeout {
		t.Errorf("Expected ErrTimeout, got %v", err)
	}

	if elapsed < 900*time.Millisecond || elapsed > 1200*time.Millisecond {
		t.Errorf("Timeout duration unexpected: %v", elapsed)
	}
}

// TestSPSCTransportCrossProcess 测试跨进程通信
func TestSPSCTransportCrossProcess(t *testing.T) {
	// 检查是否在子进程中运行
	if os.Getenv("TEST_CROSS_PROCESS") == "consumer" {
		runConsumer()
		return
	}
	if os.Getenv("TEST_CROSS_PROCESS") == "producer" {
		runProducer()
		return
	}

	shmPath := fmt.Sprintf("test-spsc-cross-%d", time.Now().UnixNano())

	// 启动生产者进程
	producerCmd := exec.Command(os.Args[0], "-test.run=TestSPSCTransportCrossProcess")
	producerCmd.Env = append(os.Environ(),
		"TEST_CROSS_PROCESS=producer",
		"TEST_SHM_PATH="+shmPath)

	if err := producerCmd.Start(); err != nil {
		t.Fatalf("Start producer process failed: %v", err)
	}

	// 等待生产者初始化
	time.Sleep(1 * time.Second)

	// 启动消费者进程
	consumerCmd := exec.Command(os.Args[0], "-test.run=TestSPSCTransportCrossProcess")
	consumerCmd.Env = append(os.Environ(),
		"TEST_CROSS_PROCESS=consumer",
		"TEST_SHM_PATH="+shmPath)

	output, err := consumerCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Consumer process failed: %v\nOutput: %s", err, string(output))
	}

	// 等待生产者完成
	producerCmd.Wait()

	t.Logf("Cross-process test completed successfully")
}

func runProducer() {
	shmPath := os.Getenv("TEST_SHM_PATH")

	config := &TransportConfig{
		Type:            TransportTypeSharedMemory,
		ShmPath:         shmPath,
		ShmSize:         64 * 1024 * 1024,
		CodecType:       CodecTypeBinary,
		ConcurrencyMode: ConcurrencyModeSPSC,
		Role:            RoleProducer,
	}

	producer, err := NewSPSCTransport(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "NewSPSCTransport failed: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	if err := producer.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Start producer failed: %v\n", err)
		os.Exit(1)
	}
	defer producer.Stop()

	// 发送10条消息
	for i := 0; i < 10; i++ {
		msg := NewMessage("test", []byte(fmt.Sprintf("Cross-Process Message %d", i)))
		if err := producer.Send(ctx, msg); err != nil {
			fmt.Fprintf(os.Stderr, "Send %d failed: %v\n", i, err)
			os.Exit(1)
		}
		time.Sleep(100 * time.Millisecond)
	}

	fmt.Println("Producer completed successfully")
}

func runConsumer() {
	shmPath := os.Getenv("TEST_SHM_PATH")

	config := &TransportConfig{
		Type:            TransportTypeSharedMemory,
		ShmPath:         shmPath,
		ShmSize:         64 * 1024 * 1024,
		CodecType:       CodecTypeBinary,
		ConcurrencyMode: ConcurrencyModeSPSC,
		Role:            RoleConsumer,
	}

	consumer, err := NewSPSCTransport(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "NewSPSCTransport failed: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	if err := consumer.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Start consumer failed: %v\n", err)
		os.Exit(1)
	}
	defer consumer.Stop()

	// 接收10条消息
	for i := 0; i < 10; i++ {
		msg, err := consumer.Receive(ctx, 5*time.Second)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Receive %d failed: %v\n", i, err)
			os.Exit(1)
		}

		expected := fmt.Sprintf("Cross-Process Message %d", i)
		if string(msg.Payload) != expected {
			fmt.Fprintf(os.Stderr, "Message %d mismatch: got %s, want %s\n",
				i, string(msg.Payload), expected)
			os.Exit(1)
		}
	}

	fmt.Println("Consumer completed successfully")
}

// BenchmarkSPSCTransportThroughput 吞吐量测试
func BenchmarkSPSCTransportThroughput(b *testing.B) {
	shmPath := fmt.Sprintf("bench-spsc-%d", time.Now().UnixNano())

	config := &TransportConfig{
		Type:            TransportTypeSharedMemory,
		ShmPath:         shmPath,
		ShmSize:         512 * 1024 * 1024, // 512MB
		CodecType:       CodecTypeBinary,
		ConcurrencyMode: ConcurrencyModeSPSC,
		Role:            RoleProducer,
	}

	producer, err := NewSPSCTransport(config)
	if err != nil {
		b.Fatalf("NewSPSCTransport failed: %v", err)
	}

	ctx := context.Background()
	if err := producer.Start(ctx); err != nil {
		b.Fatalf("Start producer failed: %v", err)
	}
	defer producer.Stop()

	// 创建消费者
	consumerConfig := *config
	consumerConfig.Role = RoleConsumer

	consumer, err := NewSPSCTransport(&consumerConfig)
	if err != nil {
		b.Fatalf("NewSPSCTransport consumer failed: %v", err)
	}

	if err := consumer.Start(ctx); err != nil {
		b.Fatalf("Start consumer failed: %v", err)
	}
	defer consumer.Stop()

	// 准备消息
	msg := NewMessage("bench", make([]byte, 1024)) // 1KB消息

	// 启动消费者goroutine
	done := make(chan struct{})
	go func() {
		for i := 0; i < b.N; i++ {
			_, err := consumer.Receive(ctx, 10*time.Second)
			if err != nil {
				b.Errorf("Receive failed: %v", err)
				break
			}
		}
		close(done)
	}()

	b.ResetTimer()

	// 发送消息
	for i := 0; i < b.N; i++ {
		if err := producer.Send(ctx, msg); err != nil {
			b.Fatalf("Send failed: %v", err)
		}
	}

	// 等待消费完成
	<-done

	b.StopTimer()

	// 报告统计
	stats := producer.GetStats()
	bytesPerSec := float64(stats.BytesSent) / b.Elapsed().Seconds()
	b.ReportMetric(bytesPerSec/1024/1024, "MB/s")
}
