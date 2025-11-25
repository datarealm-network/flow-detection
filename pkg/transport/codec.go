package transport

// Codec 编解码器接口
// 定义消息的序列化和反序列化方法
type Codec interface {
	// Encode 编码消息
	// msg: 要编码的消息
	// 返回: 编码后的字节数组和error
	Encode(msg *Message) ([]byte, error)

	// Decode 解码消息
	// data: 编码后的字节数组
	// 返回: 解码后的消息和error
	Decode(data []byte) (*Message, error)

	// Name 获取编解码器名称
	// 返回: 名称字符串
	Name() string

	// ContentType 获取内容类型
	// 返回: 内容类型
	ContentType() string
}

// CodecType 编解码器类型
type CodecType string

const (
	// CodecTypeJSON JSON编解码器
	CodecTypeJSON CodecType = "json"

	// CodecTypeProtobuf Protocol Buffers编解码器
	CodecTypeProtobuf CodecType = "protobuf"

	// CodecTypeMsgpack MessagePack编解码器
	CodecTypeMsgpack CodecType = "msgpack"

	// CodecTypeGob Go二进制编解码器
	CodecTypeGob CodecType = "gob"

	// CodecTypeBinary 纯二进制编解码器（零拷贝优化）
	CodecTypeBinary CodecType = "binary"
)

// CodecRegistry 编解码器注册表
var codecRegistry = make(map[CodecType]Codec)

// RegisterCodec 注册编解码器
func RegisterCodec(codecType CodecType, codec Codec) {
	codecRegistry[codecType] = codec
}

// GetCodec 获取编解码器
func GetCodec(codecType CodecType) (Codec, error) {
	codec, ok := codecRegistry[codecType]
	if !ok {
		return nil, ErrCodecNotFound
	}
	return codec, nil
}

// NewCodec 创建编解码器
func NewCodec(codecType CodecType) (Codec, error) {
	switch codecType {
	case CodecTypeJSON:
		return NewJSONCodec(), nil

	case CodecTypeProtobuf:
		// TODO: 实现Protobuf编解码器
		return nil, ErrNotImplemented

	case CodecTypeMsgpack:
		// TODO: 实现MessagePack编解码器
		return nil, ErrNotImplemented

	case CodecTypeGob:
		return NewGobCodec(), nil

	case CodecTypeBinary:
		return NewBinaryCodec(), nil

	default:
		return nil, ErrUnsupportedCodecType
	}
}

// jsonCodec JSON编解码器
type jsonCodec struct{}

// NewJSONCodec 创建JSON编解码器
func NewJSONCodec() Codec {
	return &jsonCodec{}
}

func (c *jsonCodec) Encode(msg *Message) ([]byte, error) {
	// TODO: 实现JSON编码
	return nil, ErrNotImplemented
}

func (c *jsonCodec) Decode(data []byte) (*Message, error) {
	// TODO: 实现JSON解码
	return nil, ErrNotImplemented
}

func (c *jsonCodec) Name() string {
	return string(CodecTypeJSON)
}

func (c *jsonCodec) ContentType() string {
	return string(ContentTypeJSON)
}

// gobCodec Gob编解码器
type gobCodec struct{}

// NewGobCodec 创建Gob编解码器
func NewGobCodec() Codec {
	return &gobCodec{}
}

func (c *gobCodec) Encode(msg *Message) ([]byte, error) {
	// TODO: 实现Gob编码
	return nil, ErrNotImplemented
}

func (c *gobCodec) Decode(data []byte) (*Message, error) {
	// TODO: 实现Gob解码
	return nil, ErrNotImplemented
}

func (c *gobCodec) Name() string {
	return string(CodecTypeGob)
}

func (c *gobCodec) ContentType() string {
	return string(ContentTypeBinary)
}

// binaryCodec 二进制编解码器（零拷贝优化）
type binaryCodec struct{}

// NewBinaryCodec 创建二进制编解码器
func NewBinaryCodec() Codec {
	return &binaryCodec{}
}

func (c *binaryCodec) Encode(msg *Message) ([]byte, error) {
	// TODO: 实现高效的二进制编码（零拷贝）
	// 使用固定格式的二进制协议
	return nil, ErrNotImplemented
}

func (c *binaryCodec) Decode(data []byte) (*Message, error) {
	// TODO: 实现高效的二进制解码（零拷贝）
	return nil, ErrNotImplemented
}

func (c *binaryCodec) Name() string {
	return string(CodecTypeBinary)
}

func (c *binaryCodec) ContentType() string {
	return string(ContentTypeBinary)
}

// CompressionCodec 压缩编解码器装饰器
type CompressionCodec struct {
	codec      Codec
	compressor Compressor
}

// Compressor 压缩器接口
type Compressor interface {
	// Compress 压缩数据
	Compress(data []byte) ([]byte, error)

	// Decompress 解压数据
	Decompress(data []byte) ([]byte, error)

	// Name 获取压缩器名称
	Name() string
}

// NewCompressionCodec 创建压缩编解码器
func NewCompressionCodec(codec Codec, compressor Compressor) Codec {
	return &CompressionCodec{
		codec:      codec,
		compressor: compressor,
	}
}

func (c *CompressionCodec) Encode(msg *Message) ([]byte, error) {
	// 先编码
	data, err := c.codec.Encode(msg)
	if err != nil {
		return nil, err
	}

	// 再压缩
	compressed, err := c.compressor.Compress(data)
	if err != nil {
		return nil, err
	}

	msg.Header.Compressed = true
	msg.Header.ContentEncoding = c.compressor.Name()

	return compressed, nil
}

func (c *CompressionCodec) Decode(data []byte) (*Message, error) {
	// 先解压
	decompressed, err := c.compressor.Decompress(data)
	if err != nil {
		return nil, err
	}

	// 再解码
	return c.codec.Decode(decompressed)
}

func (c *CompressionCodec) Name() string {
	return c.codec.Name() + "+compression"
}

func (c *CompressionCodec) ContentType() string {
	return c.codec.ContentType()
}

// EncryptionCodec 加密编解码器装饰器
type EncryptionCodec struct {
	codec     Codec
	encryptor Encryptor
}

// Encryptor 加密器接口
type Encryptor interface {
	// Encrypt 加密数据
	Encrypt(data []byte) ([]byte, error)

	// Decrypt 解密数据
	Decrypt(data []byte) ([]byte, error)

	// Name 获取加密器名称
	Name() string
}

// NewEncryptionCodec 创建加密编解码器
func NewEncryptionCodec(codec Codec, encryptor Encryptor) Codec {
	return &EncryptionCodec{
		codec:     codec,
		encryptor: encryptor,
	}
}

func (c *EncryptionCodec) Encode(msg *Message) ([]byte, error) {
	// 先编码
	data, err := c.codec.Encode(msg)
	if err != nil {
		return nil, err
	}

	// 再加密
	encrypted, err := c.encryptor.Encrypt(data)
	if err != nil {
		return nil, err
	}

	msg.Header.Encrypted = true

	return encrypted, nil
}

func (c *EncryptionCodec) Decode(data []byte) (*Message, error) {
	// 先解密
	decrypted, err := c.encryptor.Decrypt(data)
	if err != nil {
		return nil, err
	}

	// 再解码
	return c.codec.Decode(decrypted)
}

func (c *EncryptionCodec) Name() string {
	return c.codec.Name() + "+encryption"
}

func (c *EncryptionCodec) ContentType() string {
	return c.codec.ContentType()
}
