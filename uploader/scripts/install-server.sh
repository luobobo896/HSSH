#!/bin/bash
# 远程服务器安装脚本

set -e

DEPLOY_DIR="/opt/uploader"

# 创建目录
mkdir -p "$DEPLOY_DIR/logs"
mkdir -p "/data/uploads"

# 设置权限
chmod +x "$DEPLOY_DIR/bin/uploader-server"

# 创建环境文件
cat > "$DEPLOY_DIR/.env" <<'EOF'
UPLOAD_DIR=/data/uploads
PORT=8080
EOF

# 创建 systemd 服务
cat > "/etc/systemd/system/uploader-server.service" <<EOF
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
StandardOutput=append:$DEPLOY_DIR/logs/server.log
StandardError=append:$DEPLOY_DIR/logs/server.log

[Install]
WantedBy=multi-user.target
EOF

# 启动服务
systemctl daemon-reload
systemctl enable uploader-server
systemctl start uploader-server

echo "安装完成"
systemctl status uploader-server --no-pager
