package main

import (
	myrpc "MyRPC"
	"MyRPC/registry"
	"MyRPC/xclient"
	"context"
	"log"
	"sync"
	"time"
)

// 服务端实现 Service 实例
type AddServiceImpl int

type Args struct {
	Num1 int
	Num2 int
}

func (s AddServiceImpl) Sum(args Args, reply *int) error {
	*reply = args.Num1 + args.Num2
	return nil
}

func (s AddServiceImpl) Sleep(args Args, reply *int) error {
	// time.Sleep(time.Second * time.Duration(args.Num1))
	*reply = args.Num1 + args.Num2
	return nil
}

func foo(xc *xclient.XClient, ctx context.Context, typ, serviceMethod string, args *Args) {
	var reply int
	var err error
	switch typ {
	case "call":
		err = xc.Call(ctx, serviceMethod, args, &reply)
	case "broadcast":
		err = xc.Broadcast(ctx, serviceMethod, args, &reply)
	}
	if err != nil {
		log.Printf("%s %s error: %v", typ, serviceMethod, err)
	} else {
		log.Printf("%s %s success: %d + %d = %d", typ, serviceMethod, args.Num1, args.Num2, reply)
	}
}

// 客户端调用远程服务的 api
func call(registryAddr string) {
	d := xclient.NewDiscoveryCenter(registryAddr, 0)       // 注册服务进程到注册中心
	xc := xclient.NewXClient(d, xclient.RandomSelect, nil) // 使用 RoundRobin 策略
	defer func() { _ = xc.Close() }()
	// send request & receive response
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			foo(xc, context.Background(), "call", "AddServiceImpl.Sum", &Args{Num1: i, Num2: i * i})
		}(i)
	}
	wg.Wait()
}

func broadcast(registerAddr string) {
	d := xclient.NewDiscoveryCenter(registerAddr, 0)
	xc := xclient.NewXClient(d, xclient.RandomSelect, nil)
	defer func() { _ = xc.Close() }()
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			foo(xc, context.Background(), "broadcast", "AddServiceImpl.Sum", &Args{Num1: i, Num2: i * i})
			// expect 2 - 5 timeout
			ctx, _ := context.WithTimeout(context.Background(), time.Second*2)
			foo(xc, ctx, "broadcast", "AddServiceImpl.Sleep", &Args{Num1: i, Num2: i * i})
		}(i)
	}
	wg.Wait()
}

func main() {
	log.SetFlags(0)
	registryAddr := registry.NewRegistry()

	ch1 := make(chan *myrpc.Server)
	ch2 := make(chan *myrpc.Server)
	go myrpc.NewServer(registryAddr, ch1)
	go myrpc.NewServer(registryAddr, ch2)
	server1 := <-ch1
	server2 := <-ch2

	var asi AddServiceImpl
	server1.Register(&asi)
	server2.Register(&asi)

	time.Sleep(time.Second)
	call(registryAddr)
	broadcast(registryAddr)
}
