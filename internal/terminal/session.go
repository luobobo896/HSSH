// Package terminal 提供高性能终端会话管理
package terminal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/luobobo896/HSSH/internal/ssh"
	"github.com/luobobo896/HSSH/pkg/types"
	"github.com/gorilla/websocket"
	gossh "golang.org/x/crypto/ssh"
)

// TerminalInput 终端输入消息
type TerminalInput struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

// TerminalOutput 终端输出消息
type TerminalOutput struct {
	Type      string `json:"type"`
	Data      string `json:"data"`
	Timestamp int64  `json:"timestamp,omitempty"`
}

// TerminalSize 终端大小
type TerminalSize struct {
	Cols int `json:"cols"`
	Rows int `json:"rows"`
}

// Session 高性能终端会话
type Session struct {
	id         string
	serverName string
	hops       []*types.Hop

	// 连接组件
	pool      *Pool
	pooledSess *PooledSession
	forwarder *Forwarder

	// WebSocket
	ws       *websocket.Conn
	upgrader *websocket.Upgrader

	// SSH 会话
	sshSession *gossh.Session
	stdin      io.WriteCloser
	stdout     io.Reader
	stderr     io.Reader

	// 终端配置
	terminalType string
	size         TerminalSize

	// 控制
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// 状态
	connected  atomic.Bool
	startTime  time.Time
	lastActive atomic.Value

	// 统计
	stats SessionStats

	// 回调
	onConnect    func()
	onDisconnect func()
	onError      func(error)
}

// SessionStats 会话统计
type SessionStats struct {
	BytesIn    atomic.Uint64
	BytesOut   atomic.Uint64
	LatencyMs  atomic.Int64
	Errors     atomic.Uint64
}

// SessionConfig 会话配置
type SessionConfig struct {
	ServerName   string
	Hops         []*types.Hop
	TerminalType string
	Cols         int
	Rows         int
	Pool         *Pool
}

// NewSession 创建新的高性能终端会话
func NewSession(config SessionConfig) *Session {
	ctx, cancel := context.WithCancel(context.Background())

	// 默认终端类型
	termType := config.TerminalType
	if termType == "" {
		termType = "xterm-256color"
	}

	return &Session{
		id:           generateSessionID(),
		serverName:   config.ServerName,
		hops:         config.Hops,
		pool:         config.Pool,
		terminalType: termType,
		size: TerminalSize{
			Cols: config.Cols,
			Rows: config.Rows,
		},
		ctx:       ctx,
		cancel:    cancel,
		startTime: time.Now(),
		upgrader: &websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // 生产环境需要更严格的检查
			},
			ReadBufferSize:  32 * 1024,
			WriteBufferSize: 32 * 1024,
		},
	}
}

// generateSessionID 生成会话 ID
func generateSessionID() string {
	return fmt.Sprintf("sess_%d_%d", time.Now().UnixNano(), time.Now().Unix())
}

// HandleWebSocket 处理 WebSocket 连接
func (s *Session) HandleWebSocket(w http.ResponseWriter, r *http.Request) error {
	// 升级 WebSocket
	ws, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return fmt.Errorf("failed to upgrade WebSocket: %w", err)
	}
	s.ws = ws

	defer func() {
		s.ws.Close()
		s.cleanup()
	}()

	// 建立 SSH 连接
	if err := s.connect(); err != nil {
		s.sendError(fmt.Sprintf("SSH connection failed: %v", err))
		return err
	}

	// 发送连接成功消息
	s.sendStatus("connected")

	// 启动数据传输
	return s.run()
}

// connect 建立 SSH 连接
func (s *Session) connect() error {
	log.Printf("[Session %s] Connecting to %s with %d hop(s)...", s.id, s.serverName, len(s.hops))

	// 使用连接池获取会话
	if s.pool != nil {
		pooledSess, err := s.pool.NewSession(s.hops)
		if err != nil {
			return fmt.Errorf("failed to acquire session from pool: %w", err)
		}
		s.pooledSess = pooledSess
		s.sshSession = pooledSess.GetSession()
	} else {
		// 回退到直接连接
		chain := ssh.NewChain(s.hops)
		if err := chain.Connect(); err != nil {
			return fmt.Errorf("failed to connect chain: %w", err)
		}

		session, err := chain.NewSession()
		if err != nil {
			chain.Disconnect()
			return fmt.Errorf("failed to create session: %w", err)
		}
		s.sshSession = session
	}

	// 获取管道
	stdin, err := s.sshSession.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}
	s.stdin = stdin

	stdout, err := s.sshSession.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}
	s.stdout = stdout

	stderr, err := s.sshSession.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}
	s.stderr = stderr

	// 请求伪终端
	modes := gossh.TerminalModes{
		gossh.ECHO:          1,
		gossh.TTY_OP_ISPEED: 14400,
		gossh.TTY_OP_OSPEED: 14400,
	}

	if err := s.sshSession.RequestPty(s.terminalType, s.size.Rows, s.size.Cols, modes); err != nil {
		return fmt.Errorf("failed to request PTY: %w", err)
	}

	// 启动 shell
	if err := s.sshSession.Shell(); err != nil {
		return fmt.Errorf("failed to start shell: %w", err)
	}

	// 创建转发器
	s.forwarder = NewForwarder(DefaultForwarderConfig())

	s.connected.Store(true)
	s.lastActive.Store(time.Now())

	if s.onConnect != nil {
		s.onConnect()
	}

	log.Printf("[Session %s] Connected successfully", s.id)
	return nil
}

// run 运行数据传输循环
func (s *Session) run() error {
	errChan := make(chan error, 3)

	// WebSocket -> SSH
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.handleWebSocketInput(); err != nil {
			errChan <- err
		}
	}()

	// SSH stdout -> WebSocket
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.handleSSHOutput(s.stdout, "stdout"); err != nil {
			errChan <- err
		}
	}()

	// SSH stderr -> WebSocket
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.handleSSHOutput(s.stderr, "stderr"); err != nil {
			errChan <- err
		}
	}()

	// 等待 SSH 会话结束
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.sshSession.Wait(); err != nil {
			log.Printf("[Session %s] SSH session ended: %v", s.id, err)
		}
		s.cancel()
	}()

	// 等待任一 goroutine 结束
	var runErr error
	select {
	case runErr = <-errChan:
		log.Printf("[Session %s] Data transfer error: %v", s.id, runErr)
	case <-s.ctx.Done():
		log.Printf("[Session %s] Context cancelled", s.id)
	}

	// 清理
	s.cancel()
	s.wg.Wait()

	return runErr
}

// handleWebSocketInput 处理 WebSocket 输入
func (s *Session) handleWebSocketInput() error {
	for {
		select {
		case <-s.ctx.Done():
			return nil
		default:
		}

		s.ws.SetReadDeadline(time.Now().Add(30 * time.Second))

		_, data, err := s.ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				return fmt.Errorf("WebSocket read error: %w", err)
			}
			return nil
		}

		s.lastActive.Store(time.Now())

		var input TerminalInput
		if err := json.Unmarshal(data, &input); err != nil {
			log.Printf("[Session %s] Invalid input format: %v", s.id, err)
			continue
		}

		switch input.Type {
		case "input":
			if _, err := s.stdin.Write([]byte(input.Data)); err != nil {
				s.stats.Errors.Add(1)
				return fmt.Errorf("stdin write error: %w", err)
			}
			s.stats.BytesIn.Add(uint64(len(input.Data)))

		case "resize":
			var size TerminalSize
			if err := json.Unmarshal([]byte(input.Data), &size); err == nil {
				s.resize(size)
			}

		case "ping":
			s.sendStatus("pong")
		}
	}
}

// handleSSHOutput 处理 SSH 输出
func (s *Session) handleSSHOutput(reader io.Reader, streamType string) error {
	// 使用自适应缓冲区
	bufferSize := s.forwarder.buffer.GetReadBuffer()
	buf := make([]byte, bufferSize)

	for {
		select {
		case <-s.ctx.Done():
			return nil
		default:
		}

		n, err := reader.Read(buf)
		if err != nil {
			if err != io.EOF {
				return fmt.Errorf("%s read error: %w", streamType, err)
			}
			return nil
		}

		if n > 0 {
			s.lastActive.Store(time.Now())
			s.stats.BytesOut.Add(uint64(n))

			// 发送输出到 WebSocket
			if err := s.sendOutput(string(buf[:n])); err != nil {
				s.stats.Errors.Add(1)
				return err
			}

			// 记录字节数用于自适应调整
			s.forwarder.buffer.RecordBytes(n)
		}
	}
}

// resize 调整终端大小
func (s *Session) resize(size TerminalSize) {
	s.size = size
	if s.sshSession != nil {
		if err := s.sshSession.WindowChange(size.Rows, size.Cols); err != nil {
			log.Printf("[Session %s] Failed to resize: %v", s.id, err)
		} else {
			log.Printf("[Session %s] Resized to %dx%d", s.id, size.Cols, size.Rows)
		}
	}
}

// sendOutput 发送输出到 WebSocket
func (s *Session) sendOutput(data string) error {
	output := TerminalOutput{
		Type:      "output",
		Data:      data,
		Timestamp: time.Now().UnixMilli(),
	}

	s.ws.SetWriteDeadline(time.Now().Add(5 * time.Second))
	return s.ws.WriteJSON(output)
}

// sendStatus 发送状态消息
func (s *Session) sendStatus(status string) error {
	output := TerminalOutput{
		Type:      "status",
		Data:      status,
		Timestamp: time.Now().UnixMilli(),
	}
	return s.ws.WriteJSON(output)
}

// sendError 发送错误消息
func (s *Session) sendError(err string) {
	output := TerminalOutput{
		Type:      "error",
		Data:      err,
		Timestamp: time.Now().UnixMilli(),
	}
	s.ws.WriteJSON(output)
}

// cleanup 清理资源
func (s *Session) cleanup() {
	s.connected.Store(false)

	if s.sshSession != nil {
		s.sshSession.Close()
	}

	if s.pooledSess != nil {
		s.pooledSess.Close()
	}

	if s.forwarder != nil {
		s.forwarder.Close()
	}

	if s.onDisconnect != nil {
		s.onDisconnect()
	}

	duration := time.Since(s.startTime)
	log.Printf("[Session %s] Session ended. Duration: %v, In: %d bytes, Out: %d bytes",
		s.id, duration, s.stats.BytesIn.Load(), s.stats.BytesOut.Load())
}

// SetOnConnect 设置连接回调
func (s *Session) SetOnConnect(fn func()) {
	s.onConnect = fn
}

// SetOnDisconnect 设置断开回调
func (s *Session) SetOnDisconnect(fn func()) {
	s.onDisconnect = fn
}

// SetOnError 设置错误回调
func (s *Session) SetOnError(fn func(error)) {
	s.onError = fn
}

// GetID 获取会话 ID
func (s *Session) GetID() string {
	return s.id
}

// GetStats 获取会话统计
func (s *Session) GetStats() SessionStats {
	return s.stats
}

// IsConnected 检查是否已连接
func (s *Session) IsConnected() bool {
	return s.connected.Load()
}

// GetDuration 获取会话持续时间
func (s *Session) GetDuration() time.Duration {
	return time.Since(s.startTime)
}

// GetLastActive 获取最后活动时间
func (s *Session) GetLastActive() time.Time {
	v := s.lastActive.Load()
	if v == nil {
		return s.startTime
	}
	return v.(time.Time)
}

// Close 主动关闭会话
func (s *Session) Close() error {
	s.cancel()
	return nil
}
