package transport

import (
	"encoding/binary"
)

// BinaryCodec 二进制编解码器（零拷贝优化）
// 使用固定格式的二进制编码，避免序列化开销
type BinaryCodec struct{}

// NewBinaryCodec 创建Binary编解码器
func NewBinaryCodec() *BinaryCodec {
	return &BinaryCodec{}
}

// Encode 编码消息
func (c *BinaryCodec) Encode(msg *Message) ([]byte, error) {
	if msg == nil {
		return nil, ErrInvalidMessage
	}

	// 计算总大小
	headerSize := c.getHeaderSize(msg.Header)
	totalSize := headerSize + len(msg.Payload)

	// 分配缓冲区
	buf := make([]byte, totalSize)
	offset := 0

	// 编码头部
	offset = c.encodeHeader(buf, offset, msg.Header)

	// 编码负载
	copy(buf[offset:], msg.Payload)

	return buf, nil
}

// Decode 解码消息
func (c *BinaryCodec) Decode(data []byte) (*Message, error) {
	if len(data) < 4 {
		return nil, ErrInvalidMessage
	}

	offset := 0

	// 解码头部
	header, bytesRead := c.decodeHeader(data, offset)
	if header == nil {
		return nil, ErrInvalidMessage
	}
	offset += bytesRead

	// 解码负载
	payload := make([]byte, len(data)-offset)
	copy(payload, data[offset:])

	return &Message{
		Header:  header,
		Payload: payload,
	}, nil
}

// Name 返回编解码器名称
func (c *BinaryCodec) Name() string {
	return "binary"
}

// ContentType 返回内容类型
func (c *BinaryCodec) ContentType() string {
	return "application/octet-stream"
}

// encodeHeader 编码消息头
func (c *BinaryCodec) encodeHeader(buf []byte, offset int, header *MessageHeader) int {
	// 格式：
	// [4字节] 头部总长度
	// [64字节] ID（固定长度字符串）
	// [2字节] Type长度 + Type内容
	// [2字节] Source长度 + Source内容
	// [2字节] Destination长度 + Destination内容
	// [8字节] Timestamp（UnixNano）
	// [4字节] Priority
	// [8字节] TTL（纳秒）
	// [64字节] CorrelationID
	// [64字节] ReplyTo
	// [2字节] Error长度 + Error内容
	// [4字节] Metadata数量
	// [N字节] Metadata键值对

	startOffset := offset

	// 预留头部长度字段
	offset += 4

	// ID（固定64字节）
	id := []byte(header.ID)
	if len(id) > 64 {
		id = id[:64]
	}
	copy(buf[offset:offset+64], id)
	offset += 64

	// Type
	offset = c.encodeString(buf, offset, string(header.Type))

	// Source
	offset = c.encodeString(buf, offset, header.Source)

	// Destination
	offset = c.encodeString(buf, offset, header.Destination)

	// Timestamp（int64）
	binary.LittleEndian.PutUint64(buf[offset:], uint64(header.Timestamp))
	offset += 8

	// Priority
	binary.LittleEndian.PutUint32(buf[offset:], uint32(header.Priority))
	offset += 4

	// TTL（int64）
	binary.LittleEndian.PutUint64(buf[offset:], uint64(header.TTL))
	offset += 8

	// CorrelationID（固定64字节）
	corrID := []byte(header.CorrelationID)
	if len(corrID) > 64 {
		corrID = corrID[:64]
	}
	copy(buf[offset:offset+64], corrID)
	offset += 64

	// ReplyTo（固定64字节）
	replyTo := []byte(header.ReplyTo)
	if len(replyTo) > 64 {
		replyTo = replyTo[:64]
	}
	copy(buf[offset:offset+64], replyTo)
	offset += 64

	// 写入头部总长度
	headerLen := offset - startOffset
	binary.LittleEndian.PutUint32(buf[startOffset:], uint32(headerLen))

	return offset
}

// decodeHeader 解码消息头
func (c *BinaryCodec) decodeHeader(data []byte, offset int) (*MessageHeader, int) {
	_ = offset // 使用offset

	// 读取头部长度
	if len(data) < offset+4 {
		return nil, 0
	}
	headerLen := binary.LittleEndian.Uint32(data[offset:])
	offset += 4

	header := &MessageHeader{}

	// ID
	if len(data) < offset+64 {
		return nil, 0
	}
	header.ID = string(trimNull(data[offset : offset+64]))
	offset += 64

	// Type
	typeStr, bytesRead := c.decodeString(data, offset)
	if bytesRead == 0 {
		return nil, 0
	}
	header.Type = typeStr
	offset += bytesRead

	// Source
	header.Source, bytesRead = c.decodeString(data, offset)
	if bytesRead == 0 {
		return nil, 0
	}
	offset += bytesRead

	// Destination
	header.Destination, bytesRead = c.decodeString(data, offset)
	if bytesRead == 0 {
		return nil, 0
	}
	offset += bytesRead

	// Timestamp（int64）
	if len(data) < offset+8 {
		return nil, 0
	}
	header.Timestamp = int64(binary.LittleEndian.Uint64(data[offset:]))
	offset += 8

	// Priority
	if len(data) < offset+4 {
		return nil, 0
	}
	header.Priority = int(binary.LittleEndian.Uint32(data[offset:]))
	offset += 4

	// TTL（int64）
	if len(data) < offset+8 {
		return nil, 0
	}
	header.TTL = int64(binary.LittleEndian.Uint64(data[offset:]))
	offset += 8

	// CorrelationID
	if len(data) < offset+64 {
		return nil, 0
	}
	header.CorrelationID = string(trimNull(data[offset : offset+64]))
	offset += 64

	// ReplyTo
	if len(data) < offset+64 {
		return nil, 0
	}
	header.ReplyTo = string(trimNull(data[offset : offset+64]))
	offset += 64

	return header, int(headerLen)
}

// encodeString 编码字符串（2字节长度 + 内容）
func (c *BinaryCodec) encodeString(buf []byte, offset int, str string) int {
	strBytes := []byte(str)
	strLen := len(strBytes)

	if strLen > 65535 {
		strLen = 65535
		strBytes = strBytes[:strLen]
	}

	binary.LittleEndian.PutUint16(buf[offset:], uint16(strLen))
	offset += 2

	copy(buf[offset:], strBytes)
	offset += strLen

	return offset
}

// decodeString 解码字符串
func (c *BinaryCodec) decodeString(data []byte, offset int) (string, int) {
	if len(data) < offset+2 {
		return "", 0
	}

	strLen := binary.LittleEndian.Uint16(data[offset:])
	offset += 2

	if len(data) < offset+int(strLen) {
		return "", 0
	}

	str := string(data[offset : offset+int(strLen)])
	offset += int(strLen)

	return str, 2 + int(strLen)
}

// getHeaderSize 计算头部大小
func (c *BinaryCodec) getHeaderSize(header *MessageHeader) int {
	size := 4 + // 头部长度
		64 + // ID
		2 + len(header.Type) + // Type
		2 + len(header.Source) + // Source
		2 + len(header.Destination) + // Destination
		8 + // Timestamp
		4 + // Priority
		8 + // TTL
		64 + // CorrelationID
		64 // ReplyTo

	return size
}

// trimNull 去除尾部的null字节
func trimNull(b []byte) []byte {
	for i := len(b) - 1; i >= 0; i-- {
		if b[i] != 0 {
			return b[:i+1]
		}
	}
	return b[:0]
}

// 注册Binary编解码器
func init() {
	// Binary编解码器已经通过GetCodec自动可用
}
