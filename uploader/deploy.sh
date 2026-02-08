#!/bin/bash
#
# 一键构建部署脚本
# 用法: ./deploy.sh [server|client|all]
#

set -e

PROJECT_DIR="$(cd "$(dirname "$0")" && pwd)"
BUILD_DIR="$PROJECT_DIR/bin"
DEPLOY_DIR="/opt/uploader"
SERVICE_NAME="uploader-server"

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log() { echo -e "${GREEN}[INFO]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }

# 构建服务端
build_server() {
    log "构建服务端..."
    cd "$PROJECT_DIR/server"
    go build -ldflags="-s -w" -o "$BUILD_DIR/uploader-server" server.go
    log "服务端构建完成: $BUILD_DIR/uploader-server"
}

# 构建客户端
build_client() {
    log "构建客户端 (简单版)..."
    cd "$PROJECT_DIR/client_simple"
    go mod init uploader_simple 2>/dev/null || true
    go build -ldflags="-s -w" -o "$BUILD_DIR/uploader" main.go
    log "客户端构建完成: $BUILD_DIR/uploader"
}

# 交叉编译客户端
build_client_cross() {
    log "交叉编译客户端..."
    cd "$PROJECT_DIR/client_simple"

    # Linux AMD64
    GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o "$BUILD_DIR/uploader-linux-amd64" main.go
    log "  - Linux AMD64"

    # macOS AMD64
    GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o "$BUILD_DIR/uploader-darwin-amd64" main.go
    log "  - macOS AMD64"

    # macOS ARM64 (M1/M2)
    GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o "$BUILD_DIR/uploader-darwin-arm64" main.go
    log "  - macOS ARM64"

    # Windows
    GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o "$BUILD_DIR/uploader-windows-amd64.exe" main.go
    log "  - Windows AMD64"
}

# 本地部署服务端
install_local() {
    log "本地安装服务端..."

    # 创建目录
    sudo mkdir -p "$DEPLOY_DIR/bin"
    sudo mkdir -p "$DEPLOY_DIR/logs"
    sudo mkdir -p "/data/uploads"

    # 复制二进制
    sudo cp "$BUILD_DIR/uploader-server" "$DEPLOY_DIR/bin/"
    sudo chmod +x "$DEPLOY_DIR/bin/uploader-server"

    # 创建环境变量文件
    sudo tee "$DEPLOY_DIR/.env" > /dev/null <<'EOF'
# 上传服务配置
UPLOAD_DIR=/data/uploads
PORT=8080
EOF

    # 创建启动脚本
    sudo tee "$DEPLOY_DIR/start.sh" > /dev/null <<'EOF'
#!/bin/bash
cd /opt/uploader
source .env
exec ./bin/uploader-server >> logs/server.log 2>&1
EOF
    sudo chmod +x "$DEPLOY_DIR/start.sh"

    log "本地安装完成: $DEPLOY_DIR"
}

# 创建 systemd 服务
install_systemd() {
    log "创建 systemd 服务..."

    sudo tee "/etc/systemd/system/$SERVICE_NAME.service" > /dev/null <<EOF
[Unit]
Description=File Upload Server
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=$DEPLOY_DIR
Environment="UPLOAD_DIR=/data/uploads"
Environment="PORT=8080"
ExecStart=$DEPLOY_DIR/bin/uploader-server
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

    sudo systemctl daemon-reload
    log "systemd 服务创建完成"
}

# 启动服务
start_service() {
    log "启动服务..."
    sudo systemctl start "$SERVICE_NAME"
    sudo systemctl enable "$SERVICE_NAME"
    sleep 1
    if systemctl is-active --quiet "$SERVICE_NAME"; then
        log "服务运行成功"
        systemctl status "$SERVICE_NAME" --no-pager
    else
        error "服务启动失败"
    fi
}

# 停止服务
stop_service() {
    log "停止服务..."
    sudo systemctl stop "$SERVICE_NAME" 2>/dev/null || true
}

# 检查状态
status_service() {
    systemctl status "$SERVICE_NAME" --no-pager 2>/dev/null || echo "服务未安装"
}

# 部署到远程服务器
deploy_remote() {
    local host="$1"
    if [[ -z "$host" ]]; then
        error "请指定远程服务器地址: ./deploy.sh remote user@host"
    fi

    log "部署到远程服务器: $host"

    # 构建
    build_server

    # 上传
    ssh "$host" "mkdir -p $DEPLOY_DIR/bin $DEPLOY_DIR/logs /data/uploads"
    scp "$BUILD_DIR/uploader-server" "$host:$DEPLOY_DIR/bin/"
    scp "$PROJECT_DIR/scripts/install-server.sh" "$host:/tmp/"

    # 执行远程安装
    ssh "$host" "bash /tmp/install-server.sh"

    log "远程部署完成"
}

# 显示使用帮助
usage() {
    cat <<EOF
用法: $0 [命令]

构建命令:
    server          构建服务端
    client          构建客户端
    client-all      交叉编译所有平台客户端
    all             构建服务端和客户端

部署命令:
    install         本地安装服务端
    systemd         创建 systemd 服务
    start           启动服务
    stop            停止服务
    status          查看服务状态
    restart         重启服务
    remote HOST     部署到远程服务器 (如: user@192.168.1.100)

快速部署:
    quick           构建 + 安装 + 启动 (本地完整部署)

示例:
    $0 all                          # 构建所有组件
    $0 quick                        # 本地一键部署
    $0 remote admin@192.168.1.100   # 部署到远程服务器
EOF
}

# 主逻辑
case "${1:-}" in
    server)
        build_server
        ;;
    client)
        build_client
        ;;
    client-all)
        build_client_cross
        ;;
    all)
        mkdir -p "$BUILD_DIR"
        build_server
        build_client
        ;;
    install)
        install_local
        ;;
    systemd)
        install_systemd
        ;;
    start)
        start_service
        ;;
    stop)
        stop_service
        ;;
    status)
        status_service
        ;;
    restart)
        stop_service
        start_service
        ;;
    remote)
        deploy_remote "$2"
        ;;
    quick)
        log "快速部署模式..."
        mkdir -p "$BUILD_DIR"
        build_server
        build_client
        install_local
        install_systemd
        start_service
        log "部署完成！"
        log "服务地址: http://localhost:8080"
        log "上传目录: /data/uploads"
        ;;
    *)
        usage
        exit 1
        ;;
esac
