//go:build linux

package capture

import (
	"context"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

// tpacketReq PACKET_RX_RING 请求结构
type tpacketReq struct {
	blockSize uint32 // 块大小
	blockNr   uint32 // 块数量
	frameSize uint32 // 帧大小
	frameNr   uint32 // 帧数量
}

// tpacket2Hdr TPACKET_V2 头部结构
type tpacket2Hdr struct {
	status   uint32 // TP_STATUS_* 标志
	len      uint32 // 数据包长度
	snaplen  uint32 // 捕获长度
	mac      uint16 // MAC 头偏移
	net      uint16 // 网络层头偏移
	sec      uint32 // 时间戳（秒）
	nsec     uint32 // 时间戳（纳秒）
	vlanTCI  uint16 // VLAN TCI
	vlanTPID uint16 // VLAN TPID
	padding  [4]uint8
}

// tpacket3Hdr TPACKET_V3 头部结构（更高效）
type tpacket3Hdr struct {
	nextOffset uint32
	sec        uint32
	nsec       uint32
	snaplen    uint32
	len        uint32
	status     uint32
	mac        uint16
	net        uint16
	hv1        tpacketHdrVariant1
	padding    [8]uint8
}

type tpacketHdrVariant1 struct {
	rxHash   uint32
	vlanTCI  uint32
	vlanTPID uint16
	padding  uint16
}

// TP_STATUS 标志
const (
	TP_STATUS_KERNEL          = 0
	TP_STATUS_USER            = 1
	TP_STATUS_COPY            = 1 << 1
	TP_STATUS_LOSING          = 1 << 2
	TP_STATUS_CSUMNOTREADY    = 1 << 3
	TP_STATUS_VLAN_VALID      = 1 << 4
	TP_STATUS_BLK_TMO         = 1 << 5
	TP_STATUS_VLAN_TPID_VALID = 1 << 6
	TP_STATUS_CSUM_VALID      = 1 << 7
)

// Socket 选项
const (
	PACKET_RX_RING      = 5
	PACKET_STATISTICS   = 6
	PACKET_VERSION      = 10
	PACKET_FANOUT       = 18
	PACKET_VNET_HDR     = 15
	PACKET_QDISC_BYPASS = 20

	// TPACKET 版本
	TPACKET_V1 = 0
	TPACKET_V2 = 1
	TPACKET_V3 = 2
)

// AFPacketCapture AF_PACKET + PACKET_MMAP 捕获实现
type AFPacketCapture struct {
	config *CaptureConfig

	// Socket 相关
	socket     int
	sockAddr   syscall.SockaddrLinklayer
	ifaceIndex int

	// mmap Ring Buffer
	ringBuffer []byte
	ringBlocks [][]byte // 块指针数组
	blockIndex int      // 当前块索引
	frameIndex int      // 当前帧索引

	// 统计
	stats      *CaptureStats
	statsMutex sync.RWMutex

	// 状态
	running atomic.Bool
	ctx     context.Context
	cancel  context.CancelFunc
	mu      sync.Mutex
}

// newAFPacketCaptureImpl 创建 AF_PACKET 捕获实例
func newAFPacketCaptureImpl(config *CaptureConfig) (Capture, error) {
	// 查找网卡索引
	iface, err := net.InterfaceByName(config.InterfaceName)
	if err != nil {
		return nil, NewCaptureError("lookup_interface", config.InterfaceName, err, "interface not found")
	}

	c := &AFPacketCapture{
		config:     config,
		ifaceIndex: iface.Index,
		stats: &CaptureStats{
			StartTime: time.Now(),
		},
	}

	return c, nil
}

// Start 启动捕获
func (c *AFPacketCapture) Start(ctx context.Context) error {
	if c.running.Load() {
		return ErrAlreadyRunning
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// 创建原始 socket
	socket, err := syscall.Socket(syscall.AF_PACKET, syscall.SOCK_RAW, htons(syscall.ETH_P_ALL))
	if err != nil {
		if err == syscall.EPERM {
			return ErrPermissionDenied
		}
		return NewCaptureError("socket", c.config.InterfaceName, err, "failed to create raw socket")
	}
	c.socket = socket

	// 设置 TPACKET_V3（最高效）
	version := TPACKET_V3
	if err := syscall.SetsockoptInt(c.socket, syscall.SOL_PACKET, PACKET_VERSION, version); err != nil {
		syscall.Close(c.socket)
		// 降级到 V2
		version = TPACKET_V2
		socket, _ = syscall.Socket(syscall.AF_PACKET, syscall.SOCK_RAW, htons(syscall.ETH_P_ALL))
		c.socket = socket
		if err := syscall.SetsockoptInt(c.socket, syscall.SOL_PACKET, PACKET_VERSION, version); err != nil {
			syscall.Close(c.socket)
			return NewCaptureError("setsockopt", c.config.InterfaceName, err, "failed to set TPACKET version")
		}
	}

	// 配置 Ring Buffer
	if err := c.setupRingBuffer(); err != nil {
		syscall.Close(c.socket)
		return err
	}

	// 绑定到网卡
	c.sockAddr = syscall.SockaddrLinklayer{
		Protocol: uint16(htons(syscall.ETH_P_ALL)),
		Ifindex:  c.ifaceIndex,
	}
	if err := syscall.Bind(c.socket, &c.sockAddr); err != nil {
		c.cleanup()
		return NewCaptureError("bind", c.config.InterfaceName, err, "failed to bind socket")
	}

	// 设置混杂模式
	if c.config.Promiscuous {
		if err := c.SetPromiscuous(true); err != nil {
			c.cleanup()
			return err
		}
	}

	// 设置 BPF 过滤器
	if c.config.BPFFilter != "" {
		if err := c.SetBPFFilter(c.config.BPFFilter); err != nil {
			c.cleanup()
			return err
		}
	}

	// 启用 BPF JIT
	if c.config.EnableBPFJIT {
		c.enableBPFJIT()
	}

	// 设置 Fanout（多进程捕获）
	if c.config.FanoutGroup > 0 {
		if err := c.setupFanout(); err != nil {
			c.cleanup()
			return err
		}
	}

	// 启用 QDISC bypass（绕过 qdisc 队列，更低延迟）
	syscall.SetsockoptInt(c.socket, syscall.SOL_PACKET, PACKET_QDISC_BYPASS, 1)

	c.ctx, c.cancel = context.WithCancel(ctx)
	c.running.Store(true)
	c.stats.StartTime = time.Now()

	return nil
}

// setupRingBuffer 设置 Ring Buffer
func (c *AFPacketCapture) setupRingBuffer() error {
	// 计算块数量
	blockSize := uint32(c.config.BlockSize)
	frameSize := uint32(c.config.FrameSize)
	blockNr := uint32(c.config.RingBufferSize / c.config.BlockSize)
	frameNr := (blockSize / frameSize) * blockNr

	req := tpacketReq{
		blockSize: blockSize,
		blockNr:   blockNr,
		frameSize: frameSize,
		frameNr:   frameNr,
	}

	// 设置 RX_RING
	if err := setsockoptTpacketReq(c.socket, syscall.SOL_PACKET, PACKET_RX_RING, &req); err != nil {
		return NewCaptureError("setsockopt_rxring", c.config.InterfaceName, err, "failed to setup RX_RING")
	}

	// mmap Ring Buffer（零拷贝关键）
	size := int(req.blockSize * req.blockNr)
	buf, err := syscall.Mmap(
		c.socket,
		0,
		size,
		syscall.PROT_READ|syscall.PROT_WRITE,
		syscall.MAP_SHARED,
	)
	if err != nil {
		return NewCaptureError("mmap", c.config.InterfaceName, err, "failed to mmap ring buffer")
	}

	c.ringBuffer = buf

	// 初始化块指针数组
	c.ringBlocks = make([][]byte, req.blockNr)
	for i := uint32(0); i < req.blockNr; i++ {
		offset := i * req.blockSize
		c.ringBlocks[i] = c.ringBuffer[offset : offset+req.blockSize]
	}

	c.blockIndex = 0
	c.frameIndex = 0

	return nil
}

// ReadPacket 读取单个数据包（阻塞）
func (c *AFPacketCapture) ReadPacket() (*Packet, error) {
	if !c.running.Load() {
		return nil, ErrNotRunning
	}

	// 零拷贝读取
	packet, err := c.ReadPacketZeroCopy()
	if err != nil {
		return nil, err
	}

	// 拷贝数据（因为需要返回独立的数据）
	data := make([]byte, len(packet.Data))
	copy(data, packet.Data)
	packet.Data = data
	packet.Metadata.IsZeroCopy = false

	return packet, nil
}

// ReadPacketZeroCopy 零拷贝读取数据包
func (c *AFPacketCapture) ReadPacketZeroCopy() (*Packet, error) {
	if !c.running.Load() {
		return nil, ErrNotRunning
	}

	deadline := time.Now().Add(c.config.Timeout)

	for {
		// 检查上下文
		select {
		case <-c.ctx.Done():
			return nil, c.ctx.Err()
		default:
		}

		// 获取当前块
		block := c.ringBlocks[c.blockIndex]
		hdr := (*tpacket3Hdr)(unsafe.Pointer(&block[0]))

		// 检查块状态
		status := atomic.LoadUint32(&hdr.status)
		if status&TP_STATUS_USER == 0 {
			// 块还在内核态，等待
			if time.Now().After(deadline) {
				return nil, ErrTimeout
			}
			time.Sleep(10 * time.Microsecond) // 微秒级轮询
			continue
		}

		// 解析数据包
		packet := &Packet{
			Timestamp:      int64(hdr.sec)*1e9 + int64(hdr.nsec),
			CaptureLength:  hdr.snaplen,
			OriginalLength: hdr.len,
			InterfaceIndex: c.ifaceIndex,
			Data:           block[hdr.mac : hdr.mac+uint16(hdr.snaplen)], // 零拷贝：直接引用 mmap 区域
			Metadata: &PacketMetadata{
				VLANTag:    uint16(hdr.hv1.vlanTCI),
				RXHash:     hdr.hv1.rxHash,
				Status:     hdr.status,
				IsZeroCopy: true,
			},
		}

		// 释放块给内核
		atomic.StoreUint32(&hdr.status, TP_STATUS_KERNEL)

		// 移动到下一块
		c.blockIndex = (c.blockIndex + 1) % len(c.ringBlocks)

		// 更新统计
		c.statsMutex.Lock()
		c.stats.PacketsReceived++
		c.stats.BytesReceived += uint64(packet.CaptureLength)
		c.stats.ZeroCopyCount++
		c.stats.LastPacketTime = time.Now()
		c.statsMutex.Unlock()

		return packet, nil
	}
}

// SetBPFFilter 设置 BPF 过滤器
func (c *AFPacketCapture) SetBPFFilter(filter string) error {
	if filter == "" {
		return nil
	}

	// 编译 BPF 过滤器（从 bpf_filter.go 导入）
	prog, err := compileBPFFilterInternal(filter, c.config.SnapLen)
	if err != nil {
		return NewCaptureError("bpf_compile", c.config.InterfaceName, err, "failed to compile BPF filter")
	}

	// 应用过滤器
	if err := setsockoptSockFprogInternal(c.socket, syscall.SOL_SOCKET, syscall.SO_ATTACH_FILTER, prog); err != nil {
		return NewCaptureError("bpf_attach", c.config.InterfaceName, err, "failed to attach BPF filter")
	}

	return nil
}

// PacketMreq 结构体（如果syscall包中没有定义）
type PacketMreq struct {
	Ifindex int32
	Type    uint16
	Alen    uint16
	Address [8]byte
}

const (
	PACKET_ADD_MEMBERSHIP  = 1
	PACKET_DROP_MEMBERSHIP = 2
	PACKET_MR_PROMISC      = 1
)

// SetPromiscuous 设置混杂模式
func (c *AFPacketCapture) SetPromiscuous(enable bool) error {
	mreq := PacketMreq{
		Ifindex: int32(c.ifaceIndex),
		Type:    PACKET_MR_PROMISC,
	}

	option := PACKET_ADD_MEMBERSHIP
	if !enable {
		option = PACKET_DROP_MEMBERSHIP
	}

	// 使用 setsockopt 系统调用
	_, _, errno := syscall.Syscall6(
		syscall.SYS_SETSOCKOPT,
		uintptr(c.socket),
		uintptr(syscall.SOL_PACKET),
		uintptr(option),
		uintptr(unsafe.Pointer(&mreq)),
		unsafe.Sizeof(mreq),
		0,
	)
	if errno != 0 {
		return NewCaptureError("promiscuous", c.config.InterfaceName, errno, "failed to set promiscuous mode")
	}

	return nil
}

// setupFanout 设置 Fanout（多进程负载均衡）
func (c *AFPacketCapture) setupFanout() error {
	fanoutArg := (c.config.FanoutGroup & 0xFFFF) | (int(c.config.FanoutType) << 16)
	if err := syscall.SetsockoptInt(c.socket, syscall.SOL_PACKET, PACKET_FANOUT, fanoutArg); err != nil {
		return NewCaptureError("fanout", c.config.InterfaceName, err, "failed to setup fanout")
	}
	return nil
}

// enableBPFJIT 启用 BPF JIT 编译
func (c *AFPacketCapture) enableBPFJIT() {
	// 写入 /proc/sys/net/core/bpf_jit_enable
	// 需要 root 权限
	// 这里只是尝试，失败不影响功能
	_ = os.WriteFile("/proc/sys/net/core/bpf_jit_enable", []byte("1\n"), 0644)
}

// GetStats 获取统计信息
func (c *AFPacketCapture) GetStats() *CaptureStats {
	c.statsMutex.RLock()
	defer c.statsMutex.RUnlock()

	stats := *c.stats

	// 获取内核统计
	if c.socket > 0 {
		kstats, err := c.getKernelStats()
		if err == nil {
			stats.PacketsDropped = kstats.Drops
			stats.PacketsIfDropped = kstats.Freeze
		}
	}

	return &stats
}

// getKernelStats 获取内核统计
func (c *AFPacketCapture) getKernelStats() (*tpacketStats, error) {
	stats := &tpacketStats{}
	_, _, errno := syscall.Syscall6(
		syscall.SYS_GETSOCKOPT,
		uintptr(c.socket),
		uintptr(syscall.SOL_PACKET),
		uintptr(PACKET_STATISTICS),
		uintptr(unsafe.Pointer(stats)),
		uintptr(unsafe.Sizeof(*stats)),
		0,
	)
	if errno != 0 {
		return nil, errno
	}
	return stats, nil
}

type tpacketStats struct {
	Packets uint32
	Drops   uint64
	Freeze  uint64
}

// Stop 停止捕获
func (c *AFPacketCapture) Stop() error {
	if !c.running.Load() {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.running.Store(false)
	if c.cancel != nil {
		c.cancel()
	}

	return c.cleanup()
}

// cleanup 清理资源
func (c *AFPacketCapture) cleanup() error {
	// 解除 mmap
	if len(c.ringBuffer) > 0 {
		syscall.Munmap(c.ringBuffer)
		c.ringBuffer = nil
	}

	// 关闭 socket
	if c.socket > 0 {
		syscall.Close(c.socket)
		c.socket = 0
	}

	return nil
}

// IsRunning 是否正在运行
func (c *AFPacketCapture) IsRunning() bool {
	return c.running.Load()
}

// Close 关闭捕获
func (c *AFPacketCapture) Close() error {
	return c.Stop()
}

// htons 主机字节序转网络字节序
func htons(v uint16) int {
	return int((v << 8) | (v >> 8))
}

// setsockoptTpacketReq 设置 tpacket 请求
func setsockoptTpacketReq(fd, level, opt int, req *tpacketReq) error {
	_, _, errno := syscall.Syscall6(
		syscall.SYS_SETSOCKOPT,
		uintptr(fd),
		uintptr(level),
		uintptr(opt),
		uintptr(unsafe.Pointer(req)),
		unsafe.Sizeof(*req),
		0,
	)
	if errno != 0 {
		return errno
	}
	return nil
}
