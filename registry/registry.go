/*

注册中心具备监听请求并发送响应的功能
- 接收来自 server 的保活请求
- 接收来自 discovery 的发现更新请求

*/

package registry

import (
	"log"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	defaultPath = "/myrpc/registry"
	defaultTimeout = time.Minute * 5
)



type Registry struct {
	timeout time.Duration
	mu sync.Mutex // 为下面的 map 服务
	servers map[string]*ServerItem
}

type ServerItem struct {
	Addr string
	startTime time.Time
}


func NewRegistry() string {
	l, _ := net.Listen("tcp", ":8088")
	registryAddr := l.Addr().String()
	registry := New(defaultTimeout)
	registry.HandleHTTP(defaultPath)
	go func() {
		_ = http.Serve(l, nil)
	}()
	go func() { // 定时检测心跳
		ticker := time.Tick(time.Second)
		for range ticker {
			registry.getAliveServers()
		}
	}()
	time.Sleep(time.Second)
	return "http://" + registryAddr + defaultPath
}


func New(timeout time.Duration) *Registry {
	return &Registry{
		servers: make(map[string]*ServerItem),
		timeout: timeout,
	}
}


// *Registry 实现了 http.Handler 接口
// 用于接收 server 的保活心跳
// 当每个 http 请求到来时调用
func (r *Registry) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case "GET": // 请求所有可用服务的列表
		w.Header().Set("X-rpc-servers", strings.Join(r.getAliveServers(), ","))
	case "POST": // 添加服务实例 / 发送心跳
		addr := req.Header.Get("X-rpc-servers")
		if addr == "" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		r.putServer(addr)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (r *Registry) HandleHTTP(registryPath string) {
	http.Handle(registryPath, r) // 路由注册；尚未启动持续监听
}




// 增加注册的进程 / 更新服务进程的启动时间
func (r *Registry) putServer(addr string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.servers[addr]
	if s == nil {
		r.servers[addr] = &ServerItem{Addr: addr, startTime: time.Now()}
	} else {
		s.startTime = time.Now()
	}
}

// 获取所有 alive 的服务进程
func (r *Registry) getAliveServers() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	ret := make([]string, 0)
	for addr, s := range r.servers {
		if r.timeout == 0 || s.startTime.Add(r.timeout).After(time.Now()) {
			ret = append(ret, addr)
		} else {
			delete(r.servers, addr)
		}
	}
	sort.Strings(ret)
	return ret
}








// 为 server 提供，用于 server 定期向 Registry 发送心跳
func Heartbeat(registry, addr string, duration time.Duration) {
	if duration == 0 {
		duration = defaultTimeout - time.Duration(1) * time.Minute // 将 1 转换为 time.Duration 类型
	}
	sendHeartbeat(registry, addr)
	go func() {
		t := time.Tick(duration)
		for _ = range t {
			sendHeartbeat(registry, addr)
		}
	}()
}

// 起一个 http 客户端，发送心跳
func sendHeartbeat(registry, addr string) error {
	log.Println(addr, "sendHeartbeat to", registry)
	httpClient := &http.Client{}
	req, _ := http.NewRequest("POST", registry, nil)
	req.Header.Set("X-rpc-servers", addr)
	_, err := httpClient.Do(req)
	if err != nil {
		log.Println("server: send heartbeat error", err)
		return err
	}
	return nil
}


