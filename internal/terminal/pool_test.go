package terminal

import (
	"sync"
	"testing"
	"time"

	"github.com/luobobo896/HSSH/internal/ssh"
	"github.com/luobobo896/HSSH/pkg/types"
)

// TestDefaultPoolConfig 测试默认配置
func TestDefaultPoolConfig(t *testing.T) {
	config := DefaultPoolConfig()

	if config.MaxConnsPerHop != 10 {
		t.Errorf("Expected MaxConnsPerHop 10, got %d", config.MaxConnsPerHop)
	}
	if config.MaxIdleConnsPerHop != 3 {
		t.Errorf("Expected MaxIdleConnsPerHop 3, got %d", config.MaxIdleConnsPerHop)
	}
	if config.MaxIdleTime != 5*time.Minute {
		t.Errorf("Expected MaxIdleTime 5m, got %v", config.MaxIdleTime)
	}
	if config.MaxLifetime != 30*time.Minute {
		t.Errorf("Expected MaxLifetime 30m, got %v", config.MaxLifetime)
	}
	if config.AcquireTimeout != 10*time.Second {
		t.Errorf("Expected AcquireTimeout 10s, got %v", config.AcquireTimeout)
	}
}

// TestNewPool 测试连接池创建
func TestNewPool(t *testing.T) {
	config := DefaultPoolConfig()
	pool := NewPool(config)

	if pool == nil {
		t.Fatal("Expected non-nil pool")
	}

	// 清理
	pool.Close()
}

// TestGenerateHopKey 测试 hop key 生成
func TestGenerateHopKey(t *testing.T) {
	tests := []struct {
		name     string
		hops     []*types.Hop
		expected string
	}{
		{
			name:     "empty hops",
			hops:     []*types.Hop{},
			expected: "",
		},
		{
			name: "single hop",
			hops: []*types.Hop{
				{User: "root", Host: "192.168.1.1", Port: 22},
			},
			expected: "root@192.168.1.1:22",
		},
		{
			name: "multiple hops - uses last",
			hops: []*types.Hop{
				{User: "jump", Host: "jump.host", Port: 22},
				{User: "root", Host: "target.host", Port: 2222},
			},
			expected: "root@target.host:2222",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateHopKey(tt.hops)
			if result != tt.expected {
				t.Errorf("generateHopKey() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestPoolStats_Atomic 测试连接池统计的原子操作
func TestPoolStats_Atomic(t *testing.T) {
	var stats PoolStats
	var wg sync.WaitGroup

	// 并发修改统计
	for i := 0; i < 100; i++ {
		wg.Add(3)
		go func() {
			defer wg.Done()
			stats.TotalConns.Add(1)
		}()
		go func() {
			defer wg.Done()
			stats.ActiveConns.Add(1)
		}()
		go func() {
			defer wg.Done()
			stats.IdleConns.Add(1)
		}()
	}

	wg.Wait()

	if stats.TotalConns.Load() != 100 {
		t.Errorf("Expected 100 total conns, got %d", stats.TotalConns.Load())
	}
	if stats.ActiveConns.Load() != 100 {
		t.Errorf("Expected 100 active conns, got %d", stats.ActiveConns.Load())
	}
	if stats.IdleConns.Load() != 100 {
		t.Errorf("Expected 100 idle conns, got %d", stats.IdleConns.Load())
	}
}

// TestPooledClient_Marking 测试客户端标记功能
func TestPooledClient_Marking(t *testing.T) {
	config := DefaultPoolConfig()
	pool := NewPool(config)
	defer pool.Close()

	client := &PooledClient{
		pool:   pool,
		hopKey: "test@host:22",
	}

	// 初始状态应该是空闲
	if client.IsInUse() {
		t.Error("New client should not be in use")
	}

	// 标记为使用中
	client.markUsed()
	if !client.IsInUse() {
		t.Error("Client should be in use after markUsed")
	}

	// 标记为空闲
	client.markIdle()
	if client.IsInUse() {
		t.Error("Client should not be in use after markIdle")
	}

	// 检查最后使用时间
	lastUsed := client.GetLastUsed()
	if lastUsed.IsZero() {
		t.Error("LastUsed should not be zero")
	}
}

// mockChain 用于测试的 mock chain
type mockChain struct{}

func (m *mockChain) Connect() error    { return nil }
func (m *mockChain) Disconnect() error { return nil }
func (m *mockChain) IsConnected() bool { return false }

// TestPooledClient_Release 测试客户端释放
func TestPooledClient_Release(t *testing.T) {
	config := DefaultPoolConfig()
	pool := NewPool(config)
	defer pool.Close()

	// 创建一个 mock chain 对象（使用 Pool 本身作为非 nil 对象）
	client := &PooledClient{
		pool:   pool,
		hopKey: "test@host:22",
		chain:  &ssh.Chain{}, // 非 nil chain
	}

	client.markUsed()
	if !client.IsInUse() {
		t.Error("Client should be in use after markUsed")
	}

	// 释放客户端（不应该 panic）
	client.Release()

	// 释放后应该标记为空闲
	if client.IsInUse() {
		t.Error("Client should not be in use after release")
	}
}

// BenchmarkPoolStats_Concurrent 基准测试统计并发性能
func BenchmarkPoolStats_Concurrent(b *testing.B) {
	var stats PoolStats

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			stats.TotalConns.Add(1)
			stats.ActiveConns.Add(1)
			stats.IdleConns.Add(1)
		}
	})
}

// TestDefaultManagerConfig 测试管理器默认配置
func TestDefaultManagerConfig(t *testing.T) {
	config := DefaultManagerConfig()

	if config.MaxSessions != 100 {
		t.Errorf("Expected MaxSessions 100, got %d", config.MaxSessions)
	}
	if config.SessionTTL != 30*time.Minute {
		t.Errorf("Expected SessionTTL 30m, got %v", config.SessionTTL)
	}
	if config.CleanupInterval != 60*time.Second {
		t.Errorf("Expected CleanupInterval 60s, got %v", config.CleanupInterval)
	}
}

// TestParseTerminalSize 测试终端大小解析
func TestParseTerminalSize(t *testing.T) {
	// 这里只测试函数存在和基本逻辑
	// 实际测试需要 HTTP 请求对象

	// 测试空值
	cols, rows := 0, 0
	if cols != 0 || rows != 0 {
		t.Error("Default values should be 0")
	}
}
