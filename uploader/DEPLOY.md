# 部署指南

## 快速开始（本地）

```bash
# 1. 构建
./deploy.sh all

# 2. 启动服务端（前台运行）
UPLOAD_DIR=./uploads PORT=8080 ./bin/uploader-server

# 3. 测试
curl http://localhost:8080/health
```

## 生产部署

### 方案 1: 一键部署（systemd）

```bash
# 本地一键部署
./deploy.sh quick

# 或分步执行
./deploy.sh all        # 构建
./deploy.sh install    # 安装到 /opt/uploader
./deploy.sh systemd    # 创建服务
./deploy.sh start      # 启动服务
```

服务管理：
```bash
systemctl status uploader-server   # 查看状态
systemctl stop uploader-server     # 停止
systemctl restart uploader-server  # 重启
journalctl -u uploader-server -f   # 查看日志
```

### 方案 2: Docker 部署

```bash
# 构建镜像
docker-compose build

# 启动
docker-compose up -d

# 查看日志
docker-compose logs -f

# 停止
docker-compose down
```

### 方案 3: 远程部署

```bash
# 部署到远程服务器
./deploy.sh remote user@192.168.1.100
```

需要确保：
- SSH 免密登录配置完成
- 远程服务器有 Go 环境（用于构建）

## 客户端使用

### 基础用法

```bash
# 配置环境变量
export HK_HOST=hk-relay.example.com
export GW_HOST=gateway.corp.internal
export SSH_USER=your_username
export SSH_KEY=~/.ssh/id_rsa
export GATEWAY_URL=http://gateway:8080
export WORKERS=10

# 上传文件
./bin/uploader file.xlsx
```

### 批量上传脚本

```bash
#!/bin/bash
# upload_batch.sh

UPLOADER="./bin/uploader"
REMOTE_DIR="/data/uploads/$(date +%Y%m%d)"

for file in "$@"; do
    echo "上传: $file"
    $UPLOADER "$file"
done
```

## 目录结构（部署后）

```
/opt/uploader/              # 服务端安装目录
├── bin/
│   └── uploader-server     # 服务端二进制
├── logs/
│   └── server.log          # 运行日志
└── .env                    # 环境变量

/data/uploads/              # 上传文件存储
└── .chunks/                # 临时分片目录
```

## 配置说明

### 服务端环境变量

| 变量 | 说明 | 默认值 |
|-----|------|--------|
| `UPLOAD_DIR` | 上传文件存储目录 | `/data/uploads` |
| `PORT` | HTTP 服务端口 | `8080` |

### 客户端环境变量

| 变量 | 说明 | 示例 |
|-----|------|------|
| `HK_HOST` | 香港中转服务器 | `hk-relay.example.com` |
| `GW_HOST` | 内网网关 | `gateway.corp.internal` |
| `SSH_USER` | SSH 用户名 | `fileuser` |
| `SSH_KEY` | SSH 私钥路径 | `~/.ssh/id_rsa` |
| `WORKERS` | 并发连接数 | `10` |
| `CHUNK_SIZE` | 分片大小 | `524288` (512KB) |
| `GATEWAY_URL` | 网关 HTTP API | `http://localhost:8080` |

## 性能调优

### 根据网络调整并发数

```bash
# 高延迟跨境链路（推荐）
export WORKERS=20

# 低延迟专线
export WORKERS=5
```

### 调整分片大小

```bash
# 高丢包网络（小分片）
export CHUNK_SIZE=262144  # 256KB

# 稳定网络（大分片）
export CHUNK_SIZE=1048576  # 1MB
```

## 故障排查

### 服务端无法启动

```bash
# 检查端口占用
lsof -i :8080

# 检查目录权限
ls -la /data/uploads

# 手动运行查看错误
/opt/uploader/bin/uploader-server
```

### 客户端上传失败

```bash
# 测试 SSH 连接
ssh -J $SSH_USER@$HK_HOST $SSH_USER@$GW_HOST "echo ok"

# 测试网关 API
curl $GATEWAY_URL/health

# 启用调试（修改客户端代码添加 -v 参数）
ssh -vvv ...
```

### 合并失败

```bash
# 检查分片是否完整
ls -la /data/uploads/.chunks/{upload_id}/

# 手动合并测试
cat /data/uploads/.chunks/xxx/chunk_* > /tmp/test.out
```

## 安全建议

1. **SSH 密钥**：使用专用上传密钥，限制命令执行权限
2. **防火墙**：网关只开放 8080 端口给香港服务器
3. **路径校验**：服务端已做目录遍历防护
4. **定期清理**：服务端自动清理 24h 未完成的上传

## 监控

```bash
# 查看服务状态
curl http://localhost:8080/health

# 查看上传状态
curl "http://localhost:8080/status?id=xxx"

# 磁盘使用
df -h /data/uploads
```
