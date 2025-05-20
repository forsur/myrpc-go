package xclient

import (
	"errors"
	"log"
	"math"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"
)

type SelectMode int

const (
	RandomSelect SelectMode = iota // 0
	RoundRobinSelect
)

type Discovery interface {
	Refresh() error
	Get(mod SelectMode) (string, error)
	GetAll() ([]string, error)
}

type DiscoveryClientCache struct {
	r       *rand.Rand
	mu      sync.Mutex
	servers []string
	index   int
}

// 在没有 registry center 情况下多个服务的服务发现
func NewMultiServerDiscovery(servers []string) *DiscoveryClientCache {
	ret := &DiscoveryClientCache{
		servers: servers,
		r:       rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	ret.index = ret.r.Intn(math.MaxInt32 - 1)
	return ret
}

func (d *DiscoveryClientCache) GetFromCache(mode SelectMode) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	n := len(d.servers)
	if n == 0 {
		return "", errors.New("discovery: no server available")
	}

	switch mode {
	case RandomSelect:
		return d.servers[d.r.Intn(n)], nil
	case RoundRobinSelect:
		s := d.servers[d.index%n]
		d.index = (d.index + 1) % n
		return s, nil
	default:
		return "", errors.New("discover: mode not support")
	}
}

// 返回可被发现的所有 servers
func (d *DiscoveryClientCache) GetAllFromCache() ([]string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	// 注意，这里需要返回的是副本
	ret := make([]string, len(d.servers))
	copy(ret, d.servers)
	return ret, nil
}

// 发现中心通过 DiscoveryClientCache 维护一个可用服务的列表，并提供通过 http 请求进行更新的功能
type DiscoveryCenter struct {
	*DiscoveryClientCache
	registryAddr string        // 表示注册中心
	timeout      time.Duration // 服务列表过期时间
	lastUpdate   time.Time     // 最后从注册中心更新服务列表的时间
}

const defaultUpdateTimeout = time.Second * 10

func NewDiscoveryCenter(registerAddr string, timeout time.Duration) *DiscoveryCenter {
	if timeout == 0 {
		timeout = defaultUpdateTimeout
	}
	d := &DiscoveryCenter{
		DiscoveryClientCache: NewMultiServerDiscovery(make([]string, 0)),
		registryAddr:         registerAddr,
		timeout:              timeout,
	}

	return d
}

func (d *DiscoveryCenter) Refresh() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	// 两次更新间隔小于 timeout，不需要重新拉取
	if d.lastUpdate.Add(d.timeout).After(time.Now()) {
		return nil
	}
	log.Println("discovery: refresh servers from registry:", d.registryAddr)

	// 发送一个 Get 请求到注册中心
	rsp, err := http.Get(d.registryAddr)
	if err != nil {
		log.Println("discovery: refresh: get from registry error", err)
		return err
	}

	// 接收注册中心的响应
	servers := strings.Split(rsp.Header.Get("X-rpc-servers"), ",")
	d.servers = make([]string, 0)
	for _, server := range servers {
		if strings.TrimSpace(server) != "" {
			d.servers = append(d.servers, strings.TrimSpace(server))
		}
	}
	d.lastUpdate = time.Now()
	return nil
}

// 注册中心对客户端提供的工具方法
func (d *DiscoveryCenter) Get(mode SelectMode) (string, error) {
	err := d.Refresh()
	if err != nil {
		return "", err
	}
	return d.DiscoveryClientCache.GetFromCache(mode)
}

func (d *DiscoveryCenter) GetAll() ([]string, error) {
	if err := d.Refresh(); err != nil {
		return nil, err
	}
	return d.DiscoveryClientCache.GetAllFromCache()
}
