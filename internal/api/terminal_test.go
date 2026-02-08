package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/luobobo896/HSSH/pkg/types"
	"github.com/gorilla/websocket"
)

func TestHandleTerminal_ExternalServer(t *testing.T) {
	server, err := NewServer()
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Add a test external server
	server.config.Hops = append(server.config.Hops, &types.Hop{
		Name:       "test-external",
		Host:       "localhost",
		Port:       2222,
		User:       "test",
		AuthType:   types.AuthPassword,
		Password:   "test",
		ServerType: types.ServerExternal,
	})

	// Create test HTTP server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.handleTerminal(w, r)
	}))
	defer ts.Close()

	// Convert http:// to ws://
	wsURL := strings.Replace(ts.URL, "http://", "ws://", 1) + "?server=test-external"

	// Connect WebSocket
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		// Expected to fail since there's no actual SSH server
		t.Logf("WebSocket connection failed as expected: %v", err)
		return
	}
	defer ws.Close()

	// Set a read deadline
	ws.SetReadDeadline(time.Now().Add(2 * time.Second))

	// Try to read the first message (connection status)
	_, msg, err := ws.ReadMessage()
	if err != nil {
		t.Logf("Read message failed as expected: %v", err)
		return
	}

	var status map[string]interface{}
	if err := json.Unmarshal(msg, &status); err != nil {
		t.Fatalf("Failed to unmarshal status: %v", err)
	}

	t.Logf("Received status: %v", status)
}

func TestHandleTerminal_InternalServerWithGateway(t *testing.T) {
	server, err := NewServer()
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Add gateway server
	server.config.Hops = append(server.config.Hops, &types.Hop{
		Name:       "gateway",
		Host:       "localhost",
		Port:       2222,
		User:       "test",
		AuthType:   types.AuthPassword,
		Password:   "test",
		ServerType: types.ServerExternal,
	})

	// Add internal server
	server.config.Hops = append(server.config.Hops, &types.Hop{
		Name:       "test-internal",
		Host:       "192.168.1.100",
		Port:       22,
		User:       "test",
		AuthType:   types.AuthPassword,
		Password:   "test",
		ServerType: types.ServerInternal,
		Gateway:    "gateway",
	})

	// Create test HTTP server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.handleTerminal(w, r)
	}))
	defer ts.Close()

	wsURL := strings.Replace(ts.URL, "http://", "ws://", 1) + "?server=test-internal"

	// Connect WebSocket
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Logf("WebSocket connection failed as expected (no actual SSH): %v", err)
		return
	}
	defer ws.Close()

	t.Log("WebSocket connected for internal server through gateway")
}

func TestHandleTerminal_ServerNotFound(t *testing.T) {
	server, err := NewServer()
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Create test HTTP server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.handleTerminal(w, r)
	}))
	defer ts.Close()

	wsURL := strings.Replace(ts.URL, "http://", "ws://", 1) + "?server=non-existent"

	// Connect WebSocket - should fail with 404
	_, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err == nil {
		t.Fatal("Expected connection to fail for non-existent server")
	}

	if resp != nil && resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.StatusCode)
	}
}

func TestHandleTerminal_MissingServerParam(t *testing.T) {
	server, err := NewServer()
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Create test HTTP server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.handleTerminal(w, r)
	}))
	defer ts.Close()

	wsURL := strings.Replace(ts.URL, "http://", "ws://", 1) // No server param

	// Connect WebSocket - should fail with 400
	_, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err == nil {
		t.Fatal("Expected connection to fail without server parameter")
	}

	if resp != nil && resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}

func TestBuildHopChain_ExternalServer(t *testing.T) {
	server := &Server{
		config: &types.Config{
			Hops: []*types.Hop{
				{
					Name:       "external-server",
					Host:       "example.com",
					Port:       22,
					User:       "root",
					ServerType: types.ServerExternal,
				},
			},
		},
	}

	chain := server.buildHopChain("external-server")
	if len(chain) != 1 {
		t.Errorf("Expected 1 hop in chain, got %d", len(chain))
	}

	if chain[0].Name != "external-server" {
		t.Errorf("Expected server name 'external-server', got '%s'", chain[0].Name)
	}
}

func TestBuildHopChain_InternalServer(t *testing.T) {
	server := &Server{
		config: &types.Config{
			Hops: []*types.Hop{
				{
					Name:       "gateway",
					Host:       "gateway.example.com",
					Port:       22,
					User:       "root",
					ServerType: types.ServerExternal,
				},
				{
					Name:       "internal-server",
					Host:       "192.168.1.100",
					Port:       22,
					User:       "root",
					ServerType: types.ServerInternal,
					Gateway:    "gateway",
				},
			},
		},
	}

	chain := server.buildHopChain("internal-server")
	if len(chain) != 2 {
		t.Errorf("Expected 2 hops in chain (gateway + target), got %d", len(chain))
	}

	if chain[0].Name != "gateway" {
		t.Errorf("Expected first hop to be gateway, got '%s'", chain[0].Name)
	}

	if chain[1].Name != "internal-server" {
		t.Errorf("Expected second hop to be internal-server, got '%s'", chain[1].Name)
	}
}

func TestBuildHopChain_InternalServerNoGateway(t *testing.T) {
	server := &Server{
		config: &types.Config{
			Hops: []*types.Hop{
				{
					Name:       "internal-server",
					Host:       "192.168.1.100",
					Port:       22,
					User:       "root",
					ServerType: types.ServerInternal,
					// No Gateway configured
				},
			},
		},
	}

	chain := server.buildHopChain("internal-server")
	// Should return just the target server even without gateway
	// The connection will fail later when trying to connect
	if len(chain) != 1 {
		t.Errorf("Expected 1 hop in chain when gateway missing, got %d", len(chain))
	}
}

func TestTerminalMessageTypes(t *testing.T) {
	// Test message structure
	input := TerminalInput{
		Type: "input",
		Data: "ls -la\n",
	}

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Failed to marshal input: %v", err)
	}

	var decoded TerminalInput
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal input: %v", err)
	}

	if decoded.Type != "input" {
		t.Errorf("Expected type 'input', got '%s'", decoded.Type)
	}

	if decoded.Data != "ls -la\n" {
		t.Errorf("Expected data 'ls -la\\n', got '%s'", decoded.Data)
	}
}
