package main

import (
	"encoding/json"
	"fmt"
	"MyRPC"
	"MyRPC/codec"
	"log"
	"net"
	"time"
)

func startSvr(addr chan string) {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatal("network error:", err)
	}
	log.Println("start rpc server on", l.Addr())
	addr <- l.Addr().String()
	myrpc.Accept(l)
}

func main() {
	addr := make(chan string)
	go startSvr(addr)

	// client
	conn, _ := net.Dial("tcp", <-addr)
	defer func() {
		_ = conn.Close()
	}()

	time.Sleep(time.Second)
	_ = json.NewEncoder(conn).Encode(myrpc.DefaultOption) // 构造 option
	
	// 构造一个新的编解码器，并绑定 socket
	cc := codec.NewGobCodec(conn)
	
	// 发送请求 / 接收响应
	for i := 0; i < 5; i++ {
		// 发送
		h := &codec.Header{
			ServiceMethod: "Foo.Sum",
			Seq: uint64(i),
		}
		_ = cc.Write(h, fmt.Sprintf("rpc request %d", h.Seq)) // 发送消息头和消息体

		// 接收
		_ = cc.ReadHeader(h)
		var reply string
		_ = cc.ReadBody(&reply)
		log.Println("reply:", reply)
	}
}