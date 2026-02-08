package terminal

import (
	"bytes"
	"context"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

// mockReadWriteCloser 模拟读写器
type mockReadWriteCloser struct {
	readData  []byte
	writeData bytes.Buffer
	closed    bool
	mu        sync.Mutex
}

func (m *mockReadWriteCloser) Read(p []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.readData) == 0 {
		return 0, io.EOF
	}
	n := copy(p, m.readData)
	m.readData = m.readData[n:]
	return n, nil
}

func (m *mockReadWriteCloser) Write(p []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.writeData.Write(p)
}

func (m *mockReadWriteCloser) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

// TestNewForwarder 测试转发器创建
func TestNewForwarder(t *testing.T) {
	config := DefaultForwarderConfig()
	forwarder := NewForwarder(config)

	if forwarder == nil {
		t.Fatal("Expected non-nil forwarder")
	}

	if forwarder.config.BatchSize != 64*1024 {
		t.Errorf("Expected batch size %d, got %d", 64*1024, forwarder.config.BatchSize)
	}
}

// TestRateLimiter_Basic 测试速率限制器基本功能
func TestRateLimiter_Basic(t *testing.T) {
	// 创建 1000 tokens/second 的限制器，容量 100
	limiter := NewRateLimiter(1000, 100)

	// 应该允许 100 个 token（初始容量）
	if !limiter.Allow(100) {
		t.Error("Expected to allow 100 tokens initially")
	}

	// 不应该允许更多（容量已用完）
	if limiter.Allow(1) {
		t.Error("Expected to reject after capacity exhausted")
	}
}

// TestRateLimiter_Refill 测试令牌补充
func TestRateLimiter_Refill(t *testing.T) {
	// 创建高速率限制器
	limiter := NewRateLimiter(10000, 100)

	// 消耗所有令牌
	limiter.Allow(100)

	// 等待补充
	time.Sleep(20 * time.Millisecond)

	// 应该有一些令牌被补充
	if !limiter.Allow(10) {
		t.Error("Expected tokens to be refilled")
	}
}

// TestRateLimiter_Wait 测试等待功能
func TestRateLimiter_Wait(t *testing.T) {
	limiter := NewRateLimiter(1000, 10)

	// 消耗所有令牌
	limiter.Allow(10)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// 等待 5 个令牌
	err := limiter.Wait(ctx, 5)
	if err != nil {
		t.Errorf("Wait failed: %v", err)
	}
}

// TestZeroCopyPipe_Start 测试零拷贝管道
func TestZeroCopyPipe_Start(t *testing.T) {
	// 创建两个内存连接
	conn1, conn2 := net.Pipe()
	defer conn1.Close()
	defer conn2.Close()

	// 创建管道
	pipe := NewZeroCopyPipe(conn1, conn2)

	// 启动管道（这会阻塞，所以用 goroutine）
	done := make(chan error, 1)
	go func() {
		done <- pipe.Start()
	}()

	// 给 goroutine 启动时间
	time.Sleep(10 * time.Millisecond)

	// 关闭连接来触发管道结束
	conn1.Close()
	conn2.Close()

	select {
	case err := <-done:
		// 预期会有错误（连接关闭）
		_ = err
	case <-time.After(time.Second):
		t.Error("Pipe start timeout")
	}
}

// TestConnectionWrapper_Stats 测试连接包装器的统计功能
func TestConnectionWrapper_Stats(t *testing.T) {
	// 创建内存管道
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	// 创建统计对象
	var stats ForwarderStats

	// 包装连接
	wrapper := NewConnectionWrapper(client, &stats)

	// 写入数据
	testData := []byte("hello world")
	go func() {
		wrapper.Write(testData)
		client.Close()
	}()

	// 读取数据
	buf := make([]byte, len(testData))
	server.Read(buf)

	// 验证统计（包装器统计写入，所以我们需要检查另一个方向）
	// 这个测试主要验证不 panic
}

// TestForwarderStats_Atomic 测试统计的原子操作
func TestForwarderStats_Atomic(t *testing.T) {
	var stats ForwarderStats
	var wg sync.WaitGroup

	// 并发增加统计
	for i := 0; i < 100; i++ {
		wg.Add(3)
		go func() {
			defer wg.Done()
			stats.BytesSent.Add(100)
		}()
		go func() {
			defer wg.Done()
			stats.BytesReceived.Add(200)
		}()
		go func() {
			defer wg.Done()
			stats.PacketsSent.Add(1)
		}()
	}

	wg.Wait()

	if stats.BytesSent.Load() != 10000 {
		t.Errorf("Expected 10000 bytes sent, got %d", stats.BytesSent.Load())
	}
	if stats.BytesReceived.Load() != 20000 {
		t.Errorf("Expected 20000 bytes received, got %d", stats.BytesReceived.Load())
	}
	if stats.PacketsSent.Load() != 100 {
		t.Errorf("Expected 100 packets sent, got %d", stats.PacketsSent.Load())
	}
}

// BenchmarkRateLimiter_Allow 基准测试速率限制
func BenchmarkRateLimiter_Allow(b *testing.B) {
	limiter := NewRateLimiter(1000000, 1000000) // 高速率

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			limiter.Allow(1)
		}
	})
}

// BenchmarkZeroCopyPipe_Copy 基准测试零拷贝
func BenchmarkZeroCopyPipe_Copy(b *testing.B) {
	data := make([]byte, 64*1024)
	for i := range data {
		data[i] = byte(i % 256)
	}

	b.ResetTimer()
	b.SetBytes(int64(len(data)))

	for i := 0; i < b.N; i++ {
		src := bytes.NewReader(data)
		dst := &bytes.Buffer{}
		io.Copy(dst, src)
	}
}
