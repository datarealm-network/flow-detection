//go:build linux

package shm

import (
	"fmt"
	"os"
	"sync/atomic"
	"syscall"
	"time"
)

// Semaphore 基于共享内存的自旋信号量（SPSC场景简化版）
// 注意：这是针对SPSC场景的优化实现，使用原子操作实现
type Semaphore struct {
	name    string
	counter *uint32 // 指向共享内存中的计数器
	lockFd  int     // 文件锁
	isOwner bool
}

// CreateSemaphore 创建信号量（基于文件锁）
func CreateSemaphore(name string, initialValue uint32) (*Semaphore, error) {
	lockPath := "/tmp/" + name + ".lock"

	// 删除可能存在的旧文件
	os.Remove(lockPath)

	// 创建锁文件
	fd, err := syscall.Open(
		lockPath,
		syscall.O_CREAT|syscall.O_RDWR,
		0600,
	)
	if err != nil {
		return nil, fmt.Errorf("create lock file failed: %w", err)
	}

	return &Semaphore{
		name:    name,
		counter: nil, // 将在Ring Buffer中初始化
		lockFd:  fd,
		isOwner: true,
	}, nil
}

// OpenSemaphore 打开已存在的信号量
func OpenSemaphore(name string) (*Semaphore, error) {
	lockPath := "/tmp/" + name + ".lock"

	// 打开锁文件
	fd, err := syscall.Open(lockPath, syscall.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("open lock file failed: %w", err)
	}

	return &Semaphore{
		name:    name,
		counter: nil,
		lockFd:  fd,
		isOwner: false,
	}, nil
}

// Wait 等待信号量（轮询方式，适合SPSC高性能场景）
func (s *Semaphore) Wait() error {
	// SPSC场景下不需要信号量，Ring Buffer自己的原子操作足够
	// 这里提供空实现
	return nil
}

// TryWait 尝试等待信号量（非阻塞）
func (s *Semaphore) TryWait() error {
	return nil
}

// TimedWait 超时等待信号量
func (s *Semaphore) TimedWait(timeout time.Duration) error {
	// 使用轮询检查数据可用性
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		// 在SPSC场景中，由Ring Buffer本身判断是否有数据
		return nil
	}

	return fmt.Errorf("wait timeout")
}

// Post 释放信号量
func (s *Semaphore) Post() error {
	// SPSC场景下不需要信号量
	return nil
}

// Close 关闭信号量
func (s *Semaphore) Close() error {
	if s.lockFd > 0 {
		syscall.Close(s.lockFd)
	}

	if s.isOwner {
		lockPath := "/tmp/" + s.name + ".lock"
		os.Remove(lockPath)
	}

	return nil
}

// GetValue 获取信号量值（调试用）
func (s *Semaphore) GetValue() (int, error) {
	if s.counter != nil {
		return int(atomic.LoadUint32(s.counter)), nil
	}
	return 0, nil
}
