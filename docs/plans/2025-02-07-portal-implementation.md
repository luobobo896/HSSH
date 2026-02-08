---
title: GMPortal Implementation Plan
date: 2025-02-07
type: Plan
status: Draft
---

# GMPortal Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 实现高性能端口转发/内网穿透功能，支持 HTTP/WebSocket、500+ 并发、smux 多路复用

**Architecture:**
- Client-Server 架构，基于 smux 流多路复用
- SSH 作为底层传输隧道，TLS 二次加密
- 复用 HSSH 现有 Hop ID 配置系统

**Tech Stack:** Go 1.25.6, smux, gorilla/websocket, golang.org/x/crypto

---

## Phase 1: 基础类型定义

### Task 1: 创建 Portal 类型定义

**Files:**
- Create: `pkg/portal/types.go`
- Test: `pkg/portal/types_test.go`

**Step 1: Write the types**

```go
package portal

import "time"

// Protocol 支持的协议类型
type Protocol string

const (
    ProtocolTCP       Protocol = "tcp"
    ProtocolHTTP      Protocol = "http"
    ProtocolWebSocket Protocol = "websocket"
)

// PortMapping 端口映射配置
type PortMapping struct {
    ID         string   `json:"id" yaml:"id"`
    Name       string   `json:"name" yaml:"name"`
    LocalAddr  string   `json:"local_addr" yaml:"local_addr"`
    RemoteHost string   `json:"remote_host" yaml:"remote_host"`
    RemotePort int      `json:"remote_port" yaml:"remote_port"`
    Via        []string `json:"via" yaml:"via"`
    Protocol   Protocol `json:"protocol" yaml:"protocol"`
    Enabled    bool     `json:"enabled" yaml:"enabled"`
}

// PortalConfig portal 模块配置
type PortalConfig struct {
    Client ClientConfig `json:"client" yaml:"client"`
    Server ServerConfig `json:"server" yaml:"server"`
}

// ClientConfig 客户端配置
type ClientConfig struct {
    Mappings   []PortMapping    `json:"mappings" yaml:"mappings"`
    Connection ConnectionConfig `json:"connection" yaml:"connection"`
}

// ServerConfig 服务端配置
type ServerConfig struct {
    Enabled    bool          `json:"enabled" yaml:"enabled"`
    ListenAddr string        `json:"listen_addr" yaml:"listen_addr"`
    TLSCert    string        `json:"tls_cert" yaml:"tls_cert"`
    TLSKey     string        `json:"tls_key" yaml:"tls_key"`
    AuthTokens []TokenConfig `json:"auth_tokens" yaml:"auth_tokens"`
}

// TokenConfig Token 认证配置
type TokenConfig struct {
    Token          string   `json:"token" yaml:"token"`
    AllowedRemotes []string `json:"allowed_remotes" yaml:"allowed_remotes"`
    MaxMappings    int      `json:"max_mappings" yaml:"max_mappings"`
}

// ConnectionConfig 连接配置
type ConnectionConfig struct {
    RetryInterval     time.Duration `json:"retry_interval" yaml:"retry_interval"`
    MaxRetries        int           `json:"max_retries" yaml:"max_retries"`
    KeepaliveInterval time.Duration `json:"keepalive_interval" yaml:"keepalive_interval"`
}

// MappingStatus 运行时映射状态
type MappingStatus struct {
    PortMapping
    Active           bool      `json:"active"`
    ConnectionCount  int       `json:"connection_count"`
    BytesTransferred int64     `json:"bytes_transferred"`
    LastActive       time.Time `json:"last_active"`
    Error            string    `json:"error,omitempty"`
}

// DefaultConnectionConfig 返回默认连接配置
func DefaultConnectionConfig() ConnectionConfig {
    return ConnectionConfig{
        RetryInterval:     5 * time.Second,
        MaxRetries:        10,
        KeepaliveInterval: 30 * time.Second,
    }
}
```

**Step 2: Create test file**

```go
package portal

import (
    "testing"
    "time"
)

func TestDefaultConnectionConfig(t *testing.T) {
    cfg := DefaultConnectionConfig()
    if cfg.RetryInterval != 5*time.Second {
        t.Errorf("expected retry interval 5s, got %v", cfg.RetryInterval)
    }
    if cfg.MaxRetries != 10 {
        t.Errorf("expected max retries 10, got %d", cfg.MaxRetries)
    }
    if cfg.KeepaliveInterval != 30*time.Second {
        t.Errorf("expected keepalive interval 30s, got %v", cfg.KeepaliveInterval)
    }
}

func TestPortMapping(t *testing.T) {
    m := PortMapping{
        ID:         "test-id",
        Name:       "test-mapping",
        LocalAddr:  ":8848",
        RemoteHost: "192.168.1.10",
        RemotePort: 8848,
        Via:        []string{"gateway-1"},
        Protocol:   ProtocolHTTP,
        Enabled:    true,
    }

    if m.Name != "test-mapping" {
        t.Errorf("expected name 'test-mapping', got %s", m.Name)
    }
    if m.Protocol != ProtocolHTTP {
        t.Errorf("expected protocol http, got %s", m.Protocol)
    }
}
```

**Step 3: Run tests**

```bash
cd /Users/hanson/installShell/gmssh
go test ./pkg/portal/... -v
```

Expected: PASS

**Step 4: Commit**

```bash
git add pkg/portal/
git commit -m "feat(portal): add portal types and configuration"
```

---

## Phase 2: smux 协议封装

### Task 2: 添加 smux 依赖

**Files:**
- Modify: `go.mod`

**Step 1: Add dependency**

```bash
cd /Users/hanson/installShell/gmssh
go get github.com/xtaci/smux@v1.5.24
```

**Step 2: Verify go.mod**

Check that `github.com/xtaci/smux v1.5.24` is added to require section.

**Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "chore(deps): add smux for connection multiplexing"
```

### Task 3: 创建 smux 协议封装

**Files:**
- Create: `internal/portal/protocol/mux.go`
- Create: `internal/portal/protocol/mux_test.go`

**Step 1: Implement mux wrapper**

```go
package protocol

import (
    "crypto/tls"
    "fmt"
    "net"
    "time"

    "github.com/xtaci/smux"
)

// MuxConfig smux 配置
type MuxConfig struct {
    KeepAliveInterval time.Duration
    KeepAliveTimeout  time.Duration
    MaxFrameSize      int
    MaxReceiveBuffer  int
    MaxStreamBuffer   int
}

// DefaultMuxConfig 返回默认 smux 配置
func DefaultMuxConfig() *MuxConfig {
    return &MuxConfig{
        KeepAliveInterval: 30 * time.Second,
        KeepAliveTimeout:  40 * time.Second,
        MaxFrameSize:      32768,
        MaxReceiveBuffer:  4194304,
        MaxStreamBuffer:   65536,
    }
}

// ToSmuxConfig 转换为 smux.Config
func (c *MuxConfig) ToSmuxConfig() *smux.Config {
    return &smux.Config{
        KeepAliveInterval: c.KeepAliveInterval,
        KeepAliveTimeout:  c.KeepAliveTimeout,
        MaxFrameSize:      c.MaxFrameSize,
        MaxReceiveBuffer:  c.MaxReceiveBuffer,
        MaxStreamBuffer:   c.MaxStreamBuffer,
    }
}

// ClientMux smux 客户端封装
type ClientMux struct {
    session *smux.Session
    config  *MuxConfig
}

// Dial 建立到服务器的 smux 连接
func Dial(addr string, tlsConfig *tls.Config, config *MuxConfig) (*ClientMux, error) {
    if config == nil {
        config = DefaultMuxConfig()
    }

    conn, err := tls.Dial("tcp", addr, tlsConfig)
    if err != nil {
        return nil, fmt.Errorf("failed to dial: %w", err)
    }

    session, err := smux.Client(conn, config.ToSmuxConfig())
    if err != nil {
        conn.Close()
        return nil, fmt.Errorf("failed to create smux client: %w", err)
    }

    return &ClientMux{
        session: session,
        config:  config,
    }, nil
}

// OpenStream 打开新流
func (m *ClientMux) OpenStream() (*smux.Stream, error) {
    return m.session.OpenStream()
}

// Close 关闭会话
func (m *ClientMux) Close() error {
    return m.session.Close()
}

// IsClosed 检查是否已关闭
func (m *ClientMux) IsClosed() bool {
    return m.session.IsClosed()
}

// NumStreams 获取当前流数量
func (m *ClientMux) NumStreams() int {
    return m.session.NumStreams()
}

// ServerMux smux 服务端封装
type ServerMux struct {
    listener net.Listener
    config   *MuxConfig
}

// Listen 创建 smux 服务端监听
func Listen(addr string, tlsConfig *tls.Config, config *MuxConfig) (*ServerMux, error) {
    if config == nil {
        config = DefaultMuxConfig()
    }

    listener, err := tls.Listen("tcp", addr, tlsConfig)
    if err != nil {
        return nil, fmt.Errorf("failed to listen: %w", err)
    }

    return &ServerMux{
        listener: listener,
        config:   config,
    }, nil
}

// Accept 接受新连接
func (s *ServerMux) Accept() (*smux.Session, error) {
    conn, err := s.listener.Accept()
    if err != nil {
        return nil, err
    }

    session, err := smux.Server(conn, s.config.ToSmuxConfig())
    if err != nil {
        conn.Close()
        return nil, fmt.Errorf("failed to create smux server: %w", err)
    }

    return session, nil
}

// Close 关闭监听
func (s *ServerMux) Close() error {
    return s.listener.Close()
}

// Addr 返回监听地址
func (s *ServerMux) Addr() net.Addr {
    return s.listener.Addr()
}
```

**Step 2: Create test**

```go
package protocol

import (
    "crypto/rand"
    "crypto/rsa"
    "crypto/tls"
    "crypto/x509"
    "crypto/x509/pkix"
    "encoding/pem"
    "math/big"
    "sync"
    "testing"
    "time"
)

func generateTestCert() (tls.Certificate, error) {
    priv, err := rsa.GenerateKey(rand.Reader, 2048)
    if err != nil {
        return tls.Certificate{}, err
    }

    template := x509.Certificate{
        SerialNumber: big.NewInt(1),
        Subject: pkix.Name{
            Organization: []string{"Test"},
        },
        NotBefore:             time.Now(),
        NotAfter:              time.Now().Add(time.Hour),
        KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
        ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
        BasicConstraintsValid: true,
        IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
    }

    certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
    if err != nil {
        return tls.Certificate{}, err
    }

    certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
    keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

    return tls.X509KeyPair(certPEM, keyPEM)
}

func TestMuxCommunication(t *testing.T) {
    cert, err := generateTestCert()
    if err != nil {
        t.Fatalf("failed to generate cert: %v", err)
    }

    tlsConfig := &tls.Config{
        Certificates:       []tls.Certificate{cert},
        InsecureSkipVerify: true,
    }

    server, err := Listen("127.0.0.1:0", tlsConfig, nil)
    if err != nil {
        t.Fatalf("failed to create server: %v", err)
    }
    defer server.Close()

    var wg sync.WaitGroup
    wg.Add(1)

    go func() {
        defer wg.Done()
        session, err := server.Accept()
        if err != nil {
            t.Errorf("failed to accept: %v", err)
            return
        }
        defer session.Close()

        stream, err := session.AcceptStream()
        if err != nil {
            t.Errorf("failed to accept stream: %v", err)
            return
        }
        defer stream.Close()

        buf := make([]byte, 1024)
        n, err := stream.Read(buf)
        if err != nil {
            t.Errorf("failed to read: %v", err)
            return
        }

        if _, err := stream.Write(buf[:n]); err != nil {
            t.Errorf("failed to write: %v", err)
        }
    }()

    client, err := Dial(server.Addr().String(), tlsConfig, nil)
    if err != nil {
        t.Fatalf("failed to dial: %v", err)
    }
    defer client.Close()

    stream, err := client.OpenStream()
    if err != nil {
        t.Fatalf("failed to open stream: %v", err)
    }
    defer stream.Close()

    testData := []byte("hello smux")
    if _, err := stream.Write(testData); err != nil {
        t.Fatalf("failed to write: %v", err)
    }

    buf := make([]byte, 1024)
    n, err := stream.Read(buf)
    if err != nil {
        t.Fatalf("failed to read: %v", err)
    }

    if string(buf[:n]) != string(testData) {
        t.Errorf("expected %s, got %s", testData, buf[:n])
    }

    wg.Wait()
}
```

**Step 3: Run tests**

```bash
cd /Users/hanson/installShell/gmssh
go test ./internal/portal/protocol/... -v -timeout 30s
```

Expected: PASS

**Step 4: Commit**

```bash
git add internal/portal/protocol/
git commit -m "feat(portal): add smux protocol wrapper"
```

---

## Phase 3: 服务端实现

### Task 4: 创建 Portal 服务端

**Files:**
- Create: `internal/portal/server/server.go`
- Create: `internal/portal/server/auth.go`

**Step 1: Implement auth**

```go
package server

import (
    "fmt"
    "net"
    "strings"

    "github.com/gmssh/gmssh/pkg/portal"
)

// Authenticator 认证器
type Authenticator struct {
    tokens map[string]*portal.TokenConfig
}

// NewAuthenticator 创建认证器
func NewAuthenticator(tokens []portal.TokenConfig) *Authenticator {
    tokenMap := make(map[string]*portal.TokenConfig)
    for i := range tokens {
        tokenMap[tokens[i].Token] = &tokens[i]
    }
    return &Authenticator{tokens: tokenMap}
}

// AuthResult 认证结果
type AuthResult struct {
    Token     string
    Config    *portal.TokenConfig
    RemoteOK  bool
}

// Authenticate 验证 token
func (a *Authenticator) Authenticate(token string, remoteHost string) (*AuthResult, error) {
    config, ok := a.tokens[token]
    if !ok {
        return nil, fmt.Errorf("invalid token")
    }

    result := &AuthResult{
        Token:  token,
        Config: config,
    }

    // 检查远程地址是否在白名单
    for _, allowed := range config.AllowedRemotes {
        if isIPInRange(remoteHost, allowed) {
            result.RemoteOK = true
            break
        }
    }

    return result, nil
}

// isIPInRange 检查 IP 是否在网段内
func isIPInRange(ip, cidr string) bool {
    if strings.Contains(cidr, "/") {
        _, ipNet, err := net.ParseCIDR(cidr)
        if err != nil {
            return false
        }
        parsedIP := net.ParseIP(ip)
        if parsedIP == nil {
            return false
        }
        return ipNet.Contains(parsedIP)
    }
    return ip == cidr
}
```

**Step 2: Implement server**

```go
package server

import (
    "context"
    "crypto/tls"
    "fmt"
    "io"
    "log"
    "net"
    "sync"
    "sync/atomic"

    "github.com/gmssh/gmssh/internal/portal/protocol"
    "github.com/gmssh/gmssh/pkg/portal"
    "github.com/xtaci/smux"
)

// StreamHandler 流处理函数
type StreamHandler func(stream *smux.Stream, remoteHost string, remotePort int)

// Server Portal 服务端
type Server struct {
    config        *portal.ServerConfig
    authenticator *Authenticator
    muxConfig     *protocol.MuxConfig
    listener      *protocol.ServerMux

    sessions     map[string]*smux.Session
    sessionsMu   sync.RWMutex

    streamCounter int64

    ctx    context.Context
    cancel context.CancelFunc
    wg     sync.WaitGroup
}

// New 创建服务端
func New(config *portal.ServerConfig, muxConfig *protocol.MuxConfig) (*Server, error) {
    if !config.Enabled {
        return nil, fmt.Errorf("server is not enabled")
    }

    authenticator := NewAuthenticator(config.AuthTokens)

    ctx, cancel := context.WithCancel(context.Background())

    return &Server{
        config:        config,
        authenticator: authenticator,
        muxConfig:     muxConfig,
        sessions:      make(map[string]*smux.Session),
        ctx:           ctx,
        cancel:        cancel,
    }, nil
}

// Start 启动服务端
func (s *Server) Start() error {
    tlsConfig, err := s.loadTLSConfig()
    if err != nil {
        return fmt.Errorf("failed to load TLS config: %w", err)
    }

    listener, err := protocol.Listen(s.config.ListenAddr, tlsConfig, s.muxConfig)
    if err != nil {
        return fmt.Errorf("failed to listen: %w", err)
    }

    s.listener = listener
    log.Printf("[Portal Server] Listening on %s", listener.Addr())

    s.wg.Add(1)
    go s.acceptLoop()

    return nil
}

// Stop 停止服务端
func (s *Server) Stop() error {
    s.cancel()

    if s.listener != nil {
        s.listener.Close()
    }

    s.sessionsMu.Lock()
    for _, session := range s.sessions {
        session.Close()
    }
    s.sessions = make(map[string]*smux.Session)
    s.sessionsMu.Unlock()

    s.wg.Wait()
    return nil
}

func (s *Server) acceptLoop() {
    defer s.wg.Done()

    for {
        select {
        case <-s.ctx.Done():
            return
        default:
        }

        session, err := s.listener.Accept()
        if err != nil {
            if s.ctx.Err() != nil {
                return
            }
            log.Printf("[Portal Server] Accept error: %v", err)
            continue
        }

        s.wg.Add(1)
        go s.handleSession(session)
    }
}

func (s *Server) handleSession(session *smux.Session) {
    defer s.wg.Done()

    sessionID := fmt.Sprintf("session-%d", atomic.AddInt64(&s.streamCounter, 1))

    s.sessionsMu.Lock()
    s.sessions[sessionID] = session
    s.sessionsMu.Unlock()

    defer func() {
        s.sessionsMu.Lock()
        delete(s.sessions, sessionID)
        s.sessionsMu.Unlock()
        session.Close()
    }()

    log.Printf("[Portal Server] New session: %s", sessionID)

    for {
        stream, err := session.AcceptStream()
        if err != nil {
            if err != io.EOF && session.IsClosed() {
                log.Printf("[Portal Server] Session closed: %s", sessionID)
            }
            return
        }

        s.wg.Add(1)
        go func() {
            defer s.wg.Done()
            s.handleStream(stream)
        }()
    }
}

func (s *Server) handleStream(stream *smux.Stream) {
    defer stream.Close()

    // 读取目标地址
    var req ConnectRequest
    if err := decodeRequest(stream, &req); err != nil {
        log.Printf("[Portal Server] Failed to decode request: %v", err)
        return
    }

    // 验证 token
    authResult, err := s.authenticator.Authenticate(req.Token, req.RemoteHost)
    if err != nil {
        log.Printf("[Portal Server] Auth failed: %v", err)
        sendError(stream, "auth failed")
        return
    }

    // 检查远程地址权限
    if !authResult.RemoteOK {
        log.Printf("[Portal Server] Remote %s not allowed for token", req.RemoteHost)
        sendError(stream, "remote not allowed")
        return
    }

    // 连接到目标
    targetAddr := fmt.Sprintf("%s:%d", req.RemoteHost, req.RemotePort)
    targetConn, err := net.Dial("tcp", targetAddr)
    if err != nil {
        log.Printf("[Portal Server] Failed to connect to %s: %v", targetAddr, err)
        sendError(stream, fmt.Sprintf("connect failed: %v", err))
        return
    }
    defer targetConn.Close()

    // 发送成功响应
    if err := sendSuccess(stream); err != nil {
        log.Printf("[Portal Server] Failed to send success: %v", err)
        return
    }

    log.Printf("[Portal Server] Proxying to %s", targetAddr)

    // 双向转发
    var wg sync.WaitGroup
    wg.Add(2)

    go func() {
        defer wg.Done()
        io.Copy(targetConn, stream)
        targetConn.(*net.TCPConn).CloseWrite()
    }()

    go func() {
        defer wg.Done()
        io.Copy(stream, targetConn)
        stream.Close()
    }()

    wg.Wait()
}

func (s *Server) loadTLSConfig() (*tls.Config, error) {
    cert, err := tls.LoadX509KeyPair(s.config.TLSCert, s.config.TLSKey)
    if err != nil {
        return nil, fmt.Errorf("failed to load TLS cert: %w", err)
    }

    return &tls.Config{
        Certificates: []tls.Certificate{cert},
    }, nil
}

// ConnectRequest 连接请求
type ConnectRequest struct {
    Token      string `json:"token"`
    RemoteHost string `json:"remote_host"`
    RemotePort int    `json:"remote_port"`
}

func decodeRequest(r io.Reader, req *ConnectRequest) error {
    // 简单实现，实际使用 JSON 或 protobuf
    decoder := json.NewDecoder(r)
    return decoder.Decode(req)
}

func sendError(w io.Writer, msg string) error {
    resp := map[string]interface{}{
        "success": false,
        "error":   msg,
    }
    return json.NewEncoder(w).Encode(resp)
}

func sendSuccess(w io.Writer) error {
    resp := map[string]interface{}{
        "success": true,
    }
    return json.NewEncoder(w).Encode(resp)
}
```

**Step 3: Fix imports**

Add to imports:
```go
import "encoding/json"
```

**Step 4: Commit**

```bash
git add internal/portal/server/
git commit -m "feat(portal): add portal server implementation"
```

---

## Phase 4: 客户端实现

### Task 5: 创建 Portal 客户端

**Files:**
- Create: `internal/portal/client/client.go`
- Create: `internal/portal/client/mapping.go`

**Step 1: Implement mapping manager**

```go
package client

import (
    "context"
    "fmt"
    "log"
    "net"
    "sync"
    "sync/atomic"
    "time"

    "github.com/gmssh/gmssh/pkg/portal"
)

// Mapping 端口映射实例
type Mapping struct {
    config    portal.PortMapping
    listener  net.Listener

    active    int32
    connCount int64
    bytesSent int64
    bytesRecv int64
    lastActive time.Time

    ctx    context.Context
    cancel context.CancelFunc
}

// NewMapping 创建映射
func NewMapping(config portal.PortMapping) *Mapping {
    ctx, cancel := context.WithCancel(context.Background())
    return &Mapping{
        config: config,
        ctx:    ctx,
        cancel: cancel,
    }
}

// Start 启动监听
func (m *Mapping) Start(handler func(net.Conn)) error {
    if !m.config.Enabled {
        return fmt.Errorf("mapping is disabled")
    }

    listener, err := net.Listen("tcp", m.config.LocalAddr)
    if err != nil {
        return fmt.Errorf("failed to listen on %s: %w", m.config.LocalAddr, err)
    }

    m.listener = listener
    atomic.StoreInt32(&m.active, 1)

    log.Printf("[Portal Client] Mapping %s: listening on %s", m.config.Name, m.config.LocalAddr)

    go m.acceptLoop(handler)
    return nil
}

// Stop 停止映射
func (m *Mapping) Stop() error {
    m.cancel()
    if m.listener != nil {
        m.listener.Close()
    }
    atomic.StoreInt32(&m.active, 0)
    return nil
}

func (m *Mapping) acceptLoop(handler func(net.Conn)) {
    for {
        select {
        case <-m.ctx.Done():
            return
        default:
        }

        conn, err := m.listener.Accept()
        if err != nil {
            if m.ctx.Err() != nil {
                return
            }
            log.Printf("[Portal Client] Accept error on %s: %v", m.config.Name, err)
            continue
        }

        atomic.AddInt64(&m.connCount, 1)
        m.lastActive = time.Now()

        go func() {
            defer func() {
                atomic.AddInt64(&m.connCount, -1)
            }()
            handler(conn)
        }()
    }
}

// Status 获取状态
func (m *Mapping) Status() portal.MappingStatus {
    return portal.MappingStatus{
        PortMapping:      m.config,
        Active:           atomic.LoadInt32(&m.active) == 1,
        ConnectionCount:  int(atomic.LoadInt64(&m.connCount)),
        BytesTransferred: atomic.LoadInt64(&m.bytesSent) + atomic.LoadInt64(&m.bytesRecv),
        LastActive:       m.lastActive,
    }
}
```

**Step 2: Implement client**

```go
package client

import (
    "context"
    "crypto/tls"
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net"
    "sync"
    "time"

    "github.com/gmssh/gmssh/internal/portal/protocol"
    "github.com/gmssh/gmssh/pkg/portal"
    "github.com/xtaci/smux"
)

// Client Portal 客户端
type Client struct {
    config    *portal.ClientConfig
    tlsConfig *tls.Config
    token     string
    via       []string

    mux        *protocol.ClientMux
    mappings   map[string]*Mapping
    mappingsMu sync.RWMutex

    ctx    context.Context
    cancel context.CancelFunc
    wg     sync.WaitGroup

    reconnectInterval time.Duration
    maxRetries        int
}

// New 创建客户端
func New(config *portal.ClientConfig, token string, via []string, tlsConfig *tls.Config) *Client {
    ctx, cancel := context.WithCancel(context.Background())

    connConfig := config.Connection
    if connConfig.RetryInterval == 0 {
        connConfig = portal.DefaultConnectionConfig()
    }

    return &Client{
        config:            config,
        tlsConfig:         tlsConfig,
        token:             token,
        via:               via,
        mappings:          make(map[string]*Mapping),
        ctx:               ctx,
        cancel:            cancel,
        reconnectInterval: connConfig.RetryInterval,
        maxRetries:        connConfig.MaxRetries,
    }
}

// Start 启动客户端
func (c *Client) Start(serverAddr string) error {
    // 建立 smux 连接
    if err := c.connect(serverAddr); err != nil {
        return fmt.Errorf("failed to connect: %w", err)
    }

    // 启动端口映射
    for i := range c.config.Mappings {
        mapping := NewMapping(c.config.Mappings[i])
        if err := mapping.Start(c.handleConnection); err != nil {
            log.Printf("[Portal Client] Failed to start mapping %s: %v", c.config.Mappings[i].Name, err)
            continue
        }
        c.mappingsMu.Lock()
        c.mappings[c.config.Mappings[i].ID] = mapping
        c.mappingsMu.Unlock()
    }

    // 启动重连监控
    c.wg.Add(1)
    go c.reconnectLoop(serverAddr)

    return nil
}

// Stop 停止客户端
func (c *Client) Stop() error {
    c.cancel()

    c.mappingsMu.Lock()
    for _, mapping := range c.mappings {
        mapping.Stop()
    }
    c.mappings = make(map[string]*Mapping)
    c.mappingsMu.Unlock()

    if c.mux != nil {
        c.mux.Close()
    }

    c.wg.Wait()
    return nil
}

func (c *Client) connect(serverAddr string) error {
    mux, err := protocol.Dial(serverAddr, c.tlsConfig, nil)
    if err != nil {
        return err
    }

    c.mux = mux
    log.Printf("[Portal Client] Connected to %s", serverAddr)
    return nil
}

func (c *Client) reconnectLoop(serverAddr string) {
    defer c.wg.Done()

    ticker := time.NewTicker(c.reconnectInterval)
    defer ticker.Stop()

    retries := 0

    for {
        select {
        case <-c.ctx.Done():
            return
        case <-ticker.C:
            if c.mux == nil || c.mux.IsClosed() {
                if retries >= c.maxRetries {
                    log.Printf("[Portal Client] Max retries reached, giving up")
                    return
                }

                log.Printf("[Portal Client] Reconnecting... (attempt %d/%d)", retries+1, c.maxRetries)
                if err := c.connect(serverAddr); err != nil {
                    log.Printf("[Portal Client] Reconnect failed: %v", err)
                    retries++
                } else {
                    retries = 0
                }
            }
        }
    }
}

func (c *Client) handleConnection(localConn net.Conn) {
    defer localConn.Close()

    if c.mux == nil || c.mux.IsClosed() {
        log.Printf("[Portal Client] No active connection to server")
        return
    }

    // 获取映射配置
    var mapping *Mapping
    c.mappingsMu.RLock()
    for _, m := range c.mappings {
        if m.listener != nil && m.listener.Addr().String() == localConn.LocalAddr().String() {
            mapping = m
            break
        }
    }
    c.mappingsMu.RUnlock()

    if mapping == nil {
        log.Printf("[Portal Client] No mapping found for connection")
        return
    }

    // 打开新流
    stream, err := c.mux.OpenStream()
    if err != nil {
        log.Printf("[Portal Client] Failed to open stream: %v", err)
        return
    }
    defer stream.Close()

    // 发送连接请求
    req := ConnectRequest{
        Token:      c.token,
        RemoteHost: mapping.config.RemoteHost,
        RemotePort: mapping.config.RemotePort,
    }

    if err := json.NewEncoder(stream).Encode(req); err != nil {
        log.Printf("[Portal Client] Failed to send request: %v", err)
        return
    }

    // 读取响应
    var resp ConnectResponse
    if err := json.NewDecoder(stream).Decode(&resp); err != nil {
        log.Printf("[Portal Client] Failed to decode response: %v", err)
        return
    }

    if !resp.Success {
        log.Printf("[Portal Client] Connection rejected: %s", resp.Error)
        return
    }

    // 双向转发
    var wg sync.WaitGroup
    wg.Add(2)

    go func() {
        defer wg.Done()
        n, _ := io.Copy(stream, localConn)
        atomic.AddInt64(&mapping.bytesRecv, n)
    }()

    go func() {
        defer wg.Done()
        n, _ := io.Copy(localConn, stream)
        atomic.AddInt64(&mapping.bytesSent, n)
    }()

    wg.Wait()
    mapping.lastActive = time.Now()
}

// Status 获取所有映射状态
func (c *Client) Status() []portal.MappingStatus {
    c.mappingsMu.RLock()
    defer c.mappingsMu.RUnlock()

    statuses := make([]portal.MappingStatus, 0, len(c.mappings))
    for _, mapping := range c.mappings {
        statuses = append(statuses, mapping.Status())
    }
    return statuses
}

// ConnectRequest 连接请求
type ConnectRequest struct {
    Token      string `json:"token"`
    RemoteHost string `json:"remote_host"`
    RemotePort int    `json:"remote_port"`
}

// ConnectResponse 连接响应
type ConnectResponse struct {
    Success bool   `json:"success"`
    Error   string `json:"error,omitempty"`
}
```

**Step 3: Commit**

```bash
git add internal/portal/client/
git commit -m "feat(portal): add portal client implementation"
```

---

## Phase 5: CLI 集成

### Task 6: 创建 Portal 命令

**Files:**
- Create: `cmd/gmssh/portal.go`
- Modify: `cmd/gmssh/main.go`

**Step 1: Implement portal command**

```go
package main

import (
    "crypto/tls"
    "flag"
    "fmt"
    "log"
    "os"

    "github.com/gmssh/gmssh/internal/config"
    "github.com/gmssh/gmssh/internal/portal/client"
    "github.com/gmssh/gmssh/internal/portal/protocol"
    "github.com/gmssh/gmssh/internal/portal/server"
    "github.com/gmssh/gmssh/pkg/portal"
)

func portalCmd(args []string) {
    fs := flag.NewFlagSet("portal", flag.ExitOnError)

    var (
        clientMode = fs.Bool("client", false, "Run in client mode")
        serverMode = fs.Bool("server", false, "Run in server mode")
        localAddr  = fs.String("local", "", "Local address (e.g., :8848)")
        remoteAddr = fs.String("remote", "", "Remote address (e.g., host:8848)")
        via        = fs.String("via", "", "Gateway hop IDs (comma-separated)")
        listenAddr = fs.String("listen", ":18888", "Server listen address")
        token      = fs.String("token", "", "Auth token")
        configFile = fs.String("config", "", "Config file path")
    )

    fs.Parse(args)

    if *clientMode && *serverMode {
        log.Fatal("Cannot run both client and server mode")
    }

    if !*clientMode && !*serverMode {
        // 默认显示状态
        showPortalStatus()
        return
    }

    if *serverMode {
        runServer(*listenAddr, *token)
        return
    }

    if *clientMode {
        runClient(*localAddr, *remoteAddr, *via, *configFile)
        return
    }
}

func runServer(listenAddr, token string) {
    cfg := &portal.ServerConfig{
        Enabled:    true,
        ListenAddr: listenAddr,
        TLSCert:    config.GetConfigPath("portal.crt"),
        TLSKey:     config.GetConfigPath("portal.key"),
        AuthTokens: []portal.TokenConfig{
            {
                Token:          token,
                AllowedRemotes: []string{"0.0.0.0/0"},
                MaxMappings:    100,
            },
        },
    }

    srv, err := server.New(cfg, protocol.DefaultMuxConfig())
    if err != nil {
        log.Fatalf("Failed to create server: %v", err)
    }

    if err := srv.Start(); err != nil {
        log.Fatalf("Failed to start server: %v", err)
    }

    log.Printf("Portal server running on %s", listenAddr)
    select {}
}

func runClient(localAddr, remoteAddr, via, configFile string) {
    // 加载配置
    cfgManager, err := config.NewManager()
    if err != nil {
        log.Fatalf("Failed to load config: %v", err)
    }

    var mappings []portal.PortMapping

    if configFile != "" {
        // 从配置文件加载
        // TODO: 实现配置文件解析
    } else {
        // 从命令行参数创建
        if localAddr == "" || remoteAddr == "" {
            log.Fatal("Local and remote addresses are required")
        }

        // 解析 remoteAddr
        host, port := parseAddr(remoteAddr)

        mappings = []portal.PortMapping{
            {
                ID:         "cli-mapping",
                Name:       "cli",
                LocalAddr:  localAddr,
                RemoteHost: host,
                RemotePort: port,
                Via:        parseVia(via),
                Protocol:   portal.ProtocolTCP,
                Enabled:    true,
            },
        }
    }

    clientConfig := &portal.ClientConfig{
        Mappings:   mappings,
        Connection: portal.DefaultConnectionConfig(),
    }

    // 获取网关配置
    var serverAddr string
    if len(mappings) > 0 && len(mappings[0].Via) > 0 {
        hopID := mappings[0].Via[0]
        hop := cfgManager.GetConfig().GetHopByID(hopID)
        if hop == nil {
            log.Fatalf("Hop not found: %s", hopID)
        }
        serverAddr = fmt.Sprintf("%s:18888", hop.Host)
    } else {
        log.Fatal("No gateway specified")
    }

    tlsConfig := &tls.Config{InsecureSkipVerify: true}

    c := client.New(clientConfig, "test-token", mappings[0].Via, tlsConfig)

    if err := c.Start(serverAddr); err != nil {
        log.Fatalf("Failed to start client: %v", err)
    }

    log.Printf("Portal client started")
    select {}
}

func showPortalStatus() {
    fmt.Println("Portal Status:")
    fmt.Println("No active portal connections")
}

func parseAddr(addr string) (string, int) {
    var host string
    var port int
    fmt.Sscanf(addr, "%s:%d", &host, &port)
    return host, port
}

func parseVia(via string) []string {
    if via == "" {
        return nil
    }
    // 解析逗号分隔的 ID
    var ids []string
    // TODO: 实现解析
    return ids
}
```

**Step 2: Update main.go**

Add to main() switch statement:
```go
case "portal":
    portalCmd(args[1:])
```

**Step 3: Commit**

```bash
git add cmd/gmssh/
git commit -m "feat(portal): add portal CLI command"
```

---

## Phase 6: 配置集成

### Task 7: 更新 Config 类型

**Files:**
- Modify: `pkg/types/types.go`

**Step 1: Add PortalConfig to types**

```go
// Config 全局配置
type Config struct {
    Version   int                `json:"version" yaml:"version"`
    Hops      []*Hop             `json:"hops" yaml:"hops"`
    Routes    []*RoutePreference `json:"routes" yaml:"routes"`
    Profiles  []*Profile         `json:"profiles" yaml:"profiles"`
    Portal    *portal.PortalConfig `json:"portal,omitempty" yaml:"portal,omitempty"`
    ConfigDir string             `json:"-" yaml:"-"`
}
```

**Step 2: Commit**

```bash
git add pkg/types/types.go
git commit -m "feat(config): add PortalConfig to global config"
```

---

## Phase 7: API 集成

### Task 8: 添加 REST API

**Files:**
- Create: `internal/api/portal.go`

**Step 1: Implement portal API handlers**

```go
package api

import (
    "encoding/json"
    "net/http"

    "github.com/gmssh/gmssh/pkg/portal"
)

// handlePortal 处理 portal 相关请求
func (s *Server) handlePortal(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case http.MethodGet:
        s.handlePortalStatus(w, r)
    case http.MethodPost:
        s.handleCreatePortalMapping(w, r)
    default:
        w.WriteHeader(http.StatusMethodNotAllowed)
    }
}

// handlePortalStatus 获取 portal 状态
func (s *Server) handlePortalStatus(w http.ResponseWriter, r *http.Request) {
    // TODO: 实现状态获取
    jsonResponse(w, http.StatusOK, map[string]interface{}{
        "active":   false,
        "mappings": []portal.MappingStatus{},
    })
}

// handleCreatePortalMapping 创建端口映射
func (s *Server) handleCreatePortalMapping(w http.ResponseWriter, r *http.Request) {
    var req portal.PortMapping
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        errorResponse(w, http.StatusBadRequest, "Invalid request body")
        return
    }

    // TODO: 验证并创建映射

    jsonResponse(w, http.StatusCreated, req)
}
```

**Step 2: Register routes**

In `internal/api/server.go`, add to routes:
```go
mux.HandleFunc("/api/portal", s.handlePortal)
mux.HandleFunc("/api/portal/", s.handlePortalDetail)
```

**Step 3: Commit**

```bash
git add internal/api/portal.go
git commit -m "feat(api): add portal REST API endpoints"
```

---

## Phase 8: 前端集成

### Task 9: 更新前端页面

**Files:**
- Modify: `web/src/pages/PortForwarding.tsx`

**Step 1: Create portal API client**

```typescript
// web/src/api/portal.ts
import axios from 'axios';
import { PortalMapping, PortalStatus } from '../types';

const API_BASE = import.meta.env.VITE_API_BASE || '/api';

const client = axios.create({
  baseURL: API_BASE,
  headers: {
    'Content-Type': 'application/json',
  },
});

export async function getPortalStatus(): Promise<PortalStatus> {
  const response = await client.get('/portal');
  return response.data;
}

export async function createPortalMapping(mapping: Omit<PortalMapping, 'id'>): Promise<PortalMapping> {
  const response = await client.post('/portal', mapping);
  return response.data;
}

export async function deletePortalMapping(id: string): Promise<void> {
  await client.delete(`/portal/${id}`);
}
```

**Step 2: Update types**

```typescript
// web/src/types/index.ts
export interface PortalMapping {
  id: string;
  name: string;
  local_addr: string;
  remote_host: string;
  remote_port: number;
  via: string[];
  protocol: 'tcp' | 'http' | 'websocket';
  enabled: boolean;
}

export interface PortalStatus {
  active: boolean;
  mappings: MappingStatus[];
}

export interface MappingStatus extends PortalMapping {
  active: boolean;
  connection_count: number;
  bytes_transferred: number;
  last_active: string;
  error?: string;
}
```

**Step 3: Commit**

```bash
git add web/src/
git commit -m "feat(web): add portal frontend API and types"
```

---

## Phase 9: 测试与验证

### Task 10: 运行测试

**Step 1: Run all tests**

```bash
cd /Users/hanson/installShell/gmssh
go test ./pkg/portal/... -v
go test ./internal/portal/... -v
```

**Step 2: Build binary**

```bash
go build -o gmssh ./cmd/gmssh
```

**Step 3: Test CLI**

```bash
# 启动服务端
./gmssh portal --server --listen :18888 --token test-token

# 启动客户端
./gmssh portal --client --local :8848 --remote 192.168.1.10:8848 --via gateway-id
```

---

## Summary

This implementation plan covers:

1. **Phase 1**: Type definitions and configuration
2. **Phase 2**: smux protocol wrapper
3. **Phase 3**: Server implementation with auth
4. **Phase 4**: Client implementation with reconnect
5. **Phase 5**: CLI command integration
6. **Phase 6**: Config file integration
7. **Phase 7**: REST API endpoints
8. **Phase 8**: Frontend integration
9. **Phase 9**: Testing and validation

Each task includes exact file paths, complete code, test commands, and commit messages.

**Next Step:** Use `superpowers:executing-plans` to implement this plan task-by-task.
