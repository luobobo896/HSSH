package main

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// UploadTask 上传任务
type UploadTask struct {
	UploadID   string  `json:"upload_id"`
	FileName   string  `json:"file_name"`
	TotalSize  int64   `json:"total_size"`
	ChunkSize  int     `json:"chunk_size"`
	ChunkCount int     `json:"chunk_count"`
	Chunks     []Chunk `json:"chunks"`
	RemoteDir  string  `json:"remote_dir"`
}

// Chunk 分片信息
type Chunk struct {
	Index    int    `json:"index"`
	Offset   int64  `json:"offset"`
	Size     int    `json:"size"`
	Checksum string `json:"checksum"`
	Data     []byte `json:"-"`
}

// Uploader 上传器
type Uploader struct {
	config     *Config
	httpClient *http.Client
}

// NewUploader 创建上传器
func NewUploader(cfg *Config) (*Uploader, error) {
	return &Uploader{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}, nil
}

// createSSHClient 创建 SSH 客户端（支持 ProxyJump）
func (u *Uploader) createSSHClient() (*ssh.Client, error) {
	key, err := os.ReadFile(expandPath(u.config.SSH.PrivateKey))
	if err != nil {
		return nil, fmt.Errorf("读取私钥失败: %w", err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("解析私钥失败: %w", err)
	}

	sshConfig := &ssh.ClientConfig{
		User:            u.config.SSH.Username,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}

	gatewayAddr := fmt.Sprintf("%s:%d", u.config.SSH.GatewayHost, u.config.SSH.GatewayPort)

	// 如果配置了跳板机，使用 ProxyJump
	if u.config.SSH.JumpHost != "" {
		return u.dialViaJump(sshConfig, gatewayAddr)
	}

	// 直连网关
	return ssh.Dial("tcp", gatewayAddr, sshConfig)
}

// dialViaJump 通过跳板机连接
func (u *Uploader) dialViaJump(sshConfig *ssh.ClientConfig, gatewayAddr string) (*ssh.Client, error) {
	jumpAddr := fmt.Sprintf("%s:%d", u.config.SSH.JumpHost, u.config.SSH.JumpPort)

	// 连接跳板机
	jumpClient, err := ssh.Dial("tcp", jumpAddr, sshConfig)
	if err != nil {
		return nil, fmt.Errorf("连接跳板机失败: %w", err)
	}

	// 通过跳板机连接目标网关
	conn, err := jumpClient.Dial("tcp", gatewayAddr)
	if err != nil {
		jumpClient.Close()
		return nil, fmt.Errorf("通过跳板机连接网关失败: %w", err)
	}

	// 在跳板机上建立到网关的 SSH 连接
	ncc, chans, reqs, err := ssh.NewClientConn(conn, gatewayAddr, sshConfig)
	if err != nil {
		conn.Close()
		jumpClient.Close()
		return nil, fmt.Errorf("建立 SSH 连接失败: %w", err)
	}

	client := ssh.NewClient(ncc, chans, reqs)
	return client, nil
}

// SplitFile 文件分片
func (u *Uploader) SplitFile(filePath string) (*UploadTask, error) {
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
	chunkSize := int64(u.config.Upload.ChunkSize)
	chunkCount := int((size + chunkSize - 1) / chunkSize)

	task := &UploadTask{
		UploadID:   generateUploadID(filePath, size),
		FileName:   filepath.Base(filePath),
		TotalSize:  size,
		ChunkSize:  u.config.Upload.ChunkSize,
		ChunkCount: chunkCount,
		Chunks:     make([]Chunk, chunkCount),
	}

	for i := 0; i < chunkCount; i++ {
		offset := int64(i) * chunkSize
		sz := chunkSize
		if offset+sz > size {
			sz = size - offset
		}

		data := make([]byte, sz)
		_, err := file.ReadAt(data, offset)
		if err != nil && err != io.EOF {
			return nil, err
		}

		task.Chunks[i] = Chunk{
			Index:    i,
			Offset:   offset,
			Size:     int(sz),
			Checksum: computeMD5(data),
			Data:     data,
		}
	}

	return task, nil
}

// UploadChunk 上传单个分片
func (u *Uploader) UploadChunk(task *UploadTask, chunk *Chunk, remoteDir string) error {
	client, err := u.createSSHClient()
	if err != nil {
		return err
	}
	defer client.Close()

	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return err
	}
	defer sftpClient.Close()

	chunkDir := filepath.Join(remoteDir, ".chunks", task.UploadID)
	if err := sftpClient.MkdirAll(chunkDir); err != nil {
		return err
	}

	remotePath := filepath.Join(chunkDir, fmt.Sprintf("chunk_%04d", chunk.Index))

	// 检查是否已存在（断点续传）
	if info, err := sftpClient.Stat(remotePath); err == nil {
		if info.Size() == int64(chunk.Size) {
			return nil // 已上传，跳过
		}
	}

	f, err := sftpClient.Create(remotePath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(chunk.Data)
	return err
}

// Upload 执行上传
func (u *Uploader) Upload(filePath string, remoteDir string) (*UploadTask, error) {
	start := time.Now()

	// 1. 分片
	task, err := u.SplitFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("分片失败: %w", err)
	}

	log.Printf("[INFO] 文件分片完成: %d 片, 总大小 %s",
		task.ChunkCount, formatBytes(task.TotalSize))

	// 2. 并发上传
	progress := NewUploadProgress(task.ChunkCount, task.TotalSize, "上传中")

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, u.config.Upload.Workers)
	errChan := make(chan error, task.ChunkCount)

	for i := range task.Chunks {
		wg.Add(1)
		semaphore <- struct{}{}

		go func(chunk *Chunk) {
			defer wg.Done()
			defer func() { <-semaphore }()

			maxRetries := u.config.Upload.MaxRetries
			for attempt := 0; attempt < maxRetries; attempt++ {
				err := u.UploadChunk(task, chunk, remoteDir)
				if err == nil {
					progress.ChunkComplete(int64(chunk.Size))
					chunk.Data = nil // 释放内存
					return
				}
				if attempt < maxRetries-1 {
					time.Sleep(time.Duration(u.config.Upload.RetryDelay*(attempt+1)) * time.Second)
				} else {
					errChan <- fmt.Errorf("分片 %d 上传失败: %w", chunk.Index, err)
				}
			}
		}(&task.Chunks[i])
	}

	wg.Wait()
	close(errChan)

	progress.Finish()

	if len(errChan) > 0 {
		return nil, <-errChan
	}

	log.Printf("[INFO] 全部分片上传完成，耗时 %v", time.Since(start))

	// 3. 触发合并
	mergeStart := time.Now()
	if err := u.Merge(task, remoteDir); err != nil {
		return nil, fmt.Errorf("合并失败: %w", err)
	}

	log.Printf("[INFO] 合并完成，耗时 %v", time.Since(mergeStart))
	log.Printf("[INFO] 上传完成: %s，总耗时 %v", task.FileName, time.Since(start))

	return task, nil
}

// Merge 触发远程合并
func (u *Uploader) Merge(task *UploadTask, remoteDir string) error {
	reqBody, _ := json.Marshal(map[string]interface{}{
		"upload_id":   task.UploadID,
		"file_name":   task.FileName,
		"chunk_count": task.ChunkCount,
		"total_size":  task.TotalSize,
		"remote_dir":  remoteDir,
	})

	resp, err := u.httpClient.Post(
		u.config.Server.GatewayURL+"/merge",
		"application/json",
		bytes.NewReader(reqBody),
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("合并请求失败: %s", body)
	}

	return nil
}

func generateUploadID(filePath string, size int64) string {
	h := md5.New()
	h.Write([]byte(filePath))
	h.Write([]byte(fmt.Sprintf("%d", size)))
	h.Write([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func computeMD5(data []byte) string {
	hash := md5.Sum(data)
	return hex.EncodeToString(hash[:])
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

func expandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[1:])
	}
	return path
}

func main() {
	var (
		configPath = flag.String("config", GetConfigPath(), "配置文件路径")
		remoteDir  = flag.String("dir", "/data/uploads", "远程目录")
		initConfig = flag.Bool("init", false, "生成示例配置文件")
	)
	flag.Parse()

	if *initConfig {
		cfg := DefaultConfig()
		if err := cfg.SaveConfig(*configPath); err != nil {
			log.Fatal("保存配置失败:", err)
		}
		fmt.Printf("示例配置已保存到: %s\n", *configPath)
		fmt.Println("请编辑配置文件后重新运行")
		return
	}

	if len(flag.Args()) < 1 {
		fmt.Println("用法: uploader [选项] <文件路径>")
		fmt.Println("\n选项:")
		flag.PrintDefaults()
		fmt.Println("\n示例:")
		fmt.Println("  uploader -init                          # 生成配置文件")
		fmt.Println("  uploader file.xlsx                      # 上传文件")
		fmt.Println("  uploader -dir /tmp file.xlsx            # 指定远程目录")
		os.Exit(1)
	}

	filePath := flag.Args()[0]

	// 加载配置
	config, err := LoadConfig(*configPath)
	if err != nil {
		log.Fatal("加载配置失败:", err)
	}

	if err := config.Validate(); err != nil {
		log.Fatal("配置无效:", err)
	}

	// 创建上传器
	uploader, err := NewUploader(config)
	if err != nil {
		log.Fatal(err)
	}

	// 执行上传
	task, err := uploader.Upload(filePath, *remoteDir)
	if err != nil {
		log.Fatalf("上传失败: %v", err)
	}

	fmt.Printf("✅ 上传成功 (ID: %s)\n", task.UploadID)
}
