// Package terminal 提供高性能终端会话管理器
package terminal

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/luobobo896/HSSH/pkg/types"
)

// Manager 终端会话管理器
type Manager struct {
	config *types.Config
	pool   *Pool

	// 会话存储
	sessions sync.Map // map[string]*Session

	// 统计
	stats ManagerStats

	// 控制
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// 配置
	maxSessions   int
	sessionTTL    time.Duration
	cleanupInterval time.Duration
}

// ManagerStats 管理器统计
type ManagerStats struct {
	TotalSessions   atomic.Int64
	ActiveSessions  atomic.Int64
	TotalConnects   atomic.Int64
	TotalDisconnects atomic.Int64
	Errors          atomic.Int64
}

// ManagerConfig 管理器配置
type ManagerConfig struct {
	PoolConfig      PoolConfig
	MaxSessions     int
	SessionTTL      time.Duration
	CleanupInterval time.Duration
}

// DefaultManagerConfig 返回默认管理器配置
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		PoolConfig:      DefaultPoolConfig(),
		MaxSessions:     100,
		SessionTTL:      30 * time.Minute,
		CleanupInterval: 60 * time.Second,
	}
}

// NewManager 创建新的会话管理器
func NewManager(cfg *types.Config, managerConfig ManagerConfig) (*Manager, error) {
	ctx, cancel := context.WithCancel(context.Background())

	pool := NewPool(managerConfig.PoolConfig)

	m := &Manager{
		config:          cfg,
		pool:            pool,
		ctx:             ctx,
		cancel:          cancel,
		maxSessions:     managerConfig.MaxSessions,
		sessionTTL:      managerConfig.SessionTTL,
		cleanupInterval: managerConfig.CleanupInterval,
	}

	// 启动后台清理 goroutine
	m.wg.Add(1)
	go m.cleanupLoop()

	return m, nil
}

// HandleTerminal 处理终端 WebSocket 连接
func (m *Manager) HandleTerminal(w http.ResponseWriter, r *http.Request) {
	serverName := r.URL.Query().Get("server")
	if serverName == "" {
		http.Error(w, "server parameter is required", http.StatusBadRequest)
		return
	}

	// 检查会话数限制
	activeCount := m.stats.ActiveSessions.Load()
	if int(activeCount) >= m.maxSessions {
		http.Error(w, "too many active sessions", http.StatusServiceUnavailable)
		m.stats.Errors.Add(1)
		return
	}

	// 查找服务器配置
	hop := m.config.GetHopByName(serverName)
	if hop == nil {
		http.Error(w, "server not found", http.StatusNotFound)
		return
	}

	// 构建 hop 链
	hops := m.buildHopChain(hop)
	if len(hops) == 0 {
		http.Error(w, "failed to build hop chain", http.StatusInternalServerError)
		return
	}

	// 创建会话配置
	sessionConfig := SessionConfig{
		ServerName:   serverName,
		Hops:         hops,
		TerminalType: "xterm-256color",
		Cols:         80,
		Rows:         24,
		Pool:         m.pool,
	}

	// 从 URL 参数获取终端大小
	if cols, rows := parseTerminalSize(r); cols > 0 && rows > 0 {
		sessionConfig.Cols = cols
		sessionConfig.Rows = rows
	}

	// 创建会话
	session := NewSession(sessionConfig)

	// 设置回调
	session.SetOnConnect(func() {
		m.stats.ActiveSessions.Add(1)
		m.stats.TotalConnects.Add(1)
		m.sessions.Store(session.GetID(), session)
	})

	session.SetOnDisconnect(func() {
		m.stats.ActiveSessions.Add(-1)
		m.stats.TotalDisconnects.Add(1)
		m.sessions.Delete(session.GetID())
	})

	session.SetOnError(func(err error) {
		m.stats.Errors.Add(1)
		log.Printf("[Manager] Session %s error: %v", session.GetID(), err)
	})

	// 处理 WebSocket 连接
	if err := session.HandleWebSocket(w, r); err != nil {
		log.Printf("[Manager] Session %s error: %v", session.GetID(), err)
	}
}

// buildHopChain 构建 hop 链
func (m *Manager) buildHopChain(targetHop *types.Hop) []*types.Hop {
	var hops []*types.Hop

	// 如果配置了网关，先添加网关
	if targetHop.Gateway != "" {
		gatewayHop := m.config.GetHopByName(targetHop.Gateway)
		if gatewayHop != nil {
			log.Printf("[Manager] Adding gateway %s for server %s", targetHop.Gateway, targetHop.Name)
			hops = append(hops, gatewayHop)
		} else {
			log.Printf("[Manager] Warning: Gateway %s not found for server %s", targetHop.Gateway, targetHop.Name)
		}
	}

	// 添加目标服务器
	hops = append(hops, targetHop)

	return hops
}

// GetSession 获取会话
func (m *Manager) GetSession(id string) (*Session, bool) {
	val, ok := m.sessions.Load(id)
	if !ok {
		return nil, false
	}
	return val.(*Session), true
}

// ListSessions 列出所有会话
func (m *Manager) ListSessions() []SessionInfo {
	var sessions []SessionInfo

	m.sessions.Range(func(key, value interface{}) bool {
		session := value.(*Session)
		stats := session.GetStats()
		sessions = append(sessions, SessionInfo{
			ID:          session.GetID(),
			ServerName:  session.serverName,
			Connected:   session.IsConnected(),
			Duration:    session.GetDuration(),
			LastActive:  session.GetLastActive(),
			BytesIn:     stats.BytesIn.Load(),
			BytesOut:    stats.BytesOut.Load(),
		})
		return true
	})

	return sessions
}

// CloseSession 关闭指定会话
func (m *Manager) CloseSession(id string) error {
	val, ok := m.sessions.Load(id)
	if !ok {
		return fmt.Errorf("session not found: %s", id)
	}

	session := val.(*Session)
	return session.Close()
}

// cleanupLoop 定期清理过期会话
func (m *Manager) cleanupLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.cleanup()
			m.logStats()
		}
	}
}

// cleanup 清理过期会话
func (m *Manager) cleanup() {
	now := time.Now()
	var toRemove []string

	m.sessions.Range(func(key, value interface{}) bool {
		session := value.(*Session)
		sessionID := key.(string)

		// 检查是否已断开连接且超过 TTL
		if !session.IsConnected() {
			toRemove = append(toRemove, sessionID)
			return true
		}

		// 检查是否超过最大空闲时间
		if now.Sub(session.GetLastActive()) > m.sessionTTL {
			log.Printf("[Manager] Closing idle session: %s", sessionID)
			session.Close()
			toRemove = append(toRemove, sessionID)
		}

		return true
	})

	// 删除已清理的会话
	for _, id := range toRemove {
		m.sessions.Delete(id)
	}
}

// logStats 记录统计信息
func (m *Manager) logStats() {
	total := m.stats.TotalSessions.Load()
	active := m.stats.ActiveSessions.Load()
	connects := m.stats.TotalConnects.Load()
	disconnects := m.stats.TotalDisconnects.Load()
	errors := m.stats.Errors.Load()

	log.Printf("[Manager] Stats - Total: %d, Active: %d, Connects: %d, Disconnects: %d, Errors: %d",
		total, active, connects, disconnects, errors)
}

// GetStats 获取管理器统计
func (m *Manager) GetStats() ManagerStats {
	return ManagerStats{
		TotalSessions:    atomic.Int64{},
		ActiveSessions:   atomic.Int64{},
		TotalConnects:    atomic.Int64{},
		TotalDisconnects: atomic.Int64{},
		Errors:           atomic.Int64{},
	}
}

// Close 关闭管理器
func (m *Manager) Close() error {
	log.Printf("[Manager] Shutting down...")

	// 关闭所有会话
	m.sessions.Range(func(key, value interface{}) bool {
		session := value.(*Session)
		session.Close()
		return true
	})

	// 取消上下文
	m.cancel()

	// 等待清理 goroutine 结束
	m.wg.Wait()

	// 关闭连接池
	if m.pool != nil {
		m.pool.Close()
	}

	log.Printf("[Manager] Shutdown complete")
	return nil
}

// SessionInfo 会话信息
type SessionInfo struct {
	ID           string
	ServerName   string
	Connected    bool
	Duration     time.Duration
	LastActive   time.Time
	BytesIn      uint64
	BytesOut     uint64
}

// parseTerminalSize 从请求中解析终端大小
func parseTerminalSize(r *http.Request) (cols, rows int) {
	// 从 URL 参数解析
	if c := r.URL.Query().Get("cols"); c != "" {
		fmt.Sscanf(c, "%d", &cols)
	}
	if ro := r.URL.Query().Get("rows"); ro != "" {
		fmt.Sscanf(ro, "%d", &rows)
	}
	return
}

// GetPoolStats 获取连接池统计
func (m *Manager) GetPoolStats() PoolStats {
	if m.pool != nil {
		return m.pool.GetStats()
	}
	return PoolStats{}
}

// APIHandler HTTP API 处理器
func (m *Manager) APIHandler() http.Handler {
	mux := http.NewServeMux()

	// 获取会话列表
	mux.HandleFunc("/api/sessions", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			sessions := m.ListSessions()
			writeJSON(w, sessions)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// 获取统计信息
	mux.HandleFunc("/api/stats", func(w http.ResponseWriter, r *http.Request) {
		stats := map[string]interface{}{
			"manager": m.GetStats(),
			"pool":    m.GetPoolStats(),
		}
		writeJSON(w, stats)
	})

	// 关闭会话
	mux.HandleFunc("/api/sessions/close", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		sessionID := r.URL.Query().Get("id")
		if sessionID == "" {
			http.Error(w, "id parameter required", http.StatusBadRequest)
			return
		}

		if err := m.CloseSession(sessionID); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		writeJSON(w, map[string]string{"status": "ok"})
	})

	return mux
}

// writeJSON 写入 JSON 响应
func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	// 使用 json.NewEncoder 直接写入
	// 避免先生成 []byte 再写入的内存拷贝
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("[Manager] Failed to encode JSON: %v", err)
	}
}
