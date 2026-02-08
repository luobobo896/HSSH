package cli

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/luobobo896/HSSH/internal/portal/client"
	"github.com/luobobo896/HSSH/internal/portal/server"
	"github.com/luobobo896/HSSH/pkg/portal"
	"github.com/google/uuid"
)

// PortalCommand portal CLI command
type PortalCommand struct {
	// Common flags
	isServer bool
	isClient bool
	config   string

	// Server flags
	listen  string
	token   string
	tlsCert string
	tlsKey  string

	// Client flags
	local      string
	remote     string
	serverAddr string
	via        string
}

// Name returns command name
func (c *PortalCommand) Name() string {
	return "portal"
}

// Synopsis returns short description
func (c *PortalCommand) Synopsis() string {
	return "高性能端口转发/内网穿透"
}

// Usage returns detailed usage
func (c *PortalCommand) Usage() string {
	return `Usage: hssh portal [options]

Options:
  --server          以服务端模式运行
  --client          以客户端模式运行
  --config PATH     配置文件路径

Server Mode:
  --listen ADDR     监听地址 (默认 :18888)
  --token TOKEN     认证令牌
  --tls-cert PATH   TLS 证书路径
  --tls-key PATH    TLS 密钥路径

Client Mode:
  --local ADDR      本地监听地址 (例如 :8080)
  --remote HOST:PORT 远程目标地址
  --server-addr ADDR     Portal服务器地址 (例如 portal.example.com:18888)
  --via IDS         中转服务器 ID，逗号分隔

Examples:
  # 服务端模式
  hssh portal --server --listen :18888 --token "my-token"

  # 客户端模式 (单映射)
  hssh portal --client --local :8080 --remote 192.168.1.10:80 --server-addr portal.example.com:18888
`
}

// SetFlags sets up command flags
func (c *PortalCommand) SetFlags(f *flag.FlagSet) {
	f.BoolVar(&c.isServer, "server", false, "Run in server mode")
	f.BoolVar(&c.isClient, "client", false, "Run in client mode")
	f.StringVar(&c.config, "config", "", "Config file path")

	// Server flags
	f.StringVar(&c.listen, "listen", ":18888", "Server listen address")
	f.StringVar(&c.token, "token", "", "Auth token")
	f.StringVar(&c.tlsCert, "tls-cert", "", "TLS certificate path")
	f.StringVar(&c.tlsKey, "tls-key", "", "TLS key path")

	// Client flags
	f.StringVar(&c.local, "local", "", "Local listen address")
	f.StringVar(&c.remote, "remote", "", "Remote target (host:port)")
	f.StringVar(&c.serverAddr, "server-addr", "", "Portal server address")
	f.StringVar(&c.via, "via", "", "Comma-separated hop IDs")
}

// Run executes the command
func (c *PortalCommand) Run(args []string) int {
	if c.isServer {
		return c.runServer()
	}
	if c.isClient {
		return c.runClient()
	}

	fmt.Fprintln(os.Stderr, "Error: must specify --server or --client")
	fmt.Println(c.Usage())
	return 1
}

// runServer runs in server mode
func (c *PortalCommand) runServer() int {
	// Load TLS config
	tlsConfig, err := c.loadServerTLS()
	if err != nil {
		log.Printf("[Portal] Failed to load TLS: %v", err)
		return 1
	}

	// Create server config
	serverConfig := &portal.ServerConfig{
		Enabled:    true,
		ListenAddr: c.listen,
		AuthTokens: []portal.TokenConfig{
			{
				Token:          c.token,
				AllowedRemotes: []string{"0.0.0.0/0"}, // Allow all for now
				MaxMappings:    10,
			},
		},
	}

	// Create and start server
	srv := server.NewServer(serverConfig, tlsConfig)

	if err := srv.Listen(c.listen); err != nil {
		log.Printf("[Portal] Failed to listen: %v", err)
		return 1
	}

	// Setup signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("[Portal] Shutting down...")
		srv.Close()
		cancel()
	}()

	log.Printf("[Portal] Server starting on %s", c.listen)

	// Start server in a goroutine
	serverErr := make(chan error, 1)
	go func() {
		if err := srv.Serve(); err != nil {
			serverErr <- err
		}
	}()

	// Wait for server to stop or context cancellation
	select {
	case err := <-serverErr:
		if err != nil {
			log.Printf("[Portal] Server error: %v", err)
			return 1
		}
	case <-ctx.Done():
		// Graceful shutdown
	}

	return 0
}

// runClient runs in client mode
func (c *PortalCommand) runClient() int {
	if c.local == "" || c.remote == "" {
		fmt.Fprintln(os.Stderr, "Error: --local and --remote are required in client mode")
		return 1
	}

	if c.serverAddr == "" {
		fmt.Fprintln(os.Stderr, "Error: --server is required in client mode (portal server address)")
		return 1
	}

	// Parse remote address
	remoteHost, remotePortStr, err := net.SplitHostPort(c.remote)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid remote address format '%s': %v\n", c.remote, err)
		return 1
	}

	remotePort, err := strconv.Atoi(remotePortStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid remote port '%s': %v\n", remotePortStr, err)
		return 1
	}

	// Create TLS config (insecure for now)
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
	}

	// Create client config
	clientConfig := &portal.ClientConfig{
		Connection: portal.ConnectionConfig{
			RetryInterval:     5 * time.Second,
			MaxRetries:        10,
			KeepaliveInterval: 30 * time.Second,
		},
	}

	// Parse via hops
	var viaHops []string
	if c.via != "" {
		viaHops = strings.Split(c.via, ",")
		for i := range viaHops {
			viaHops[i] = strings.TrimSpace(viaHops[i])
		}
	}

	// Create client
	cli := client.NewClient(clientConfig, tlsConfig, c.token, c.serverAddr)

	// Connect to server
	if err := cli.Connect(); err != nil {
		log.Printf("[Portal] Failed to connect: %v", err)
		return 1
	}
	defer cli.Close()

	// Create mapping
	mapping := portal.PortMapping{
		ID:         uuid.New().String(),
		Name:       "cli-mapping",
		LocalAddr:  c.local,
		RemoteHost: remoteHost,
		RemotePort: remotePort,
		Via:        viaHops,
		Protocol:   portal.ProtocolTCP,
		Enabled:    true,
	}

	if err := cli.StartMapping(mapping); err != nil {
		log.Printf("[Portal] Failed to start mapping: %v", err)
		return 1
	}

	log.Printf("[Portal] Client started: %s -> %s:%d", c.local, remoteHost, remotePort)

	// Wait for interrupt
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	<-sigCh
	log.Println("[Portal] Shutting down...")

	return 0
}

// loadServerTLS loads TLS configuration for server
func (c *PortalCommand) loadServerTLS() (*tls.Config, error) {
	if c.tlsCert == "" || c.tlsKey == "" {
		// Generate self-signed cert for development
		log.Println("[Portal] Warning: Using auto-generated TLS certificate")
		return generateSelfSignedTLS()
	}

	cert, err := tls.LoadX509KeyPair(c.tlsCert, c.tlsKey)
	if err != nil {
		return nil, fmt.Errorf("failed to load TLS certs: %w", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
	}, nil
}

// generateSelfSignedTLS generates a self-signed certificate for testing
func generateSelfSignedTLS() (*tls.Config, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"HSSH Portal"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	privBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal private key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes})

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to load key pair: %w", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
	}, nil
}
