---
title: SSH 跳板机系统设计文档
date: 2025-02-05
type: Spec
status: Approved
---

# SSH 跳板机系统设计文档

## 1. 架构概述

### 核心架构

采用**链式 SSH 客户端**设计，每个节点作为一个 `Hop` 对象，维护独立的 SSH 连接。

```
┌─────────┐     ┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│  本机   │────→│ 跳板机(可选) │────→│  外网服务器  │────→│  内网服务器  │
│  (Mac)  │     │ (香港/美国)  │     │  (网关角色)  │     │  (目标机器)  │
└─────────┘     └─────────────┘     └─────────────┘     └─────────────┘
```

### 关键组件

1. **Hop（跳点）**：封装单个 SSH 连接，包含主机地址、端口、认证信息
2. **Chain（链路）**：管理 Hop 列表，建立逐级连接
3. **FileTransfer（文件传输）**：基于 SCP/SFTP 的多跳文件上传，支持流式转发
4. **PortForwarder（端口转发）**：本地端口 → 远程端口的隧道
5. **NetworkProfiler（网络分析器）**：实时探测各路径延迟，智能选择最优路径

### 认证设计

每个 Hop 独立配置认证方式：

```go
type AuthMethod int
const (
    AuthKey AuthMethod = iota
    AuthPassword
)

type Hop struct {
    Host     string
    Port     int
    User     string
    AuthType AuthMethod
    KeyPath  string  // 用于 Key 认证
    Password string  // 用于密码认证（支持 keyring 存储）
}
```

## 2. 智能路径选择

### 路径选择机制

采用**配置优先 + 探测验证**策略：

```go
type RoutePreference struct {
    From       string  // 源节点标识
    To         string  // 目标节点标识
    Via        string  // 首选中转节点 (空=直连)
    Threshold  int     // 延迟差异阈值(ms)，超过则切换
}

type NetworkProfiler struct {
    cache      map[Path]LatencyReport
    cacheTTL   time.Duration  // 缓存有效期，如 5分钟
}

type Path struct {
    Hops []string  // 节点链，如 ["localhost", "bastion", "gateway", "internal"]
}
```

**决策流程：**
1. 检查配置中是否有预定义路由
2. 若无，探测直连 vs 经跳板 的延迟
3. 若探测结果与配置差异超过 Threshold，更新缓存并提示用户

### 文件传输数据流

**场景 A：本机→跳板机快，跳板机→外网快**
```
本机文件 ──SCP──→ 跳板机内存缓冲区 ──SCP──→ 外网 ──SCP──→ 内网
                    (流式转发，不落盘)
```

**场景 B：直连外网更快**
```
本机文件 ──SCP──────────────→ 外网 ──SCP──→ 内网
```

**流式转发的关键实现**：使用 `io.Pipe` 或内存缓冲区，避免在跳板机上落盘。

## 3. 端口转发与错误处理

### 端口转发实现

基于 SSH 的 `direct-tcpip` 通道实现多级转发：

```go
type PortForwarder struct {
    chain       *Chain      // SSH 连接链
    localAddr   string      // 本机监听地址，如 ":8080"
    remoteHost  string      // 最终目标，如 "internal-server"
    remotePort  int         // 目标端口，如 8080
}

// 建立本地监听，每个连接创建一条反向隧道链
func (pf *PortForwarder) Start() error {
    listener, _ := net.Listen("tcp", pf.localAddr)
    for {
        localConn, _ := listener.Accept()
        go pf.handleConnection(localConn)
    }
}

func (pf *PortForwarder) handleConnection(local net.Conn) {
    // 通过最末端的 SSH 客户端建立到目标的直接通道
    remote, _ := pf.chain.LastHop().Dial("tcp",
        fmt.Sprintf("%s:%d", pf.remoteHost, pf.remotePort))
    go io.Copy(remote, local)
    go io.Copy(local, remote)
}
```

### 错误处理策略

| 场景 | 处理方式 |
|------|----------|
| 某跳连接断开 | 自动重连（指数退避，最大3次） |
| 认证失败 | 立即报错，提示检查密钥/密码 |
| 传输中断 | 支持断点续传（SFTP 支持，SCP 需额外实现） |
| 路径性能下降 | 触发重新探测，提示用户切换路径 |

**连接池优化**：维护长连接，避免频繁建立 SSH 握手开销。

## 4. 配置与 CLI 设计

### 配置文件结构（YAML）

```yaml
# ~/.gmssh/config.yaml
hops:
  - name: bastion-hk
    host: hk.example.com
    port: 22
    user: admin
    auth: key
    key_path: ~/.ssh/id_rsa

  - name: bastion-us
    host: us.example.com
    port: 22
    user: admin
    auth: password
    # 密码从 keyring 获取，或配置中加密存储

  - name: gateway
    host: gateway.corp.com
    port: 22
    user: jumper
    auth: key
    key_path: ~/.ssh/corp_key

routes:
  - from: localhost
    to: gateway
    via: bastion-hk  # 默认经香港跳板
    threshold: 50    # 延迟差异超50ms则提示切换

  - from: localhost
    to: internal-web
    via: bastion-us  # 另一条路径

profiles:
  - name: upload-to-internal
    path: ["bastion-hk", "gateway", "internal-server"]
    target_dir: /data/uploads

  - name: proxy-db
    path: ["gateway"]
    local_port: 3306
    remote_host: internal-db
    remote_port: 3306
```

### CLI 设计

```bash
# 文件上传（自动选择最优路径）
gmssh upload ./local-file.txt --to internal-server:/data/

# 指定路径上传
gmssh upload ./local-file.txt --via bastion-hk,gateway --to internal-server:/data/

# 端口转发
gmssh proxy --to internal-db:3306 --local :3306

# 网络探测
gmssh probe --to internal-server

# 路径性能报告
gmssh status

# 启动 Web UI（本地模式）
gmssh web --local

# 启动 Web UI（服务器模式）
gmssh web --bind 0.0.0.0:8080
```

## 5. Web UI 技术架构

### 技术栈

| 层级 | 技术 | 用途 |
|------|------|------|
| 前端 | React + TypeScript + Tailwind CSS | 现代化界面，类型安全 |
| 状态管理 | Zustand | 轻量状态管理 |
| 后端 API | Go + Gin/Echo | 高性能 HTTP API |
| 实时通信 | WebSocket | 传输进度、终端会话 |
| 文件存储 | 本地 SQLite / 配置文件 | 服务器配置、路由规则 |

### API 设计

```go
// REST API
GET    /api/servers              // 列出所有服务器
POST   /api/servers              // 添加服务器
PUT    /api/servers/:id          // 更新服务器
DELETE /api/servers/:id          // 删除服务器

GET    /api/routes               // 获取路由配置
POST   /api/routes               // 创建路由

POST   /api/upload               // 上传文件（multipart/form-data）
POST   /api/proxy                // 创建端口转发
GET    /api/proxy                // 列出活跃代理
DELETE /api/proxy/:id            // 关闭代理

GET    /api/metrics/latency      // 获取节点延迟
GET    /api/metrics/bandwidth    // 获取带宽统计

// WebSocket
WS     /ws/terminal/:sessionId   // 交互式终端
WS     /ws/progress/:taskId      // 文件传输进度
```

### 核心页面

| 页面 | 功能 |
|------|------|
| **服务器管理** | 添加/编辑/删除 SSH 节点（跳板机、外网、内网） |
| **路由配置** | 可视化拖拽配置传输路径，自由组合节点 |
| **文件传输** | 拖拽上传文件，显示实时进度，选择目标路径 |
| **端口转发** | 创建/管理本地代理隧道，显示连接状态 |
| **性能监控** | 实时显示各节点延迟、传输速度、路由健康度 |

### 部署模式

**本地模式**（`gmssh web --local`）：
- 监听 `localhost:8080`
- 自动打开浏览器
- 配置文件存储在 `~/.gmssh/`

**服务器模式**（`gmssh web --bind 0.0.0.0:8080`）：
- 支持多用户（简单 token 认证或 LDAP）
- 配置文件持久化在指定路径
- 支持反向代理（Nginx/Traefik）

## 6. 项目结构

```
gmssh/
├── cmd/
│   └── gmssh/
│       └── main.go           # 入口
├── internal/
│   ├── ssh/                  # SSH 客户端、链路管理
│   ├── transfer/             # 文件传输（SCP/SFTP）
│   ├── proxy/                # 端口转发
│   ├── profiler/             # 网络探测
│   ├── config/               # 配置管理
│   └── api/                  # HTTP API + WebSocket
├── web/                      # React 前端
│   ├── src/
│   └── package.json
├── pkg/
│   └── types/                # 共享类型定义
├── go.mod
├── go.sum
└── README.md
```

## 7. 关键实现要点

### 高性能设计

1. **连接复用**：维护 SSH 连接池，避免频繁握手
2. **流式转发**：跳板机使用内存管道，不落临时文件
3. **并发传输**：大文件分块并发传输（类似 `rsync`）
4. **零拷贝**：使用 `splice` 或 `sendfile` 减少内核态拷贝

### 安全设计

1. **密钥管理**：支持 macOS Keychain，不硬编码密码
2. **权限隔离**：服务器模式下支持用户隔离配置
3. **审计日志**：记录所有文件传输和端口转发操作
4. **连接加密**：全程 SSH 协议加密，不暴露明文

## 8. 后续扩展

- [ ] 交互式终端（Web 版 SSH 客户端）
- [ ] 文件同步（双向 rsync 风格）
- [ ] 批量操作（多服务器并行执行命令）
- [ ] 历史记录（传输历史、命令历史）
