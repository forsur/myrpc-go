/*
只需要协商消息的解码和编码方式，因为 header 和 body 分离了

批量传输：
| Option | Header1 | Body1 | Header2 | Body2 | ...

| Option{MagicNumber: xxx, CodecType: xxx} | Header{ServiceMethod ...} | Body interface{} |
| <------      固定 JSON 编码      ------>  | <-------    编码方式由 CodeType 决定    ------->|

response 的 header 沿用 request 的 header

工作流程：
1. 一个 server 每监听到一个连接，就启动一个协程处理这个连接。
2. 处理连接的协程在每次读出一组数据之后就启动一个子协程，handleRequest
*/

package myrpc

import (
	"MyRPC/codec"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"reflect"
	"strings"
	"sync"
	"time"
)

const MagicNumber = 0x3bef5c

type Option struct {
	MagicNumber int // 标记这是一个 myrpc 的 request
	CodecType   codec.Type

	// 超时处理参数
	ConnectionTimeout time.Duration 
	HandleTimeout time.Duration
}

var DefaultOption = &Option{
	MagicNumber: MagicNumber,
	CodecType:   codec.GobType,

	// 默认超时时间为 10s
	ConnectionTimeout: 10 * time.Second,
}

type Server struct {
	serviceMap sync.Map
}

func NewServer() *Server {
	return &Server{}
}

// 注册服务到 sync.Map 中
func (svr *Server) Register(rcvr interface{}) error {
	s := newService(rcvr) // rcvr 类似于 AuthServiceImpl，是一个绑定了若干 rpc 方法的结构体
	_, isDup := svr.serviceMap.LoadOrStore(s.name, s)
	if isDup {
		return errors.New("server: service already exist" + s.name)
	}
	return nil
}

func (server *Server) findService(serviceMethod string) (svc *service, mtype *methodType, err error) {
	dotIdx := strings.LastIndex(serviceMethod, ".")
	if dotIdx < 0 {
		err = errors.New("server: wrong service.method format")
		return
	}
	serviceName, methodName := serviceMethod[:dotIdx], serviceMethod[dotIdx+1:]
	serviceStruct, ok := server.serviceMap.Load(serviceName)
	if !ok {
		err = errors.New("server: cann't find service")
		return
	}
	svc = serviceStruct.(*service)
	mtype = svc.method[methodName]
	if mtype == nil {
		err = errors.New("server: can't find method")
		return
	}
	return
}

func (svr *Server) Accept(lis net.Listener) {
	// 每轮循环建立一个与新的客户端的连接
	for {
		// socket 通过 Accept() 得到
		conn, err := lis.Accept() // 阻塞等待新的客户端的连接，返回一个新的 conn
		if err != nil {
			log.Println("rpc server: accept error:", err)
			return
		}

		go svr.ServeConn(conn)
	}
}

// 以连接 (conn) 为单位处理请求
func (svr *Server) ServeConn(conn io.ReadWriteCloser) {
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
	svr.serveCodec(f(conn), &opt)
}

var invalidRequest = struct{}{} // 出错时的空占位符

// 一个客户端的可能会连续发送多个请求
// 处理每个客户端请求的主体逻辑，使用与客户端一一对应的 Mutex 保证 response 不会发生并发混乱
func (svr *Server) serveCodec(cc codec.Codec, opt *Option) {
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
		go svr.handleRequest(cc, req, sending, wg, opt.HandleTimeout)
	}

	wg.Wait()
	_ = cc.Close()
}

type request struct {
	h            *codec.Header
	argv, replyv reflect.Value
	mtype 		 *methodType
	svc 		 *service
}

// 最终目标是取得 argv 类型的指针，供 cc.ReadBody() 使用
func (svr *Server) readRequest(cc codec.Codec) (*request, error) {
	h, err := svr.readRequestHeader(cc)
	if err != nil {
		return nil, err
	}
	req := &request{h: h}
	req.svc, req.mtype, err = svr.findService(h.ServiceMethod)
	if err != nil {
		return req, err
	}

	// 基于 server 端 map 中存储的 method 信息拿到参数和返回值信息
	req.argv = req.mtype.newArgv()
	req.replyv = req.mtype.newReplyv()

	argvi := req.argv.Interface() // 通过 reflect.Value 获取原始值（空）
	if req.argv.Type().Kind() != reflect.Ptr {
		argvi = req.argv.Addr().Interface() // 转为指针
	}
	
	err = cc.ReadBody(argvi)
	if err != nil {
		log.Println("server: code.Codec ReadBody() wrong")
		return req, err
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

func (svr *Server) handleRequest(cc codec.Codec, req *request, sending *sync.Mutex, wg *sync.WaitGroup, timeout time.Duration) {
	defer wg.Done()

	called := make(chan struct{})
	sent := make(chan struct{})

	isTimeout := false

	go func() {
		err := req.svc.call(req.mtype, req.argv, req.replyv)
		fmt.Println("svc call finished")
		if isTimeout {
			return
		}
		fmt.Println("isTimeout == false")
		called <- struct{}{}
		if err != nil {
			req.h.Error = err.Error()
			svr.sendResponse(cc, req.h, invalidRequest, sending)
			sent <- struct{}{}
			return
		}
		svr.sendResponse(cc, req.h, req.replyv.Interface(), sending)
		sent <- struct{}{}
	}()

	if timeout == 0 {
		<- called
		<- sent
		return
	}
	select {
	case <-time.After(timeout):
		fmt.Println("detected timeout")
		isTimeout = true
		req.h.Error = "server: call method timeout"
		fmt.Println("sending rsp")
		svr.sendResponse(cc, req.h, invalidRequest, sending)
	case <-called:
		<-sent
	}

	fmt.Println("ready to return")
}

// 将传入的 rsp header 和 rsp body 作为 rsp 写入到 conn
func (svr *Server) sendResponse(cc codec.Codec, h *codec.Header, body interface{}, sending *sync.Mutex) {
	sending.Lock()
	fmt.Printf("called sendResponse,header: %v, body: %v", h, body)
	defer sending.Unlock()
	if err := cc.Write(h, body); err != nil {
		log.Println("rpc server: write response error:", err)
	}
	fmt.Println("sendResponse finished")
}

// 方便使用
var DefaultServer = NewServer()

func Accept(lis net.Listener) {
	DefaultServer.Accept(lis)
}

func Register(rcvr interface{}) error { 
	return DefaultServer.Register(rcvr) 
}
