package main

import (
	myrpc "MyRPC"
	"log"
)

const (
	registryURL = "http://127.0.0.1:8088/myrpc/registry"
)

type AddServiceImpl struct{}

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

func main() {
	log.Println("Starting server...")
	ch := make(chan *myrpc.Server)
	go myrpc.NewServer(registryURL, ch)
	server := <-ch
	log.Println("Server received, registering service...")
	server.Register(&AddServiceImpl{})
	log.Println("Service registered, server is running...")

	// 保持服务器运行
	select {}
}
