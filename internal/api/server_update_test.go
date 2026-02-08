package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/luobobo896/HSSH/pkg/types"
)

func TestUpdateServerWithGateway(t *testing.T) {
	// 创建临时配置目录
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, ".gmssh")
	os.MkdirAll(configDir, 0700)

	// 创建初始配置文件
	initialConfig := `hops:
  - name: gateway
    host: 1.2.3.4
    port: 22
    user: root
    auth: 0
    server_type: 0
  - name: internal-server
    host: 192.168.1.100
    port: 22
    user: root
    auth: 0
    server_type: 1
`
	configPath := filepath.Join(configDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(initialConfig), 0600); err != nil {
		t.Fatalf("failed to write initial config: %v", err)
	}

	// 设置环境变量让 config.Manager 使用临时目录
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	// 创建 API 服务器
	server, err := NewServer()
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// 测试1: 更新内网服务器，设置网关（使用数字 server_type）
	reqBody := CreateServerRequest{
		Host:       "192.168.1.100",
		Port:       22,
		User:       "root",
		AuthType:   "key",
		ServerType: "1", // 数字格式的内网类型
		GatewayID:  "gateway",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPut, "/api/servers/internal-server", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.handleServerDetail(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// 解析响应
	var response types.Hop
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response.GatewayID != "gateway" {
		t.Errorf("expected gateway_id 'gateway', got '%s'", response.GatewayID)
	}

	if response.ServerType != types.ServerInternal {
		t.Errorf("expected server_type %d, got %d", types.ServerInternal, response.ServerType)
	}

	// 测试2: 重新获取服务器列表，验证 gateway 是否保存
	req2 := httptest.NewRequest(http.MethodGet, "/api/servers", nil)
	w2 := httptest.NewRecorder()
	server.handleServers(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w2.Code)
	}

	var hops []*types.Hop
	if err := json.Unmarshal(w2.Body.Bytes(), &hops); err != nil {
		t.Fatalf("failed to unmarshal hops: %v", err)
	}

	var internalServer *types.Hop
	for _, h := range hops {
		if h.Name == "internal-server" {
			internalServer = h
			break
		}
	}

	if internalServer == nil {
		t.Fatal("internal-server not found in response")
	}

	if internalServer.GatewayID != "gateway" {
		t.Errorf("after reload: expected gateway_id 'gateway', got '%s'", internalServer.GatewayID)
	}

	t.Logf("Server data: Name=%s, GatewayID=%s, ServerType=%d",
		internalServer.Name, internalServer.GatewayID, internalServer.ServerType)
}
