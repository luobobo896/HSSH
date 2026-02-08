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

func setupPortalTestServer(t *testing.T) (*Server, string) {
	// 创建临时配置目录
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, ".gmssh")
	os.MkdirAll(configDir, 0700)

	// 创建初始配置文件
	initialConfig := `version: 2
hops:
  - id: test-gateway
    name: gateway
    host: 1.2.3.4
    port: 22
    user: root
    auth: 0
    server_type: 0
portal:
  client:
    mappings:
      - id: test-mapping-1
        name: test-mapping
        local_addr: :8080
        remote_host: internal.example.com
        remote_port: 80
        protocol: tcp
        enabled: true
        via:
          - test-gateway
  server:
    enabled: false
`
	configPath := filepath.Join(configDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(initialConfig), 0600); err != nil {
		t.Fatalf("failed to write initial config: %v", err)
	}

	// 设置环境变量让 config.Manager 使用临时目录
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	t.Cleanup(func() { os.Setenv("HOME", originalHome) })

	// 创建 API 服务器
	server, err := NewServer()
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	return server, tempDir
}

func TestHandlePortalStatus(t *testing.T) {
	server, _ := setupPortalTestServer(t)

	// 测试获取 Portal 状态
	req := httptest.NewRequest(http.MethodGet, "/api/portal", nil)
	w := httptest.NewRecorder()

	server.handlePortal(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response PortalStatusResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if !response.Active {
		t.Error("expected portal to be active")
	}

	if len(response.Mappings) != 1 {
		t.Errorf("expected 1 mapping, got %d", len(response.Mappings))
	}

	if len(response.Mappings) > 0 {
		m := response.Mappings[0]
		if m.ID != "test-mapping-1" {
			t.Errorf("expected mapping id 'test-mapping-1', got '%s'", m.ID)
		}
		if m.Name != "test-mapping" {
			t.Errorf("expected mapping name 'test-mapping', got '%s'", m.Name)
		}
		if m.LocalAddr != ":8080" {
			t.Errorf("expected local_addr ':8080', got '%s'", m.LocalAddr)
		}
	}
}

func TestHandleListPortalMappings(t *testing.T) {
	server, _ := setupPortalTestServer(t)

	// 测试列出所有映射
	req := httptest.NewRequest(http.MethodGet, "/api/portal/mappings", nil)
	w := httptest.NewRecorder()

	server.handlePortalMappings(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var mappings []PortalMappingStatus
	if err := json.Unmarshal(w.Body.Bytes(), &mappings); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(mappings) != 1 {
		t.Errorf("expected 1 mapping, got %d", len(mappings))
	}
}

func TestHandleCreatePortalMapping(t *testing.T) {
	server, _ := setupPortalTestServer(t)

	// 测试创建新映射
	reqBody := CreatePortalMappingRequest{
		Name:       "new-mapping",
		LocalAddr:  ":9090",
		RemoteHost: "new.example.com",
		RemotePort: 443,
		Protocol:   "tcp",
		Via:        []string{"test-gateway"},
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/portal/mappings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.handlePortalMappings(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var response PortalMappingStatus
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response.Name != "new-mapping" {
		t.Errorf("expected name 'new-mapping', got '%s'", response.Name)
	}
	if response.LocalAddr != ":9090" {
		t.Errorf("expected local_addr ':9090', got '%s'", response.LocalAddr)
	}
	if response.RemoteHost != "new.example.com" {
		t.Errorf("expected remote_host 'new.example.com', got '%s'", response.RemoteHost)
	}
	if response.RemotePort != 443 {
		t.Errorf("expected remote_port 443, got %d", response.RemotePort)
	}
	if response.Protocol != "tcp" {
		t.Errorf("expected protocol 'tcp', got '%s'", response.Protocol)
	}
	if !response.Enabled {
		t.Error("expected mapping to be enabled")
	}

	// 验证映射已保存到配置
	if len(server.config.Portal.Client.Mappings) != 2 {
		t.Errorf("expected 2 mappings in config, got %d", len(server.config.Portal.Client.Mappings))
	}
}

func TestHandleCreatePortalMappingValidation(t *testing.T) {
	server, _ := setupPortalTestServer(t)

	tests := []struct {
		name       string
		req        CreatePortalMappingRequest
		wantStatus int
	}{
		{
			name:       "missing name",
			req:        CreatePortalMappingRequest{LocalAddr: ":8080", RemoteHost: "test.com", RemotePort: 80},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing local_addr",
			req:        CreatePortalMappingRequest{Name: "test", RemoteHost: "test.com", RemotePort: 80},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing remote_host",
			req:        CreatePortalMappingRequest{Name: "test", LocalAddr: ":8080", RemotePort: 80},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing remote_port",
			req:        CreatePortalMappingRequest{Name: "test", LocalAddr: ":8080", RemoteHost: "test.com"},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.req)
			req := httptest.NewRequest(http.MethodPost, "/api/portal/mappings", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			server.handlePortalMappings(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d: %s", tt.wantStatus, w.Code, w.Body.String())
			}
		})
	}
}

func TestHandleGetPortalMapping(t *testing.T) {
	server, _ := setupPortalTestServer(t)

	// 测试获取存在的映射
	req := httptest.NewRequest(http.MethodGet, "/api/portal/mappings/test-mapping-1", nil)
	w := httptest.NewRecorder()

	server.handlePortalMappingDetail(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response PortalMappingStatus
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response.ID != "test-mapping-1" {
		t.Errorf("expected id 'test-mapping-1', got '%s'", response.ID)
	}

	// 测试获取不存在的映射
	req2 := httptest.NewRequest(http.MethodGet, "/api/portal/mappings/non-existent", nil)
	w2 := httptest.NewRecorder()

	server.handlePortalMappingDetail(w2, req2)

	if w2.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w2.Code)
	}
}

func TestHandleDeletePortalMapping(t *testing.T) {
	server, _ := setupPortalTestServer(t)

	// 验证初始状态
	if len(server.config.Portal.Client.Mappings) != 1 {
		t.Fatalf("expected 1 mapping initially, got %d", len(server.config.Portal.Client.Mappings))
	}

	// 测试删除存在的映射
	req := httptest.NewRequest(http.MethodDelete, "/api/portal/mappings/test-mapping-1", nil)
	w := httptest.NewRecorder()

	server.handlePortalMappingDetail(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d: %s", w.Code, w.Body.String())
	}

	// 验证映射已删除
	if len(server.config.Portal.Client.Mappings) != 0 {
		t.Errorf("expected 0 mappings after delete, got %d", len(server.config.Portal.Client.Mappings))
	}

	// 测试删除不存在的映射
	req2 := httptest.NewRequest(http.MethodDelete, "/api/portal/mappings/non-existent", nil)
	w2 := httptest.NewRecorder()

	server.handlePortalMappingDetail(w2, req2)

	if w2.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w2.Code)
	}
}

func TestHandleUpdatePortalMapping(t *testing.T) {
	server, _ := setupPortalTestServer(t)

	// 测试更新映射
	reqBody := CreatePortalMappingRequest{
		Name:       "updated-mapping",
		LocalAddr:  ":9090",
		RemoteHost: "updated.example.com",
		RemotePort: 443,
		Protocol:   "http",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPut, "/api/portal/mappings/test-mapping-1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.handlePortalMappingDetail(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response PortalMappingStatus
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response.Name != "updated-mapping" {
		t.Errorf("expected name 'updated-mapping', got '%s'", response.Name)
	}
	if response.LocalAddr != ":9090" {
		t.Errorf("expected local_addr ':9090', got '%s'", response.LocalAddr)
	}
	if response.RemoteHost != "updated.example.com" {
		t.Errorf("expected remote_host 'updated.example.com', got '%s'", response.RemoteHost)
	}
	if response.RemotePort != 443 {
		t.Errorf("expected remote_port 443, got %d", response.RemotePort)
	}
	if response.Protocol != "http" {
		t.Errorf("expected protocol 'http', got '%s'", response.Protocol)
	}

	// 验证配置已更新
	for _, m := range server.config.Portal.Client.Mappings {
		if m.ID == "test-mapping-1" {
			if m.Name != "updated-mapping" {
				t.Errorf("config name not updated: got '%s'", m.Name)
			}
			break
		}
	}

	// 测试更新不存在的映射
	req2 := httptest.NewRequest(http.MethodPut, "/api/portal/mappings/non-existent", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()

	server.handlePortalMappingDetail(w2, req2)

	if w2.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w2.Code)
	}
}

func TestDefaultProtocol(t *testing.T) {
	server, _ := setupPortalTestServer(t)

	// 测试不指定协议时默认为 tcp
	reqBody := CreatePortalMappingRequest{
		Name:       "default-protocol-test",
		LocalAddr:  ":9090",
		RemoteHost: "test.example.com",
		RemotePort: 80,
		// Protocol 为空
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/portal/mappings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.handlePortalMappings(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var response PortalMappingStatus
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response.Protocol != "tcp" {
		t.Errorf("expected default protocol 'tcp', got '%s'", response.Protocol)
	}
}

func TestPortalMappingTypes(t *testing.T) {
	// 验证请求/响应类型定义
	req := CreatePortalMappingRequest{
		Name:       "test",
		LocalAddr:  ":8080",
		RemoteHost: "example.com",
		RemotePort: 80,
		Via:        []string{"gateway1", "gateway2"},
		Protocol:   "websocket",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	var decoded CreatePortalMappingRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal request: %v", err)
	}

	if decoded.Name != req.Name {
		t.Error("Name mismatch")
	}
	if decoded.LocalAddr != req.LocalAddr {
		t.Error("LocalAddr mismatch")
	}
	if decoded.RemoteHost != req.RemoteHost {
		t.Error("RemoteHost mismatch")
	}
	if decoded.RemotePort != req.RemotePort {
		t.Error("RemotePort mismatch")
	}
	if len(decoded.Via) != len(req.Via) {
		t.Error("Via mismatch")
	}
	if decoded.Protocol != req.Protocol {
		t.Error("Protocol mismatch")
	}

	// 验证响应类型
	status := PortalMappingStatus{
		ID:               "test-id",
		Name:             "test",
		LocalAddr:        ":8080",
		RemoteHost:       "example.com",
		RemotePort:       80,
		Protocol:         "tcp",
		Enabled:          true,
		Active:           true,
		ConnectionCount:  5,
		BytesTransferred: 1024,
	}

	data, err = json.Marshal(status)
	if err != nil {
		t.Fatalf("failed to marshal status: %v", err)
	}

	var decodedStatus PortalMappingStatus
	if err := json.Unmarshal(data, &decodedStatus); err != nil {
		t.Fatalf("failed to unmarshal status: %v", err)
	}

	if decodedStatus.ConnectionCount != 5 {
		t.Errorf("ConnectionCount mismatch: expected 5, got %d", decodedStatus.ConnectionCount)
	}
	if decodedStatus.BytesTransferred != 1024 {
		t.Errorf("BytesTransferred mismatch: expected 1024, got %d", decodedStatus.BytesTransferred)
	}

	// 验证状态响应类型
	portalStatus := PortalStatusResponse{
		Active:     true,
		Mappings:   []PortalMappingStatus{status},
		ServerAddr: "localhost:8080",
	}

	data, err = json.Marshal(portalStatus)
	if err != nil {
		t.Fatalf("failed to marshal portal status: %v", err)
	}

	var decodedPortalStatus PortalStatusResponse
	if err := json.Unmarshal(data, &decodedPortalStatus); err != nil {
		t.Fatalf("failed to unmarshal portal status: %v", err)
	}

	if !decodedPortalStatus.Active {
		t.Error("Active mismatch")
	}
	if len(decodedPortalStatus.Mappings) != 1 {
		t.Errorf("Mappings length mismatch: expected 1, got %d", len(decodedPortalStatus.Mappings))
	}
	if decodedPortalStatus.ServerAddr != "localhost:8080" {
		t.Errorf("ServerAddr mismatch: expected 'localhost:8080', got '%s'", decodedPortalStatus.ServerAddr)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	server, _ := setupPortalTestServer(t)

	// 测试 /api/portal 不支持 POST
	req := httptest.NewRequest(http.MethodPost, "/api/portal", nil)
	w := httptest.NewRecorder()
	server.handlePortal(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}

	// 测试 /api/portal/mappings 不支持 PUT/DELETE
	req2 := httptest.NewRequest(http.MethodPut, "/api/portal/mappings", nil)
	w2 := httptest.NewRecorder()
	server.handlePortalMappings(w2, req2)
	if w2.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w2.Code)
	}

	req3 := httptest.NewRequest(http.MethodDelete, "/api/portal/mappings", nil)
	w3 := httptest.NewRecorder()
	server.handlePortalMappings(w3, req3)
	if w3.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w3.Code)
	}

	// 测试 /api/portal/mappings/:id 不支持 POST
	req4 := httptest.NewRequest(http.MethodPost, "/api/portal/mappings/test-id", nil)
	w4 := httptest.NewRecorder()
	server.handlePortalMappingDetail(w4, req4)
	if w4.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w4.Code)
	}
}

func TestEmptyMappings(t *testing.T) {
	// 创建没有映射的配置
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, ".gmssh")
	os.MkdirAll(configDir, 0700)

	initialConfig := `version: 2
hops: []
portal:
  client:
    mappings: []
`
	configPath := filepath.Join(configDir, "config.yaml")
	os.WriteFile(configPath, []byte(initialConfig), 0600)

	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	server, err := NewServer()
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// 测试空映射列表
	req := httptest.NewRequest(http.MethodGet, "/api/portal", nil)
	w := httptest.NewRecorder()
	server.handlePortal(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response PortalStatusResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response.Active {
		t.Error("expected portal to be inactive with no mappings")
	}

	if len(response.Mappings) != 0 {
		t.Errorf("expected 0 mappings, got %d", len(response.Mappings))
	}
}

func TestPortalWithTypesPackage(t *testing.T) {
	// 验证 API 类型与 types 包的兼容性
	mapping := types.PortMapping{
		ID:         "test-id",
		Name:       "test-mapping",
		LocalAddr:  ":8080",
		RemoteHost: "example.com",
		RemotePort: 80,
		Via:        []string{"gateway"},
		Protocol:   types.PortalProtocolTCP,
		Enabled:    true,
	}

	// 转换为 API 响应类型
	status := PortalMappingStatus{
		ID:         mapping.ID,
		Name:       mapping.Name,
		LocalAddr:  mapping.LocalAddr,
		RemoteHost: mapping.RemoteHost,
		RemotePort: mapping.RemotePort,
		Protocol:   string(mapping.Protocol),
		Enabled:    mapping.Enabled,
		Active:     mapping.Enabled,
	}

	if status.ID != mapping.ID {
		t.Error("ID mismatch")
	}
	if status.Protocol != "tcp" {
		t.Errorf("Protocol conversion failed: expected 'tcp', got '%s'", status.Protocol)
	}
}
