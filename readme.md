
1. 用 map 存一个编码解码方式到codec构造函数的映射。
根据从 option 头中 json 解码出来的字段判断具体的解码方式是怎么样的，然后再调用构造函数基于 conn 来拿到一个可以向 socket 中写入和读出的 codec 实现类的实例

1. client 端首先 Dial() 建立连接，然后起一个 Client 实例，实例开启一个 receive 协程；一个 Server 实例每接收到一个 Client 实例的连接，就开启一个协程处理这个 Client 的请求
为了可以接收响应，client 需要把其进程接收响应的内存地址作为参数提供给框架的 client

2. 核心问题：多个协程分别使用同一个客户端并发发送请求，如何保证客户端在接收到批量响应后能够将每个协程的请求与其期望的响应匹配起来

3. 编写了 ServiceImpl 结构体之后，绑定的远程调用方法如何与 sever 端解码的客户端请求方法对应起来
   * 一个 sever 实例首先调用 readRequest() 解码请求并在其中 findService()
   * 随后开启一个协程 handleRequest()

4. 一个 Server 实例通过一个 map 来记录所有创建的服务；一个 service 实例通过一个 map 来记录所有绑定的方法

5. handleRequest 方法中，通过 Atomic 来实现内存可见性，在协程外部修改闭包变量，保证修改结果在协程内可见

6. 类似于 gRPC 的思路，gRPC 使用 HTTP/2 作为传输层，但在连接建立后，通信是基于流的、双向的，而不受限于传统 HTTP 请求-响应模式。