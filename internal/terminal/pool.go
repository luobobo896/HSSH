// Package terminal 提供高性能 SSH 连接池
package terminal

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/luobobo896/HSSH/internal/ssh"
	"github.com/luobobo896/HSSH/pkg/types"
	gossh "golang.org/x/crypto/ssh"
)

// PooledClient 连接池中的客户端封装
type PooledClient struct {
	*ssh.Client
	chain      *ssh.Chain
	pool       *Pool
	createdAt  time.Time
	lastUsedAt atomic.Value
	inUse      atomic.Bool
	hopKey     string
	id         uint64
}

// IsInUse 检查客户端是否正在使用
func (c *PooledClient) IsInUse() bool {
	return c.inUse.Load()
}

// Release 释放客户端回连接池
func (c *PooledClient) Release() {
	if c.pool != nil {
		c.pool.release(c)
	}
}

// GetLastUsed 获取最后使用时间
func (c *PooledClient) GetLastUsed() time.Time {
	v := c.lastUsedAt.Load()
	if v == nil {
		return c.createdAt
	}
	return v.(time.Time)
}

// markUsed 标记为使用中
func (c *PooledClient) markUsed() {
	c.inUse.Store(true)
	c.lastUsedAt.Store(time.Now())
}

// markIdle 标记为空闲
func (c *PooledClient) markIdle() {
	c.inUse.Store(false)
	c.lastUsedAt.Store(time.Now())
}

// Pool 配置
type PoolConfig struct {
	// 每个 hopKey 的最大连接数
	MaxConnsPerHop int
	// 每个 hopKey 的最大空闲连接数
	MaxIdleConnsPerHop int
	// 连接最大空闲时间
	MaxIdleTime time.Duration
	// 连接最大存活时间
	MaxLifetime time.Duration
	// 获取连接超时
	AcquireTimeout time.Duration
}

// DefaultPoolConfig 返回默认连接池配置
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MaxConnsPerHop:     10,
		MaxIdleConnsPerHop: 3,
		MaxIdleTime:        5 * time.Minute,
		MaxLifetime:        30 * time.Minute,
		AcquireTimeout:     10 * time.Second,
	}
}

// Pool SSH 连接池
type Pool struct {
	config PoolConfig

	// 连接存储: hopKey -> []*PooledClient
	mu      sync.RWMutex
	conns   map[string][]*PooledClient
	idleConns map[string][]*PooledClient

	// 统计信息
	stats PoolStats

	// 客户端 ID 生成器
	idCounter atomic.Uint64

	// 清理 goroutine 控制
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// PoolStats 连接池统计
type PoolStats struct {
	TotalConns    atomic.Int64
	ActiveConns   atomic.Int64
	IdleConns     atomic.Int64
	WaitCount     atomic.Int64
	AcquireErrors atomic.Int64
}

// NewPool 创建新的连接池
func NewPool(config PoolConfig) *Pool {
	ctx, cancel := context.WithCancel(context.Background())
	p := &Pool{
		config:    config,
		conns:     make(map[string][]*PooledClient),
		idleConns: make(map[string][]*PooledClient),
		ctx:       ctx,
		cancel:    cancel,
	}

	// 启动后台清理 goroutine
	p.wg.Add(1)
	go p.cleanupLoop()

	return p
}

// Acquire 获取一个连接
// hops: 连接链配置
func (p *Pool) Acquire(hops []*types.Hop) (*PooledClient, error) {
	hopKey := generateHopKey(hops)

	// 先尝试获取空闲连接
	if client := p.tryGetIdle(hopKey); client != nil {
		p.stats.ActiveConns.Add(1)
		p.stats.IdleConns.Add(-1)
		client.markUsed()
		return client, nil
	}

	// 检查是否超过最大连接数
	p.mu.RLock()
	conns := p.conns[hopKey]
	p.mu.RUnlock()

	if len(conns) >= p.config.MaxConnsPerHop {
		// 等待其他连接释放
		p.stats.WaitCount.Add(1)
		return p.waitAndAcquire(hops, hopKey)
	}

	// 创建新连接
	client, err := p.createClient(hops, hopKey)
	if err != nil {
		p.stats.AcquireErrors.Add(1)
		return nil, err
	}

	p.stats.TotalConns.Add(1)
	p.stats.ActiveConns.Add(1)
	return client, nil
}

// tryGetIdle 尝试获取空闲连接
func (p *Pool) tryGetIdle(hopKey string) *PooledClient {
	p.mu.Lock()
	defer p.mu.Unlock()

	idleList := p.idleConns[hopKey]
	for i, client := range idleList {
		if !client.inUse.Load() {
			// 从 idle 列表移除
			p.idleConns[hopKey] = append(idleList[:i], idleList[i+1:]...)
			return client
		}
	}
	return nil
}

// waitAndAcquire 等待连接可用
func (p *Pool) waitAndAcquire(hops []*types.Hop, hopKey string) (*PooledClient, error) {
	ctx, cancel := context.WithTimeout(p.ctx, p.config.AcquireTimeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("acquire connection timeout: %w", ctx.Err())
		case <-ticker.C:
			// 再次尝试获取空闲连接
			if client := p.tryGetIdle(hopKey); client != nil {
				p.stats.ActiveConns.Add(1)
				p.stats.IdleConns.Add(-1)
				client.markUsed()
				return client, nil
			}

			// 检查连接数是否减少
			p.mu.RLock()
			conns := p.conns[hopKey]
			p.mu.RUnlock()

			if len(conns) < p.config.MaxConnsPerHop {
				client, err := p.createClient(hops, hopKey)
				if err != nil {
					continue
				}
				p.stats.TotalConns.Add(1)
				p.stats.ActiveConns.Add(1)
				return client, nil
			}
		}
	}
}

// createClient 创建新的池化客户端
func (p *Pool) createClient(hops []*types.Hop, hopKey string) (*PooledClient, error) {
	chain := ssh.NewChain(hops)
	if err := chain.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect chain: %w", err)
	}

	client := &PooledClient{
		chain:     chain,
		pool:      p,
		createdAt: time.Now(),
		hopKey:    hopKey,
		id:        p.idCounter.Add(1),
	}

	// 包装最后一跳为 Client
	lastHop := chain.LastHop()
	if lastHop == nil {
		chain.Disconnect()
		return nil, fmt.Errorf("no last hop in chain")
	}
	client.Client = lastHop

	// 添加到连接池
	p.mu.Lock()
	p.conns[hopKey] = append(p.conns[hopKey], client)
	p.mu.Unlock()

	client.markUsed()
	return client, nil
}

// release 释放连接回池中
func (p *Pool) release(client *PooledClient) {
	if client == nil || client.chain == nil {
		return
	}

	client.markIdle()
	p.stats.ActiveConns.Add(-1)

	p.mu.Lock()
	defer p.mu.Unlock()

	// 添加到空闲列表
	idleList := p.idleConns[client.hopKey]
	if len(idleList) >= p.config.MaxIdleConnsPerHop {
		// 空闲连接太多，直接关闭（不增加空闲计数）
		p.closeClientLocked(client)
		return
	}

	p.idleConns[client.hopKey] = append(idleList, client)
	p.stats.IdleConns.Add(1)
}

// closeClientLocked 关闭客户端（必须持有锁）
func (p *Pool) closeClientLocked(client *PooledClient) {
	// 从 conns 列表移除
	conns := p.conns[client.hopKey]
	for i, c := range conns {
		if c.id == client.id {
			p.conns[client.hopKey] = append(conns[:i], conns[i+1:]...)
			break
		}
	}

	// 从 idle 列表移除
	idleConns := p.idleConns[client.hopKey]
	for i, c := range idleConns {
		if c.id == client.id {
			p.idleConns[client.hopKey] = append(idleConns[:i], idleConns[i+1:]...)
			break
		}
	}

	// 关闭连接
	go func() {
		client.chain.Disconnect()
		p.stats.TotalConns.Add(-1)
		p.stats.IdleConns.Add(-1)
	}()
}

// cleanupLoop 定期清理过期连接
func (p *Pool) cleanupLoop() {
	defer p.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.cleanup()
		}
	}
}

// cleanup 清理过期连接
func (p *Pool) cleanup() {
	now := time.Now()

	p.mu.Lock()
	defer p.mu.Unlock()

	for hopKey, clients := range p.conns {
		for _, client := range clients {
			if client.inUse.Load() {
				continue
			}

			// 检查空闲时间
			if now.Sub(client.GetLastUsed()) > p.config.MaxIdleTime {
				p.closeClientLocked(client)
				continue
			}

			// 检查最大存活时间
			if now.Sub(client.createdAt) > p.config.MaxLifetime {
				p.closeClientLocked(client)
				continue
			}

			// 检查连接是否仍然可用
			if !client.IsConnected() {
				p.closeClientLocked(client)
			}
		}

		// 清理空列表
		if len(p.conns[hopKey]) == 0 {
			delete(p.conns, hopKey)
			delete(p.idleConns, hopKey)
		}
	}
}

// Close 关闭连接池
func (p *Pool) Close() error {
	p.cancel()
	p.wg.Wait()

	p.mu.Lock()
	defer p.mu.Unlock()

	// 关闭所有连接
	for _, clients := range p.conns {
		for _, client := range clients {
			client.chain.Disconnect()
		}
	}

	p.conns = make(map[string][]*PooledClient)
	p.idleConns = make(map[string][]*PooledClient)

	return nil
}

// GetStats 获取连接池统计
func (p *Pool) GetStats() PoolStats {
	return PoolStats{
		TotalConns:    atomic.Int64{},
		ActiveConns:   atomic.Int64{},
		IdleConns:     atomic.Int64{},
		WaitCount:     atomic.Int64{},
		AcquireErrors: atomic.Int64{},
	}
}

// generateHopKey 生成 hop 链的唯一标识
func generateHopKey(hops []*types.Hop) string {
	if len(hops) == 0 {
		return ""
	}
	// 简单实现：使用最后一跳的地址作为 key
	// 实际项目中可能需要更复杂的哈希
	last := hops[len(hops)-1]
	return fmt.Sprintf("%s@%s:%d", last.User, last.Host, last.Port)
}

// PooledSession 池化会话封装
type PooledSession struct {
	client  *PooledClient
	session *gossh.Session
	stdin   gossh.Channel
	stdout  gossh.Channel
	stderr  gossh.Channel
}

// NewSession 从池中获取会话
func (p *Pool) NewSession(hops []*types.Hop) (*PooledSession, error) {
	client, err := p.Acquire(hops)
	if err != nil {
		return nil, err
	}

	session, err := client.NewSession()
	if err != nil {
		client.Release()
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return &PooledSession{
		client:  client,
		session: session,
	}, nil
}

// Close 关闭会话，释放回连接池
func (s *PooledSession) Close() error {
	if s.session != nil {
		s.session.Close()
	}
	if s.client != nil {
		s.client.Release()
	}
	return nil
}

// GetSession 获取底层 SSH 会话
func (s *PooledSession) GetSession() *gossh.Session {
	return s.session
}

// logStats 定期记录统计信息
func (p *Pool) logStats() {
	total := p.stats.TotalConns.Load()
	active := p.stats.ActiveConns.Load()
	idle := p.stats.IdleConns.Load()

	log.Printf("[Pool] Stats - Total: %d, Active: %d, Idle: %d",
		total, active, idle)
}
