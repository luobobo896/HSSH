package protocol

import (
	"crypto/tls"
	"fmt"
	"net"
	"time"

	"github.com/xtaci/smux"
)

// MuxConfig smux configuration
type MuxConfig struct {
	KeepAliveInterval time.Duration
	KeepAliveTimeout  time.Duration
	MaxFrameSize      int
	MaxReceiveBuffer  int
	MaxStreamBuffer   int
}

// DefaultMuxConfig returns default smux configuration
func DefaultMuxConfig() *MuxConfig {
	return &MuxConfig{
		KeepAliveInterval: 10 * time.Second,
		KeepAliveTimeout:  30 * time.Second,
		MaxFrameSize:      32768,
		MaxReceiveBuffer:  4194304,
		MaxStreamBuffer:   65536,
	}
}

// ServerMux wraps smux server session
type ServerMux struct {
	session *smux.Session
	config  *MuxConfig
}

// ClientMux wraps smux client session
type ClientMux struct {
	session *smux.Session
	config  *MuxConfig
}

// NewServerMux creates a server-side smux session over a TLS connection
func NewServerMux(conn net.Conn, tlsConfig *tls.Config, config *MuxConfig) (*ServerMux, error) {
	if config == nil {
		config = DefaultMuxConfig()
	}

	// Wrap connection with TLS
	tlsConn := tls.Server(conn, tlsConfig)
	if err := tlsConn.Handshake(); err != nil {
		return nil, fmt.Errorf("TLS handshake failed: %w", err)
	}

	// Create smux session
	smuxConfig := &smux.Config{
		Version:           1, // Use version 1 for compatibility
		KeepAliveInterval: config.KeepAliveInterval,
		KeepAliveTimeout:  config.KeepAliveTimeout,
		MaxFrameSize:      config.MaxFrameSize,
		MaxReceiveBuffer:  config.MaxReceiveBuffer,
		MaxStreamBuffer:   config.MaxStreamBuffer,
	}

	session, err := smux.Server(tlsConn, smuxConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create smux server session: %w", err)
	}

	return &ServerMux{
		session: session,
		config:  config,
	}, nil
}

// NewClientMux creates a client-side smux session over a TLS connection
func NewClientMux(conn net.Conn, tlsConfig *tls.Config, config *MuxConfig) (*ClientMux, error) {
	if config == nil {
		config = DefaultMuxConfig()
	}

	// Wrap connection with TLS
	tlsConn := tls.Client(conn, tlsConfig)
	if err := tlsConn.Handshake(); err != nil {
		return nil, fmt.Errorf("TLS handshake failed: %w", err)
	}

	// Create smux session
	smuxConfig := &smux.Config{
		Version:           1, // Use version 1 for compatibility
		KeepAliveInterval: config.KeepAliveInterval,
		KeepAliveTimeout:  config.KeepAliveTimeout,
		MaxFrameSize:      config.MaxFrameSize,
		MaxReceiveBuffer:  config.MaxReceiveBuffer,
		MaxStreamBuffer:   config.MaxStreamBuffer,
	}

	session, err := smux.Client(tlsConn, smuxConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create smux client session: %w", err)
	}

	return &ClientMux{
		session: session,
		config:  config,
	}, nil
}

// AcceptStream accepts a new stream from the server session
func (s *ServerMux) AcceptStream() (*smux.Stream, error) {
	return s.session.AcceptStream()
}

// OpenStream opens a new stream from the client session
func (c *ClientMux) OpenStream() (*smux.Stream, error) {
	return c.session.OpenStream()
}

// Close closes the mux session
func (s *ServerMux) Close() error {
	return s.session.Close()
}

// Close closes the mux session
func (c *ClientMux) Close() error {
	return c.session.Close()
}

// IsClosed checks if the session is closed
func (s *ServerMux) IsClosed() bool {
	return s.session.IsClosed()
}

// IsClosed checks if the session is closed
func (c *ClientMux) IsClosed() bool {
	return c.session.IsClosed()
}

// NumStreams returns the number of active streams
func (s *ServerMux) NumStreams() int {
	return s.session.NumStreams()
}

// NumStreams returns the number of active streams
func (c *ClientMux) NumStreams() int {
	return c.session.NumStreams()
}
