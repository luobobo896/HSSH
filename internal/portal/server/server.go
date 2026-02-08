package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"sync"
	"sync/atomic"

	"github.com/luobobo896/HSSH/internal/portal/protocol"
	"github.com/luobobo896/HSSH/pkg/portal"
	"github.com/xtaci/smux"
)

// Server portal server
type Server struct {
	config    *portal.ServerConfig
	tlsConfig *tls.Config
	listener  net.Listener
	mux       *protocol.ServerMux

	// Connection management
	mappings map[string]*MappingState // mapping_id -> state
	mu       sync.RWMutex

	// Lifecycle
	ctx     context.Context
	cancel  context.CancelFunc
	running atomic.Bool
	wg      sync.WaitGroup
}

// MappingState tracks a single port mapping
type MappingState struct {
	Mapping     portal.PortMapping
	StreamCount atomic.Int32
	BytesIn     atomic.Int64
	BytesOut    atomic.Int64
}

// NewServer creates a new portal server
func NewServer(config *portal.ServerConfig, tlsConfig *tls.Config) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		config:    config,
		tlsConfig: tlsConfig,
		mappings:  make(map[string]*MappingState),
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Listen starts listening for connections
func (s *Server) Listen(addr string) error {
	if s.config != nil && s.config.ListenAddr != "" {
		addr = s.config.ListenAddr
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	s.listener = listener
	log.Printf("[Portal Server] Listening on %s", addr)
	return nil
}

// Serve accepts and handles connections
func (s *Server) Serve() error {
	if s.listener == nil {
		return fmt.Errorf("server not listening")
	}

	s.running.Store(true)
	defer s.running.Store(false)

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return nil
			default:
				log.Printf("[Portal Server] Accept error: %v", err)
				continue
			}
		}

		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

// handleConnection handles a single client connection
func (s *Server) handleConnection(conn net.Conn) {
	defer s.wg.Done()

	// Create smux server session over TLS
	mux, err := protocol.NewServerMux(conn, s.tlsConfig, nil)
	if err != nil {
		log.Printf("[Portal Server] Failed to create mux: %v", err)
		conn.Close()
		return
	}

	s.mux = mux
	defer mux.Close()

	log.Printf("[Portal Server] Client connected")

	// Handle streams
	for {
		stream, err := mux.AcceptStream()
		if err != nil {
			if !mux.IsClosed() {
				log.Printf("[Portal Server] AcceptStream error: %v", err)
			}
			return
		}

		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.handleStream(stream)
		}()
	}
}

// handleStream handles a single stream
func (s *Server) handleStream(stream *smux.Stream) {
	defer stream.Close()
	// TODO: Read mapping ID from stream, validate, and forward
	// For now, just close - full implementation in forwarder.go
}

// Close stops the server
func (s *Server) Close() error {
	s.cancel()

	if s.mux != nil {
		s.mux.Close()
	}

	if s.listener != nil {
		s.listener.Close()
	}

	s.wg.Wait()
	log.Printf("[Portal Server] Stopped")
	return nil
}

// IsRunning returns true if the server is running
func (s *Server) IsRunning() bool {
	return s.running.Load()
}
