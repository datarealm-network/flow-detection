//go:build linux

package shm

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestSharedMemory(t *testing.T) {
	name := fmt.Sprintf("test-shm-%d", time.Now().UnixNano())
	size := int64(1024 * 1024) // 1MB

	// 创建共享内存
	shm, err := CreateSharedMemory(name, size)
	if err != nil {
		t.Fatalf("CreateSharedMemory failed: %v", err)
	}
	defer shm.Close()

	// 验证属性
	if shm.GetName() != name {
		t.Errorf("Name mismatch: got %s, want %s", shm.GetName(), name)
	}
	if shm.GetSize() != size {
		t.Errorf("Size mismatch: got %d, want %d", shm.GetSize(), size)
	}
	if !shm.IsOwner() {
		t.Error("Should be owner")
	}

	// 写入数据
	data := shm.GetData()
	testData := []byte("Hello, Shared Memory!")
	copy(data[4096:], testData) // 跳过控制区

	// 读取数据验证
	readData := make([]byte, len(testData))
	copy(readData, data[4096:4096+len(testData)])
	if string(readData) != string(testData) {
		t.Errorf("Data mismatch: got %s, want %s", string(readData), string(testData))
	}
}

func TestRingBuffer(t *testing.T) {
	name := fmt.Sprintf("test-rb-%d", time.Now().UnixNano())
	size := int64(256 * 1024) // 256KB

	// 创建共享内存
	shm, err := CreateSharedMemory(name, size)
	if err != nil {
		t.Fatalf("CreateSharedMemory failed: %v", err)
	}
	defer shm.Close()

	// 创建Ring Buffer
	rb := NewRingBuffer(shm)

	// 测试1: 写入和读取单条消息
	msg1 := []byte("Test Message 1")
	if err := rb.Write(msg1); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	readMsg1, err := rb.Read()
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if string(readMsg1) != string(msg1) {
		t.Errorf("Message mismatch: got %s, want %s", string(readMsg1), string(msg1))
	}

	// 测试2: 写入多条消息
	messages := []string{
		"Message 2",
		"Message 3",
		"Message 4",
		"Message 5",
	}

	for _, msg := range messages {
		if err := rb.Write([]byte(msg)); err != nil {
			t.Fatalf("Write failed: %v", err)
		}
	}

	// 读取并验证
	for i, expected := range messages {
		readMsg, err := rb.Read()
		if err != nil {
			t.Fatalf("Read %d failed: %v", i, err)
		}
		if string(readMsg) != expected {
			t.Errorf("Message %d mismatch: got %s, want %s", i, string(readMsg), expected)
		}
	}

	// 测试3: 空缓冲区读取
	_, err = rb.Read()
	if err != ErrBufferEmpty {
		t.Errorf("Expected ErrBufferEmpty, got %v", err)
	}
}

func TestRingBufferFull(t *testing.T) {
	name := fmt.Sprintf("test-rb-full-%d", time.Now().UnixNano())
	size := int64(64 * 1024) // 64KB

	shm, err := CreateSharedMemory(name, size)
	if err != nil {
		t.Fatalf("CreateSharedMemory failed: %v", err)
	}
	defer shm.Close()

	rb := NewRingBuffer(shm)

	// 写入大量数据直到满
	msgSize := 1024 // 1KB消息
	msg := make([]byte, msgSize)
	for i := 0; i < msgSize; i++ {
		msg[i] = byte(i % 256)
	}

	writeCount := 0
	for {
		err := rb.Write(msg)
		if err == ErrBufferFull {
			break
		}
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		writeCount++

		// 防止无限循环
		if writeCount > 100 {
			break
		}
	}

	t.Logf("Successfully wrote %d messages before buffer full", writeCount)

	// 读取一条消息
	readMsg, err := rb.Read()
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if len(readMsg) != msgSize {
		t.Errorf("Message size mismatch: got %d, want %d", len(readMsg), msgSize)
	}

	// 现在应该可以再写入一条
	if err := rb.Write(msg); err != nil {
		t.Errorf("Should be able to write after read, got error: %v", err)
	}
}

func TestRingBufferStatistics(t *testing.T) {
	name := fmt.Sprintf("test-rb-stats-%d", time.Now().UnixNano())
	size := int64(256 * 1024)

	shm, err := CreateSharedMemory(name, size)
	if err != nil {
		t.Fatalf("CreateSharedMemory failed: %v", err)
	}
	defer shm.Close()

	rb := NewRingBuffer(shm)

	// 初始状态
	if !rb.IsEmpty() {
		t.Error("Buffer should be empty initially")
	}
	if rb.IsFull() {
		t.Error("Buffer should not be full initially")
	}

	// 写入几条消息
	messages := []string{"Msg1", "Msg2", "Msg3"}
	for _, msg := range messages {
		rb.Write([]byte(msg))
	}

	// 检查统计
	if rb.IsEmpty() {
		t.Error("Buffer should not be empty")
	}
	if rb.GetMessageCount() != uint64(len(messages)) {
		t.Errorf("Message count mismatch: got %d, want %d", rb.GetMessageCount(), len(messages))
	}

	// 读取所有消息
	for range messages {
		rb.Read()
	}

	// 应该又为空了
	if !rb.IsEmpty() {
		t.Error("Buffer should be empty after reading all")
	}
}

func TestRingBufferWrapAround(t *testing.T) {
	name := fmt.Sprintf("test-rb-wrap-%d", time.Now().UnixNano())
	size := int64(16 * 1024) // 小缓冲区测试环绕

	shm, err := CreateSharedMemory(name, size)
	if err != nil {
		t.Fatalf("CreateSharedMemory failed: %v", err)
	}
	defer shm.Close()

	rb := NewRingBuffer(shm)

	// 写入和读取多轮，触发环绕
	for round := 0; round < 10; round++ {
		msg := fmt.Sprintf("Round %d Message", round)

		if err := rb.Write([]byte(msg)); err != nil {
			t.Fatalf("Round %d write failed: %v", round, err)
		}

		readMsg, err := rb.Read()
		if err != nil {
			t.Fatalf("Round %d read failed: %v", round, err)
		}

		if string(readMsg) != msg {
			t.Errorf("Round %d message mismatch: got %s, want %s", round, string(readMsg), msg)
		}
	}
}

// BenchmarkRingBufferWrite 写入性能测试
func BenchmarkRingBufferWrite(b *testing.B) {
	name := fmt.Sprintf("bench-rb-write-%d", time.Now().UnixNano())
	size := int64(256 * 1024 * 1024) // 256MB

	shm, err := CreateSharedMemory(name, size)
	if err != nil {
		b.Fatalf("CreateSharedMemory failed: %v", err)
	}
	defer shm.Close()

	rb := NewRingBuffer(shm)
	msg := make([]byte, 1024) // 1KB消息

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if err := rb.Write(msg); err != nil {
			if err == ErrBufferFull {
				// 缓冲区满，读取一些
				for j := 0; j < 100; j++ {
					rb.Read()
				}
				rb.Write(msg)
			} else {
				b.Fatalf("Write failed: %v", err)
			}
		}
	}
}

// BenchmarkRingBufferRead 读取性能测试
func BenchmarkRingBufferRead(b *testing.B) {
	name := fmt.Sprintf("bench-rb-read-%d", time.Now().UnixNano())
	size := int64(256 * 1024 * 1024) // 256MB

	shm, err := CreateSharedMemory(name, size)
	if err != nil {
		b.Fatalf("CreateSharedMemory failed: %v", err)
	}
	defer shm.Close()

	rb := NewRingBuffer(shm)
	msg := make([]byte, 1024) // 1KB消息

	// 预填充数据
	for i := 0; i < 10000; i++ {
		rb.Write(msg)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := rb.Read()
		if err == ErrBufferEmpty {
			// 重新填充
			for j := 0; j < 1000; j++ {
				rb.Write(msg)
			}
			rb.Read()
		}
	}
}

func TestMain(m *testing.M) {
	// 清理旧的测试文件
	os.RemoveAll("/dev/shm/test-*")
	os.RemoveAll("/dev/shm/bench-*")

	// 运行测试
	code := m.Run()

	// 清理
	os.RemoveAll("/dev/shm/test-*")
	os.RemoveAll("/dev/shm/bench-*")

	os.Exit(code)
}
