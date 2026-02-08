# 高性能终端连接功能

## 概述

`gmssh` 提供了高性能的 SSH 终端连接功能，支持双层跳板机架构（如 HK -> 网关 -> 内网主机），并针对高并发和低延迟场景进行了优化。

## 核心组件

### 1. 自适应缓冲区 (Adaptive Buffer)

根据数据传输速率动态调整缓冲区大小，优化吞吐量。

```go
buffer := terminal.NewAdaptiveBuffer()

// 使用当前缓冲区大小进行 I/O
readSize := buffer.GetReadBuffer()  // 默认 32KB，可根据负载调整
writeSize := buffer.GetWriteBuffer() // 默认 64KB

// 记录处理的字节数，触发自适应调整
buffer.RecordBytes(n)
```

**优化策略：**
- 高吞吐量 (>10MB/s)：增大缓冲区至 256KB，减少系统调用
- 中等吞吐量 (1-10MB/s)：适度增大缓冲区
- 低吞吐量 (<100KB/s)：减小缓冲区降低延迟

### 2. SSH 连接池 (Connection Pool)

复用 SSH 连接，避免频繁的握手开销。

```go
config := terminal.DefaultPoolConfig()
config.MaxConnsPerHop = 10      // 每跳最大连接数
config.MaxIdleConnsPerHop = 3   // 最大空闲连接数
config.MaxIdleTime = 5 * time.Minute
config.MaxLifetime = 30 * time.Minute

pool := terminal.NewPool(config)
defer pool.Close()

// 获取会话
session, err := pool.NewSession(hops)
if err != nil {
    log.Fatal(err)
}
defer session.Close()
```

### 3. 批量写入器 (Batched Writer)

累积小数据包批量发送，减少系统调用和网络开销。

```go
batchWriter := terminal.NewBatchedWriter(
    func(data []byte) error {
        return ws.WriteMessage(websocket.TextMessage, data)
    },
    64*1024,              // 最大批量大小
    5*time.Millisecond,   // 最大延迟
)
defer batchWriter.Close()

// 写入数据（自动批量处理）
batchWriter.Write(data)
```

### 4. 零拷贝转发 (Zero-Copy Forwarding)

使用缓冲池减少内存分配，提高数据传输效率。

```go
// 使用连接包装器自动统计流量
wrapper := terminal.NewConnectionWrapper(conn, &stats)

// 或者使用零拷贝管道
pipe := terminal.NewZeroCopyPipe(conn1, conn2)
pipe.Start() // 双向转发
```

### 5. 速率限制 (Rate Limiting)

可选的流控功能，防止网络拥塞。

```go
// 限制 100KB/s，突发 10KB
limiter := terminal.NewRateLimiter(100*1024, 10*1024)

// 检查是否允许传输
if limiter.Allow(1024) {
    // 传输数据
}

// 或者等待直到允许
ctx, cancel := context.WithTimeout(context.Background(), time.Second)
defer cancel()
limiter.Wait(ctx, 1024)
```

## 双层跳板机架构

支持本地 -> HK -> 网关 -> 内网 的连接链：

```go
hops := []*types.Hop{
    // 第一层：HK 加速节点
    {
        Name:     "hk-jump",
        Host:     "hk.example.com",
        Port:     22,
        User:     "admin",
        AuthType: types.AuthKey,
        KeyPath:  "~/.ssh/id_rsa",
    },
    // 第二层：网关
    {
        Name:     "gateway",
        Host:     "gateway.example.com",
        Port:     22,
        User:     "admin",
        AuthType: types.AuthKey,
        KeyPath:  "~/.ssh/id_rsa",
    },
    // 目标：内网服务器
    {
        Name:     "internal-server",
        Host:     "10.0.0.16",
        Port:     22,
        User:     "root",
        AuthType: types.AuthKey,
        KeyPath:  "~/.ssh/id_rsa",
    },
}
```

## 会话管理器

集成所有组件的高性能会话管理：

```go
// 创建管理器
managerConfig := terminal.DefaultManagerConfig()
manager, err := terminal.NewManager(config, managerConfig)
if err != nil {
    log.Fatal(err)
}
defer manager.Close()

// WebSocket 终端端点
http.HandleFunc("/terminal", manager.HandleTerminal)

// API 端点
http.Handle("/api/", manager.APIHandler())
```

### API 端点

- `GET /api/sessions` - 列出所有活跃会话
- `GET /api/stats` - 获取统计信息
- `POST /api/sessions/close?id=<session_id>` - 关闭指定会话

## 性能基准

```
BenchmarkAdaptiveBuffer_RecordBytes-10    4222419    286.8 ns/op    0 B/op    0 allocs/op
BenchmarkBatchedWriter_Write-10          75338830     15.76 ns/op   14 B/op   0 allocs/op
BenchmarkRateLimiter_Allow-10             6334040    190.7 ns/op    0 B/op    0 allocs/op
BenchmarkZeroCopyPipe_Copy-10              168974    7290 ns/op     8990.20 MB/s
BenchmarkPoolStats_Concurrent-10          8966824    136.4 ns/op    0 B/op    0 allocs/op
```

## 最佳实践

1. **连接池配置**：根据并发连接数调整 `MaxConnsPerHop`，避免连接过多导致服务器拒绝

2. **缓冲区大小**：使用自适应缓冲区，让系统自动调整

3. **批量写入**：对于高频小数据写入，启用批量处理（5-10ms 延迟）

4. **错误处理**：始终检查连接错误，让连接池自动清理失效连接

5. **资源清理**：使用 `defer` 确保会话和池正确关闭

## 完整示例

参见 `internal/terminal/example_test.go` 获取完整使用示例。
