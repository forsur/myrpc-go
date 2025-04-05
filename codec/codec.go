package codec // 消息编码解码相关

import (
	"io"
)

type Header struct {
	ServiceMethod string // format : "Service.Method"
	Seq uint64 // 客户端请求序列号
	Error string
}

// 抽象出 接口是为了实现不同的 Codec 实例
type Codec interface { // 接口只关心方法是否被实现，允许实现类自定义结构体字段
	io.Closer						
	ReadHeader(*Header) error
	ReadBody(interface{}) error
	Write(*Header, interface{}) error
}

type NewCodecFunc func(io.ReadWriteCloser) Codec // 定义一个函数类型；这是 Codec 的构造函数

type Type string

const (
	GobType Type = "application/gob"
	JsonType Type = "application/json" // 尚未实现
)

var NewCodecFuncMap map[Type]NewCodecFunc

func init() {
	NewCodecFuncMap = make(map[Type]NewCodecFunc)
	NewCodecFuncMap[GobType] = NewGobCodec
}

