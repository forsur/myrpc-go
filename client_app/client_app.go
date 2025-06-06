package main

import (
	"MyRPC/xclient"
	"context"
	"log"
	"sync"
	"time"
)

type Args struct {
	Num1 int
	Num2 int
}

const (
	registryURL = "http://127.0.0.1:8088/myrpc/registry"
)

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
	time.Sleep(time.Second)
	call(registryURL)
	broadcast(registryURL)
}
