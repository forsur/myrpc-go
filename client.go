/*

当 Go 返回 Call 实例后，调用方可以开启协程，通过 Done 字段来检查异步调用是否获得了返回结果

传入的 args 和 &reply 这两个结构体分别用来写入 socket 和 承接从 socket 中读出的服务端端响应

client 实例是有状态的，seq 全局递增；一个 client 只在 New Client 时发送一次 option

工作流程：
*** 将发送的请求和接收的响应进行配对由客户端完成
0. send() 通过 registerCall 分配 Seq 并存入 pending map 中
1. receive() 解析出 sever 响应的 header，然后根据 header 得到响应的 Seq
2. 通过 Seq 作为 key 从 pending 这个 map 中找到对应的请求 Call，即根据 map 判断写到谁的 &reply 中
3. Call 的结构体里面就有 reply 的结构，然后直接使用 Codec 解码读数据到 这个 call.reply 中，
4. 最后通知对应的 Call 处理完成

*/

package myrpc

import (
	"MyRPC/codec"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

// 支持异步调用，当调用结束时，会调用 call.done() 通知调用方
type Call struct {
	Seq           uint64
	ServiceMethod string
	Args          interface{}
	Reply         interface{}
	Error         error
	Done          chan *Call // 用于接受 receive 拿到的返回
}

func (call *Call) done() {
	call.Done <- call
}

type Client struct {
	cc      codec.Codec
	opt     *Option
	sending sync.Mutex
	// header codec.Header
	mu       sync.Mutex
	seq      uint64           // 相当于作用域为整个 client 的全局变量，用于分配
	pending  map[uint64]*Call // 存储未处理完的请求，key 为编号
	closing  bool             // user 调用了 Close 方法
	shutdown bool             // 置为 true 时表示有错误发生
}

var _ io.Closer = (*Client)(nil) // 通过指向 Client 类型的空指针进行接口实现检查

var ErrShutDown = errors.New("connection is shut down")

// 实现 io.Closer 接口的 close() 方法
// io.Closer 是一个接口，只定义了方法是签名，不提供任何实现
func (client *Client) Close() error {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.closing {
		return ErrShutDown
	}
	client.closing = true
	err := client.cc.Close()
	return err
}

func (client *Client) IsAvailable() bool {
	client.mu.Lock()
	defer client.mu.Unlock()
	return !client.shutdown && !client.closing
}

// 将请求添加到 client.pending 中
func (client *Client) registerCall(call *Call) (uint64, error) {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.closing || client.shutdown {
		return 0, ErrShutDown
	}
	call.Seq = client.seq
	client.pending[call.Seq] = call
	client.seq++
	return call.Seq, nil
}

func (client *Client) removeCall(seq uint64) *Call {
	client.mu.Lock()
	defer client.mu.Unlock()
	call := client.pending[seq]
	delete(client.pending, seq)
	return call
}

// 服务端或客户端发生错误时调用，将 shutdown 设置为 true，将错误信息放到所有 pending 的 call 中
func (client *Client) terminateCalls(err error) {
	client.sending.Lock()
	defer client.sending.Unlock()
	client.mu.Lock()
	defer client.mu.Unlock() // FIFO，先执行
	client.shutdown = true
	for _, call := range client.pending {
		call.Error = err
		call.done()
	}
}

// 新建客户端时开启的一个协程，顺序读取响应，将处理完的 call 放到 Done 这个 chann 中
func (client *Client) receive() {
	var err error
	for err == nil {
		// 读取响应的核心：cc 的 ReadHeader / ReadBody 方法，也就是 glob 的 decode
		var H codec.Header
		if err = client.cc.ReadHeader(&H); err != nil {
			break
		}
		call := client.removeCall(H.Seq)
		switch {
		case call == nil:
			err = client.cc.ReadBody(nil)
		case H.Error != "":
			call.Error = fmt.Errorf("%s", H.Error)
			err = client.cc.ReadBody(nil)
			call.done()
		default:
			err = client.cc.ReadBody(call.Reply)
			if err != nil {
				call.Error = errors.New("reading body " + err.Error())
			}
			call.done() // 通知异步调用方已处理完调用返回值
		}
	}
	client.terminateCalls(err)
}

// 返回 client 实例的同时，启动 receive() 方法
func NewClient(conn net.Conn, opt *Option) (*Client, error) {
	f := codec.NewCodecFuncMap[opt.CodecType]
	if f == nil {
		err := fmt.Errorf("undefined codec type")
		log.Println("rpc client: option undefined error: ", err)
		_ = conn.Close()
		return nil, err
	}

	err := json.NewEncoder(conn).Encode(opt)
	if err != nil {
		log.Println("rpc client: option decode error:", err)
		_ = conn.Close()
		return nil, err
	}
	return newClientCodec(f(conn), opt), nil
}

func newClientCodec(cc codec.Codec, opt *Option) *Client {
	client := &Client{
		seq:     1,
		cc:      cc,
		opt:     opt,
		pending: make(map[uint64]*Call),
	}
	go client.receive()
	return client // 协程的启动不会因为函数的返回而终止，而是在后台执行
}

func parseOptions(opts ...*Option) (*Option, error) { // 可变参数，函数可以接收任意数量的 *Option
	if len(opts) == 0 || opts[0] == nil {
		return DefaultOption, nil
	}
	if len(opts) != 1 {
		return nil, errors.New("number of option > 1")
	}
	opt := opts[0]
	opt.MagicNumber = DefaultOption.MagicNumber
	if opt.CodecType == "" {
		opt.CodecType = DefaultOption.CodecType
	}
	return opt, nil
}

type clientResult struct {
	client *Client
	err    error
}

// 为 NewClient() 创建一个对应的类型，用于后面定义函数的参数类型
type newClientFunc func(conn net.Conn, opt *Option) (client *Client, err error)

func dialWithTimeout(f newClientFunc, network, address string, opts ...*Option) (client *Client, err error) {
	opt, err := parseOptions(opts...)
	if err != nil {
		return nil, err
	}

	/*
		使用 net 包的 DialTimeout 方法，获取连接 net.Conn
		network: 指定网络协议，如 tcp / udp / unix 等
		address: 形如 host:port
		timeout: 如果超时时间内没有成功连接，返回 error
		返回一个字节流/数据报的原始连接，如果需要支持应用层协议，如 HTTP 协议可以使用 net/http 库处理
		阻塞直到连接成功
	*/
	conn, err := net.DialTimeout(network, address, opt.ConnectionTimeout)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err != nil {
			_ = conn.Close()
		}
	}()
	ch := make(chan clientResult)
	// 使用闭包，开启协程异步创建一个 client，通过 chan 拿到返回内容
	// 防止 NewClient 执行超时
	go func() {
		client, err := f(conn, opt) /* 关键：创建一个 client 结构体的实例 */
		ch <- clientResult{client: client, err: err}
	}()

	if opt.ConnectionTimeout == 0 { // 不设置超时时间
		result := <-ch // 阻塞等待创建完成
		return result.client, result.err
	}

	select {
	case <-time.After(opt.ConnectionTimeout):
		return nil, fmt.Errorf("client: connect timeout")
	case result := <-ch:
		return result.client, result.err
	}
}

func Dial(network, address string, opts ...*Option) (client *Client, err error) {
	return dialWithTimeout(NewClient, network, address, opts...)
}

func (client *Client) send(call *Call) {
	client.sending.Lock()
	defer client.sending.Unlock()

	seq, err := client.registerCall(call)
	if err != nil {
		call.Error = err
		call.done()
		return
	}

	// client.header.ServiceMethod = call.ServiceMethod
	// client.header.Seq = seq // 这里可以保证每个发向服务端的请求都有一一对应的字段标识
	// client.header.Error = ""

	// if err := client.cc.Write(&client.header, call.Args); err != nil {

	header := codec.Header{
		ServiceMethod: call.ServiceMethod,
		Seq:           seq,
		Error:         "",
	}
	err = client.cc.Write(&header, call.Args)
	if err != nil { // 这里的 Write 要防止数据竞争
		call := client.removeCall(seq)
		if call != nil {
			call.Error = err
			call.done()
		}
	}
}

// 暴露给框架使用者的接口
// 同步和异步的区别：监听 Call.Done 这个 channel 的工作是交给框架的 client 来做还是交给用户自己做

// 异步：传入一个 channel，在 send 之后直接返回，等 receive() 协程异步写入 call 的 Reply
func (client *Client) Go(serviceMethod string, args, reply interface{}, done chan *Call) *Call {
	if done == nil {
		done = make(chan *Call, 10) // 允许在没有立即消费的情况下存储一定数量的值
	} else if cap(done) == 0 {
		log.Panic("rpc client: done channel is unbuffered")
	}
	call := &Call{
		ServiceMethod: serviceMethod,
		Args:          args,
		Reply:         reply,
		Done:          done,
	}
	client.send(call)
	return call
}

// 同步
func (client *Client) Call(ctx context.Context, serviceMethod string, args, reply interface{}) error {
	call := client.Go(serviceMethod, args, reply, make(chan *Call, 1))
	select {
	case <-ctx.Done():
		client.removeCall(call.Seq)
		return errors.New("client: Call timeout" + ctx.Err().Error())
	case result := <-call.Done:
		return result.Error
	}
}
