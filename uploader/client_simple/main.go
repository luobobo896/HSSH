// +build ignore

package main

// 纯标准库版本的简单 SFTP 上传客户端
// 不依赖外部库，仅需 Go 标准库
// 适用于无法下载依赖的环境

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// 使用系统 sftp 命令行工具

type Uploader struct {
	jumpHost    string
	gatewayHost string
	username    string
	keyFile     string
	workers     int
	chunkSize   int
	gatewayURL  string
	httpClient  *http.Client
}

func NewUploader() *Uploader {
	return &Uploader{
		jumpHost:    getEnv("HK_HOST", "hk-relay.example.com"),
		gatewayHost: getEnv("GW_HOST", "gateway.corp.internal"),
		username:    getEnv("SSH_USER", os.Getenv("USER")),
		keyFile:     getEnv("SSH_KEY", filepath.Join(os.Getenv("HOME"), ".ssh/id_rsa")),
		workers:     getEnvInt("WORKERS", runtime.NumCPU()*2),
		chunkSize:   getEnvInt("CHUNK_SIZE", 512*1024),
		gatewayURL:  getEnv("GATEWAY_URL", "http://localhost:8080"),
		httpClient:  &http.Client{Timeout: 60 * time.Second},
	}
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultVal
}

type Chunk struct {
	Index    int
	Data     []byte
	Checksum string
}

type Task struct {
	ID         string
	FileName   string
	TotalSize  int64
	ChunkCount int
	Chunks     []Chunk
}

func (u *Uploader) splitFile(filePath string) (*Task, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}

	size := stat.Size()
	chunkCount := int((size + int64(u.chunkSize) - 1) / int64(u.chunkSize))

	h := md5.New()
	h.Write([]byte(filePath))
	h.Write([]byte(fmt.Sprintf("%d", size)))
	h.Write([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	id := hex.EncodeToString(h.Sum(nil))[:16]

	task := &Task{
		ID:         id,
		FileName:   filepath.Base(filePath),
		TotalSize:  size,
		ChunkCount: chunkCount,
		Chunks:     make([]Chunk, chunkCount),
	}

	for i := 0; i < chunkCount; i++ {
		offset := int64(i) * int64(u.chunkSize)
		sz := u.chunkSize
		if offset+int64(sz) > size {
			sz = int(size - offset)
		}

		data := make([]byte, sz)
		_, err := file.ReadAt(data, offset)
		if err != nil && err != io.EOF {
			return nil, err
		}

		checksum := md5.Sum(data)
		task.Chunks[i] = Chunk{
			Index:    i,
			Data:     data,
			Checksum: hex.EncodeToString(checksum[:]),
		}
	}

	return task, nil
}

func (u *Uploader) uploadChunk(task *Task, chunk *Chunk, remoteDir string) error {
	// 使用临时文件存储分片数据
	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("chunk_%s_%04d", task.ID, chunk.Index))
	if err := os.WriteFile(tmpFile, chunk.Data, 0600); err != nil {
		return err
	}
	defer os.Remove(tmpFile)

	// 构建远程路径
	chunkDir := filepath.Join(remoteDir, ".chunks", task.ID)
	remotePath := filepath.Join(chunkDir, fmt.Sprintf("chunk_%04d", chunk.Index))

	// 检查分片是否已存在
	checkCmd := u.buildSSHCommand("test", "-f", remotePath, "&&", "echo", "exists")
	if out, _ := checkCmd.CombinedOutput(); strings.Contains(string(out), "exists") {
		return nil // 已存在，跳过
	}

	// 使用 sftp 上传
	// sftp -i key -o ProxyJump=user@jump user@gateway
	sftpScript := fmt.Sprintf("mkdir -p %s\nput %s %s\n", chunkDir, tmpFile, remotePath)

	var cmd *exec.Cmd
	if u.jumpHost != "" {
		cmd = exec.Command("sftp",
			"-i", u.keyFile,
			"-o", fmt.Sprintf("ProxyJump=%s@%s", u.username, u.jumpHost),
			"-o", "StrictHostKeyChecking=no",
			"-b", "-",
			fmt.Sprintf("%s@%s", u.username, u.gatewayHost),
		)
	} else {
		cmd = exec.Command("sftp",
			"-i", u.keyFile,
			"-o", "StrictHostKeyChecking=no",
			"-b", "-",
			fmt.Sprintf("%s@%s", u.username, u.gatewayHost),
		)
	}

	cmd.Stdin = strings.NewReader(sftpScript)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sftp failed: %w, output: %s", err, out)
	}

	return nil
}

func (u *Uploader) buildSSHCommand(args ...string) *exec.Cmd {
	sshArgs := []string{
		"-i", u.keyFile,
		"-o", "StrictHostKeyChecking=no",
		"-o", "ConnectTimeout=30",
	}
	if u.jumpHost != "" {
		sshArgs = append(sshArgs, "-o", fmt.Sprintf("ProxyJump=%s@%s", u.username, u.jumpHost))
	}
	sshArgs = append(sshArgs, fmt.Sprintf("%s@%s", u.username, u.gatewayHost))
	sshArgs = append(sshArgs, args...)
	return exec.Command("ssh", sshArgs...)
}

func (u *Uploader) merge(task *Task, remoteDir string) error {
	reqBody, _ := json.Marshal(map[string]interface{}{
		"upload_id":   task.ID,
		"file_name":   task.FileName,
		"chunk_count": task.ChunkCount,
		"total_size":  task.TotalSize,
		"remote_dir":  remoteDir,
	})

	resp, err := u.httpClient.Post(
		u.gatewayURL+"/merge",
		"application/json",
		bytes.NewReader(reqBody),
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("merge failed: %s", body)
	}

	return nil
}

func (u *Uploader) Upload(filePath, remoteDir string) error {
	start := time.Now()

	// 1. 分片
	task, err := u.splitFile(filePath)
	if err != nil {
		return fmt.Errorf("split failed: %w", err)
	}

	log.Printf("[INFO] File split: %d chunks, %s total",
		task.ChunkCount, formatBytes(task.TotalSize))

	// 2. 并发上传
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, u.workers)
	errChan := make(chan error, task.ChunkCount)
	var completed int32

	for i := range task.Chunks {
		wg.Add(1)
		semaphore <- struct{}{}

		go func(chunk *Chunk) {
			defer wg.Done()
			defer func() { <-semaphore }()

			for attempt := 0; attempt < 3; attempt++ {
				if err := u.uploadChunk(task, chunk, remoteDir); err == nil {
					atomic.AddInt32(&completed, 1)
					chunk.Data = nil
					if atomic.LoadInt32(&completed)%10 == 0 || int(atomic.LoadInt32(&completed)) == task.ChunkCount {
						log.Printf("[INFO] Progress: %d/%d", atomic.LoadInt32(&completed), task.ChunkCount)
					}
					return
				} else if attempt < 2 {
					time.Sleep(time.Duration(attempt+1) * time.Second)
				} else {
					errChan <- fmt.Errorf("chunk %d failed: %w", chunk.Index, err)
				}
			}
		}(&task.Chunks[i])
	}

	wg.Wait()
	close(errChan)

	if len(errChan) > 0 {
		return <-errChan
	}

	log.Printf("[INFO] All chunks uploaded in %v", time.Since(start))

	// 3. 合并
	if err := u.merge(task, remoteDir); err != nil {
		return fmt.Errorf("merge failed: %w", err)
	}

	log.Printf("[INFO] Upload complete: %s in %v", task.FileName, time.Since(start))
	return nil
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

func main() {
	if len(os.Args) < 2 {
		fmt.Println("用法: uploader_simple <文件路径>")
		fmt.Println("\n环境变量:")
		fmt.Println("  HK_HOST     - 香港中转服务器")
		fmt.Println("  GW_HOST     - 内网网关")
		fmt.Println("  SSH_USER    - SSH 用户名")
		fmt.Println("  SSH_KEY     - SSH 私钥路径")
		fmt.Println("  WORKERS     - 并发数")
		fmt.Println("  CHUNK_SIZE  - 分片大小")
		fmt.Println("  GATEWAY_URL - 网关 HTTP API")
		os.Exit(1)
	}

	filePath := os.Args[1]
	remoteDir := "/data/uploads"

	uploader := NewUploader()

	if err := uploader.Upload(filePath, remoteDir); err != nil {
		log.Fatalf("Upload failed: %v", err)
	}

	fmt.Println("✅ Upload successful")
}
