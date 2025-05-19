package xclient

import (
	"errors"
	"math"
	"math/rand"
	"sync"
	"time"
)

type SelectMode int

const (
	RandomSelect SelectMode = iota // 0
	RoundRobinSelect
)

var _ Discovery = (*MultiServerDiscovery)(nil) // 指针类型实现了接口

type Discovery interface {
	Refresh() error
	Update(servers []string) error
	Get(mod SelectMode) (string, error)
	GetAll() ([]string, error)
}

type MultiServerDiscovery struct {
	r *rand.Rand
	mu sync.Mutex
	servers []string
	index int
}

// 在没有 registry center 情况下多个服务的服务发现
func NewMultiServerDiscovery(servers []string) *MultiServerDiscovery {
	ret := &MultiServerDiscovery{
		servers: servers,
		r: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	ret.index = ret.r.Intn(math.MaxInt32 - 1) 
	return ret
}

func (d *MultiServerDiscovery) Refresh() error {
	return nil
}


func (d *MultiServerDiscovery) Update(servers []string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.servers = servers
	return nil
}

func (d *MultiServerDiscovery) Get(mode SelectMode) (string, error) {
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
		s := d.servers[d.index % n]
		d.index = (d.index + 1) % n
		return s, nil
	default:
		return "", errors.New("discover: mode not support")
	}
}

// 返回可被发现的所有 servers
func (d *MultiServerDiscovery) GetAll() ([]string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	// 注意，这里需要返回的是副本
	ret := make([]string, len(d.servers))
	copy(ret, d.servers)
	return ret, nil
}

