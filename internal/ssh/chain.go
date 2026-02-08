package ssh

import (
	"bytes"
	"fmt"
	"net"

	"github.com/luobobo896/HSSH/pkg/types"
	"golang.org/x/crypto/ssh"
)

// Chain 管理 SSH 连接链
type Chain struct {
	hops    []*types.Hop
	clients []*Client
	connected bool
}

// NewChain 创建新的连接链
func NewChain(hops []*types.Hop) *Chain {
	return &Chain{
		hops:    hops,
		clients: make([]*Client, 0, len(hops)),
	}
}

// Connect 建立整个连接链
func (c *Chain) Connect() error {
	if c.connected {
		return nil
	}

	if len(c.hops) == 0 {
		return fmt.Errorf("no hops in chain")
	}

	// 建立第一跳连接
	firstClient, err := NewClient(c.hops[0])
	if err != nil {
		return fmt.Errorf("failed to create first hop client: %w", err)
	}

	if err := firstClient.Connect(); err != nil {
		return fmt.Errorf("failed to connect to first hop: %w", err)
	}

	c.clients = append(c.clients, firstClient)

	// 建立后续跳（通过前一跳作为跳板）
	for i := 1; i < len(c.hops); i++ {
		client, err := NewClient(c.hops[i])
		if err != nil {
			c.Disconnect()
			return fmt.Errorf("failed to create client for hop %d: %w", i, err)
		}

		// 通过上一跳连接
		if err := client.ConnectThrough(c.clients[i-1]); err != nil {
			c.Disconnect()
			return fmt.Errorf("failed to connect through hop %d: %w", i-1, err)
		}

		c.clients = append(c.clients, client)
	}

	c.connected = true
	return nil
}

// Disconnect 断开整个连接链
func (c *Chain) Disconnect() error {
	var lastErr error
	// 反向断开（从内网到外网）
	for i := len(c.clients) - 1; i >= 0; i-- {
		if err := c.clients[i].Disconnect(); err != nil {
			lastErr = err
		}
	}
	c.clients = c.clients[:0]
	c.connected = false
	return lastErr
}

// IsConnected 检查连接链是否已建立
func (c *Chain) IsConnected() bool {
	return c.connected && len(c.clients) == len(c.hops)
}

// LastHop 获取最后一跳客户端
func (c *Chain) LastHop() *Client {
	if len(c.clients) == 0 {
		return nil
	}
	return c.clients[len(c.clients)-1]
}

// FirstHop 获取第一跳客户端
func (c *Chain) FirstHop() *Client {
	if len(c.clients) == 0 {
		return nil
	}
	return c.clients[0]
}

// GetHop 获取指定索引的客户端
func (c *Chain) GetHop(index int) *Client {
	if index < 0 || index >= len(c.clients) {
		return nil
	}
	return c.clients[index]
}

// HopCount 获取跳数
func (c *Chain) HopCount() int {
	return len(c.hops)
}

// Dial 通过最后一跳建立到目标的连接
func (c *Chain) Dial(network, addr string) (net.Conn, error) {
	if !c.connected {
		return nil, fmt.Errorf("chain not connected")
	}
	return c.LastHop().Dial(network, addr)
}

// NewSession 在最后一跳创建会话
func (c *Chain) NewSession() (*ssh.Session, error) {
	if !c.connected {
		return nil, fmt.Errorf("chain not connected")
	}
	return c.LastHop().NewSession()
}

// Execute 在最后一跳执行命令
func (c *Chain) Execute(command string) (string, string, error) {
	session, err := c.NewSession()
	if err != nil {
		return "", "", err
	}
	defer session.Close()

	var stdoutBuf, stderrBuf bytes.Buffer
	session.Stdout = &stdoutBuf
	session.Stderr = &stderrBuf

	err = session.Run(command)
	return stdoutBuf.String(), stderrBuf.String(), err
}
