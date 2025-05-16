/*

初始连接：
首先建立一个普通的TCP连接（物理层连接）
TCP连接本身没有"HTTP连接"或"RPC连接"的概念，它只是一个字节流通道

协议协商：
客户端通过这个TCP连接发送HTTP CONNECT请求
服务器通过HTTP协议理解并响应这个请求
此阶段，TCP连接上传输的是HTTP格式的数据

协议切换：
服务器通过Hijack()获取TCP连接的控制权
服务器发送HTTP 200响应表示协商成功
此后，传输的数据不再使用HTTP格式，而是使用RPC格式

客户端                                           服务器
   |                                              |
   |--- [HTTP CONNECT请求] ------------------->   |
   |                                              |
   |<-- [HTTP 200 Connected响应] --------------   |
   |                                              |
   |--- [RPC请求二进制数据] ------------------->   |  ← 这里开始直接处理二进制流（TCP 连接 conn）
   |                                              |     不再按HTTP协议解析
   |<-- [RPC响应二进制数据] -------------------    |
   |                                              |

整个过程中TCP连接始终是同一个，只是上层协议发生了变化

*/



package myrpc

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
)

func NewHTTPClient(conn net.Conn, opt *Option) (*Client, error) {
	// 使用 http 协议：在建立连接前写入 http 格式的数据，确保客户端可以解析
	// http 协议本质上即一种共识性的规范
	_, _ = io.WriteString(conn, fmt.Sprintf("CONNECT %s HTTP/1.0\n\n", defaultRPCPath))

	rsp, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: "CONNECT"})

	if err == nil && rsp.Status == connected {
		return NewClient(conn, opt)
	} else {
		return nil, err
	}
}

func DialHTTP(network, address string, opts ...*Option) (*Client, error) {
	return dialWithTimeout(NewHTTPClient, network, address, opts...)
}

// 参数 rpcAddr 形如 http@10.0.0.1:8080，tcp@10.0.0.1:8089, unix@tmp/myrpc.sock
func XDial(rpcAddr string, opts ...*Option) (*Client, error) {
	parts := strings.Split(rpcAddr, "@")
	if len(parts) != 2 {
		return nil, fmt.Errorf("client error: wrong rpcAddr")
	}
	protocol, addr := parts[0], parts[1]
	switch protocol {
	case "http":
		return DialHTTP("tcp", addr, opts...)
	default:
		return Dial(protocol, addr, opts...)
	}
}
