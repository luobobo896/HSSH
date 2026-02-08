# 构建阶段
FROM golang:1.21-alpine AS builder

WORKDIR /build

# 复制服务端代码（纯标准库，无需下载依赖）
COPY server/server.go .

# 构建静态二进制
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o uploader-server server.go

# 运行阶段
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

# 从构建阶段复制二进制
COPY --from=builder /build/uploader-server .

# 创建上传目录
RUN mkdir -p /data/uploads

# 暴露端口
EXPOSE 8080

# 环境变量
ENV UPLOAD_DIR=/data/uploads
ENV PORT=8080

# 运行
ENTRYPOINT ["./uploader-server"]
