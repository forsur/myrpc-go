/* 
Codec 的一个实现类 

Codec 的每个实例被唯一绑定到一个连接
*/

package codec

import (
	"bufio"
	"encoding/gob" // 专门用于将 go 的结构体 / 切片 / Map 等转换为二进制形式（序列化）；以及将二进制形式的数据解码成 go 数据结构（反序列化）
	"io"
	"log"
)

type GobCodec struct {
	conn io.ReadWriteCloser // socket 连接实例，实现了 io.ReadWriteCloser 接口（包含 Read / Write / Close 方法）
	buf *bufio.Writer // 数据不会立刻写入连接，而是先缓存在内存中，减少系统调用次数
	dec *gob.Decoder
	enc *gob.Encoder
}

var _ Codec = (*GobCodec)(nil)

func NewGobCodec(conn io.ReadWriteCloser) Codec {
	buf := bufio.NewWriter(conn)
	return &GobCodec {
		conn: conn,
		buf: buf,
		dec: gob.NewDecoder(conn), // 创建解码器从连接读取数据，从 conn 中读
		enc: gob.NewEncoder(buf), // 创建编码器，写入 buf
	}
}

// 从 conn 读取完整 Header 数据，停在 Header 末尾
func (c *GobCodec) ReadHeader(h *Header) error {
	return c.dec.Decode(h) // 将解码后的内容写到 h 中，这里传入的 h 相当于一个固定形状的容器
}

// body 的 .Type.Kind() 需要是指针类型，因为会将解码的数据写入 body 位置
func (c *GobCodec) ReadBody(body interface{}) error {
	return c.dec.Decode(body)
}

func (c *GobCodec) Write(h *Header, body interface{}) (err error) {
	defer func() {
		_ = c.buf.Flush()
		if err != nil {
			_ = c.Close()
		}
	}()
	
	// 将 header 和 body 编码成二进制数据先后写入 buf 只能够
	if err = c.enc.Encode(h); err != nil {
		log.Println("codec: gob error encoding header:", err)
	}

	if err = c.enc.Encode(body); err != nil {
		log.Println("codec: gob error encoding body:", err)
	}

	return nil
}

func (c *GobCodec) Close() error {
	return c.conn.Close()
}