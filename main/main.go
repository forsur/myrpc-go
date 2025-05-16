package main

import (
	"MyRPC"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

func startSvr(addr chan string) {
	l, err := net.Listen("tcp", ":0") // l 是 listener
	if err != nil {
		log.Fatal("network error:", err)
	}
	log.Println("start rpc server on", l.Addr())
	addr <- l.Addr().String() // 将服务端监听的地址发送给主协程
	myrpc.Accept(l)
}

func main() {
	log.SetFlags(0)
	addr := make(chan string) // 具体的地址是需要从协程外传入的，所以需要将 channel 作为函数参数
	go startSvr(addr)
	client, _ := myrpc.Dial("tcp", <- addr) // 连接的同时 NewClient，启动了一个 receive 协程 
	defer func() {
		client.Close()
	}()

	time.Sleep(time.Second)
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			args := fmt.Sprintf("myrpc request no.%d", i)
			var reply string
			if err := client.Go("Service.Sum", args, &reply); err != nil {
				log.Fatal("rpc call error:", err)
			}
			log.Printf("reply to no.%d call: %v\n", i, reply)
		}(i)
	}
	wg.Wait()
}