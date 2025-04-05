/*
只需要协商消息的解码和编码方式，因为 header 和 body 分离了

批量传输：
| Option | Header1 | Body1 | Header2 | Body2 | ...

| Option{MagicNumber: xxx, CodecType: xxx} | Header{ServiceMethod ...} | Body interface{} |
| <------      固定 JSON 编码      ------>  | <-------    编码方式由 CodeType 决定    ------->|

*/

package myrpc

import (
	"MyRPC/codec"
	"encoding/json"
	"io"
	"log"
	"net"
	"reflect"
	"sync"
	"fmt"
)

const MagicNumber = 0x3bef5c

type Option struct {
	MagicNumber int // 标记这是一个 myrpc 的 request
	CodecType codec.Type
}

var DefaultOption = &Option {
	MagicNumber: MagicNumber,
	CodecType: codec.GobType,
}


type Server struct{} 

func NewServer() *Server {
	return &Server{}
}

func (svr *Server) Accept(lis net.Listener) {
	for {
		// socket 是通过 Accept() 得到的
		conn, err := lis.Accept()
		if err != nil {
			log.Println("rpc server: accept error:", err)
			return 
		}

		go svr.WrapConnToCodec(conn)
	}
}

func (svr *Server) WrapConnToCodec(conn io.ReadWriteCloser) {
	defer func() {
		_ = conn.Close()
	}()
	
	// option 字段是使用 json 序列化的
	var opt Option
	if err := json.NewDecoder(conn).Decode(&opt); err != nil {
		log.Println("rpc server: option decode error:", err)
		return
	}
	if opt.MagicNumber != MagicNumber {
		log.Printf("rpc sever: invalid magic number %x", opt.MagicNumber)
		return
	}

	// 拿到 Codec 的构造函数
	f := codec.NewCodecFuncMap[opt.CodecType]
	if f == nil {
		log.Printf("rpc server: invalid codec type")
	}
	svr.serveCodec(f(conn))
}

var invalidRequest = struct{}{} // 出错时的空占位符

func (svr *Server) serveCodec(cc codec.Codec) { // 此时传入的为经过 NewCodecFunc 作用的 codec.Codec 结构体
	sending := new(sync.Mutex)
	wg := new(sync.WaitGroup)
	for {
		req, err := svr.readRequest(cc) 
		if err != nil {
			if req == nil {
				break
			}
			req.h.Error = err.Error()
			svr.sendResponse(cc, req.h, invalidRequest, sending)
			continue
		}
		wg.Add(1)
		go svr.handleRequest(cc, req, sending, wg)
	}

	wg.Wait()
	_ = cc.Close()
}

type request struct {
	h *codec.Header
	argv, replyv reflect.Value // 任意类型
}

// 读请求，gob 编码可以保证读取出的数据为 header + body
func (svr *Server) readRequest(cc codec.Codec) (*request, error) {
	h, err := svr.readRequestHeader(cc)
	if err != nil {
		return nil ,err
	}
	req := &request{h: h}
	// 下面假设 body 为 string
	req.argv = reflect.New(reflect.TypeOf("")) // 空字符串类型的指针
	if err = cc.ReadBody(req.argv.Interface()); err != nil { 
		log.Println("rpc sever: read argv err:", err)
	}
	return req, nil
}

func (svr *Server) readRequestHeader(cc codec.Codec) (*codec.Header, error) {
	var h codec.Header
	if err := cc.ReadHeader(&h); err != nil {
		if err != io.EOF && err != io.ErrUnexpectedEOF {
			log.Println("rpc server, read header error:", err)
		}
		return nil, err
	}
	return &h, nil
}

// 返回 response
// 使用锁保证写入 buf 时不会出现数据竞争
func (svr *Server) sendResponse(cc codec.Codec, h *codec.Header, body interface{}, sending *sync.Mutex) {
	sending.Lock()
	defer sending.Unlock()
	if err := cc.Write(h, body); err != nil {
		log.Println("rpc server: write response error:", err)
	}
}

// 处理 request （本质上也是 sendResponse）
func (svr *Server) handleRequest(cc codec.Codec, req *request, sending *sync.Mutex, wg *sync.WaitGroup) {
	defer wg.Done()
	log.Println(req.h, req.argv.Elem())
	req.replyv = reflect.ValueOf(fmt.Sprintf("myrpc resp %d", req.h.Seq))
	svr.sendResponse(cc, req.h, req.replyv.Interface(), sending)
}

// 方便使用
var DefaultServer = NewServer()
func Accept(lis net.Listener) {
	DefaultServer.Accept(lis)
}