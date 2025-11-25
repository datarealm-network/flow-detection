//go:build linux

package shm

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

// SharedMemory 共享内存管理器
type SharedMemory struct {
	name    string
	size    int64
	fd      int
	data    []byte
	isOwner bool // 是否是创建者
}

// CreateSharedMemory 创建共享内存
func CreateSharedMemory(name string, size int64) (*SharedMemory, error) {
	// 完整路径
	fullPath := "/dev/shm/" + name

	// 删除可能存在的旧文件
	os.Remove(fullPath)

	// 创建共享内存文件
	fd, err := syscall.Open(
		fullPath,
		syscall.O_CREAT|syscall.O_RDWR|syscall.O_EXCL,
		0600,
	)
	if err != nil {
		return nil, fmt.Errorf("create shared memory failed: %w", err)
	}

	// 设置文件大小
	if err := syscall.Ftruncate(fd, size); err != nil {
		syscall.Close(fd)
		os.Remove(fullPath)
		return nil, fmt.Errorf("ftruncate failed: %w", err)
	}

	// 映射到进程地址空间
	data, err := syscall.Mmap(
		fd,
		0,
		int(size),
		syscall.PROT_READ|syscall.PROT_WRITE,
		syscall.MAP_SHARED,
	)
	if err != nil {
		syscall.Close(fd)
		os.Remove(fullPath)
		return nil, fmt.Errorf("mmap failed: %w", err)
	}

	sm := &SharedMemory{
		name:    name,
		size:    size,
		fd:      fd,
		data:    data,
		isOwner: true,
	}

	// 初始化控制区
	sm.initControlArea()

	return sm, nil
}

// OpenSharedMemory 打开已存在的共享内存
func OpenSharedMemory(name string, size int64) (*SharedMemory, error) {
	fullPath := "/dev/shm/" + name

	// 打开共享内存文件
	fd, err := syscall.Open(fullPath, syscall.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("open shared memory failed: %w", err)
	}

	// 映射到进程地址空间
	data, err := syscall.Mmap(
		fd,
		0,
		int(size),
		syscall.PROT_READ|syscall.PROT_WRITE,
		syscall.MAP_SHARED,
	)
	if err != nil {
		syscall.Close(fd)
		return nil, fmt.Errorf("mmap failed: %w", err)
	}

	return &SharedMemory{
		name:    name,
		size:    size,
		fd:      fd,
		data:    data,
		isOwner: false,
	}, nil
}

// Close 关闭共享内存
func (sm *SharedMemory) Close() error {
	// 解除内存映射
	if len(sm.data) > 0 {
		if err := syscall.Munmap(sm.data); err != nil {
			return fmt.Errorf("munmap failed: %w", err)
		}
	}

	// 关闭文件描述符
	if sm.fd > 0 {
		if err := syscall.Close(sm.fd); err != nil {
			return fmt.Errorf("close fd failed: %w", err)
		}
	}

	// 如果是创建者，删除共享内存文件
	if sm.isOwner {
		fullPath := "/dev/shm/" + sm.name
		os.Remove(fullPath)
	}

	return nil
}

// GetData 获取数据区（零拷贝访问）
func (sm *SharedMemory) GetData() []byte {
	return sm.data
}

// ControlArea 控制区结构（4KB）
type ControlArea struct {
	MagicNumber    uint32     // 魔数标识
	Version        uint32     // 版本号
	RingBufferSize uint64     // Ring Buffer大小
	WriteOffset    uint64     // 写入位置（原子操作）
	ReadOffset     uint64     // 读取位置（原子操作）
	MessageCount   uint64     // 消息数量（原子操作）
	ProducerPID    uint32     // 生产者进程ID
	ConsumerPID    uint32     // 消费者进程ID
	Padding        [4040]byte // 填充到4KB
}

// initControlArea 初始化控制区
func (sm *SharedMemory) initControlArea() {
	ctrl := sm.getControlArea()
	ctrl.MagicNumber = 0x534D4D47 // "SMMG"
	ctrl.Version = 1
	ctrl.RingBufferSize = uint64(sm.size - 4096)
	ctrl.WriteOffset = 0
	ctrl.ReadOffset = 0
	ctrl.MessageCount = 0
	ctrl.ProducerPID = uint32(os.Getpid())
	ctrl.ConsumerPID = 0
}

// GetControlArea 获取控制区（零拷贝）
func (sm *SharedMemory) GetControlArea() *ControlArea {
	return (*ControlArea)(unsafe.Pointer(&sm.data[0]))
}

// getControlArea 获取控制区（内部使用）
func (sm *SharedMemory) getControlArea() *ControlArea {
	return sm.GetControlArea()
}

// getDataArea 获取数据区（零拷贝）
func (sm *SharedMemory) getDataArea() []byte {
	return sm.data[4096:]
}

// GetSize 获取大小
func (sm *SharedMemory) GetSize() int64 {
	return sm.size
}

// GetName 获取名称
func (sm *SharedMemory) GetName() string {
	return sm.name
}

// IsOwner 是否是创建者
func (sm *SharedMemory) IsOwner() bool {
	return sm.isOwner
}
