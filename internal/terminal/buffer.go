// Package terminal 提供高性能终端连接功能
package terminal

import (
	"sync"
	"time"
)

// AdaptiveBuffer 自适应缓冲区，根据数据速率动态调整大小
type AdaptiveBuffer struct {
	mu sync.RWMutex

	// 当前缓冲区配置
	readSize  int
	writeSize int

	// 性能指标
	bytesProcessed int64
	lastAdjustTime time.Time

	// 配置限制
	minSize int
	maxSize int
}

// BufferStats 缓冲区统计
type BufferStats struct {
	ReadSize       int
	WriteSize      int
	BytesProcessed int64
}

// 默认缓冲区配置
const (
	DefaultMinBufferSize = 4 * 1024       // 4KB 最小
	DefaultMaxBufferSize = 256 * 1024     // 256KB 最大
	DefaultReadBufferSize  = 32 * 1024    // 32KB 初始读缓冲
	DefaultWriteBufferSize = 64 * 1024    // 64KB 初始写缓冲

	bufferAdjustInterval = 5 * time.Second
)

// NewAdaptiveBuffer 创建新的自适应缓冲区
func NewAdaptiveBuffer() *AdaptiveBuffer {
	return &AdaptiveBuffer{
		readSize:       DefaultReadBufferSize,
		writeSize:      DefaultWriteBufferSize,
		lastAdjustTime: time.Now(),
		minSize:        DefaultMinBufferSize,
		maxSize:        DefaultMaxBufferSize,
	}
}

// GetReadBuffer 获取当前读缓冲区大小
func (b *AdaptiveBuffer) GetReadBuffer() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.readSize
}

// GetWriteBuffer 获取当前写缓冲区大小
func (b *AdaptiveBuffer) GetWriteBuffer() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.writeSize
}

// RecordBytes 记录处理的字节数，用于自适应调整
func (b *AdaptiveBuffer) RecordBytes(n int) {
	b.mu.Lock()
	b.bytesProcessed += int64(n)
	b.mu.Unlock()

	// 尝试自适应调整
	b.maybeAdjust()
}

// maybeAdjust 根据性能指标调整缓冲区大小
func (b *AdaptiveBuffer) maybeAdjust() {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	if now.Sub(b.lastAdjustTime) < bufferAdjustInterval {
		return
	}

	// 计算吞吐率 (bytes/second)
	elapsed := now.Sub(b.lastAdjustTime).Seconds()
	if elapsed < 1 {
		return
	}

	throughput := float64(b.bytesProcessed) / elapsed

	// 根据吞吐率调整缓冲区大小
	// 高吞吐量 -> 增大缓冲区减少系统调用
	// 低吞吐量 -> 减小缓冲区降低延迟
	switch {
	case throughput > 10*1024*1024: // > 10MB/s
		b.readSize = min(b.readSize*2, b.maxSize)
		b.writeSize = min(b.writeSize*2, b.maxSize)
	case throughput > 1*1024*1024: // > 1MB/s
		b.readSize = min(int(float64(b.readSize)*1.5), b.maxSize)
		b.writeSize = min(int(float64(b.writeSize)*1.5), b.maxSize)
	case throughput < 100*1024: // < 100KB/s
		b.readSize = max(b.readSize/2, b.minSize)
		b.writeSize = max(b.writeSize/2, b.minSize)
	}

	b.bytesProcessed = 0
	b.lastAdjustTime = now
}

// GetStats 获取当前统计信息
func (b *AdaptiveBuffer) GetStats() BufferStats {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return BufferStats{
		ReadSize:       b.readSize,
		WriteSize:      b.writeSize,
		BytesProcessed: b.bytesProcessed,
	}
}

// BatchedWriter 批量写入器，减少小数据包的系统调用
type BatchedWriter struct {
	mu     sync.Mutex
	buffer []byte
	size   int
	flush  func([]byte) error
	timer  *time.Timer
	delay  time.Duration
}

// NewBatchedWriter 创建新的批量写入器
// flush: 实际刷新函数
// maxSize: 最大批量大小
// maxDelay: 最大延迟时间
func NewBatchedWriter(flush func([]byte) error, maxSize int, maxDelay time.Duration) *BatchedWriter {
	return &BatchedWriter{
		buffer: make([]byte, 0, maxSize),
		flush:  flush,
		delay:  maxDelay,
	}
}

// Write 写入数据到批量缓冲区
func (w *BatchedWriter) Write(data []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// 如果数据太大，直接发送
	if len(data) >= cap(w.buffer) {
		return w.flush(data)
	}

	// 如果缓冲区满了，先刷新
	if len(w.buffer)+len(data) > cap(w.buffer) {
		if err := w.flushLocked(); err != nil {
			return err
		}
	}

	w.buffer = append(w.buffer, data...)

	// 启动延迟刷新定时器
	if w.timer == nil && w.delay > 0 {
		w.timer = time.AfterFunc(w.delay, func() {
			w.Flush()
		})
	}

	return nil
}

// Flush 立即刷新缓冲区
func (w *BatchedWriter) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.flushLocked()
}

// flushLocked 在持有锁的情况下刷新缓冲区
func (w *BatchedWriter) flushLocked() error {
	if len(w.buffer) == 0 {
		return nil
	}

	// 停止定时器
	if w.timer != nil {
		w.timer.Stop()
		w.timer = nil
	}

	// 复制数据避免在 flush 过程中持有锁
	data := make([]byte, len(w.buffer))
	copy(data, w.buffer)
	w.buffer = w.buffer[:0]

	return w.flush(data)
}

// Close 关闭批量写入器，刷新剩余数据
func (w *BatchedWriter) Close() error {
	return w.Flush()
}

// min 返回较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// max 返回较大值
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
