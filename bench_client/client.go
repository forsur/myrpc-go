package main

import (
	"MyRPC/xclient"
	"context"
	"flag"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

type Args struct {
	Num1 int
	Num2 int
}

type Stats struct {
	totalRequests   int64
	successRequests int64
	errorRequests   int64
	totalLatency    int64 // 微秒
}

func main() {
	var (
		registryAddr = flag.String("registry", "http://127.0.0.1:8088/myrpc/registry", "注册中心地址")
		concurrency  = flag.Int("c", 100, "并发数")
		duration     = flag.Duration("d", 1*time.Second, "测试时长")
		method       = flag.String("m", "AddServiceImpl.Sum", "测试方法")
	)
	flag.Parse()

	log.Printf("开始压测: 并发=%d, 时长=%v, 方法=%s, 使用单个XClient测试3个server进程的性能", *concurrency, *duration, *method)

	// 创建服务发现客户端
	d := xclient.NewDiscoveryCenter(*registryAddr, 0)

	// 等待服务发现完成，确保能发现所有服务
	log.Println("正在发现服务...")
	services, err := d.GetAll()
	if err != nil {
		log.Printf("服务发现失败: %v", err)
	} else {
		log.Printf("发现 %d 个服务实例: %v", len(services), services)
	}

	// 如果没有发现服务，等待一下再试
	if len(services) == 0 {
		time.Sleep(2 * time.Second)
		services, err = d.GetAll()
		if err != nil {
			log.Printf("重新服务发现失败: %v", err)
		} else {
			log.Printf("重新发现 %d 个服务实例: %v", len(services), services)
		}
	}

	stats := &Stats{}
	ctx, cancel := context.WithTimeout(context.Background(), *duration)
	defer cancel()
	var wg sync.WaitGroup
	startTime := time.Now()

	// 启动实时监控协程
	go func() {
		for {
			ticker := time.NewTicker(200 * time.Millisecond) // 每200ms打印一次进度
			defer ticker.Stop()
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				current := atomic.LoadInt64(&stats.totalRequests)
				success := atomic.LoadInt64(&stats.successRequests)
				errors := atomic.LoadInt64(&stats.errorRequests)
				elapsed := time.Since(startTime)

				if current > 0 {
					currentQPS := float64(success) / elapsed.Seconds()
					log.Printf("\r[实时监控] 已发送: %d, 成功: %d, 失败: %d, 当前QPS: %.0f",
						current, success, errors, currentQPS)
				}
			}
		}
	}()
	// 创建一个共享的XClient
	xc := xclient.NewXClient(d, xclient.RandomSelect, nil)
	defer xc.Close()

	// 启动并发测试
	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for {
				select {
				case <-ctx.Done():
					return
				default:
					atomic.AddInt64(&stats.totalRequests, 1)

					reqStart := time.Now()
					var reply int
					args := &Args{Num1: workerID, Num2: 42}

					err := xc.Call(context.Background(), *method, args, &reply)
					reqDuration := time.Since(reqStart)

					if err != nil {
						atomic.AddInt64(&stats.errorRequests, 1)
						if atomic.LoadInt64(&stats.errorRequests) <= 10 {
							log.Printf("Request error: %v", err)
						}
					} else {
						atomic.AddInt64(&stats.successRequests, 1)
						atomic.AddInt64(&stats.totalLatency, reqDuration.Microseconds())
					}
				}
			}
		}(i)
	}
	// 等待所有worker完成
	wg.Wait()
	totalTime := time.Since(startTime)

	// 清理实时监控输出，换行
	fmt.Println() // 打印结果
	fmt.Println("\n==================== 单个XClient性能测试结果 ====================")
	fmt.Printf("测试配置: 并发=%d, 时长=%v, 单个XClient连接3个server\n", *concurrency, *duration)
	fmt.Printf("实际发现服务数: %d\n", len(services))
	fmt.Printf("实际测试时长: %v\n", totalTime)
	fmt.Println("----------------------------------------------------------------")
	fmt.Printf("总请求数: %d\n", stats.totalRequests)
	fmt.Printf("成功请求: %d\n", stats.successRequests)
	fmt.Printf("失败请求: %d\n", stats.errorRequests)

	if stats.successRequests > 0 {
		qps := float64(stats.successRequests) / totalTime.Seconds()
		avgLatency := float64(stats.totalLatency) / float64(stats.successRequests) / 1000.0 // 转换为毫秒

		fmt.Printf("QPS (每秒请求数): %.2f\n", qps)
		fmt.Printf("平均延迟: %.2f ms\n", avgLatency)
		fmt.Printf("成功率: %.2f%%\n", float64(stats.successRequests)/float64(stats.totalRequests)*100)

		// 额外的性能分析
		fmt.Println("----------------------------------------------------------------")
		fmt.Printf("单个XClient 1秒内最大处理能力: %d 请求\n", int64(qps))
		if len(services) > 0 {
			fmt.Printf("平均每个Server处理: %.2f 请求/秒 (假设负载均衡)\n", qps/float64(len(services)))
		}

		// 吞吐量评估
		if qps > 1000 {
			fmt.Println("单XClient吞吐量评级: 优秀 (>1000 QPS)")
		} else if qps > 500 {
			fmt.Println("单XClient吞吐量评级: 良好 (>500 QPS)")
		} else if qps > 100 {
			fmt.Println("单XClient吞吐量评级: 一般 (>100 QPS)")
		} else {
			fmt.Println("单XClient吞吐量评级: 需要优化 (<100 QPS)")
		}
	}
	fmt.Println("================================================================")
}
