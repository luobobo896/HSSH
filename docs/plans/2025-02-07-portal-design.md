---
title: GMPortal 高性能端口转发/内网穿透工具设计
date: 2025-02-07
type: Spec
status: Approved
---

# GMPortal 设计文档

## 1. 概述

### 1.1 背景

HSSH 是一个 SSH 跳板机工具，支持多跳 SSH 连接、文件传输和端口转发。当前项目需要实现一个高性能的端口转发/内网穿透模块，使本地用户能够通过 SSH 网关安全访问内网服务（如 Nacos、MySQL 等）。

### 1.2 目标

- 支持 HTTP/WebSocket 协议透传
- 单连接支持 500+ 并发
- 使用连接多路复用降低延迟
- 提供 CLI 和 Web UI 两种控制方式
- 复用现有 HSSH 配置系统（Hop ID）

### 1.3 非目标

- 不替代专业内网穿透工具（如 frp、ngrok）
- 不支持 UDP 协议（首版）
- 不提供公网服务发现机制

## 2. 架构设计

### 2.1 整体架构

```
┌─────────────────────────────────────────────────────────────────┐
│                          GMPortal 架构                           │
├─────────────────────────────────────────────────────────────────┤
│  ┌──────────────┐      ┌──────────────┐      ┌──────────────┐   │
│  │   Client     │◄────►│   Tunnel     │◄────►│   Server     │   │
│  │   (本地)      │ smux │   (SSH链)     │ smux │   (网关)      │   │
│  └──────────────┘      └──────────────┘      └──────────────┘   │
│         │                                           │            │
│         ▼                                           ▼            │
│  ┌──────────────┐                           ┌──────────────┐   │
│  │  localhost   │                           │  Internal    │   │
│  │  :8848/nacos │                           │  Nacos:8848  │   │
│  └──────────────┘                           └──────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

### 2.2 组件职责

| 组件 | 职责 | 部署位置 |
|-----|------|---------|
| Client | 本地端口监听、流量转发、smux 客户端 | 用户本地机器 |
| Tunnel | SSH 连接管理、链式跳转 | 复用 HSSH SSH 层 |
| Server | smux 服务端、目标地址拨号、认证 | 网关服务器 |

### 2.3 技术选型

- **多路复用**: smux (github.com/xtaci/smux)
  - 现代设计，性能优于 yamux
  - 支持 1000+ streams per session
  - 内置流控和 KeepAlive

- **传输安全**: SSH + TLS 双层加密
  - SSH 作为底层传输隧道
  - TLS 1.3 在 SSH 内二次加密 smux 控制流

- **协议支持**: TCP 透传 + HTTP/WebSocket 特殊处理

## 3. 核心设计

### 3.1 连接模型

```
┌─────────────────────────────────────────────────────────────┐
│                    连接建立流程                              │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  1. SSH Connection (复用 HSSH)                              │
│     Local ──SSH──► Gateway (Jump Host)                       │
│                                                             │
│  2. TLS Handshake (在 SSH Channel 内)                        │
│     Client ──TLS 1.3──► Server                               │
│     • 可选: 双向证书认证                                      │
│                                                             │
│  3. smux Session 建立                                        │
│     • Version Negotiation                                   │
│     • KeepAlive 配置 (30s interval)                         │
│     • MaxStreams: 1000                                      │
│                                                             │
│  4. Token 认证 (Control Stream 0)                            │
│     Client ──AUTH──► Server                                 │
│     • Token 验证                                            │
│     • 权限校验 (allowed_remotes)                            │
│     • 配额检查 (max_mappings)                               │
│                                                             │
│  5. 端口映射注册 (Per Mapping)                               │
│     Stream Open: {local_port, remote_host, remote_port}     │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### 3.2 数据传输

```
┌─────────────────────────────────────────────────────────────┐
│                    数据转发模型                              │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Browser ──HTTP──► Local:8848                               │
│                         │                                   │
│                         ▼                                   │
│                    ┌─────────┐                              │
│                    │ Accept  │                              │
│                    │ Conn    │                              │
│                    └────┬────┘                              │
│                         │                                   │
│                    Open smux Stream {id: 8848:001}          │
│                         │                                   │
│                         ▼                                   │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │   Client     │──│ smux Stream  │──│   Server     │      │
│  │   (local)    │  │ (multiplex)  │  │   (gateway)  │      │
│  └──────────────┘  └──────────────┘  └──────┬───────┘      │
│                                              │              │
│                                         Dial remote         │
│                                         (nacos:8848)        │
│                                              │              │
│                                              ▼              │
│  ┌─────────────────────────────────────────────────────┐   │
│  │              双向数据 Copy (goroutine)                 │   │
│  │  LocalConn ◄──────────────────────────────► RemoteConn │   │
│  │         • io.CopyBuffer with sync.Pool                  │   │
│  │         • TCP_NODELAY enabled                           │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### 3.3 WebSocket 处理

WebSocket 连接作为普通 TCP 连接处理，不做应用层解析：

- HTTP Upgrade 请求通过 smux stream 透传
- WebSocket 帧直接透传，不做组装/拆分
- 保持帧边界（通过 TCP 流的有序性保证）

## 4. 配置设计

### 4.1 配置文件扩展

```yaml
# ~/.gmssh/config.yaml
version: 2
hops:
  - id: "uuid-gateway-hk"
    name: "gateway-hk"
    host: "hk.example.com"
    port: 22
    user: "admin"
    auth_type: 0
    key_path: "~/.ssh/id_rsa"
    server_type: 0  # external

  - id: "uuid-nacos-server"
    name: "nacos-server"
    host: "192.168.1.10"
    port: 22
    user: "root"
    auth_type: 1
    server_type: 1  # internal
    gateway_id: "uuid-gateway-hk"

# 新增 portal 配置段
portal:
  client:
    mappings:
      - name: "nacos-console"
        local_addr: ":8848"
        remote_host: "192.168.1.10"
        remote_port: 8848
        via: ["uuid-gateway-hk"]
        protocol: "http"

      - name: "mysql-dev"
        local_addr: ":3306"
        remote_host: "192.168.1.20"
        remote_port: 3306
        via: ["uuid-gateway-hk"]
        protocol: "tcp"

    connection:
      retry_interval: 5s
      max_retries: 10
      keepalive_interval: 30s

  server:
    enabled: false
    listen_addr: ":18888"
    tls_cert: "~/.gmssh/portal.crt"
    tls_key: "~/.gmssh/portal.key"
    auth_tokens:
      - token: "${PORTAL_TOKEN}"
        allowed_remotes: ["192.168.0.0/16", "10.0.0.0/8"]
        max_mappings: 10
```

### 4.2 类型定义

```go
// pkg/portal/types.go
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
    ID           string   `json:"id" yaml:"id"`
    Name         string   `json:"name" yaml:"name"`
    LocalAddr    string   `json:"local_addr" yaml:"local_addr"`
    RemoteHost   string   `json:"remote_host" yaml:"remote_host"`
    RemotePort   int      `json:"remote_port" yaml:"remote_port"`
    Via          []string `json:"via" yaml:"via"`           // Hop ID 列表
    Protocol     Protocol `json:"protocol" yaml:"protocol"`
    Enabled      bool     `json:"enabled" yaml:"enabled"`
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
    Enabled     bool           `json:"enabled" yaml:"enabled"`
    ListenAddr  string         `json:"listen_addr" yaml:"listen_addr"`
    TLSCert     string         `json:"tls_cert" yaml:"tls_cert"`
    TLSKey      string         `json:"tls_key" yaml:"tls_key"`
    AuthTokens  []TokenConfig  `json:"auth_tokens" yaml:"auth_tokens"`
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
```

## 5. CLI 设计

### 5.1 命令结构

```bash
# 客户端模式 - 单端口映射
gmssh portal --client \
  --local :8848 \
  --remote 192.168.1.10:8848 \
  --via uuid-gateway-hk

# 多端口映射（配置文件模式）
gmssh portal --client --config ~/.gmssh/portal.yaml

# 服务端模式
gmssh portal --server \
  --listen :18888 \
  --token "${PORTAL_TOKEN}"

# 查看状态
gmssh portal status

# 停止指定映射
gmssh portal stop nacos-console

# 重新加载配置
gmssh portal reload
```

### 5.2 状态输出示例

```
$ gmssh portal status
NAME            LOCAL     REMOTE              VIA           STATUS  CONN  BYTES
nacos-console   :8848     192.168.1.10:8848   gateway-hk    active  12    1.2MB
mysql-dev       :3306     192.168.1.20:3306   gateway-hk    active  3     45KB
redis-test      :6379     192.168.1.30:6379   gateway-hk    error   0     0B
```

## 6. API 设计

### 6.1 REST API

```go
// POST /api/portal/mappings
// 创建端口映射
type CreateMappingRequest struct {
    Name       string   `json:"name"`
    LocalAddr  string   `json:"local_addr"`
    RemoteHost string   `json:"remote_host"`
    RemotePort int      `json:"remote_port"`
    Via        []string `json:"via"`
    Protocol   string   `json:"protocol"`
}

// GET /api/portal/status
// 获取当前状态
type PortalStatusResponse struct {
    Active    bool            `json:"active"`
    Mappings  []MappingStatus `json:"mappings"`
    Session   *SessionInfo    `json:"session,omitempty"`
}

type SessionInfo struct {
    ID           string    `json:"id"`
    ConnectedAt  time.Time `json:"connected_at"`
    LastPingAt   time.Time `json:"last_ping_at"`
    BytesSent    int64     `json:"bytes_sent"`
    BytesRecv    int64     `json:"bytes_recv"`
}

// DELETE /api/portal/mappings/:id
// 停止指定映射
```

### 6.2 WebSocket 实时推送

```javascript
// WebSocket 连接用于实时状态推送
const ws = new WebSocket('ws://localhost:18080/api/portal/ws');

ws.onmessage = (event) => {
  const update = JSON.parse(event.data);
  // update.type: 'status' | 'metrics' | 'error'
  // update.data: MappingStatus
};
```

## 7. 安全设计

### 7.1 传输安全

| 层级 | 机制 | 说明 |
|-----|------|-----|
| L1 | SSH 加密 | 复用 HSSH 的 SSH 连接 |
| L2 | TLS 1.3 | 在 SSH Channel 内二次加密 |
| L3 | Token 认证 | PSK 预共享密钥认证 |

### 7.2 访问控制

- **Token 白名单**: 每个 Token 绑定允许访问的目标网段
- **敏感端口保护**: 22, 3306, 6379 等需要显式声明
- **localhost 禁止**: 禁止通过 portal 访问服务端本地回环地址

### 7.3 审计日志

```go
// 日志事件类型
const (
    EventSessionConnect    = "session_connect"
    EventSessionDisconnect = "session_disconnect"
    EventMappingCreate     = "mapping_create"
    EventMappingDelete     = "mapping_delete"
    EventAuthFailed        = "auth_failed"
    EventAccessDenied      = "access_denied"
)
```

## 8. 错误处理

### 8.1 错误码定义

```go
const (
    ErrCodeOK              = 0  // 成功
    ErrCodeAuthFailed      = 1  // 认证失败
    ErrCodeUnauthorized    = 2  // 未授权访问
    ErrCodeQuotaExceeded   = 3  // 配额超限
    ErrCodeRemoteDenied    = 4  // 目标地址被禁止
    ErrCodeProtocolError   = 5  // 协议错误
    ErrCodeNetworkError    = 6  // 网络错误
    ErrCodeInternal        = 99 // 内部错误
)
```

### 8.2 重试策略

| 错误场景 | 处理方式 | 重试策略 |
|---------|---------|---------|
| SSH 断开 | 自动重连 | 指数退避 1s→2s→4s... 最大 30s |
| smux 失效 | 重建 session | 立即重试，最多 3 次 |
| 单 stream 失败 | 关闭该 stream | 不 retry，新建 stream |
| 目标不可达 | 返回错误 | 不 retry，立即报错 |
| TLS 证书错误 | 终止连接 | 不重试，人工介入 |

## 9. 性能目标

| 指标 | 目标值 | 测试方法 |
|-----|-------|---------|
| 并发连接 | 500+ | wrk -c 500 |
| 吞吐量 | 100MB/s+ | iperf3 |
| 延迟开销 | < 5ms | ping vs portal-ping |
| 内存占用 | < 200MB | 500 并发时 RSS |
| 重连恢复 | < 3s | 网络断开测试 |

## 10. 实现计划

### Phase 1: 核心框架
- [ ] `pkg/portal/types` - 类型定义
- [ ] `internal/portal/protocol` - smux 封装
- [ ] `internal/portal/client` - 客户端实现
- [ ] `internal/portal/server` - 服务端实现

### Phase 2: 集成
- [ ] `cmd/gmssh/portal` - CLI 命令
- [ ] 配置文件解析集成
- [ ] SSH 连接复用集成

### Phase 3: API & UI
- [ ] REST API 实现
- [ ] WebSocket 推送
- [ ] 前端页面实现

### Phase 4: 优化
- [ ] 性能测试与优化
- [ ] 压力测试
- [ ] 文档完善

## 11. 依赖

```go
// go.mod
require (
    github.com/xtaci/smux v1.5.24
    github.com/google/uuid v1.6.0
)
```

## 12. 参考

- [smux 文档](https://github.com/xtaci/smux)
- [HSSH 架构文档](./ARCHITECTURE.md)
