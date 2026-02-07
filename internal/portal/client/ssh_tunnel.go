package client

import (
	"fmt"
	"log"
	"net"

	"github.com/gmssh/gmssh/internal/ssh"
	"github.com/gmssh/gmssh/pkg/types"
)

// SSHTunnel creates a tunnel through SSH chain to reach portal server
type SSHTunnel struct {
	chain     *ssh.Chain
	localAddr string
}

// NewSSHTunnel creates a new SSH tunnel
func NewSSHTunnel(hops []*types.Hop) *SSHTunnel {
	chain := ssh.NewChain(hops)

	return &SSHTunnel{
		chain: chain,
	}
}

// Connect establishes the SSH chain connection
func (t *SSHTunnel) Connect() error {
	if err := t.chain.Connect(); err != nil {
		return fmt.Errorf("failed to connect SSH chain: %w", err)
	}
	return nil
}

// Dial connects to portal server through SSH tunnel
func (t *SSHTunnel) Dial(serverHost string, serverPort int) (net.Conn, error) {
	// Use the SSH chain to dial the remote server
	conn, err := t.chain.Dial("tcp", fmt.Sprintf("%s:%d", serverHost, serverPort))
	if err != nil {
		return nil, fmt.Errorf("failed to dial through SSH tunnel: %w", err)
	}

	log.Printf("[SSHTunnel] Connected to %s:%d through SSH chain", serverHost, serverPort)
	return conn, nil
}

// IsConnected returns true if the SSH chain is connected
func (t *SSHTunnel) IsConnected() bool {
	return t.chain.IsConnected()
}

// Close closes the SSH tunnel
func (t *SSHTunnel) Close() error {
	if t.chain != nil {
		return t.chain.Disconnect()
	}
	return nil
}

// GetChain returns the underlying SSH chain
func (t *SSHTunnel) GetChain() *ssh.Chain {
	return t.chain
}
