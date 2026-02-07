package server

import (
	"fmt"
	"io"
	"log"
	"net"
	"sync"

	"github.com/xtaci/smux"
)

// Forwarder handles forwarding between smux streams and remote connections
type Forwarder struct {
	bufferPool sync.Pool
}

// NewForwarder creates a new forwarder
func NewForwarder() *Forwarder {
	return &Forwarder{
		bufferPool: sync.Pool{
			New: func() interface{} {
				return make([]byte, 32*1024)
			},
		},
	}
}

// Forward forwards traffic between a smux stream and a remote connection
func (f *Forwarder) Forward(stream *smux.Stream, remoteConn net.Conn) error {
	defer stream.Close()
	defer remoteConn.Close()

	errCh := make(chan error, 2)

	// Stream -> Remote
	go func() {
		buf := f.bufferPool.Get().([]byte)
		defer f.bufferPool.Put(buf)

		_, err := io.CopyBuffer(remoteConn, stream, buf)
		errCh <- err
	}()

	// Remote -> Stream
	go func() {
		buf := f.bufferPool.Get().([]byte)
		defer f.bufferPool.Put(buf)

		_, err := io.CopyBuffer(stream, remoteConn, buf)
		errCh <- err
	}()

	// Wait for either direction to finish
	err := <-errCh
	<-errCh // Drain the second error

	return err
}

// DialAndForward connects to a remote address and forwards traffic
func (f *Forwarder) DialAndForward(stream *smux.Stream, remoteHost string, remotePort int) error {
	addr := net.JoinHostPort(remoteHost, fmt.Sprintf("%d", remotePort))

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		log.Printf("[Forwarder] Failed to connect to %s: %v", addr, err)
		return fmt.Errorf("failed to connect to %s: %w", addr, err)
	}

	log.Printf("[Forwarder] Connected to %s", addr)
	return f.Forward(stream, conn)
}
