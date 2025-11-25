package transport

import (
	"time"
)

// Message 消息结构
// 定义了进程间传递的标准消息格式
type Message struct {
	// Header 消息头
	Header *MessageHeader `json:"header"`

	// Payload 消息载荷（实际数据）
	Payload []byte `json:"payload"`

	// Metadata 元数据（可选）
	Metadata map[string]string `json:"metadata,omitempty"`
}

// MessageHeader 消息头
type MessageHeader struct {
	// ID 消息唯一标识符
	ID string `json:"id"`

	// Type 消息类型
	Type string `json:"type"`

	// Source 消息来源（模块名）
	Source string `json:"source"`

	// Destination 消息目标（模块名）
	Destination string `json:"destination"`

	// Timestamp 消息时间戳（纳秒）
	Timestamp int64 `json:"timestamp"`

	// CorrelationID 关联ID（用于请求-响应关联）
	CorrelationID string `json:"correlation_id,omitempty"`

	// ReplyTo 回复地址
	ReplyTo string `json:"reply_to,omitempty"`

	// Priority 消息优先级（0-9，9最高）
	Priority int `json:"priority"`

	// TTL 消息生存时间（毫秒，0表示永不过期）
	TTL int64 `json:"ttl"`

	// ContentType 内容类型
	ContentType string `json:"content_type"`

	// ContentEncoding 内容编码
	ContentEncoding string `json:"content_encoding,omitempty"`

	// Compressed 是否压缩
	Compressed bool `json:"compressed"`

	// Encrypted 是否加密
	Encrypted bool `json:"encrypted"`

	// Version 协议版本
	Version string `json:"version"`
}

// MessageType 消息类型常量
type MessageType string

const (
	// MessageTypePacket 数据包消息（采集层→解析层）
	MessageTypePacket MessageType = "packet"

	// MessageTypeFlow 流记录消息（解析层→存储层/分析层）
	MessageTypeFlow MessageType = "flow"

	// MessageTypeAlert 告警消息（分析层→其他）
	MessageTypeAlert MessageType = "alert"

	// MessageTypeStats 统计信息消息
	MessageTypeStats MessageType = "stats"

	// MessageTypeCommand 命令消息（控制指令）
	MessageTypeCommand MessageType = "command"

	// MessageTypeResponse 响应消息
	MessageTypeResponse MessageType = "response"

	// MessageTypeHeartbeat 心跳消息
	MessageTypeHeartbeat MessageType = "heartbeat"

	// MessageTypeEvent 事件消息
	MessageTypeEvent MessageType = "event"
)

// ContentType 内容类型常量
type ContentType string

const (
	// ContentTypeJSON JSON格式
	ContentTypeJSON ContentType = "application/json"

	// ContentTypeProtobuf Protocol Buffers格式
	ContentTypeProtobuf ContentType = "application/protobuf"

	// ContentTypeMsgpack MessagePack格式
	ContentTypeMsgpack ContentType = "application/msgpack"

	// ContentTypeBinary 二进制格式
	ContentTypeBinary ContentType = "application/octet-stream"

	// ContentTypeText 文本格式
	ContentTypeText ContentType = "text/plain"
)

// Priority 优先级常量
const (
	PriorityLowest  = 0
	PriorityLow     = 2
	PriorityNormal  = 5
	PriorityHigh    = 7
	PriorityHighest = 9
)

// ProtocolVersion 协议版本
const ProtocolVersion = "1.0"

// NewMessage 创建新消息
func NewMessage(msgType string, payload []byte) *Message {
	return &Message{
		Header: &MessageHeader{
			ID:          generateMessageID(),
			Type:        msgType,
			Timestamp:   time.Now().UnixNano(),
			Priority:    PriorityNormal,
			TTL:         0,
			ContentType: string(ContentTypeJSON),
			Version:     ProtocolVersion,
		},
		Payload:  payload,
		Metadata: make(map[string]string),
	}
}

// NewRequestMessage 创建请求消息
func NewRequestMessage(msgType string, payload []byte, replyTo string) *Message {
	msg := NewMessage(msgType, payload)
	msg.Header.ReplyTo = replyTo
	msg.Header.CorrelationID = generateMessageID()
	return msg
}

// NewResponseMessage 创建响应消息
func NewResponseMessage(request *Message, payload []byte) *Message {
	msg := NewMessage(string(MessageTypeResponse), payload)
	msg.Header.CorrelationID = request.Header.CorrelationID
	msg.Header.Destination = request.Header.Source
	return msg
}

// Clone 克隆消息（深拷贝）
func (m *Message) Clone() *Message {
	if m == nil {
		return nil
	}

	msg := &Message{
		Header: &MessageHeader{
			ID:              m.Header.ID,
			Type:            m.Header.Type,
			Source:          m.Header.Source,
			Destination:     m.Header.Destination,
			Timestamp:       m.Header.Timestamp,
			CorrelationID:   m.Header.CorrelationID,
			ReplyTo:         m.Header.ReplyTo,
			Priority:        m.Header.Priority,
			TTL:             m.Header.TTL,
			ContentType:     m.Header.ContentType,
			ContentEncoding: m.Header.ContentEncoding,
			Compressed:      m.Header.Compressed,
			Encrypted:       m.Header.Encrypted,
			Version:         m.Header.Version,
		},
		Payload:  make([]byte, len(m.Payload)),
		Metadata: make(map[string]string),
	}

	copy(msg.Payload, m.Payload)

	for k, v := range m.Metadata {
		msg.Metadata[k] = v
	}

	return msg
}

// IsExpired 检查消息是否已过期
func (m *Message) IsExpired() bool {
	if m.Header.TTL == 0 {
		return false
	}

	now := time.Now().UnixNano()
	expireTime := m.Header.Timestamp + (m.Header.TTL * 1e6) // 转换为纳秒
	return now > expireTime
}

// Age 获取消息年龄（纳秒）
func (m *Message) Age() int64 {
	return time.Now().UnixNano() - m.Header.Timestamp
}

// Size 获取消息大小（字节）
func (m *Message) Size() int {
	size := len(m.Payload)

	// 估算Header大小（简化计算）
	size += len(m.Header.ID)
	size += len(m.Header.Type)
	size += len(m.Header.Source)
	size += len(m.Header.Destination)
	size += len(m.Header.CorrelationID)
	size += len(m.Header.ReplyTo)
	size += len(m.Header.ContentType)
	size += len(m.Header.ContentEncoding)
	size += len(m.Header.Version)
	size += 64 // 其他字段估算

	// 元数据大小
	for k, v := range m.Metadata {
		size += len(k) + len(v)
	}

	return size
}

// SetMetadata 设置元数据
func (m *Message) SetMetadata(key, value string) {
	if m.Metadata == nil {
		m.Metadata = make(map[string]string)
	}
	m.Metadata[key] = value
}

// GetMetadata 获取元数据
func (m *Message) GetMetadata(key string) (string, bool) {
	if m.Metadata == nil {
		return "", false
	}
	val, ok := m.Metadata[key]
	return val, ok
}

// Validate 验证消息有效性
func (m *Message) Validate() error {
	if m == nil {
		return ErrInvalidMessage
	}

	if m.Header == nil {
		return ErrInvalidMessageHeader
	}

	if m.Header.ID == "" {
		return ErrMissingMessageID
	}

	if m.Header.Type == "" {
		return ErrMissingMessageType
	}

	if m.Header.Version != ProtocolVersion {
		return ErrIncompatibleVersion
	}

	if m.IsExpired() {
		return ErrMessageExpired
	}

	return nil
}

// PacketMessage 数据包消息（采集层→解析层）
type PacketMessage struct {
	// Timestamp 捕获时间戳
	Timestamp int64 `json:"timestamp"`

	// Length 数据包长度
	Length uint32 `json:"length"`

	// CaptureLength 实际捕获长度
	CaptureLength uint32 `json:"capture_length"`

	// InterfaceIndex 网卡索引
	InterfaceIndex int `json:"interface_index"`

	// Data 数据包内容（可能是零拷贝引用）
	Data []byte `json:"data"`

	// Metadata 元数据
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// FlowMessage 流记录消息（解析层→存储层/分析层）
type FlowMessage struct {
	// FlowID 流ID
	FlowID string `json:"flow_id"`

	// SrcIP 源IP
	SrcIP string `json:"src_ip"`

	// DstIP 目标IP
	DstIP string `json:"dst_ip"`

	// SrcPort 源端口
	SrcPort uint16 `json:"src_port"`

	// DstPort 目标端口
	DstPort uint16 `json:"dst_port"`

	// Protocol 协议
	Protocol uint8 `json:"protocol"`

	// FirstSeen 首次看到时间
	FirstSeen int64 `json:"first_seen"`

	// LastSeen 最后看到时间
	LastSeen int64 `json:"last_seen"`

	// PacketCount 数据包数量
	PacketCount uint64 `json:"packet_count"`

	// ByteCount 字节数
	ByteCount uint64 `json:"byte_count"`

	// Features 流特征（60+维）
	Features map[string]interface{} `json:"features,omitempty"`

	// Labels 标签
	Labels []string `json:"labels,omitempty"`
}

// AlertMessage 告警消息
type AlertMessage struct {
	// AlertID 告警ID
	AlertID string `json:"alert_id"`

	// Severity 严重程度（0-10）
	Severity int `json:"severity"`

	// Type 告警类型
	Type string `json:"type"`

	// Description 描述
	Description string `json:"description"`

	// Source 来源
	Source string `json:"source"`

	// Timestamp 时间戳
	Timestamp int64 `json:"timestamp"`

	// Details 详细信息
	Details map[string]interface{} `json:"details,omitempty"`
}

// StatsMessage 统计信息消息
type StatsMessage struct {
	// Module 模块名
	Module string `json:"module"`

	// Timestamp 时间戳
	Timestamp int64 `json:"timestamp"`

	// Metrics 指标
	Metrics map[string]interface{} `json:"metrics"`
}

// CommandMessage 命令消息
type CommandMessage struct {
	// Command 命令名
	Command string `json:"command"`

	// Args 参数
	Args map[string]interface{} `json:"args,omitempty"`

	// Timestamp 时间戳
	Timestamp int64 `json:"timestamp"`
}

// ResponseMessage 响应消息
type ResponseMessage struct {
	// Success 是否成功
	Success bool `json:"success"`

	// Code 响应码
	Code int `json:"code"`

	// Message 消息
	Message string `json:"message"`

	// Data 数据
	Data interface{} `json:"data,omitempty"`

	// Timestamp 时间戳
	Timestamp int64 `json:"timestamp"`
}

// generateMessageID 生成消息ID
func generateMessageID() string {
	// TODO: 实现高效的ID生成算法
	// 可以使用：UUID、Snowflake、时间戳+随机数等
	return time.Now().Format("20060102150405") + "-" + randomString(8)
}

// randomString 生成随机字符串
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
	}
	return string(b)
}
