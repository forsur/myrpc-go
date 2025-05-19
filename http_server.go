package myrpc

import (
	"io"
	"log"
	"net/http"
)

const (
	connected        = "200 Connected to RPC server"
	defaultRPCPath   = "/_myrpc_"
	defaultDebugPath = "/debug/myrpc"
)

/*
package http
// Handle registers the handler for the given pattern
// in the DefaultServeMux.
// The documentation for ServeMux explains how patterns are matched.

func Handle(pattern string, handler Handler) {
	DefaultServeMux.Handle(pattern, handler)
}

// 其中，Handler 接口定义如下：
type Handler interface {
	ServeHTTP(w ResponseWriter, r *Request)
}
*/

/*
CONNECT 和 GET/POST 等并列，用于建立网络连接隧道
它允许客户端通过 HTTP 代理服务器连接到另一个服务器，并直接传输原始数据
*/



// Server 实现了 Handler 接口
func (server *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method != "CONNECT" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = io.WriteString(w, "405 must CONNECT\n")
		return
	}
	conn, _, err := w.(http.Hijacker).Hijack() // 此后使用 TCP 连接，不再受限于 HTTP 的请求 - 响应模式
	if err != nil {
		log.Print("rpc hijacking", req.RemoteAddr, ": ", err.Error())
		return
	}

	_, _ = io.WriteString(conn, "HTTP/1.0" + connected + "\n\n")
	server.ServeConn(conn)
}


func (server *Server) HandleHTTP() {
	http.Handle(defaultRPCPath, server) // 我们的 server 实现了 Handler 接口
}

// 类似于 Accept
// Go的 http.Server 设计为并发处理多个客户端连接
func HandleHTTP() {
	DefaultServer.HandleHTTP()
}