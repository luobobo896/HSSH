package main

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

// ProgressBar 简单的进度条实现
type ProgressBar struct {
	total       int64
	current     int64
	width       int
	startTime   time.Time
	lastUpdate  time.Time
	description string
}

// NewProgressBar 创建进度条
func NewProgressBar(total int64, desc string) *ProgressBar {
	return &ProgressBar{
		total:       total,
		width:       40,
		startTime:   time.Now(),
		lastUpdate:  time.Now(),
		description: desc,
	}
}

// Add 增加进度
func (p *ProgressBar) Add(n int64) {
	atomic.AddInt64(&p.current, n)
	now := time.Now()
	if now.Sub(p.lastUpdate) > 100*time.Millisecond {
		p.lastUpdate = now
		p.render()
	}
}

// Set 设置进度
func (p *ProgressBar) Set(n int64) {
	atomic.StoreInt64(&p.current, n)
	now := time.Now()
	if now.Sub(p.lastUpdate) > 100*time.Millisecond {
		p.lastUpdate = now
		p.render()
	}
}

// Finish 完成进度条
func (p *ProgressBar) Finish() {
	p.Set(p.total)
	p.render()
	fmt.Println()
}

func (p *ProgressBar) render() {
	current := atomic.LoadInt64(&p.current)
	if current > p.total {
		current = p.total
	}

	percent := float64(current) / float64(p.total)
	filled := int(percent * float64(p.width))
	if filled > p.width {
		filled = p.width
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", p.width-filled)

	// 计算速度
	elapsed := time.Since(p.startTime).Seconds()
	speed := float64(current) / elapsed

	// 预估剩余时间
	var eta string
	if speed > 0 {
		remaining := float64(p.total-current) / speed
		eta = formatDuration(time.Duration(remaining) * time.Second)
	} else {
		eta = "--"
	}

	fmt.Printf("\r%s [%s] %6.2f%% %s/s ETA: %s",
		padRight(p.description, 20),
		bar,
		percent*100,
		formatBytes(int64(speed)),
		eta,
	)
}

// UploadProgress 上传进度跟踪
type UploadProgress struct {
	chunkCount  int
	completed   int32
	totalBytes  int64
	uploadedBytes int64
	bar         *ProgressBar
	startTime   time.Time
}

// NewUploadProgress 创建上传进度跟踪
func NewUploadProgress(chunkCount int, totalBytes int64, desc string) *UploadProgress {
	return &UploadProgress{
		chunkCount: chunkCount,
		totalBytes: totalBytes,
		bar:        NewProgressBar(int64(chunkCount), desc),
		startTime:  time.Now(),
	}
}

// ChunkComplete 标记一个分片完成
func (p *UploadProgress) ChunkComplete(chunkSize int64) {
	atomic.AddInt32(&p.completed, 1)
	atomic.AddInt64(&p.uploadedBytes, chunkSize)
	p.bar.Add(1)
}

// Finish 完成
func (p *UploadProgress) Finish() {
	p.bar.Finish()
	elapsed := time.Since(p.startTime)
	speed := float64(p.totalBytes) / elapsed.Seconds()

	fmt.Printf("✓ 上传完成: %s, 耗时: %s, 平均速度: %s/s\n",
		formatBytes(p.totalBytes),
		formatDuration(elapsed),
		formatBytes(int64(speed)),
	)
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}

func padRight(s string, length int) string {
	if len(s) >= length {
		return s[:length]
	}
	return s + strings.Repeat(" ", length-len(s))
}
