//go:build linux

package shm

import (
	"encoding/binary"
	"errors"
	"sync/atomic"
	"unsafe"
)

var (
	ErrBufferFull      = errors.New("ring buffer is full")
	ErrBufferEmpty     = errors.New("ring buffer is empty")
	ErrMessageTooLarge = errors.New("message too large")
)

// RingBuffer 无锁环形缓冲区（SPSC）
type RingBuffer struct {
	shm  *SharedMemory
	ctrl *ControlArea
	data []byte
	size uint64
}

// MessageHeader Ring Buffer中的消息头
type MessageHeader struct {
	Length uint32 // 消息长度（不包括头部）
	CRC32  uint32 // CRC32校验和（可选）
}

const (
	MessageHeaderSize = 8 // sizeof(MessageHeader)
)

// NewRingBuffer 创建Ring Buffer
func NewRingBuffer(shm *SharedMemory) *RingBuffer {
	ctrl := shm.getControlArea()
	data := shm.getDataArea()

	return &RingBuffer{
		shm:  shm,
		ctrl: ctrl,
		data: data,
		size: uint64(len(data)),
	}
}

// Write 写入消息（生产者调用）
func (rb *RingBuffer) Write(msg []byte) error {
	msgLen := uint64(len(msg))
	totalLen := MessageHeaderSize + msgLen

	// 检查消息大小
	if totalLen > rb.size/2 {
		return ErrMessageTooLarge
	}

	// 获取当前偏移
	writeOffset := atomic.LoadUint64(&rb.ctrl.WriteOffset)
	readOffset := atomic.LoadUint64(&rb.ctrl.ReadOffset)

	// 计算可用空间
	available := rb.availableSpace(writeOffset, readOffset)
	if available < totalLen {
		return ErrBufferFull
	}

	// 写入消息头
	header := MessageHeader{
		Length: uint32(msgLen),
		CRC32:  0, // TODO: 可选的CRC校验
	}

	// 写入头部
	rb.writeAt(writeOffset, (*[MessageHeaderSize]byte)(unsafe.Pointer(&header))[:])

	// 写入消息体
	rb.writeAt(writeOffset+MessageHeaderSize, msg)

	// 更新写入位置（原子操作）
	newWriteOffset := (writeOffset + totalLen) % rb.size
	atomic.StoreUint64(&rb.ctrl.WriteOffset, newWriteOffset)

	// 增加消息计数
	atomic.AddUint64(&rb.ctrl.MessageCount, 1)

	return nil
}

// Read 读取消息（消费者调用）
func (rb *RingBuffer) Read() ([]byte, error) {
	// 获取当前偏移
	writeOffset := atomic.LoadUint64(&rb.ctrl.WriteOffset)
	readOffset := atomic.LoadUint64(&rb.ctrl.ReadOffset)

	// 检查是否有数据
	if readOffset == writeOffset {
		return nil, ErrBufferEmpty
	}

	// 读取消息头
	var header MessageHeader
	rb.readAt(readOffset, (*[MessageHeaderSize]byte)(unsafe.Pointer(&header))[:])

	msgLen := uint64(header.Length)

	// 分配缓冲区
	msg := make([]byte, msgLen)

	// 读取消息体
	rb.readAt(readOffset+MessageHeaderSize, msg)

	// 更新读取位置（原子操作）
	totalLen := MessageHeaderSize + msgLen
	newReadOffset := (readOffset + totalLen) % rb.size
	atomic.StoreUint64(&rb.ctrl.ReadOffset, newReadOffset)

	return msg, nil
}

// ReadZeroCopy 零拷贝读取（返回共享内存中的直接指针）
// 注意：返回的数据只在下次Read之前有效
func (rb *RingBuffer) ReadZeroCopy() ([]byte, error) {
	// 获取当前偏移
	writeOffset := atomic.LoadUint64(&rb.ctrl.WriteOffset)
	readOffset := atomic.LoadUint64(&rb.ctrl.ReadOffset)

	// 检查是否有数据
	if readOffset == writeOffset {
		return nil, ErrBufferEmpty
	}

	// 读取消息头
	var header MessageHeader
	rb.readAt(readOffset, (*[MessageHeaderSize]byte)(unsafe.Pointer(&header))[:])

	msgLen := uint64(header.Length)
	msgOffset := readOffset + MessageHeaderSize

	// 检查是否需要环绕
	if msgOffset+msgLen <= rb.size {
		// 不需要环绕，直接返回指针
		return rb.data[msgOffset : msgOffset+msgLen], nil
	}

	// 需要环绕，分配缓冲区并复制
	msg := make([]byte, msgLen)
	rb.readAt(msgOffset, msg)
	return msg, nil
}

// Commit 提交读取（零拷贝读取后必须调用）
func (rb *RingBuffer) Commit() {
	readOffset := atomic.LoadUint64(&rb.ctrl.ReadOffset)

	// 读取消息头获取长度
	var header MessageHeader
	rb.readAt(readOffset, (*[MessageHeaderSize]byte)(unsafe.Pointer(&header))[:])

	msgLen := uint64(header.Length)
	totalLen := MessageHeaderSize + msgLen

	// 更新读取位置
	newReadOffset := (readOffset + totalLen) % rb.size
	atomic.StoreUint64(&rb.ctrl.ReadOffset, newReadOffset)
}

// availableSpace 计算可用空间
func (rb *RingBuffer) availableSpace(writeOffset, readOffset uint64) uint64 {
	if writeOffset >= readOffset {
		return rb.size - (writeOffset - readOffset) - 1
	}
	return readOffset - writeOffset - 1
}

// writeAt 在指定位置写入数据（处理环绕）
func (rb *RingBuffer) writeAt(offset uint64, data []byte) {
	dataLen := uint64(len(data))

	if offset+dataLen <= rb.size {
		// 不需要环绕
		copy(rb.data[offset:], data)
	} else {
		// 需要环绕
		firstPart := rb.size - offset
		copy(rb.data[offset:], data[:firstPart])
		copy(rb.data[0:], data[firstPart:])
	}
}

// readAt 从指定位置读取数据（处理环绕）
func (rb *RingBuffer) readAt(offset uint64, data []byte) {
	dataLen := uint64(len(data))

	if offset+dataLen <= rb.size {
		// 不需要环绕
		copy(data, rb.data[offset:offset+dataLen])
	} else {
		// 需要环绕
		firstPart := rb.size - offset
		copy(data[:firstPart], rb.data[offset:])
		copy(data[firstPart:], rb.data[0:dataLen-firstPart])
	}
}

// GetMessageCount 获取消息数量
func (rb *RingBuffer) GetMessageCount() uint64 {
	return atomic.LoadUint64(&rb.ctrl.MessageCount)
}

// GetUsedSpace 获取已使用空间
func (rb *RingBuffer) GetUsedSpace() uint64 {
	writeOffset := atomic.LoadUint64(&rb.ctrl.WriteOffset)
	readOffset := atomic.LoadUint64(&rb.ctrl.ReadOffset)

	if writeOffset >= readOffset {
		return writeOffset - readOffset
	}
	return rb.size - (readOffset - writeOffset)
}

// GetAvailableSpace 获取可用空间
func (rb *RingBuffer) GetAvailableSpace() uint64 {
	writeOffset := atomic.LoadUint64(&rb.ctrl.WriteOffset)
	readOffset := atomic.LoadUint64(&rb.ctrl.ReadOffset)
	return rb.availableSpace(writeOffset, readOffset)
}

// IsEmpty 是否为空
func (rb *RingBuffer) IsEmpty() bool {
	writeOffset := atomic.LoadUint64(&rb.ctrl.WriteOffset)
	readOffset := atomic.LoadUint64(&rb.ctrl.ReadOffset)
	return writeOffset == readOffset
}

// IsFull 是否已满
func (rb *RingBuffer) IsFull() bool {
	return rb.GetAvailableSpace() < MessageHeaderSize
}

// Reset 重置（仅供测试使用）
func (rb *RingBuffer) Reset() {
	atomic.StoreUint64(&rb.ctrl.WriteOffset, 0)
	atomic.StoreUint64(&rb.ctrl.ReadOffset, 0)
	atomic.StoreUint64(&rb.ctrl.MessageCount, 0)
}

// helper: binary encoding helpers for header
func encodeHeader(header MessageHeader) []byte {
	buf := make([]byte, MessageHeaderSize)
	binary.LittleEndian.PutUint32(buf[0:4], header.Length)
	binary.LittleEndian.PutUint32(buf[4:8], header.CRC32)
	return buf
}

func decodeHeader(buf []byte) MessageHeader {
	return MessageHeader{
		Length: binary.LittleEndian.Uint32(buf[0:4]),
		CRC32:  binary.LittleEndian.Uint32(buf[4:8]),
	}
}
