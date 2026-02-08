package ssh

import (
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/luobobo896/HSSH/pkg/types"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// Client SSH 客户端封装
type Client struct {
	config     *types.Hop
	sshClient  *ssh.Client
	sshConfig  *ssh.ClientConfig
	connected  bool
}

// NewClient 创建新的 SSH 客户端
func NewClient(hop *types.Hop) (*Client, error) {
	sshConfig, err := buildSSHConfig(hop)
	if err != nil {
		return nil, fmt.Errorf("failed to build SSH config: %w", err)
	}

	return &Client{
		config:    hop,
		sshConfig: sshConfig,
		connected: false,
	}, nil
}

// Connect 建立 SSH 连接
func (c *Client) Connect() error {
	if c.connected {
		return nil
	}

	addr := c.config.Address()

	// 使用自定义 dialer 启用 TCP_NODELAY，减少延迟
	// 对于终端输入响应特别重要
	dialer := &net.Dialer{
		Timeout: 10 * time.Second,
	}

	netConn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to dial %s: %w", addr, err)
	}

	// 启用 TCP_NODELAY 禁用 Nagle 算法，减少输入延迟
	if tcpConn, ok := netConn.(*net.TCPConn); ok {
		tcpConn.SetNoDelay(true)
	}

	// 建立 SSH 连接
	conn, chans, reqs, err := ssh.NewClientConn(netConn, addr, c.sshConfig)
	if err != nil {
		netConn.Close()
		return fmt.Errorf("failed to create SSH connection: %w", err)
	}

	c.sshClient = ssh.NewClient(conn, chans, reqs)
	c.connected = true
	return nil
}

// ConnectThrough 通过跳板机连接
func (c *Client) ConnectThrough(bastion *Client) error {
	if !bastion.connected {
		return fmt.Errorf("bastion client not connected")
	}

	// 在跳板机上建立到目标主机的连接
	// 使用 TCP_NODELAY 禁用 Nagle 算法，减少延迟
	targetAddr := c.config.Address()
	bastionConn, err := bastion.sshClient.Dial("tcp", targetAddr)
	if err != nil {
		return fmt.Errorf("failed to dial through bastion: %w", err)
	}

	// 尝试设置 TCP_NODELAY（如果底层连接支持）
	if tcpConn, ok := bastionConn.(interface{ SetNoDelay(bool) error }); ok {
		tcpConn.SetNoDelay(true)
	}

	// 创建 SSH 连接
	conn, chans, reqs, err := ssh.NewClientConn(bastionConn, targetAddr, c.sshConfig)
	if err != nil {
		bastionConn.Close()
		return fmt.Errorf("failed to create SSH connection through bastion: %w", err)
	}

	c.sshClient = ssh.NewClient(conn, chans, reqs)
	c.connected = true
	return nil
}

// Disconnect 断开连接
func (c *Client) Disconnect() error {
	if c.sshClient != nil {
		err := c.sshClient.Close()
		c.connected = false
		c.sshClient = nil
		return err
	}
	return nil
}

// IsConnected 检查是否已连接
func (c *Client) IsConnected() bool {
	return c.connected && c.sshClient != nil
}

// GetUnderlyingClient 获取底层 SSH 客户端
func (c *Client) GetUnderlyingClient() *ssh.Client {
	return c.sshClient
}

// Dial 在 SSH 连接上建立网络连接
func (c *Client) Dial(network, addr string) (net.Conn, error) {
	if !c.connected {
		return nil, fmt.Errorf("not connected")
	}
	return c.sshClient.Dial(network, addr)
}

// NewSession 创建新的 SSH 会话
func (c *Client) NewSession() (*ssh.Session, error) {
	if !c.connected {
		return nil, fmt.Errorf("not connected")
	}
	return c.sshClient.NewSession()
}

// buildSSHConfig 构建 SSH 客户端配置
func buildSSHConfig(hop *types.Hop) (*ssh.ClientConfig, error) {
	log.Printf("[SSH] Building config for %s@%s, AuthType=%d (%v), KeyPath=%s, Password=%s", 
		hop.User, hop.Host, hop.AuthType, hop.AuthType, hop.KeyPath, 
		func() string { if hop.Password != "" { return "***" } else { return "(empty)" } }())
	
	var authMethods []ssh.AuthMethod

	switch hop.AuthType {
	case types.AuthKey:
		if hop.KeyPath == "" {
			return nil, fmt.Errorf("key path is required for key authentication")
		}
		key, err := os.ReadFile(expandPath(hop.KeyPath))
		if err != nil {
			return nil, fmt.Errorf("failed to read key file: %w", err)
		}

		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			// 可能是加密的私钥，尝试交互式输入密码
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))

	case types.AuthPassword:
		if hop.Password == "" {
			return nil, fmt.Errorf("password is required for password authentication")
		}
		authMethods = append(authMethods, ssh.Password(hop.Password))

	default:
		return nil, fmt.Errorf("unsupported auth type: %v", hop.AuthType)
	}

	// 添加键盘交互认证（用于处理某些需要二次确认的场景）
	authMethods = append(authMethods, ssh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) ([]string, error) {
		answers := make([]string, len(questions))
		for i := range questions {
			if hop.AuthType == types.AuthPassword && hop.Password != "" {
				answers[i] = hop.Password
			}
		}
		return answers, nil
	}))

	// 使用更快的加密算法和启用压缩来优化性能
	// 顺序：优先使用更高效的算法
	preferredCiphers := []string{
		"aes128-gcm@openssh.com",
		"aes256-gcm@openssh.com",
		"chacha20-poly1305@openssh.com",
		"aes128-ctr",
		"aes192-ctr",
		"aes256-ctr",
	}

	preferredKeyExchanges := []string{
		"curve25519-sha256",
		"curve25519-sha256@libssh.org",
		"ecdh-sha2-nistp256",
		"ecdh-sha2-nistp384",
		"ecdh-sha2-nistp521",
		"diffie-hellman-group14-sha256",
	}

	preferredMACs := []string{
		"hmac-sha2-256-etm@openssh.com",
		"hmac-sha2-128-etm@openssh.com",
		"hmac-sha2-256",
		"hmac-sha2-128",
	}

	config := &ssh.ClientConfig{
		User:    hop.User,
		Auth:    authMethods,
		Timeout: 10 * time.Second,
		// 启用压缩来减少数据传输量，提高响应速度
		// 对于终端交互特别有效
		Config: ssh.Config{
			Ciphers:      preferredCiphers,
			KeyExchanges: preferredKeyExchanges,
			MACs:         preferredMACs,
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // 生产环境应使用 knownhosts
	}

	return config, nil
}

// expandPath 展开路径中的 ~
func expandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(homeDir, path[1:])
		}
	}
	return path
}

// GetHostKeyCallback 获取主机密钥回调（用于验证服务器身份）
func GetHostKeyCallback() (ssh.HostKeyCallback, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	knownHostsPath := filepath.Join(homeDir, ".ssh", "known_hosts")
	callback, err := knownhosts.New(knownHostsPath)
	if err != nil {
		return ssh.InsecureIgnoreHostKey(), nil
	}
	return callback, nil
}
