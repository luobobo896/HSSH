package portal

import (
	"testing"
	"time"
)

func TestDefaultConnectionConfig(t *testing.T) {
	cfg := DefaultConnectionConfig()
	if cfg.RetryInterval != 5*time.Second {
		t.Errorf("expected retry interval 5s, got %v", cfg.RetryInterval)
	}
	if cfg.MaxRetries != 10 {
		t.Errorf("expected max retries 10, got %d", cfg.MaxRetries)
	}
	if cfg.KeepaliveInterval != 30*time.Second {
		t.Errorf("expected keepalive interval 30s, got %v", cfg.KeepaliveInterval)
	}
}

func TestPortMapping(t *testing.T) {
	m := PortMapping{
		ID:         "test-id",
		Name:       "test-mapping",
		LocalAddr:  ":8848",
		RemoteHost: "192.168.1.10",
		RemotePort: 8848,
		Via:        []string{"gateway-1"},
		Protocol:   ProtocolHTTP,
		Enabled:    true,
	}

	if m.Name != "test-mapping" {
		t.Errorf("expected name 'test-mapping', got %s", m.Name)
	}
	if m.Protocol != ProtocolHTTP {
		t.Errorf("expected protocol http, got %s", m.Protocol)
	}
}
