package terminal

import (
	"bytes"
	"sync"
	"testing"
	"time"
)

// TestAdaptiveBuffer_Basic 测试自适应缓冲区基本功能
func TestAdaptiveBuffer_Basic(t *testing.T) {
	buf := NewAdaptiveBuffer()

	// 检查初始值
	if buf.GetReadBuffer() != DefaultReadBufferSize {
		t.Errorf("Expected read buffer %d, got %d", DefaultReadBufferSize, buf.GetReadBuffer())
	}
	if buf.GetWriteBuffer() != DefaultWriteBufferSize {
		t.Errorf("Expected write buffer %d, got %d", DefaultWriteBufferSize, buf.GetWriteBuffer())
	}
}

// TestAdaptiveBuffer_RecordBytes 测试字节记录功能
func TestAdaptiveBuffer_RecordBytes(t *testing.T) {
	buf := NewAdaptiveBuffer()

	// 记录一些字节
	buf.RecordBytes(1024)
	buf.RecordBytes(2048)

	stats := buf.GetStats()
	if stats.BytesProcessed != 3072 {
		t.Errorf("Expected 3072 bytes processed, got %d", stats.BytesProcessed)
	}
}

// TestAdaptiveBuffer_Concurrent 测试并发安全性
func TestAdaptiveBuffer_Concurrent(t *testing.T) {
	buf := NewAdaptiveBuffer()
	var wg sync.WaitGroup

	// 并发记录字节
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				buf.RecordBytes(100)
			}
		}()
	}

	// 并发读取
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = buf.GetReadBuffer()
				_ = buf.GetWriteBuffer()
			}
		}()
	}

	wg.Wait()

	// 验证最终统计
	stats := buf.GetStats()
	expectedBytes := int64(100 * 1000 * 100)
	if stats.BytesProcessed != expectedBytes {
		t.Errorf("Expected %d bytes processed, got %d", expectedBytes, stats.BytesProcessed)
	}
}

// TestBatchedWriter_Basic 测试批量写入器基本功能
func TestBatchedWriter_Basic(t *testing.T) {
	var flushed bytes.Buffer
	flush := func(data []byte) error {
		_, err := flushed.Write(data)
		return err
	}

	writer := NewBatchedWriter(flush, 1024, 100*time.Millisecond)

	// 写入小数据
	if err := writer.Write([]byte("hello")); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// 立即刷新
	if err := writer.Flush(); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	if flushed.String() != "hello" {
		t.Errorf("Expected 'hello', got '%s'", flushed.String())
	}
}

// TestBatchedWriter_AutoFlush 测试自动刷新功能
func TestBatchedWriter_AutoFlush(t *testing.T) {
	var flushed bytes.Buffer
	flush := func(data []byte) error {
		_, err := flushed.Write(data)
		return err
	}

	writer := NewBatchedWriter(flush, 100, 50*time.Millisecond)

	// 写入数据
	if err := writer.Write([]byte("test data")); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// 等待自动刷新
	time.Sleep(100 * time.Millisecond)

	if flushed.String() != "test data" {
		t.Errorf("Expected 'test data', got '%s'", flushed.String())
	}
}

// TestBatchedWriter_LargeData 测试大数据直接发送
func TestBatchedWriter_LargeData(t *testing.T) {
	var flushed bytes.Buffer
	flush := func(data []byte) error {
		_, err := flushed.Write(data)
		return err
	}

	writer := NewBatchedWriter(flush, 100, 50*time.Millisecond)

	// 写入超过缓冲区大小的数据
	largeData := make([]byte, 200)
	for i := range largeData {
		largeData[i] = byte('a' + i%26)
	}

	if err := writer.Write(largeData); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// 大数据应该立即发送
	if flushed.Len() != 200 {
		t.Errorf("Expected 200 bytes flushed, got %d", flushed.Len())
	}
}

// TestBatchedWriter_BatchAccumulation 测试批量累积
func TestBatchedWriter_BatchAccumulation(t *testing.T) {
	var flushed bytes.Buffer
	flushCount := 0
	flush := func(data []byte) error {
		flushCount++
		_, err := flushed.Write(data)
		return err
	}

	writer := NewBatchedWriter(flush, 1000, 1*time.Second)

	// 写入多个小数据
	for i := 0; i < 10; i++ {
		if err := writer.Write([]byte("data")); err != nil {
			t.Fatalf("Write failed: %v", err)
		}
	}

	// 手动刷新
	if err := writer.Flush(); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	// 应该只刷新一次
	if flushCount != 1 {
		t.Errorf("Expected 1 flush, got %d", flushCount)
	}

	// 数据应该是累积的
	if flushed.Len() != 40 { // 10 * "data" (4 bytes)
		t.Errorf("Expected 40 bytes, got %d", flushed.Len())
	}
}

// TestMinMax 测试 min/max 辅助函数
func TestMinMax(t *testing.T) {
	tests := []struct {
		a, b     int
		minWant  int
		maxWant  int
	}{
		{1, 2, 1, 2},
		{5, 3, 3, 5},
		{10, 10, 10, 10},
	}

	for _, tt := range tests {
		if got := min(tt.a, tt.b); got != tt.minWant {
			t.Errorf("min(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.minWant)
		}
		if got := max(tt.a, tt.b); got != tt.maxWant {
			t.Errorf("max(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.maxWant)
		}
	}
}

// BenchmarkAdaptiveBuffer_RecordBytes 基准测试字节记录
func BenchmarkAdaptiveBuffer_RecordBytes(b *testing.B) {
	buf := NewAdaptiveBuffer()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf.RecordBytes(1024)
		}
	})
}

// BenchmarkBatchedWriter_Write 基准测试批量写入
func BenchmarkBatchedWriter_Write(b *testing.B) {
	flush := func(data []byte) error { return nil }
	writer := NewBatchedWriter(flush, 64*1024, 1*time.Second)
	data := []byte("benchmark data")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		writer.Write(data)
	}
	writer.Flush()
}
