package main

import (
	myrpc "MyRPC"
	"log"
)

const registryURL = "http://127.0.0.1:8088/myrpc/registry"

type AddServiceImpl struct{}

type Args struct {
	Num1 int
	Num2 int
}

func (s AddServiceImpl) Sum(args Args, reply *int) error {
	*reply = args.Num1 + args.Num2
	return nil
}

func (s AddServiceImpl) Multiply(args Args, reply *int) error {
	*reply = args.Num1 * args.Num2
	return nil
}

func main() {
	ch := make(chan *myrpc.Server)
	go myrpc.NewServer(registryURL, ch)
	server := <-ch

	err := server.Register(&AddServiceImpl{})
	if err != nil {
		log.Fatal("Failed to register service:", err)
	}

	log.Println("Benchmark server started and service registered")

	// 保持服务器运行
	select {}
}
