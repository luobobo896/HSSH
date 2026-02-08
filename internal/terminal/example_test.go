package terminal

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/luobobo896/HSSH/pkg/types"
)

// ExampleManager 展示如何使用 Manager
func ExampleManager() {
	// 创建配置
	config := &types.Config{
		Hops: []*types.Hop{
			{
				Name:     "hk-jump",
				Host:     "hk.example.com",
				Port:     22,
				User:     "admin",
				AuthType: types.AuthKey,
				KeyPath:  "~/.ssh/id_rsa",
			},
			{
				Name:     "gateway",
				Host:     "gateway.example.com",
				Port:     22,
				User:     "admin",
				AuthType: types.AuthKey,
				KeyPath:  "~/.ssh/id_rsa",
			},
			{
				Name:     "internal-server",
				Host:     "10.0.0.16",
				Port:     22,
				User:     "root",
				AuthType: types.AuthKey,
				KeyPath:  "~/.ssh/id_rsa",
				Gateway:  "gateway",
			},
		},
	}

	// 创建管理器配置
	managerConfig := DefaultManagerConfig()
	managerConfig.MaxSessions = 50
	managerConfig.SessionTTL = 30 * time.Minute

	// 创建管理器
	manager, err := NewManager(config, managerConfig)
	if err != nil {
		log.Fatalf("Failed to create manager: %v", err)
	}
	defer manager.Close()

	// 设置 HTTP 路由
	mux := http.NewServeMux()

	// WebSocket 终端端点
	mux.HandleFunc("/terminal", manager.HandleTerminal)

	// API 端点
	mux.Handle("/api/", manager.APIHandler())

	// 启动服务器
	log.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

// ExampleSession 展示如何直接使用 Session
func ExampleSession() {
	// 创建 hop 链：HK -> 网关 -> 目标服务器
	hops := []*types.Hop{
		{
			Name:     "hk-jump",
			Host:     "hk.example.com",
			Port:     22,
			User:     "admin",
			AuthType: types.AuthKey,
			KeyPath:  "~/.ssh/id_rsa",
		},
		{
			Name:     "gateway",
			Host:     "gateway.example.com",
			Port:     22,
			User:     "admin",
			AuthType: types.AuthKey,
			KeyPath:  "~/.ssh/id_rsa",
		},
		{
			Name:     "internal-server",
			Host:     "10.0.0.16",
			Port:     22,
			User:     "root",
			AuthType: types.AuthKey,
			KeyPath:  "~/.ssh/id_rsa",
		},
	}

	// 创建连接池
	poolConfig := DefaultPoolConfig()
	poolConfig.MaxConnsPerHop = 10
	poolConfig.MaxIdleConnsPerHop = 3
	pool := NewPool(poolConfig)
	defer pool.Close()

	// 创建会话配置
	sessionConfig := SessionConfig{
		ServerName:   "internal-server",
		Hops:         hops,
		TerminalType: "xterm-256color",
		Cols:         80,
		Rows:         24,
		Pool:         pool,
	}

	// 创建会话
	session := NewSession(sessionConfig)

	// 设置回调
	session.SetOnConnect(func() {
		log.Println("Session connected")
	})
	session.SetOnDisconnect(func() {
		log.Println("Session disconnected")
	})
	session.SetOnError(func(err error) {
		log.Printf("Session error: %v", err)
	})

	// 在 HTTP handler 中使用
	// http.HandleFunc("/terminal", func(w http.ResponseWriter, r *http.Request) {
	//     session.HandleWebSocket(w, r)
	// })

	_ = session
}

// ExampleAdaptiveBuffer 展示自适应缓冲区的使用
func ExampleAdaptiveBuffer() {
	// 创建自适应缓冲区
	buffer := NewAdaptiveBuffer()

	// 模拟数据处理
	for i := 0; i < 1000; i++ {
		// 处理数据块
		dataSize := 8192 // 8KB 数据块
		buffer.RecordBytes(dataSize)

		// 获取当前缓冲区大小
		readSize := buffer.GetReadBuffer()
		writeSize := buffer.GetWriteBuffer()

		// 使用缓冲区大小进行 I/O 操作
		_ = readSize
		_ = writeSize
	}

	// 获取统计信息
	stats := buffer.GetStats()
	log.Printf("Buffer stats: read=%d, write=%d, processed=%d bytes",
		stats.ReadSize, stats.WriteSize, stats.BytesProcessed)
}

// ExampleBatchedWriter 展示批量写入器的使用
func ExampleBatchedWriter() {
	var flushed int
	var flushedData []byte

	// 创建批量写入器：最大 64KB，最大延迟 10ms
	batchWriter := NewBatchedWriter(
		func(data []byte) error {
			flushed += len(data)
			flushedData = append(flushedData, data...)
			return nil
		},
		64*1024,   // 64KB 批量大小
		10*time.Millisecond, // 10ms 延迟
	)
	defer batchWriter.Close()

	// 写入小数据块（会被批量处理）
	for i := 0; i < 100; i++ {
		batchWriter.Write([]byte("small data chunk\n"))
	}

	// 强制刷新
	batchWriter.Flush()

	log.Printf("Flushed %d bytes", flushed)
}

// ExampleRateLimiter 展示速率限制器的使用
func ExampleRateLimiter() {
	// 创建速率限制器：100KB/s，突发 10KB
	limiter := NewRateLimiter(100*1024, 10*1024)

	// 检查是否允许传输 5KB
	if limiter.Allow(5 * 1024) {
		log.Println("Allow 5KB transfer")
	}

	// 等待直到允许传输 1KB
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := limiter.Wait(ctx, 1024); err != nil {
		log.Printf("Rate limit wait error: %v", err)
	} else {
		log.Println("Allow 1KB transfer after wait")
	}
}
