package client

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/gmssh/gmssh/internal/portal/protocol"
	"github.com/gmssh/gmssh/pkg/portal"
	"github.com/gmssh/gmssh/pkg/types"
)

// generateTestTLSConfig generates a self-signed TLS certificate for testing
func generateTestTLSConfig(t *testing.T) *tls.Config {
	t.Helper()

	// Generate RSA key
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	// Generate certificate
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:              []string{"localhost"},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("Failed to create certificate: %v", err)
	}

	// Encode certificate and key
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})

	// Load certificate
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("Failed to load key pair: %v", err)
	}

	return &tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: true,
	}
}

// startTestServer starts a test portal server
func startTestServer(t *testing.T, tlsConfig *tls.Config) (string, *protocol.ServerMux, func()) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}

	var mux *protocol.ServerMux
	var muxOnce sync.Once
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())

	wg.Add(1)
	go func() {
		defer wg.Done()

		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				t.Errorf("Failed to accept: %v", err)
				return
			}
		}

		m, err := protocol.NewServerMux(conn, tlsConfig, nil)
		if err != nil {
			t.Errorf("Failed to create server mux: %v", err)
			conn.Close()
			return
		}

		muxOnce.Do(func() {
			mux = m
		})

		// Keep the mux open until cleanup
		<-ctx.Done()
		m.Close()
	}()

	cleanup := func() {
		cancel()
		listener.Close()
		wg.Wait()
	}

	return listener.Addr().String(), mux, cleanup
}

func TestNewClient(t *testing.T) {
	config := &portal.ClientConfig{
		Mappings: []portal.PortMapping{
			{
				ID:         "test-1",
				Name:       "Test Mapping",
				LocalAddr:  "127.0.0.1:0",
				RemoteHost: "remote.example.com",
				RemotePort: 8080,
				Protocol:   portal.ProtocolTCP,
				Enabled:    true,
			},
		},
	}

	tlsConfig := &tls.Config{InsecureSkipVerify: true}
	client := NewClient(config, tlsConfig, "test-token", "127.0.0.1:18080")

	if client == nil {
		t.Fatal("Expected client to be non-nil")
	}

	if client.config != config {
		t.Error("Expected config to match")
	}

	if client.token != "test-token" {
		t.Errorf("Expected token to be 'test-token', got %s", client.token)
	}

	if client.serverAddr != "127.0.0.1:18080" {
		t.Errorf("Expected serverAddr to be '127.0.0.1:18080', got %s", client.serverAddr)
	}

	if len(client.mappings) != 0 {
		t.Errorf("Expected empty mappings, got %d", len(client.mappings))
	}
}

func TestClientConnect(t *testing.T) {
	tlsConfig := generateTestTLSConfig(t)
	serverAddr, _, cleanup := startTestServer(t, tlsConfig)
	defer cleanup()

	config := &portal.ClientConfig{}
	client := NewClient(config, tlsConfig, "test-token", serverAddr)

	err := client.Connect()
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	if !client.IsConnected() {
		t.Error("Expected client to be connected")
	}

	if client.mux == nil {
		t.Error("Expected mux to be non-nil")
	}

	if client.conn == nil {
		t.Error("Expected conn to be non-nil")
	}
}

func TestClientConnectFailure(t *testing.T) {
	config := &portal.ClientConfig{}
	tlsConfig := &tls.Config{InsecureSkipVerify: true}
	client := NewClient(config, tlsConfig, "test-token", "127.0.0.1:1")

	err := client.Connect()
	if err == nil {
		t.Error("Expected connection to fail")
	}
}

func TestClientClose(t *testing.T) {
	tlsConfig := generateTestTLSConfig(t)
	serverAddr, _, cleanup := startTestServer(t, tlsConfig)
	defer cleanup()

	config := &portal.ClientConfig{}
	client := NewClient(config, tlsConfig, "test-token", serverAddr)

	err := client.Connect()
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	if !client.IsConnected() {
		t.Error("Expected client to be connected before close")
	}

	err = client.Close()
	if err != nil {
		t.Errorf("Close returned error: %v", err)
	}

	// Wait a bit for the connection to close
	time.Sleep(100 * time.Millisecond)

	if client.IsConnected() {
		t.Error("Expected client to be disconnected after close")
	}
}

func TestStartMapping(t *testing.T) {
	tlsConfig := generateTestTLSConfig(t)
	serverAddr, _, cleanup := startTestServer(t, tlsConfig)
	defer cleanup()

	config := &portal.ClientConfig{}
	client := NewClient(config, tlsConfig, "test-token", serverAddr)

	err := client.Connect()
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	mapping := portal.PortMapping{
		ID:         "test-mapping",
		Name:       "Test",
		LocalAddr:  "127.0.0.1:0",
		RemoteHost: "remote.example.com",
		RemotePort: 8080,
		Protocol:   portal.ProtocolTCP,
		Enabled:    true,
	}

	err = client.StartMapping(mapping)
	if err != nil {
		t.Fatalf("Failed to start mapping: %v", err)
	}

	// Check that mapping exists
	client.mu.RLock()
	state, ok := client.mappings["test-mapping"]
	client.mu.RUnlock()

	if !ok {
		t.Fatal("Expected mapping to exist")
	}

	if !state.Active.Load() {
		t.Error("Expected mapping to be active")
	}

	if state.Listener == nil {
		t.Error("Expected listener to be non-nil")
	}

	// Clean up the mapping
	err = client.StopMapping("test-mapping")
	if err != nil {
		t.Errorf("Failed to stop mapping: %v", err)
	}
}

func TestStartMappingNotConnected(t *testing.T) {
	config := &portal.ClientConfig{}
	tlsConfig := &tls.Config{InsecureSkipVerify: true}
	client := NewClient(config, tlsConfig, "test-token", "127.0.0.1:18080")

	mapping := portal.PortMapping{
		ID:         "test-mapping",
		Name:       "Test",
		LocalAddr:  "127.0.0.1:18081",
		RemoteHost: "remote.example.com",
		RemotePort: 8080,
	}

	err := client.StartMapping(mapping)
	if err == nil {
		t.Error("Expected error when starting mapping without connection")
	}
}

func TestStopMappingNotFound(t *testing.T) {
	tlsConfig := generateTestTLSConfig(t)
	serverAddr, _, cleanup := startTestServer(t, tlsConfig)
	defer cleanup()

	config := &portal.ClientConfig{}
	client := NewClient(config, tlsConfig, "test-token", serverAddr)

	err := client.Connect()
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	err = client.StopMapping("non-existent")
	if err == nil {
		t.Error("Expected error when stopping non-existent mapping")
	}
}

func TestGetMappingStatus(t *testing.T) {
	tlsConfig := generateTestTLSConfig(t)
	serverAddr, _, cleanup := startTestServer(t, tlsConfig)
	defer cleanup()

	config := &portal.ClientConfig{}
	client := NewClient(config, tlsConfig, "test-token", serverAddr)

	err := client.Connect()
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	// Initially should be empty
	status := client.GetMappingStatus()
	if len(status) != 0 {
		t.Errorf("Expected 0 mappings, got %d", len(status))
	}

	// Add a mapping
	mapping := portal.PortMapping{
		ID:         "test-mapping",
		Name:       "Test",
		LocalAddr:  "127.0.0.1:0",
		RemoteHost: "remote.example.com",
		RemotePort: 8080,
		Protocol:   portal.ProtocolTCP,
		Enabled:    true,
	}

	err = client.StartMapping(mapping)
	if err != nil {
		t.Fatalf("Failed to start mapping: %v", err)
	}

	// Check status
	status = client.GetMappingStatus()
	if len(status) != 1 {
		t.Fatalf("Expected 1 mapping, got %d", len(status))
	}

	if status[0].ID != "test-mapping" {
		t.Errorf("Expected ID 'test-mapping', got %s", status[0].ID)
	}

	if !status[0].Active {
		t.Error("Expected mapping to be active")
	}

	// Clean up
	client.StopMapping("test-mapping")
}

// Manager Tests

func TestNewManager(t *testing.T) {
	manager := NewManager()
	if manager == nil {
		t.Fatal("Expected manager to be non-nil")
	}

	if len(manager.clients) != 0 {
		t.Errorf("Expected empty clients map, got %d", len(manager.clients))
	}
}

func TestManagerAddAndGetClient(t *testing.T) {
	manager := NewManager()

	config := &portal.ClientConfig{}
	tlsConfig := &tls.Config{InsecureSkipVerify: true}
	client := NewClient(config, tlsConfig, "test-token", "127.0.0.1:18080")

	manager.AddClient("server1", client)

	retrieved, ok := manager.GetClient("server1")
	if !ok {
		t.Error("Expected to find client")
	}

	if retrieved != client {
		t.Error("Expected retrieved client to match original")
	}

	// Try to get non-existent
	_, ok = manager.GetClient("server2")
	if ok {
		t.Error("Expected not to find non-existent client")
	}
}

func TestManagerRemoveClient(t *testing.T) {
	manager := NewManager()

	config := &portal.ClientConfig{}
	tlsConfig := &tls.Config{InsecureSkipVerify: true}
	client := NewClient(config, tlsConfig, "test-token", "127.0.0.1:18080")

	manager.AddClient("server1", client)

	_, ok := manager.GetClient("server1")
	if !ok {
		t.Fatal("Expected to find client before removal")
	}

	manager.RemoveClient("server1")

	_, ok = manager.GetClient("server1")
	if ok {
		t.Error("Expected not to find client after removal")
	}
}

func TestManagerStopAll(t *testing.T) {
	tlsConfig := generateTestTLSConfig(t)
	serverAddr, _, cleanup := startTestServer(t, tlsConfig)
	defer cleanup()

	manager := NewManager()

	config := &portal.ClientConfig{}
	client := NewClient(config, tlsConfig, "test-token", serverAddr)

	err := client.Connect()
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	manager.AddClient("server1", client)

	if !client.IsConnected() {
		t.Error("Expected client to be connected before StopAll")
	}

	manager.StopAll()

	// Wait a bit for close to complete
	time.Sleep(100 * time.Millisecond)

	if client.IsConnected() {
		t.Error("Expected client to be disconnected after StopAll")
	}
}

func TestManagerGetAllStatus(t *testing.T) {
	manager := NewManager()

	config := &portal.ClientConfig{}
	tlsConfig := &tls.Config{InsecureSkipVerify: true}
	client := NewClient(config, tlsConfig, "test-token", "127.0.0.1:18080")

	manager.AddClient("server1", client)

	status := manager.GetAllStatus()
	if len(status) != 1 {
		t.Errorf("Expected 1 server status, got %d", len(status))
	}

	if _, ok := status["server1"]; !ok {
		t.Error("Expected status for server1")
	}
}

// SSHTunnel Tests

func TestNewSSHTunnel(t *testing.T) {
	hops := []*types.Hop{
		{
			Name:     "hop1",
			Host:     "host1.example.com",
			Port:     22,
			User:     "user1",
			AuthType: types.AuthKey,
			KeyPath:  "~/.ssh/id_rsa",
		},
	}

	tunnel := NewSSHTunnel(hops)

	if tunnel == nil {
		t.Fatal("Expected tunnel to be non-nil")
	}

	if tunnel.chain == nil {
		t.Error("Expected chain to be non-nil")
	}
}

func TestSSHTunnelIsConnected(t *testing.T) {
	hops := []*types.Hop{
		{
			Name:     "hop1",
			Host:     "host1.example.com",
			Port:     22,
			User:     "user1",
			AuthType: types.AuthKey,
			KeyPath:  "~/.ssh/id_rsa",
		},
	}

	tunnel := NewSSHTunnel(hops)

	if tunnel.IsConnected() {
		t.Error("Expected tunnel to be disconnected initially")
	}
}

func TestSSHTunnelClose(t *testing.T) {
	hops := []*types.Hop{
		{
			Name:     "hop1",
			Host:     "host1.example.com",
			Port:     22,
			User:     "user1",
			AuthType: types.AuthKey,
			KeyPath:  "~/.ssh/id_rsa",
		},
	}

	tunnel := NewSSHTunnel(hops)

	err := tunnel.Close()
	if err != nil {
		t.Errorf("Close returned error: %v", err)
	}
}

func TestSSHTunnelGetChain(t *testing.T) {
	hops := []*types.Hop{
		{
			Name:     "hop1",
			Host:     "host1.example.com",
			Port:     22,
			User:     "user1",
			AuthType: types.AuthKey,
			KeyPath:  "~/.ssh/id_rsa",
		},
	}

	tunnel := NewSSHTunnel(hops)

	chain := tunnel.GetChain()
	if chain == nil {
		t.Error("Expected chain to be non-nil")
	}
}
