package xclient

import (
	"MyRPC"
	"context"
	"io"
	"reflect"
	"sync"
)

type XClient struct {
	d Discovery
	mode SelectMode
	opt *myrpc.Option
	mu sync.Mutex
	clients map[string]*myrpc.Client
}

var _ io.Closer = (*XClient)(nil)

func NewXClient(d Discovery, mode SelectMode, opt *myrpc.Option) *XClient {
	return &XClient{
		d: d,
		mode: mode,
		opt: opt,
		clients: make(map[string]*myrpc.Client),
	}
}

func (xc *XClient) Close() error {
	xc.mu.Lock()
	defer xc.mu.Unlock()
	for key, client := range xc.clients {
		_ = client.Close()
		delete(xc.clients, key)
	}
	return nil
}



func (xc *XClient) dial(rpcAddr string) (*myrpc.Client, error) {
	xc.mu.Lock()
	defer xc.mu.Unlock()
	client := xc.clients[rpcAddr] // map 中添加服务端地址 rpcAddr 和 client 的映射，缓存起来
	if client == nil {
		var err error
		client, err = myrpc.XDial(rpcAddr, xc.opt)
		if err != nil {
			return nil, err
		}
		xc.clients[rpcAddr] = client
	}
	return client, nil
}

func (xc *XClient) callWithAddr(rpcAddr string, ctx context.Context, serviceMethod string, args, reply interface{}) error {
	client, err := xc.dial(rpcAddr)
	if err != nil {
		return err
	}
	return client.Call(ctx, serviceMethod, args, reply)
}

// 客户端对外提供的调用 rpc 接口的 api
// 但是对于同一个地址的所有请求都是通过同一个 Client 来发送和接收的
func (xc *XClient) Call(ctx context.Context, serviceMethod string, args, reply interface{}) error {
	rpcAddr, err := xc.d.Get(xc.mode)
	if err != nil {
		return err
	}
	return xc.callWithAddr(rpcAddr, ctx, serviceMethod, args, reply)
}



func (xc *XClient) Broadcast(ctx context.Context, serviceMethod string, args, reply interface{}) error {
	servers, err := xc.d.GetAll()
	if err != nil {
		return err
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	var e error
	ctx, cancel := context.WithCancel(ctx)
	for _, rpcAddr := range servers {
		wg.Add(1)
		go func(rpcAddr string) {
			defer wg.Done()
			var replyPtr interface{}
			if reply != nil { // 检查 rpc 方法是否有返回值
				replyPtr = reflect.New(reflect.ValueOf(reply).Elem().Type()).Interface()
			}
			err := xc.callWithAddr(rpcAddr, ctx, serviceMethod, args, replyPtr)
			mu.Lock()
			if err != nil && e == nil {
				e = err
				cancel()
			}
			if reply != nil {
				reflect.ValueOf(reply).Elem().Set(reflect.ValueOf(replyPtr).Elem())
			}
			mu.Unlock()
		}(rpcAddr)
	}
	wg.Wait()
	return e
}


