package main

import (
	"MyRPC/registry"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// 启动注册中心并获取URL
	registryURL := registry.NewRegistry()
	log.Printf("Registry started at: %s", registryURL)

	// 设置信号处理，支持优雅关闭
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 持续运行，直到接收到关闭信号或超时
	select {
	case <-time.After(20 * time.Minute):
		log.Println("Registry timeout after 20 min, shutting down...")
	case sig := <-sigChan:
		log.Printf("Received signal %v, shutting down registry...\n", sig)
	}

	log.Println("Registry shutdown completed")
}
