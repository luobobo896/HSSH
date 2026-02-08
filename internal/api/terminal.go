package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	internalSSH "github.com/luobobo896/HSSH/internal/ssh"
	"github.com/luobobo896/HSSH/pkg/types"
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
)

// TerminalInput 终端输入消息
type TerminalInput struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

// TerminalOutput 终端输出消息
type TerminalOutput struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

// TerminalSession 终端会话
type TerminalSession struct {
	serverName string
	chain      *internalSSH.Chain
	ws         *websocket.Conn
	stdin      chan []byte
	stdout     chan []byte
	done       chan struct{}
	mu         sync.Mutex
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有来源，生产环境应该更严格
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// handleTerminal 处理 WebSocket 终端连接
func (s *Server) handleTerminal(w http.ResponseWriter, r *http.Request) {
	// 从 URL 参数获取服务器名称
	serverName := r.URL.Query().Get("server")
	log.Printf("[TERMINAL] Received connection request for server: %q", serverName)

	if serverName == "" {
		log.Printf("[TERMINAL] Error: server parameter is required")
		http.Error(w, "server parameter is required", http.StatusBadRequest)
		return
	}

	// 查找服务器配置
	hop := s.config.GetHopByName(serverName)
	if hop == nil {
		// 列出所有可用的服务器以便调试
		log.Printf("[TERMINAL] Error: Server %q not found. Available servers:", serverName)
		for _, h := range s.config.Hops {
			log.Printf("[TERMINAL]   - %q (host: %s)", h.Name, h.Host)
		}
		http.Error(w, "Server not found", http.StatusNotFound)
		return
	}

	// 升级 HTTP 连接为 WebSocket
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[TERMINAL] Failed to upgrade WebSocket: %v", err)
		return
	}
	defer ws.Close()

	log.Printf("[TERMINAL] New terminal connection for server: %s (%s@%s:%d, type: %v)",
		serverName, hop.User, hop.Host, hop.Port, hop.ServerType)

	// 构建 hop 链
	hops := s.buildHopChain(serverName)
	if len(hops) == 0 {
		s.sendTerminalError(ws, "Failed to build hop chain")
		return
	}

	// 创建 SSH 链
	chain := internalSSH.NewChain(hops)

	// 连接 SSH 链
	log.Printf("[TERMINAL] Connecting SSH chain with %d hop(s)...", len(hops))
	if err := chain.Connect(); err != nil {
		log.Printf("[TERMINAL] Failed to connect SSH chain: %v", err)
		s.sendTerminalError(ws, fmt.Sprintf("SSH connection failed: %v", err))
		return
	}
	defer chain.Disconnect()

	log.Printf("[TERMINAL] SSH chain connected for %s", serverName)

	// 创建 SSH 会话
	sshSession, err := chain.NewSession()
	if err != nil {
		log.Printf("[TERMINAL] Failed to create SSH session: %v", err)
		s.sendTerminalError(ws, fmt.Sprintf("Failed to create session: %v", err))
		return
	}
	defer sshSession.Close()

	// 必须先获取 Pipe，再启动 Shell
	stdinPipe, err := sshSession.StdinPipe()
	if err != nil {
		log.Printf("[TERMINAL] Failed to get stdin pipe: %v", err)
		s.sendTerminalError(ws, fmt.Sprintf("Failed to get stdin pipe: %v", err))
		return
	}
	stdoutPipe, err := sshSession.StdoutPipe()
	if err != nil {
		log.Printf("[TERMINAL] Failed to get stdout pipe: %v", err)
		s.sendTerminalError(ws, fmt.Sprintf("Failed to get stdout pipe: %v", err))
		return
	}
	stderrPipe, err := sshSession.StderrPipe()
	if err != nil {
		log.Printf("[TERMINAL] Failed to get stderr pipe: %v", err)
		s.sendTerminalError(ws, fmt.Sprintf("Failed to get stderr pipe: %v", err))
		return
	}

	// 请求伪终端
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,     // 启用回显
		ssh.TTY_OP_ISPEED: 14400, // 输入速度
		ssh.TTY_OP_OSPEED: 14400, // 输出速度
	}

	// 获取终端大小（默认 80x24）
	width := 80
	height := 24

	if err := sshSession.RequestPty("xterm-256color", height, width, modes); err != nil {
		log.Printf("[TERMINAL] Failed to request PTY: %v", err)
		s.sendTerminalError(ws, fmt.Sprintf("Failed to request PTY: %v", err))
		return
	}

	// 启动 shell（必须在获取 Pipe 之后）
	if err := sshSession.Shell(); err != nil {
		log.Printf("[TERMINAL] Failed to start shell: %v", err)
		s.sendTerminalError(ws, fmt.Sprintf("Failed to start shell: %v", err))
		return
	}

	log.Printf("[TERMINAL] Shell started for %s", serverName)

	// 发送连接成功消息
	s.sendTerminalMessage(ws, "status", "connected")

	// 创建 done 通道和 context 用于协调关闭
	done := make(chan struct{})
	wsClosed := make(chan struct{})

	// 启动 goroutine 读取 WebSocket 消息并写入 SSH stdin
	go func() {
		defer close(wsClosed)
		for {
			_, message, err := ws.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("[TERMINAL] WebSocket closed by client: %v", err)
				} else {
					log.Printf("[TERMINAL] WebSocket read error: %v", err)
				}
				return
			}

			var input TerminalInput
			if err := json.Unmarshal(message, &input); err != nil {
				log.Printf("[TERMINAL] Failed to unmarshal input: %v", err)
				continue
			}

			switch input.Type {
			case "input":
				if _, err := stdinPipe.Write([]byte(input.Data)); err != nil {
					log.Printf("[TERMINAL] Failed to write to stdin: %v", err)
					return
				}
			case "resize":
				// 处理终端大小调整
				var resizeData struct {
					Cols int `json:"cols"`
					Rows int `json:"rows"`
				}
				if err := json.Unmarshal([]byte(input.Data), &resizeData); err == nil {
					sshSession.WindowChange(resizeData.Rows, resizeData.Cols)
				}
			}
		}
	}()

	// 启动 goroutine 读取 SSH stdout 并写入 WebSocket
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stdoutPipe.Read(buf)
			if err != nil {
				log.Printf("[TERMINAL] stdout read error: %v", err)
				return
			}
			if n > 0 {
				if err := s.sendTerminalMessage(ws, "output", string(buf[:n])); err != nil {
					log.Printf("[TERMINAL] Failed to send stdout: %v", err)
					return
				}
			}
		}
	}()

	// 启动 goroutine 读取 SSH stderr 并写入 WebSocket
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stderrPipe.Read(buf)
			if err != nil {
				log.Printf("[TERMINAL] stderr read error: %v", err)
				return
			}
			if n > 0 {
				if err := s.sendTerminalMessage(ws, "output", string(buf[:n])); err != nil {
					log.Printf("[TERMINAL] Failed to send stderr: %v", err)
					return
				}
			}
		}
	}()

	// 启动 goroutine 等待 SSH 会话结束
	go func() {
		if err := sshSession.Wait(); err != nil {
			log.Printf("[TERMINAL] Session ended with error: %v", err)
		}
		close(done)
	}()

	// 等待 WebSocket 关闭或 SSH 会话结束
	select {
	case <-wsClosed:
		log.Printf("[TERMINAL] WebSocket closed, terminating SSH session for %s", serverName)
		// WebSocket 关闭时，关闭 SSH 会话
		sshSession.Close()
		// 断开连接链
		chain.Disconnect()
	case <-done:
		log.Printf("[TERMINAL] SSH session ended for %s", serverName)
	}

	// 尝试发送断开消息（如果 WebSocket 还打开）
	s.sendTerminalMessage(ws, "status", "disconnected")
	log.Printf("[TERMINAL] Terminal session cleanup completed for %s", serverName)
}

// buildHopChain 构建服务器连接的 hop 链（递归处理网关的跳板机）
func (s *Server) buildHopChain(serverName string) []*types.Hop {
	return s.buildHopChainRecursive(serverName, make(map[string]bool))
}

// buildHopChainRecursive 递归构建 hop 链，检测循环依赖
func (s *Server) buildHopChainRecursive(serverName string, visited map[string]bool) []*types.Hop {
	// 检测循环依赖
	if visited[serverName] {
		log.Printf("[TERMINAL] buildHopChain: Circular dependency detected for %s", serverName)
		return nil
	}

	hop := s.config.GetHopByName(serverName)
	if hop == nil {
		log.Printf("[TERMINAL] buildHopChain: Server %q not found", serverName)
		return nil
	}

	log.Printf("[TERMINAL] buildHopChain: Server=%s, Host=%s, Type=%v, Gateway=%q",
		serverName, hop.Host, hop.ServerType, hop.Gateway)

	var hops []*types.Hop

	// 如果配置了网关/跳板机，递归添加网关链
	// 现在支持所有服务器类型配置跳板机，不限于内网服务器
	if hop.Gateway != "" {
		visited[serverName] = true
		// 递归获取网关的完整链路（网关可能也有自己的跳板机）
		gatewayHops := s.buildHopChainRecursive(hop.Gateway, visited)
		if len(gatewayHops) > 0 {
			log.Printf("[TERMINAL] Adding gateway chain for %s: %v", serverName, getHopNames(gatewayHops))
			hops = append(hops, gatewayHops...)
		} else {
			log.Printf("[TERMINAL] Warning: Gateway %s not found or has circular dependency for server %s", hop.Gateway, serverName)
		}
	} else {
		log.Printf("[TERMINAL] No gateway configured for server %s", serverName)
	}

	// 添加目标服务器
	hops = append(hops, hop)

	log.Printf("[TERMINAL] buildHopChain result for %s: %d hop(s): %v", serverName, len(hops), getHopNames(hops))
	return hops
}

// getHopNames 获取 hop 名称列表（用于日志）
func getHopNames(hops []*types.Hop) []string {
	names := make([]string, len(hops))
	for i, h := range hops {
		names[i] = h.Name
	}
	return names
}

// sendTerminalMessage 发送终端消息
func (s *Server) sendTerminalMessage(ws *websocket.Conn, msgType, data string) error {
	msg := TerminalOutput{
		Type: msgType,
		Data: data,
	}

	if err := ws.WriteJSON(msg); err != nil {
		log.Printf("[TERMINAL] Failed to send message: %v", err)
		return err
	}
	return nil
}

// sendTerminalError 发送终端错误消息
func (s *Server) sendTerminalError(ws *websocket.Conn, err string) {
	_ = s.sendTerminalMessage(ws, "error", err)
}
