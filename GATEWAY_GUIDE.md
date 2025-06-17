# RPC Gateway 使用指南

## 概述

这是一个基于 HTTP 的 RPC 网关，它可以将 RESTful HTTP 请求转换为 RPC 调用。网关集成了服务发现功能，可以自动发现和负载均衡到可用的 RPC 服务。

## 架构组件

1. **注册中心 (Registry)**: 服务注册与发现中心
2. **RPC 服务器 (Server)**: 提供具体的业务服务
3. **网关 (Gateway)**: HTTP-to-RPC 转换代理
4. **客户端**: 通过 HTTP 调用服务

## 快速开始

### 1. 启动演示环境

```powershell
# 一键启动所有服务
.\start_gateway_demo.ps1
```

### 2. 手动启动各个组件

```powershell
# 1. 启动注册中心
go run registry_app/registry_app.go

# 2. 启动 RPC 服务器
go run server_app/server_app.go

# 3. 启动网关 (注册中心地址 HTTP端口 负载均衡模式)
go run gateway_app/gateway_app.go "http://127.0.0.1:8088/myrpc/registry" "8080" "random"
```

### 3. 测试网关功能

```powershell
# 运行测试脚本
.\test_gateway.ps1
```

或者在浏览器中打开 `gateway/test.html` 进行图形化测试。

## API 接口

### 1. 健康检查

```http
GET /health
```

**响应示例：**
```json
{
  "status": "healthy",
  "available_servers": ["tcp@127.0.0.1:54321"],
  "timestamp": "2025-06-16T10:30:00Z"
}
```

### 2. RPC 调用

```http
POST /rpc/{ServiceName}.{MethodName}
Content-Type: application/json

{request_arguments}
```

**URL 格式：**
- `/rpc/AddServiceImpl.Sum` - 调用 AddServiceImpl 服务的 Sum 方法

**请求示例：**
```http
POST /rpc/AddServiceImpl.Sum
Content-Type: application/json

{
  "Num1": 15,
  "Num2": 25
}
```

**响应示例：**
```json
{
  "success": true,
  "data": 40
}
```

**错误响应示例：**
```json
{
  "success": false,
  "error": "RPC call failed: server: can't find method"
}
```

## 支持的服务方法

当前示例服务器提供以下方法：

### AddServiceImpl.Sum
计算两个数字的和

**参数：**
```json
{
  "Num1": int,
  "Num2": int
}
```

**返回：**
```json
int (Num1 + Num2 的结果)
```

### AddServiceImpl.Sleep
计算两个数字的和（模拟耗时操作）

**参数：**
```json
{
  "Num1": int,
  "Num2": int
}
```

**返回：**
```json
int (Num1 + Num2 的结果)
```

## 负载均衡模式

网关支持以下负载均衡策略：

- `random`: 随机选择可用服务器
- `roundrobin`: 轮询选择可用服务器

## 测试命令

### 使用 curl 测试

```bash
# 健康检查
curl http://localhost:8080/health

# RPC 调用
curl -X POST \
  http://localhost:8080/rpc/AddServiceImpl.Sum \
  -H "Content-Type: application/json" \
  -d '{"Num1": 10, "Num2": 20}'
```

### 使用 PowerShell 测试

```powershell
# 健康检查
Invoke-RestMethod -Uri "http://localhost:8080/health" -Method GET

# RPC 调用
$body = @{Num1=10; Num2=20} | ConvertTo-Json
Invoke-RestMethod -Uri "http://localhost:8080/rpc/AddServiceImpl.Sum" -Method POST -Body $body -ContentType "application/json"
```

## 错误处理

网关会处理以下类型的错误：

1. **连接错误**: 无法连接到 RPC 服务
2. **服务发现错误**: 无法找到可用的服务实例
3. **协议错误**: 请求格式不正确
4. **业务错误**: RPC 方法执行失败
5. **超时错误**: 请求处理超时

所有错误都会以统一的 JSON 格式返回：

```json
{
  "success": false,
  "error": "错误描述信息"
}
```

## 配置说明

### Gateway 配置

- **Registry Address**: 注册中心地址，格式为 `http://host:port/path`
- **HTTP Port**: 网关监听的 HTTP 端口
- **Load Balance Mode**: 负载均衡模式（random/roundrobin）
- **Timeout**: RPC 调用超时时间（默认 30 秒）

### CORS 支持

网关默认启用 CORS 支持，允许跨域请求：

- `Access-Control-Allow-Origin: *`
- `Access-Control-Allow-Methods: POST, OPTIONS`
- `Access-Control-Allow-Headers: Content-Type`

## 故障排除

### 常见问题

1. **端口被占用**
   ```
   Error: listen tcp :8080: bind: Only one usage of each socket address
   ```
   解决：更换端口或停止占用端口的进程

2. **无法连接注册中心**
   ```
   Error: discovery: refresh: get from registry error
   ```
   解决：确保注册中心正在运行且地址正确

3. **找不到服务方法**
   ```
   Error: server: can't find method
   ```
   解决：检查服务名和方法名是否正确，确保服务已注册

4. **RPC 调用超时**
   ```
   Error: client: Call timeout
   ```
   解决：检查服务器是否正常运行，增加超时时间

### 调试模式

启动时查看详细日志：

```powershell
$env:GOMAXPROCS=1
go run gateway_app/gateway_app.go "http://127.0.0.1:8088/myrpc/registry" "8080" "random"
```

## 扩展开发

### 添加新的服务

1. 创建服务实现结构体
2. 实现业务方法（签名：`func (receiver) Method(args Args, reply *Reply) error`）
3. 在服务器中注册服务：`server.Register(&YourService{})`
4. 通过网关调用：`POST /rpc/YourService.Method`

### 自定义负载均衡

在 `xclient/discovery.go` 中添加新的 `SelectMode` 和相应的选择逻辑。

### 添加认证授权

在 `gateway.go` 的 `handleRPCRequest` 方法中添加认证逻辑。
