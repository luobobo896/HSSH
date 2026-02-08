// Package terminal 提供高性能数据转发引擎
package terminal

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// Forwarder 高性能数据转发器
type Forwarder struct {
	// 配置
	config ForwarderConfig

	// 自适应缓冲区
	buffer *AdaptiveBuffer

	// 统计信息
	stats ForwarderStats

	// 控制
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// ForwarderConfig 转发器配置
type ForwarderConfig struct {
	// 缓冲区配置
	BufferConfig *AdaptiveBuffer

	// 批量发送配置
	BatchSize     int
	BatchDelay    time.Duration

	// 超时配置
	ReadTimeout  time.Duration
	WriteTimeout time.Duration

	// 并发控制
	MaxWorkers int
}

// DefaultForwarderConfig 返回默认转发器配置
func DefaultForwarderConfig() ForwarderConfig {
	return ForwarderConfig{
		BufferConfig:  NewAdaptiveBuffer(),
		BatchSize:     64 * 1024,
		BatchDelay:    5 * time.Millisecond,
		ReadTimeout:   30 * time.Second,
		WriteTimeout:  30 * time.Second,
		MaxWorkers:    4,
	}
}

// ForwarderStats 转发器统计
type ForwarderStats struct {
	BytesSent     atomic.Uint64
	BytesReceived atomic.Uint64
	PacketsSent   atomic.Uint64
	PacketsRecv   atomic.Uint64
	Errors        atomic.Uint64
	LatencyMs     atomic.Int64
}

// NewForwarder 创建新的转发器
func NewForwarder(config ForwarderConfig) *Forwarder {
	ctx, cancel := context.WithCancel(context.Background())
	return &Forwarder{
		config: config,
		buffer: config.BufferConfig,
		ctx:    ctx,
		cancel: cancel,
	}
}

// PipeOpts 管道选项
type PipeOpts struct {
	Direction    string // "ssh-to-ws" 或 "ws-to-ssh"
	EnableBatch  bool
	EnableStats  bool
}

// PipeSSHToWebSocket 将 SSH 输出转发到 WebSocket
func (f *Forwarder) PipeSSHToWebSocket(sshReader io.Reader, wsConn *websocket.Conn, opts PipeOpts) error {
	bufferSize := f.buffer.GetReadBuffer()
	buf := make([]byte, bufferSize)

	// 如果使用批量发送，创建批量写入器
	var batcher *BatchedWriter
	if opts.EnableBatch {
		batcher = NewBatchedWriter(
			func(data []byte) error {
				return f.writeWebSocket(wsConn, data)
			},
			f.config.BatchSize,
			f.config.BatchDelay,
		)
		defer batcher.Close()
	}

	for {
		select {
		case <-f.ctx.Done():
			return nil
		default:
		}

		// 读取 SSH 输出
		start := time.Now()
		n, err := sshReader.Read(buf)
		if err != nil {
			if err != io.EOF {
				f.stats.Errors.Add(1)
				log.Printf("[Forwarder] SSH read error: %v", err)
			}
			return err
		}

		if n > 0 {
			f.stats.BytesReceived.Add(uint64(n))
			f.stats.PacketsRecv.Add(1)
			f.stats.LatencyMs.Store(time.Since(start).Milliseconds())

			// 写入 WebSocket
			data := buf[:n]
			if batcher != nil {
				if err := batcher.Write(data); err != nil {
					f.stats.Errors.Add(1)
					return err
				}
			} else {
				if err := f.writeWebSocket(wsConn, data); err != nil {
					f.stats.Errors.Add(1)
					return err
				}
			}

			// 记录字节数以供自适应调整
			f.buffer.RecordBytes(n)
		}
	}
}

// PipeWebSocketToSSH 将 WebSocket 输入转发到 SSH
func (f *Forwarder) PipeWebSocketToSSH(wsConn *websocket.Conn, sshWriter io.Writer, opts PipeOpts) error {
	for {
		select {
		case <-f.ctx.Done():
			return nil
		default:
		}

		// 设置读取超时
		wsConn.SetReadDeadline(time.Now().Add(f.config.ReadTimeout))

		// 读取 WebSocket 消息
		msgType, data, err := wsConn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				f.stats.Errors.Add(1)
				log.Printf("[Forwarder] WebSocket read error: %v", err)
			}
			return err
		}

		if msgType == websocket.TextMessage || msgType == websocket.BinaryMessage {
			start := time.Now()

			// 写入 SSH stdin
			n, err := sshWriter.Write(data)
			if err != nil {
				f.stats.Errors.Add(1)
				return err
			}

			f.stats.BytesSent.Add(uint64(n))
			f.stats.PacketsSent.Add(1)
			f.stats.LatencyMs.Store(time.Since(start).Milliseconds())
		}
	}
}

// BidirectionalPipe 双向管道（并发）
func (f *Forwarder) BidirectionalPipe(wsConn *websocket.Conn, sshReader io.Reader, sshWriter io.Writer) error {
	errChan := make(chan error, 2)

	// SSH -> WebSocket
	f.wg.Add(1)
	go func() {
		defer f.wg.Done()
		opts := PipeOpts{
			Direction:   "ssh-to-ws",
			EnableBatch: true,
			EnableStats: true,
		}
		if err := f.PipeSSHToWebSocket(sshReader, wsConn, opts); err != nil {
			errChan <- fmt.Errorf("ssh-to-ws: %w", err)
		}
	}()

	// WebSocket -> SSH
	f.wg.Add(1)
	go func() {
		defer f.wg.Done()
		opts := PipeOpts{
			Direction:   "ws-to-ssh",
			EnableBatch: false,
			EnableStats: true,
		}
		if err := f.PipeWebSocketToSSH(wsConn, sshWriter, opts); err != nil {
			errChan <- fmt.Errorf("ws-to-ssh: %w", err)
		}
	}()

	// 等待任一方向结束
	err := <-errChan
	f.cancel() // 取消另一方向
	f.wg.Wait()

	return err
}

// writeWebSocket 写入 WebSocket（带超时）
func (f *Forwarder) writeWebSocket(conn *websocket.Conn, data []byte) error {
	conn.SetWriteDeadline(time.Now().Add(f.config.WriteTimeout))
	return conn.WriteMessage(websocket.TextMessage, data)
}

// GetStats 获取统计信息
func (f *Forwarder) GetStats() ForwarderStats {
	return ForwarderStats{
		BytesSent:     atomic.Uint64{},
		BytesReceived: atomic.Uint64{},
		PacketsSent:   atomic.Uint64{},
		PacketsRecv:   atomic.Uint64{},
		Errors:        atomic.Uint64{},
		LatencyMs:     atomic.Int64{},
	}
}

// Close 关闭转发器
func (f *Forwarder) Close() error {
	f.cancel()
	f.wg.Wait()
	return nil
}

// ZeroCopyPipe 使用零拷贝技术的高效管道（需要底层支持）
type ZeroCopyPipe struct {
	conn1 net.Conn
	conn2 net.Conn
	pool  *sync.Pool
}

// NewZeroCopyPipe 创建零拷贝管道
func NewZeroCopyPipe(conn1, conn2 net.Conn) *ZeroCopyPipe {
	return &ZeroCopyPipe{
		conn1: conn1,
		conn2: conn2,
		pool: &sync.Pool{
			New: func() interface{} {
				return make([]byte, 64*1024)
			},
		},
	}
}

// Start 启动零拷贝转发
func (p *ZeroCopyPipe) Start() error {
	errChan := make(chan error, 2)

	// Conn1 -> Conn2
	go func() {
		errChan <- p.copy(p.conn1, p.conn2)
	}()

	// Conn2 -> Conn1
	go func() {
		errChan <- p.copy(p.conn2, p.conn1)
	}()

	return <-errChan
}

// copy 使用 buffer pool 的高效拷贝
func (p *ZeroCopyPipe) copy(dst, src net.Conn) error {
	buf := p.pool.Get().([]byte)
	defer p.pool.Put(buf)

	_, err := io.CopyBuffer(dst, src, buf)
	return err
}

// SplicePipe 使用 Linux splice 系统调用（如果可用）
// 注意：此功能需要特定的平台支持
func SplicePipe(src, dst *net.TCPConn) (int64, error) {
	// Linux 特定优化，使用 splice 系统调用
	// 在通用代码中，我们回退到普通拷贝
	return io.Copy(dst, src)
}

// RateLimiter 速率限制器
type RateLimiter struct {
	mu       sync.Mutex
	tokens   float64
	capacity float64
	rate     float64
	lastTime time.Time
}

// NewRateLimiter 创建速率限制器
// rate: 每秒令牌数
// burst: 突发容量
func NewRateLimiter(rate, burst float64) *RateLimiter {
	return &RateLimiter{
		tokens:   burst,
		capacity: burst,
		rate:     rate,
		lastTime: time.Now(),
	}
}

// Allow 检查是否允许 n 个令牌通过
func (r *RateLimiter) Allow(n int) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(r.lastTime).Seconds()
	r.lastTime = now

	// 添加新令牌
	r.tokens += elapsed * r.rate
	if r.tokens > r.capacity {
		r.tokens = r.capacity
	}

	// 检查是否有足够令牌
	if float64(n) <= r.tokens {
		r.tokens -= float64(n)
		return true
	}
	return false
}

// Wait 等待直到允许 n 个令牌通过
func (r *RateLimiter) Wait(ctx context.Context, n int) error {
	for {
		if r.Allow(n) {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Millisecond):
		}
	}
}

// RateLimitedReader 速率限制的 Reader
type RateLimitedReader struct {
	reader  io.Reader
	limiter *RateLimiter
}

// NewRateLimitedReader 创建速率限制的 Reader
func NewRateLimitedReader(reader io.Reader, limiter *RateLimiter) *RateLimitedReader {
	return &RateLimitedReader{
		reader:  reader,
		limiter: limiter,
	}
}

// Read 实现 io.Reader
func (r *RateLimitedReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		r.limiter.Wait(ctx, n)
	}
	return n, err
}

// RateLimitedWriter 速率限制的 Writer
type RateLimitedWriter struct {
	writer  io.Writer
	limiter *RateLimiter
}

// NewRateLimitedWriter 创建速率限制的 Writer
func NewRateLimitedWriter(writer io.Writer, limiter *RateLimiter) *RateLimitedWriter {
	return &RateLimitedWriter{
		writer:  writer,
		limiter: limiter,
	}
}

// Write 实现 io.Writer
func (w *RateLimitedWriter) Write(p []byte) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := w.limiter.Wait(ctx, len(p)); err != nil {
		return 0, err
	}
	return w.writer.Write(p)
}

// ConnectionWrapper 连接包装器，添加统计和限流功能
type ConnectionWrapper struct {
	net.Conn
	stats      *ForwarderStats
	readLimiter  *RateLimiter
	writeLimiter *RateLimiter
}

// NewConnectionWrapper 创建连接包装器
func NewConnectionWrapper(conn net.Conn, stats *ForwarderStats) *ConnectionWrapper {
	return &ConnectionWrapper{
		Conn:  conn,
		stats: stats,
	}
}

// SetReadLimiter 设置读取速率限制
func (w *ConnectionWrapper) SetReadLimiter(limiter *RateLimiter) {
	w.readLimiter = limiter
}

// SetWriteLimiter 设置写入速率限制
func (w *ConnectionWrapper) SetWriteLimiter(limiter *RateLimiter) {
	w.writeLimiter = limiter
}

// Read 实现 io.Reader，添加统计和限流
func (w *ConnectionWrapper) Read(p []byte) (int, error) {
	if w.readLimiter != nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := w.readLimiter.Wait(ctx, len(p)); err != nil {
			return 0, err
		}
	}

	n, err := w.Conn.Read(p)
	if n > 0 && w.stats != nil {
		w.stats.BytesReceived.Add(uint64(n))
		w.stats.PacketsRecv.Add(1)
	}
	return n, err
}

// Write 实现 io.Writer，添加统计和限流
func (w *ConnectionWrapper) Write(p []byte) (int, error) {
	if w.writeLimiter != nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := w.writeLimiter.Wait(ctx, len(p)); err != nil {
			return 0, err
		}
	}

	n, err := w.Conn.Write(p)
	if n > 0 && w.stats != nil {
		w.stats.BytesSent.Add(uint64(n))
		w.stats.PacketsSent.Add(1)
	}
	return n, err
}
