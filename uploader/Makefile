.PHONY: all build client server simple clean install

BINARY_NAME=uploader
SERVER_NAME=uploader-server
SIMPLE_NAME=uploader_simple
GO=go
GOFLAGS=-ldflags="-s -w"

all: build

build: client server simple

client:
	@echo "Building client..."
	cd client && $(GO) build $(GOFLAGS) -o ../bin/$(BINARY_NAME) .

server:
	@echo "Building server..."
	cd server && $(GO) build $(GOFLAGS) -o ../bin/$(SERVER_NAME) .

simple:
	@echo "Building simple client (no dependencies)..."
	cd client_simple && $(GO) build $(GOFLAGS) -o ../bin/$(SIMPLE_NAME) .

# 交叉编译 - 客户端需要在多个平台运行
client-linux:
	@echo "Building client for Linux..."
	cd client && GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) -o ../bin/$(BINARY_NAME)-linux-amd64 .

client-mac:
	@echo "Building client for macOS..."
	cd client && GOOS=darwin GOARCH=amd64 $(GO) build $(GOFLAGS) -o ../bin/$(BINARY_NAME)-darwin-amd64 .
	cd client && GOOS=darwin GOARCH=arm64 $(GO) build $(GOFLAGS) -o ../bin/$(BINARY_NAME)-darwin-arm64 .

client-windows:
	@echo "Building client for Windows..."
	cd client && GOOS=windows GOARCH=amd64 $(GO) build $(GOFLAGS) -o ../bin/$(BINARY_NAME)-windows-amd64.exe .

# 简单版本交叉编译
simple-linux:
	@echo "Building simple client for Linux..."
	cd client_simple && GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) -o ../bin/$(SIMPLE_NAME)-linux-amd64 .

deps:
	$(GO) mod download
	$(GO) mod tidy

install: build
	@echo "Installing to /usr/local/bin..."
	cp bin/$(BINARY_NAME) /usr/local/bin/
	cp bin/$(SERVER_NAME) /usr/local/bin/

clean:
	rm -rf bin/

run-server: server
	./bin/$(SERVER_NAME)

test:
	$(GO) test -v -race ./...

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

# 性能测试
benchmark:
	@echo "Running benchmark..."
	dd if=/dev/zero of=/tmp/test_100m bs=1M count=100
	time ./bin/$(BINARY_NAME) /tmp/test_100m
