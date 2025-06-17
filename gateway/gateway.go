package gateway

import (
	myrpc "MyRPC"
	"MyRPC/codec"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"strings"
	"time"
)

// Gateway 网关结构
type Gateway struct {
	clientProxy *myrpc.Client
	rpcServer   *myrpc.Server
	httpServer  *http.Server
}

// GatewayResponse HTTP 响应结构
type GatewayResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// NewGateway 创建新的网关实例
// httpPort: HTTP 服务监听端口
func NewGateway(rpcServer *myrpc.Server, httpPort string) *Gateway {
	clientProxy, err := myrpc.Dial("tcp", rpcServer.Address)
	if err != nil {
		log.Fatalf("NewGateway: myrpc.Dail error: %v\n", err)
	}

	gateway := &Gateway{
		clientProxy: clientProxy,
		rpcServer:   rpcServer,
	}

	// 创建 HTTP 服务器
	mux := http.NewServeMux()
	mux.HandleFunc("/rpc/", gateway.handleRPCRequest)

	gateway.httpServer = &http.Server{
		Addr:    ":" + httpPort,
		Handler: mux,
	}

	return gateway
}

func (g *Gateway) StartHttpProxy() error {
	log.Printf("Gateway starting on %s", g.httpServer.Addr)
	return g.httpServer.ListenAndServe()
}

func (g *Gateway) Stop() error {
	log.Println("Gateway stopping...")
	if g.clientProxy != nil {
		g.clientProxy.Close()
	}
	return g.httpServer.Shutdown(context.Background())
}

// handleRPCRequest 处理 RPC 请求
// URL 格式: /rpc/{ServiceName}.{MethodName}
// 例如: /rpc/AuthService.Login
func (g *Gateway) handleRPCRequest(w http.ResponseWriter, r *http.Request) {
	// 设置 CORS 头
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")

	// 处理 OPTIONS 请求
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// 一系列读取与校验，拿到 service.method
	// 只允许 POST 请求
	if r.Method != "POST" {
		g.sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// 从 URL 路径中解析 serviceMethod
	path := r.URL.Path
	if !strings.HasPrefix(path, "/rpc/") {
		g.sendErrorResponse(w, "Invalid path format", http.StatusBadRequest)
		return
	}
	serviceMethod := strings.TrimPrefix(path, "/rpc/")
	if serviceMethod == "" {
		g.sendErrorResponse(w, "Service method not specified", http.StatusBadRequest)
		return
	}
	log.Printf("service/method is %s\n", serviceMethod)
	// 验证 serviceMethod 格式 (ServiceName.MethodName)
	if !strings.Contains(serviceMethod, ".") {
		g.sendErrorResponse(w, "Invalid service method format, expected 'ServiceName.MethodName'", http.StatusBadRequest)
		return
	}

	rpcHeader := &codec.Header{
		ServiceMethod: serviceMethod,
		Seq:           0, // http 请求不需要 seqid
		Error:         "",
	}

	req := &myrpc.Request{
		H: rpcHeader,
	}
	var err error
	req.Svc, req.Mtype, err = g.rpcServer.FindService(rpcHeader.ServiceMethod)
	// 基于 server 端 map 中存储的 method 信息拿到参数和返回值信息
	argv := req.Mtype.NewArgv()
	replyv := req.Mtype.NewReplyv()
	argvi := argv.Interface() // 通过 reflect.Value 获取原始值（空）
	if argv.Type().Kind() != reflect.Ptr {
		argvi = argv.Addr().Interface() // 转为指针
	}

	// 读取请求体到正确的类型结构中
	if r.ContentLength > 0 {
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(argvi); err != nil {
			g.sendErrorResponse(w, fmt.Sprintf("Failed to parse request body: %v", err), http.StatusBadRequest)
			return
		}
	}

	// 调用 RPC 服务（使用本地 clientProxy）
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// 使用解析好的参数和返回值类型进行调用
	// 注意：argvi 已经包含了解码后的数据，而且可能是指针类型
	var callArg interface{}
	if argv.Type().Kind() != reflect.Ptr {
		// 如果原始类型不是指针，传递值
		callArg = argv.Interface()
	} else {
		// 如果原始类型是指针，传递指针
		callArg = argvi
	}

	err = g.clientProxy.Call(ctx, serviceMethod, callArg, replyv.Interface())
	if err != nil {
		log.Printf("RPC call failed: %v", err)
		g.sendErrorResponse(w, fmt.Sprintf("RPC call failed: %v", err), http.StatusInternalServerError)
		return
	}

	// 发送成功响应
	g.sendSuccessResponse(w, replyv.Elem().Interface())
}

// sendSuccessResponse 发送成功响应
func (g *Gateway) sendSuccessResponse(w http.ResponseWriter, data interface{}) {
	response := GatewayResponse{
		Success: true,
		Data:    data,
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// sendErrorResponse 发送错误响应
func (g *Gateway) sendErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	response := GatewayResponse{
		Success: false,
		Error:   message,
	}

	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}
