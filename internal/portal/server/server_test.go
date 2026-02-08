package server

import (
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

	"github.com/luobobo896/HSSH/internal/portal/protocol"
	"github.com/luobobo896/HSSH/pkg/portal"
)

// generateTestTLSConfig generates a self-signed TLS config for testing
func generateTestTLSConfig() (*tls.Config, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

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
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: true,
	}, nil
}

func TestNewServer(t *testing.T) {
	config := &portal.ServerConfig{
		Enabled:    true,
		ListenAddr: "127.0.0.1:0",
	}

	tlsConfig, err := generateTestTLSConfig()
	if err != nil {
		t.Fatalf("Failed to generate TLS config: %v", err)
	}

	server := NewServer(config, tlsConfig)
	if server == nil {
		t.Fatal("Expected server to be created")
	}

	if server.config != config {
		t.Error("Expected config to be set")
	}

	if server.tlsConfig != tlsConfig {
		t.Error("Expected TLS config to be set")
	}

	if server.mappings == nil {
		t.Error("Expected mappings to be initialized")
	}

	if server.IsRunning() {
		t.Error("Expected server to not be running initially")
	}
}

func TestServerListen(t *testing.T) {
	config := &portal.ServerConfig{
		Enabled:    true,
		ListenAddr: "127.0.0.1:0",
	}

	tlsConfig, err := generateTestTLSConfig()
	if err != nil {
		t.Fatalf("Failed to generate TLS config: %v", err)
	}

	server := NewServer(config, tlsConfig)

	// Test listening
	err = server.Listen("")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}

	if server.listener == nil {
		t.Error("Expected listener to be set")
	}

	// Get the actual address
	addr := server.listener.Addr().String()
	if addr == "" {
		t.Error("Expected address to be set")
	}

	// Clean up
	err = server.Close()
	if err != nil {
		t.Fatalf("Failed to close server: %v", err)
	}
}

func TestServerListenWithExplicitAddr(t *testing.T) {
	config := &portal.ServerConfig{
		Enabled: true,
		// No ListenAddr set
	}

	tlsConfig, err := generateTestTLSConfig()
	if err != nil {
		t.Fatalf("Failed to generate TLS config: %v", err)
	}

	server := NewServer(config, tlsConfig)

	// Test listening with explicit address
	err = server.Listen("127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}

	if server.listener == nil {
		t.Error("Expected listener to be set")
	}

	// Clean up
	err = server.Close()
	if err != nil {
		t.Fatalf("Failed to close server: %v", err)
	}
}

func TestServerClose(t *testing.T) {
	config := &portal.ServerConfig{
		Enabled:    true,
		ListenAddr: "127.0.0.1:0",
	}

	tlsConfig, err := generateTestTLSConfig()
	if err != nil {
		t.Fatalf("Failed to generate TLS config: %v", err)
	}

	server := NewServer(config, tlsConfig)

	// Listen first
	err = server.Listen("")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}

	// Close
	err = server.Close()
	if err != nil {
		t.Fatalf("Failed to close server: %v", err)
	}
}

func TestMappingState(t *testing.T) {
	mapping := portal.PortMapping{
		ID:         "test-id",
		Name:       "test-mapping",
		LocalAddr:  "127.0.0.1:8080",
		RemoteHost: "127.0.0.1",
		RemotePort: 80,
		Enabled:    true,
	}

	state := &MappingState{
		Mapping: mapping,
	}

	// Test atomic operations
	state.StreamCount.Add(1)
	if state.StreamCount.Load() != 1 {
		t.Error("Expected stream count to be 1")
	}

	state.BytesIn.Add(100)
	if state.BytesIn.Load() != 100 {
		t.Error("Expected bytes in to be 100")
	}

	state.BytesOut.Add(200)
	if state.BytesOut.Load() != 200 {
		t.Error("Expected bytes out to be 200")
	}
}

func TestAuthenticator(t *testing.T) {
	tokens := []portal.TokenConfig{
		{
			Token:          "valid-token-1",
			AllowedRemotes: []string{"127.0.0.1/32", "10.0.0.0/8"},
			MaxMappings:    5,
		},
		{
			Token:          "valid-token-2",
			AllowedRemotes: []string{}, // No restrictions
			MaxMappings:    10,
		},
		{
			Token:          "valid-token-3",
			AllowedRemotes: []string{"0.0.0.0/0"},
			MaxMappings:    3,
		},
	}

	auth := NewAuthenticator(tokens)

	// Test valid token
	config, err := auth.ValidateToken("valid-token-1")
	if err != nil {
		t.Errorf("Expected token to be valid: %v", err)
	}
	if config == nil {
		t.Error("Expected config to be returned")
	}
	if config.Token != "valid-token-1" {
		t.Error("Expected token to match")
	}

	// Test invalid token
	config, err = auth.ValidateToken("invalid-token")
	if err == nil {
		t.Error("Expected invalid token to return error")
	}
	if config != nil {
		t.Error("Expected config to be nil for invalid token")
	}

	// Test no tokens (empty authenticator)
	emptyAuth := NewAuthenticator([]portal.TokenConfig{})
	config, err = emptyAuth.ValidateToken("any-token")
	if err == nil {
		t.Error("Expected empty authenticator to reject all tokens")
	}
}

func TestAuthenticatorIsRemoteAllowed(t *testing.T) {
	tokens := []portal.TokenConfig{
		{
			Token:          "restricted-token",
			AllowedRemotes: []string{"127.0.0.1/32", "10.0.0.0/8"},
			MaxMappings:    5,
		},
		{
			Token:          "unrestricted-token",
			AllowedRemotes: []string{}, // No restrictions
			MaxMappings:    10,
		},
		{
			Token:          "wildcard-token",
			AllowedRemotes: []string{"0.0.0.0/0"},
			MaxMappings:    3,
		},
	}

	auth := NewAuthenticator(tokens)

	// Test restricted token
	restrictedConfig, _ := auth.ValidateToken("restricted-token")

	if !auth.IsRemoteAllowed(restrictedConfig, "127.0.0.1") {
		t.Error("Expected 127.0.0.1 to be allowed")
	}

	if !auth.IsRemoteAllowed(restrictedConfig, "10.0.0.1") {
		t.Error("Expected 10.0.0.1 to be allowed")
	}

	if auth.IsRemoteAllowed(restrictedConfig, "192.168.1.1") {
		t.Error("Expected 192.168.1.1 to be denied")
	}

	// Test unrestricted token
	unrestrictedConfig, _ := auth.ValidateToken("unrestricted-token")

	if !auth.IsRemoteAllowed(unrestrictedConfig, "127.0.0.1") {
		t.Error("Expected any IP to be allowed with no restrictions")
	}

	if !auth.IsRemoteAllowed(unrestrictedConfig, "192.168.1.1") {
		t.Error("Expected any IP to be allowed with no restrictions")
	}

	// Test wildcard token
	wildcardConfig, _ := auth.ValidateToken("wildcard-token")

	if !auth.IsRemoteAllowed(wildcardConfig, "127.0.0.1") {
		t.Error("Expected any IP to be allowed with 0.0.0.0/0")
	}

	if !auth.IsRemoteAllowed(wildcardConfig, "192.168.1.1") {
		t.Error("Expected any IP to be allowed with 0.0.0.0/0")
	}

	// Test hostname with wildcard
	if !auth.IsRemoteAllowed(wildcardConfig, "example.com") {
		t.Error("Expected hostname to be allowed with 0.0.0.0/0")
	}

	// Test hostname without wildcard
	if auth.IsRemoteAllowed(restrictedConfig, "example.com") {
		t.Error("Expected hostname to be denied without 0.0.0.0/0")
	}
}

func TestNewForwarder(t *testing.T) {
	forwarder := NewForwarder()
	if forwarder == nil {
		t.Fatal("Expected forwarder to be created")
	}

	// Test buffer pool
	buf := forwarder.bufferPool.Get().([]byte)
	if len(buf) != 32*1024 {
		t.Errorf("Expected buffer size to be 32KB, got %d", len(buf))
	}
	forwarder.bufferPool.Put(buf)
}

func TestForwarderForward(t *testing.T) {
	forwarder := NewForwarder()

	// Test buffer pool functionality
	buf := forwarder.bufferPool.Get().([]byte)
	if len(buf) != 32*1024 {
		t.Errorf("Expected buffer size to be 32KB, got %d", len(buf))
	}
	forwarder.bufferPool.Put(buf)

	// Test with invalid remote host (invalid port)
	err := forwarder.DialAndForward(nil, "invalid-host", 99999)
	if err == nil {
		t.Error("Expected error for invalid port")
	}
}

func TestForwarderDialAndForward(t *testing.T) {
	forwarder := NewForwarder()

	// Test connection failure
	err := forwarder.DialAndForward(nil, "127.0.0.1", 1) // Port 1 is unlikely to be open
	if err == nil {
		t.Error("Expected connection to fail")
	}
}

func TestServerConcurrency(t *testing.T) {
	config := &portal.ServerConfig{
		Enabled:    true,
		ListenAddr: "127.0.0.1:0",
	}

	tlsConfig, err := generateTestTLSConfig()
	if err != nil {
		t.Fatalf("Failed to generate TLS config: %v", err)
	}

	server := NewServer(config, tlsConfig)

	// Test concurrent access to mappings
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			server.mu.Lock()
			server.mappings[string(rune(id))] = &MappingState{
				Mapping: portal.PortMapping{
					ID: string(rune(id)),
				},
			}
			server.mu.Unlock()
		}(i)
	}
	wg.Wait()

	if len(server.mappings) != 100 {
		t.Errorf("Expected 100 mappings, got %d", len(server.mappings))
	}
}

func TestProtocolIntegration(t *testing.T) {
	// Generate TLS config
	tlsConfig, err := generateTestTLSConfig()
	if err != nil {
		t.Fatalf("Failed to generate TLS config: %v", err)
	}

	// Create listener
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	serverAddr := listener.Addr().String()

	// Server goroutine
	var serverWg sync.WaitGroup
	serverWg.Add(1)
	go func() {
		defer serverWg.Done()
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		// Create server mux
		mux, err := protocol.NewServerMux(conn, tlsConfig, nil)
		if err != nil {
			t.Logf("Server mux creation failed: %v", err)
			return
		}
		defer mux.Close()

		// Accept one stream
		stream, err := mux.AcceptStream()
		if err != nil {
			return
		}
		defer stream.Close()

		// Read and echo
		buf := make([]byte, 1024)
		n, err := stream.Read(buf)
		if err != nil {
			return
		}
		stream.Write(buf[:n])
	}()

	// Give server time to start
	time.Sleep(10 * time.Millisecond)

	// Client connection
	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Create client mux
	clientMux, err := protocol.NewClientMux(conn, tlsConfig, nil)
	if err != nil {
		t.Fatalf("Failed to create client mux: %v", err)
	}
	defer clientMux.Close()

	// Open stream
	stream, err := clientMux.OpenStream()
	if err != nil {
		t.Fatalf("Failed to open stream: %v", err)
	}
	defer stream.Close()

	// Write data
	testData := []byte("hello world")
	_, err = stream.Write(testData)
	if err != nil {
		t.Fatalf("Failed to write: %v", err)
	}

	// Read response
	buf := make([]byte, 1024)
	n, err := stream.Read(buf)
	if err != nil {
		t.Fatalf("Failed to read: %v", err)
	}

	if string(buf[:n]) != string(testData) {
		t.Errorf("Expected %s, got %s", testData, buf[:n])
	}

	// Clean up
	listener.Close()
	serverWg.Wait()
}

func BenchmarkMappingState(b *testing.B) {
	mapping := portal.PortMapping{
		ID:   "bench-id",
		Name: "bench-mapping",
	}

	state := &MappingState{
		Mapping: mapping,
	}

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			state.StreamCount.Add(1)
			state.BytesIn.Add(100)
			state.BytesOut.Add(200)
		}
	})
}

func BenchmarkAuthenticator(b *testing.B) {
	tokens := []portal.TokenConfig{
		{
			Token:          "token-1",
			AllowedRemotes: []string{"127.0.0.1/32"},
			MaxMappings:    5,
		},
		{
			Token:          "token-2",
			AllowedRemotes: []string{"10.0.0.0/8"},
			MaxMappings:    10,
		},
	}

	auth := NewAuthenticator(tokens)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			auth.ValidateToken("token-1")
		}
	})
}
