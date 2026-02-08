package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"sync/atomic"

	"github.com/luobobo896/HSSH/internal/portal/protocol"
	"github.com/luobobo896/HSSH/pkg/portal"
)

// Client portal client
type Client struct {
	config     *portal.ClientConfig
	tlsConfig  *tls.Config
	token      string
	serverAddr string

	// Connection
	mux  *protocol.ClientMux
	conn net.Conn

	// State
	ctx     context.Context
	cancel  context.CancelFunc
	running atomic.Bool
	mu      sync.RWMutex

	// Mappings
	mappings map[string]*MappingState
	wg       sync.WaitGroup
}

// MappingState tracks a single local port mapping
type MappingState struct {
	Mapping   portal.PortMapping
	Listener  net.Listener
	Active    atomic.Bool
	ConnCount atomic.Int32
	BytesIn   atomic.Int64
	BytesOut  atomic.Int64
}

// NewClient creates a new portal client
func NewClient(config *portal.ClientConfig, tlsConfig *tls.Config, token, serverAddr string) *Client {
	ctx, cancel := context.WithCancel(context.Background())
	return &Client{
		config:     config,
		tlsConfig:  tlsConfig,
		token:      token,
		serverAddr: serverAddr,
		mappings:   make(map[string]*MappingState),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Connect establishes connection to portal server
func (c *Client) Connect() error {
	conn, err := net.Dial("tcp", c.serverAddr)
	if err != nil {
		return fmt.Errorf("failed to connect to server %s: %w", c.serverAddr, err)
	}

	// Create smux client session over TLS
	mux, err := protocol.NewClientMux(conn, c.tlsConfig, nil)
	if err != nil {
		conn.Close()
		return fmt.Errorf("failed to create mux: %w", err)
	}

	c.conn = conn
	c.mux = mux
	c.running.Store(true)

	log.Printf("[Portal Client] Connected to server %s", c.serverAddr)
	return nil
}

// StartMapping starts a single port mapping
func (c *Client) StartMapping(mapping portal.PortMapping) error {
	if !c.running.Load() {
		return fmt.Errorf("client not connected")
	}

	// Start local listener
	listener, err := net.Listen("tcp", mapping.LocalAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", mapping.LocalAddr, err)
	}

	state := &MappingState{
		Mapping:  mapping,
		Listener: listener,
	}
	state.Active.Store(true)

	c.mu.Lock()
	c.mappings[mapping.ID] = state
	c.mu.Unlock()

	// Accept connections
	c.wg.Add(1)
	go c.acceptLoop(state)

	log.Printf("[Portal Client] Started mapping %s: %s -> %s:%d",
		mapping.Name, mapping.LocalAddr, mapping.RemoteHost, mapping.RemotePort)
	return nil
}

// acceptLoop accepts local connections and forwards them
func (c *Client) acceptLoop(state *MappingState) {
	defer c.wg.Done()

	for {
		conn, err := state.Listener.Accept()
		if err != nil {
			select {
			case <-c.ctx.Done():
				return
			default:
				if state.Active.Load() {
					log.Printf("[Portal Client] Accept error on %s: %v", state.Mapping.LocalAddr, err)
				}
				continue
			}
		}

		state.ConnCount.Add(1)
		c.wg.Add(1)
		go func() {
			defer c.wg.Done()
			defer state.ConnCount.Add(-1)
			c.handleConnection(conn, state)
		}()
	}
}

// handleConnection handles a single local connection
func (c *Client) handleConnection(localConn net.Conn, state *MappingState) {
	defer localConn.Close()

	// Open stream to server
	stream, err := c.mux.OpenStream()
	if err != nil {
		log.Printf("[Portal Client] Failed to open stream: %v", err)
		return
	}
	defer stream.Close()

	// TODO: Send mapping ID to server (protocol handshake)
	// For now, just forward raw data

	// Bidirectional copy
	errCh := make(chan error, 2)

	go func() {
		n, err := io.Copy(stream, localConn)
		state.BytesIn.Add(n)
		errCh <- err
	}()

	go func() {
		n, err := io.Copy(localConn, stream)
		state.BytesOut.Add(n)
		errCh <- err
	}()

	<-errCh
}

// StopMapping stops a port mapping
func (c *Client) StopMapping(mappingID string) error {
	c.mu.Lock()
	state, ok := c.mappings[mappingID]
	if !ok {
		c.mu.Unlock()
		return fmt.Errorf("mapping %s not found", mappingID)
	}
	delete(c.mappings, mappingID)
	c.mu.Unlock()

	state.Active.Store(false)
	if state.Listener != nil {
		state.Listener.Close()
	}

	log.Printf("[Portal Client] Stopped mapping %s", state.Mapping.Name)
	return nil
}

// Close disconnects from server
func (c *Client) Close() error {
	c.cancel()
	c.running.Store(false)

	// Stop all mappings
	c.mu.Lock()
	for _, state := range c.mappings {
		state.Active.Store(false)
		if state.Listener != nil {
			state.Listener.Close()
		}
	}
	c.mu.Unlock()

	// Close mux
	if c.mux != nil {
		c.mux.Close()
	}

	// Close connection
	if c.conn != nil {
		c.conn.Close()
	}

	c.wg.Wait()
	log.Printf("[Portal Client] Disconnected")
	return nil
}

// IsConnected returns true if connected to server
func (c *Client) IsConnected() bool {
	return c.running.Load() && c.mux != nil && !c.mux.IsClosed()
}

// GetMappingStatus returns status of all mappings
func (c *Client) GetMappingStatus() []portal.MappingStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]portal.MappingStatus, 0, len(c.mappings))
	for _, state := range c.mappings {
		result = append(result, portal.MappingStatus{
			PortMapping:      state.Mapping,
			Active:           state.Active.Load(),
			ConnectionCount:  int(state.ConnCount.Load()),
			BytesTransferred: state.BytesIn.Load() + state.BytesOut.Load(),
		})
	}
	return result
}
