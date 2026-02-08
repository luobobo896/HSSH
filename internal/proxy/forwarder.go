package proxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/luobobo896/HSSH/internal/ssh"
)

// PortForwarder 端口转发器
type PortForwarder struct {
	chain      *ssh.Chain
	localAddr  string
	remoteHost string
	remotePort int
	listener   net.Listener
	active     atomic.Bool
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	connCount  atomic.Int32
}

// NewPortForwarder 创建新的端口转发器
func NewPortForwarder(chain *ssh.Chain, localAddr, remoteHost string, remotePort int) *PortForwarder {
	ctx, cancel := context.WithCancel(context.Background())
	return &PortForwarder{
		chain:      chain,
		localAddr:  localAddr,
		remoteHost: remoteHost,
		remotePort: remotePort,
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start 启动端口转发
func (pf *PortForwarder) Start() error {
	if pf.active.Load() {
		return fmt.Errorf("forwarder already active")
	}

	if !pf.chain.IsConnected() {
		return fmt.Errorf("SSH chain not connected")
	}

	listener, err := net.Listen("tcp", pf.localAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", pf.localAddr, err)
	}

	pf.listener = listener
	pf.active.Store(true)

	// 启动接受连接循环
	pf.wg.Add(1)
	go pf.acceptLoop()

	return nil
}

// Stop 停止端口转发
func (pf *PortForwarder) Stop() error {
	if !pf.active.Load() {
		return nil
	}

	pf.active.Store(false)
	pf.cancel()

	if pf.listener != nil {
		pf.listener.Close()
	}

	// 等待所有连接处理完成
	pf.wg.Wait()

	return nil
}

// IsActive 检查是否处于活动状态
func (pf *PortForwarder) IsActive() bool {
	return pf.active.Load()
}

// GetLocalAddr 获取本地监听地址
func (pf *PortForwarder) GetLocalAddr() string {
	if pf.listener != nil {
		return pf.listener.Addr().String()
	}
	return ""
}

// GetConnectionCount 获取当前连接数
func (pf *PortForwarder) GetConnectionCount() int {
	return int(pf.connCount.Load())
}

// acceptLoop 接受连接循环
func (pf *PortForwarder) acceptLoop() {
	defer pf.wg.Done()

	for {
		select {
		case <-pf.ctx.Done():
			return
		default:
		}

		conn, err := pf.listener.Accept()
		if err != nil {
			if pf.ctx.Err() != nil {
				return
			}
			continue
		}

		pf.wg.Add(1)
		pf.connCount.Add(1)
		go pf.handleConnection(conn)
	}
}

// handleConnection 处理单个连接
func (pf *PortForwarder) handleConnection(localConn net.Conn) {
	defer pf.wg.Done()
	defer pf.connCount.Add(-1)
	defer localConn.Close()

	// 通过 SSH 链建立到远程的连接
	remoteAddr := fmt.Sprintf("%s:%d", pf.remoteHost, pf.remotePort)
	remoteConn, err := pf.chain.Dial("tcp", remoteAddr)
	if err != nil {
		return
	}
	defer remoteConn.Close()

	// 双向转发
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		io.Copy(remoteConn, localConn)
	}()

	go func() {
		defer wg.Done()
		io.Copy(localConn, remoteConn)
	}()

	// 等待任一方断开
	wg.Wait()
}

// ForwarderManager 管理多个端口转发
type ForwarderManager struct {
	forwarders map[string]*PortForwarder
	mu         sync.RWMutex
}

// NewForwarderManager 创建转发管理器
func NewForwarderManager() *ForwarderManager {
	return &ForwarderManager{
		forwarders: make(map[string]*PortForwarder),
	}
}

// Add 添加转发
func (fm *ForwarderManager) Add(id string, forwarder *PortForwarder) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	if _, exists := fm.forwarders[id]; exists {
		return fmt.Errorf("forwarder with id '%s' already exists", id)
	}

	if err := forwarder.Start(); err != nil {
		return err
	}

	fm.forwarders[id] = forwarder
	return nil
}

// Remove 移除转发
func (fm *ForwarderManager) Remove(id string) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	forwarder, exists := fm.forwarders[id]
	if !exists {
		return fmt.Errorf("forwarder with id '%s' not found", id)
	}

	if err := forwarder.Stop(); err != nil {
		return err
	}

	delete(fm.forwarders, id)
	return nil
}

// Get 获取转发器
func (fm *ForwarderManager) Get(id string) *PortForwarder {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	return fm.forwarders[id]
}

// List 列出所有转发器
func (fm *ForwarderManager) List() map[string]*PortForwarder {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	result := make(map[string]*PortForwarder)
	for k, v := range fm.forwarders {
		result[k] = v
	}
	return result
}

// ForwarderInfo 转发器信息
type ForwarderInfo struct {
	ID            string    `json:"id"`
	LocalAddr     string    `json:"local_addr"`
	RemoteHost    string    `json:"remote_host"`
	RemotePort    int       `json:"remote_port"`
	Active        bool      `json:"active"`
	ConnectionCount int     `json:"connection_count"`
	StartedAt     time.Time `json:"started_at"`
}

// GetInfo 获取转发器信息
func (pf *PortForwarder) GetInfo(id string) *ForwarderInfo {
	return &ForwarderInfo{
		ID:              id,
		LocalAddr:       pf.GetLocalAddr(),
		RemoteHost:      pf.remoteHost,
		RemotePort:      pf.remotePort,
		Active:          pf.IsActive(),
		ConnectionCount: pf.GetConnectionCount(),
	}
}
