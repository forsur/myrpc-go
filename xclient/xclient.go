package xclient

import (
	"context"
	"MyRPC"
	"io"
	"reflect"
	"sync"
)

type XClient struct {
	d Discovery
	mode SelectMode
	opt *myrpc.Option
	mu sync.Mutex
	clients map[string]*myrpc.Client
}

var _ io.Closer = (*XClient)(nil)

func NewXClient(d Discovery)

