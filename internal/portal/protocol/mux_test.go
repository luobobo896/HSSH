package protocol

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"sync"
	"testing"
	"time"
)

// generateTestCert generates a self-signed certificate for testing
func generateTestCert() (tls.Certificate, *x509.CertPool, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("failed to generate key: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	privBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("failed to marshal private key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes})

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("failed to load key pair: %w", err)
	}

	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(certPEM)

	return cert, pool, nil
}

// getTestTLSConfig returns server and client TLS configs for testing
func getTestTLSConfig() (*tls.Config, *tls.Config, error) {
	cert, pool, err := generateTestCert()
	if err != nil {
		return nil, nil, err
	}

	serverConfig := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: true,
	}

	clientConfig := &tls.Config{
		RootCAs:            pool,
		InsecureSkipVerify: true,
	}

	return serverConfig, clientConfig, nil
}

func TestDefaultMuxConfig(t *testing.T) {
	config := DefaultMuxConfig()

	if config == nil {
		t.Fatal("DefaultMuxConfig() returned nil")
	}

	if config.KeepAliveInterval != 10*time.Second {
		t.Errorf("KeepAliveInterval = %v, want %v", config.KeepAliveInterval, 10*time.Second)
	}

	if config.KeepAliveTimeout != 30*time.Second {
		t.Errorf("KeepAliveTimeout = %v, want %v", config.KeepAliveTimeout, 30*time.Second)
	}

	if config.MaxFrameSize != 32768 {
		t.Errorf("MaxFrameSize = %d, want %d", config.MaxFrameSize, 32768)
	}

	if config.MaxReceiveBuffer != 4194304 {
		t.Errorf("MaxReceiveBuffer = %d, want %d", config.MaxReceiveBuffer, 4194304)
	}

	if config.MaxStreamBuffer != 65536 {
		t.Errorf("MaxStreamBuffer = %d, want %d", config.MaxStreamBuffer, 65536)
	}
}

func TestNewServerMuxAndClientMux(t *testing.T) {
	serverConfig, clientConfig, err := getTestTLSConfig()
	if err != nil {
		t.Fatalf("Failed to generate test certificates: %v", err)
	}

	// Create listener
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().(*net.TCPAddr)

	// Server goroutine
	serverReady := make(chan struct{})
	serverDone := make(chan error, 1)

	go func() {
		close(serverReady) // Signal that we're ready to accept
		conn, err := listener.Accept()
		if err != nil {
			serverDone <- fmt.Errorf("accept failed: %w", err)
			return
		}

		mux, err := NewServerMux(conn, serverConfig, nil)
		if err != nil {
			conn.Close()
			serverDone <- fmt.Errorf("NewServerMux failed: %w", err)
			return
		}

		// Accept one stream
		stream, err := mux.AcceptStream()
		if err != nil {
			mux.Close()
			serverDone <- fmt.Errorf("AcceptStream failed: %w", err)
			return
		}

		// Echo back received data
		buf := make([]byte, 1024)
		n, err := stream.Read(buf)
		if err != nil {
			stream.Close()
			mux.Close()
			serverDone <- fmt.Errorf("stream read failed: %w", err)
			return
		}

		_, err = stream.Write(buf[:n])
		stream.Close()
		mux.Close()
		if err != nil {
			serverDone <- fmt.Errorf("stream write failed: %w", err)
			return
		}
		serverDone <- nil
	}()

	// Wait for server to start accepting
	<-serverReady
	time.Sleep(50 * time.Millisecond)

	// Connect client
	conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", addr.Port))
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	t.Logf("Client connected to server")

	clientMux, err := NewClientMux(conn, clientConfig, nil)
	if err != nil {
		conn.Close()
		t.Fatalf("NewClientMux failed: %v", err)
	}
	t.Logf("Client mux created successfully")

	// Open a stream
	stream, err := clientMux.OpenStream()
	if err != nil {
		clientMux.Close()
		t.Fatalf("OpenStream failed: %v", err)
	}

	// Send data
	testData := []byte("hello, smux!")
	_, err = stream.Write(testData)
	if err != nil {
		stream.Close()
		clientMux.Close()
		t.Fatalf("stream write failed: %v", err)
	}

	// Read echo
	buf := make([]byte, 1024)
	n, err := stream.Read(buf)
	stream.Close()
	clientMux.Close()
	if err != nil {
		t.Fatalf("stream read failed: %v", err)
	}

	if string(buf[:n]) != string(testData) {
		t.Errorf("Received %q, want %q", buf[:n], testData)
	}

	// Wait for server to finish
	select {
	case err := <-serverDone:
		if err != nil {
			t.Fatalf("Server error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Server timeout")
	}
}

func TestNewServerMuxWithCustomConfig(t *testing.T) {
	serverConfig, clientConfig, err := getTestTLSConfig()
	if err != nil {
		t.Fatalf("Failed to generate test certificates: %v", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	customConfig := &MuxConfig{
		KeepAliveInterval: 5 * time.Second,
		KeepAliveTimeout:  15 * time.Second,
		MaxFrameSize:      16384,
		MaxReceiveBuffer:  2097152,
		MaxStreamBuffer:   32768,
	}

	serverDone := make(chan error, 1)

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			serverDone <- err
			return
		}

		mux, err := NewServerMux(conn, serverConfig, customConfig)
		if err != nil {
			conn.Close()
			serverDone <- err
			return
		}

		if mux.config != customConfig {
			t.Error("ServerMux config not set correctly")
		}
		mux.Close()
		serverDone <- nil
	}()

	time.Sleep(200 * time.Millisecond)

	// Connect and complete handshake
	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}

	clientMux, err := NewClientMux(conn, clientConfig, nil)
	if err != nil {
		conn.Close()
		t.Fatalf("NewClientMux failed: %v", err)
	}
	clientMux.Close()

	// Wait for server
	select {
	case err := <-serverDone:
		if err != nil {
			t.Fatalf("Server error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Server timeout")
	}
}

func TestMuxSessionMethods(t *testing.T) {
	serverConfig, clientConfig, err := getTestTLSConfig()
	if err != nil {
		t.Fatalf("Failed to generate test certificates: %v", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	serverDone := make(chan error, 1)

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			serverDone <- err
			return
		}

		mux, err := NewServerMux(conn, serverConfig, nil)
		if err != nil {
			conn.Close()
			serverDone <- err
			return
		}

		// Test IsClosed - should be false initially
		if mux.IsClosed() {
			t.Error("ServerMux.IsClosed() = true, want false")
		}

		// Test NumStreams - should be 0 initially
		if mux.NumStreams() != 0 {
			t.Errorf("ServerMux.NumStreams() = %d, want 0", mux.NumStreams())
		}

		// Accept stream to increment count
		stream, err := mux.AcceptStream()
		if err != nil {
			mux.Close()
			serverDone <- err
			return
		}

		// Wait a bit for stream to be registered
		time.Sleep(50 * time.Millisecond)

		if mux.NumStreams() != 1 {
			t.Errorf("ServerMux.NumStreams() = %d, want 1", mux.NumStreams())
		}

		stream.Close()
		mux.Close()
		serverDone <- nil
	}()

	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}

	clientMux, err := NewClientMux(conn, clientConfig, nil)
	if err != nil {
		conn.Close()
		t.Fatalf("NewClientMux failed: %v", err)
	}

	// Test IsClosed - should be false initially
	if clientMux.IsClosed() {
		t.Error("ClientMux.IsClosed() = true, want false")
	}

	// Test NumStreams - should be 0 initially
	if clientMux.NumStreams() != 0 {
		t.Errorf("ClientMux.NumStreams() = %d, want 0", clientMux.NumStreams())
	}

	// Open a stream
	stream, err := clientMux.OpenStream()
	if err != nil {
		clientMux.Close()
		t.Fatalf("OpenStream failed: %v", err)
	}

	// Wait a bit for stream to be registered
	time.Sleep(50 * time.Millisecond)

	if clientMux.NumStreams() != 1 {
		t.Errorf("ClientMux.NumStreams() = %d, want 1", clientMux.NumStreams())
	}

	stream.Close()
	clientMux.Close()

	// Test IsClosed after Close
	if !clientMux.IsClosed() {
		t.Error("ClientMux.IsClosed() = false after Close(), want true")
	}

	// Wait for server
	select {
	case err := <-serverDone:
		if err != nil {
			t.Fatalf("Server error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Server timeout")
	}
}

func TestMultipleStreams(t *testing.T) {
	serverConfig, clientConfig, err := getTestTLSConfig()
	if err != nil {
		t.Fatalf("Failed to generate test certificates: %v", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	serverDone := make(chan error, 1)
	const numStreams = 5

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			serverDone <- err
			return
		}

		mux, err := NewServerMux(conn, serverConfig, nil)
		if err != nil {
			conn.Close()
			serverDone <- err
			return
		}

		var wg sync.WaitGroup
		for i := 0; i < numStreams; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				stream, err := mux.AcceptStream()
				if err != nil {
					return
				}

				buf := make([]byte, 1024)
				n, err := stream.Read(buf)
				if err != nil {
					stream.Close()
					return
				}

				// Echo back with "echo:" prefix
				response := fmt.Sprintf("echo:%s", string(buf[:n]))
				stream.Write([]byte(response))
				stream.Close()
			}()
		}
		wg.Wait()
		mux.Close()
		serverDone <- nil
	}()

	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}

	clientMux, err := NewClientMux(conn, clientConfig, nil)
	if err != nil {
		conn.Close()
		t.Fatalf("NewClientMux failed: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < numStreams; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			stream, err := clientMux.OpenStream()
			if err != nil {
				t.Errorf("OpenStream %d failed: %v", idx, err)
				return
			}

			msg := fmt.Sprintf("hello%d", idx)
			_, err = stream.Write([]byte(msg))
			if err != nil {
				t.Errorf("Write on stream %d failed: %v", idx, err)
				stream.Close()
				return
			}

			buf := make([]byte, 1024)
			n, err := stream.Read(buf)
			if err != nil {
				t.Errorf("Read on stream %d failed: %v", idx, err)
				stream.Close()
				return
			}

			expected := fmt.Sprintf("echo:%s", msg)
			if string(buf[:n]) != expected {
				t.Errorf("Stream %d received %q, want %q", idx, buf[:n], expected)
			}
			stream.Close()
		}(i)
	}
	wg.Wait()
	clientMux.Close()

	// Wait for server
	select {
	case err := <-serverDone:
		if err != nil {
			t.Fatalf("Server error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Server timeout")
	}
}

func TestNewServerMux_InvalidTLS(t *testing.T) {
	// Create a server config without certificates - should fail handshake
	serverConfig := &tls.Config{
		InsecureSkipVerify: true,
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	serverDone := make(chan error, 1)

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			serverDone <- err
			return
		}

		_, err = NewServerMux(conn, serverConfig, nil)
		if err != nil {
			conn.Close()
			serverDone <- err
			return
		}
		conn.Close()
		serverDone <- nil
	}()

	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	// Just close immediately - this will cause TLS handshake to fail
	conn.Close()

	// Wait for server - should get an error
	select {
	case <-serverDone:
		// Expected - TLS handshake should fail
	case <-time.After(5 * time.Second):
		t.Fatal("Server timeout")
	}
}

func BenchmarkMuxStreamCreation(b *testing.B) {
	serverConfig, clientConfig, err := getTestTLSConfig()
	if err != nil {
		b.Fatalf("Failed to generate test certificates: %v", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		b.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	serverReady := make(chan struct{})
	serverDone := make(chan struct{})

	go func() {
		defer close(serverDone)
		conn, err := listener.Accept()
		if err != nil {
			return
		}

		mux, err := NewServerMux(conn, serverConfig, nil)
		if err != nil {
			conn.Close()
			return
		}

		close(serverReady)

		for {
			stream, err := mux.AcceptStream()
			if err != nil {
				mux.Close()
				return
			}
			stream.Close()
		}
	}()

	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		b.Fatalf("Failed to dial: %v", err)
	}

	clientMux, err := NewClientMux(conn, clientConfig, nil)
	if err != nil {
		conn.Close()
		b.Fatalf("NewClientMux failed: %v", err)
	}

	<-serverReady

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stream, err := clientMux.OpenStream()
		if err != nil {
			clientMux.Close()
			b.Fatalf("OpenStream failed: %v", err)
		}
		stream.Close()
	}
	b.StopTimer()
	clientMux.Close()
	<-serverDone
}

func BenchmarkMuxDataTransfer(b *testing.B) {
	serverConfig, clientConfig, err := getTestTLSConfig()
	if err != nil {
		b.Fatalf("Failed to generate test certificates: %v", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		b.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	serverReady := make(chan struct{})
	serverDone := make(chan struct{})

	go func() {
		defer close(serverDone)
		conn, err := listener.Accept()
		if err != nil {
			return
		}

		mux, err := NewServerMux(conn, serverConfig, nil)
		if err != nil {
			conn.Close()
			return
		}

		stream, err := mux.AcceptStream()
		if err != nil {
			mux.Close()
			return
		}

		close(serverReady)

		// Echo server
		buf := make([]byte, 32768)
		for {
			n, err := stream.Read(buf)
			if err != nil {
				stream.Close()
				mux.Close()
				return
			}
			_, err = stream.Write(buf[:n])
			if err != nil {
				stream.Close()
				mux.Close()
				return
			}
		}
	}()

	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		b.Fatalf("Failed to dial: %v", err)
	}

	clientMux, err := NewClientMux(conn, clientConfig, nil)
	if err != nil {
		conn.Close()
		b.Fatalf("NewClientMux failed: %v", err)
	}

	<-serverReady

	stream, err := clientMux.OpenStream()
	if err != nil {
		clientMux.Close()
		b.Fatalf("OpenStream failed: %v", err)
	}

	data := make([]byte, 1024)
	rand.Read(data)
	buf := make([]byte, 1024)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := stream.Write(data)
		if err != nil {
			stream.Close()
			clientMux.Close()
			b.Fatalf("Write failed: %v", err)
		}
		_, err = stream.Read(buf)
		if err != nil {
			stream.Close()
			clientMux.Close()
			b.Fatalf("Read failed: %v", err)
		}
	}
	b.StopTimer()
	stream.Close()
	clientMux.Close()
	<-serverDone
}
