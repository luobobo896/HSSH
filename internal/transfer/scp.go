package transfer

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/luobobo896/HSSH/internal/ssh"
	"github.com/luobobo896/HSSH/pkg/types"
)

// SCPTransfer SCP 文件传输器
type SCPTransfer struct {
	chain *ssh.Chain
}

// NewSCPTransfer 创建新的 SCP 传输器
func NewSCPTransfer(chain *ssh.Chain) *SCPTransfer {
	return &SCPTransfer{chain: chain}
}

// Upload 上传文件到最后一跳
func (t *SCPTransfer) Upload(localPath, remotePath string, progress chan<- *types.TransferProgress) error {
	if !t.chain.IsConnected() {
		return fmt.Errorf("SSH chain not connected")
	}

	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %w", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat local file: %w", err)
	}

	if stat.IsDir() {
		return t.uploadDir(file, localPath, remotePath, progress)
	}

	return t.uploadFile(file, stat.Size(), filepath.Base(localPath), remotePath, progress)
}

// uploadFile 上传单个文件
func (t *SCPTransfer) uploadFile(reader io.Reader, size int64, filename, remotePath string, progress chan<- *types.TransferProgress) error {
	log.Printf("[SCP] Starting uploadFile: filename=%s, remotePath=%s, size=%d", filename, remotePath, size)
	
	// 确定目标文件路径
	// 如果 remotePath 以 / 结尾，或是已存在的目录，则将文件放入该目录
	remoteFile := remotePath
	if strings.HasSuffix(remotePath, "/") {
		remoteFile = filepath.Join(remotePath, filename)
		log.Printf("[SCP] Remote path ends with /, using: %s", remoteFile)
	} else {
		// 检查是否是已存在的目录
		checkSession, err := t.chain.NewSession()
		if err == nil && checkSession != nil {
			testCmd := fmt.Sprintf("test -d %s", remotePath)
			if err := checkSession.Run(testCmd); err == nil {
				// 是已存在的目录
				remoteFile = filepath.Join(remotePath, filename)
				log.Printf("[SCP] Remote path is existing dir, using: %s", remoteFile)
			} else {
				log.Printf("[SCP] Remote path is not a dir, using as file path: %s", remoteFile)
			}
			checkSession.Close()
		} else {
			log.Printf("[SCP] Could not check if path is dir (session error: %v), using: %s", err, remoteFile)
		}
	}

	// 确保目标目录存在
	targetDir := filepath.Dir(remoteFile)
	log.Printf("[SCP] Creating target directory: %s", targetDir)
	mkdirSession, err := t.chain.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create mkdir session: %w", err)
	}
	mkdirCmd := fmt.Sprintf("mkdir -p %s", targetDir)
	if err := mkdirSession.Run(mkdirCmd); err != nil {
		log.Printf("[SCP] mkdir warning (may already exist): %v", err)
	} else {
		log.Printf("[SCP] Directory created or already exists")
	}
	mkdirSession.Close()

	// 创建文件传输 session
	log.Printf("[SCP] Creating transfer session")
	session, err := t.chain.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	// 使用 cat 命令接收文件内容（比SCP协议更可靠）
	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	// 启动 cat 命令写入文件
	catCmd := fmt.Sprintf("cat > %s", remoteFile)
	log.Printf("[SCP] Starting cat command: %s", catCmd)
	if err := session.Start(catCmd); err != nil {
		stdin.Close()
		return fmt.Errorf("failed to start cat command: %w", err)
	}
	log.Printf("[SCP] Cat command started, beginning file transfer")

	// 发送文件内容并报告进度
	buf := make([]byte, 32*1024) // 32KB 缓冲区
	var sent int64
	startTime := time.Now()

	for {
		n, err := reader.Read(buf)
		if n > 0 {
			_, writeErr := stdin.Write(buf[:n])
			if writeErr != nil {
				session.Wait()
				return fmt.Errorf("failed to write to remote: %w", writeErr)
			}
			sent += int64(n)

			if progress != nil {
				elapsed := time.Since(startTime).Seconds()
				speed := int64(0)
				if elapsed > 0 {
					speed = int64(float64(sent) / elapsed)
				}
				eta := time.Duration(0)
				if speed > 0 {
					eta = time.Duration(float64(size-sent)/float64(speed)) * time.Second
				}

				progress <- &types.TransferProgress{
					FileName:   filename,
					TotalBytes: size,
					SentBytes:  sent,
					Speed:      speed,
					ETA:        eta,
					Status:     "running",
				}
			}
		}
		if err == io.EOF {
			log.Printf("[SCP] Reached EOF, sent %d/%d bytes", sent, size)
			break
		}
		if err != nil {
			stdin.Close()
			session.Wait()
			return fmt.Errorf("failed to read local file: %w", err)
		}
	}

	// 关闭 stdin 表示文件传输完成
	log.Printf("[SCP] Closing stdin to signal EOF")
	stdin.Close()

	// 等待命令完成
	log.Printf("[SCP] Waiting for cat command to complete")
	if err := session.Wait(); err != nil {
		return fmt.Errorf("remote cat command failed: %w", err)
	}
	log.Printf("[SCP] Cat command completed successfully")

	// 设置文件权限 (0644)
	log.Printf("[SCP] Setting file permissions: chmod 644 %s", remoteFile)
	chmodSession, _ := t.chain.NewSession()
	if chmodSession != nil {
		if err := chmodSession.Run(fmt.Sprintf("chmod 644 %s", remoteFile)); err != nil {
			log.Printf("[SCP] chmod warning: %v", err)
		} else {
			log.Printf("[SCP] File permissions set successfully")
		}
		chmodSession.Close()
	}

	// 验证文件是否存在
	verifySession, _ := t.chain.NewSession()
	if verifySession != nil {
		lsCmd := fmt.Sprintf("ls -la %s", remoteFile)
		output, err := verifySession.Output(lsCmd)
		if err != nil {
			log.Printf("[SCP] WARNING: Failed to verify file: %v", err)
		} else {
			log.Printf("[SCP] File verified on remote: %s", strings.TrimSpace(string(output)))
		}
		verifySession.Close()
	}

	if progress != nil {
		progress <- &types.TransferProgress{
			FileName:   filename,
			TotalBytes: size,
			SentBytes:  size,
			Status:     "completed",
		}
	}

	log.Printf("[SCP] Upload completed successfully: %s", remoteFile)
	return nil
}

// uploadDir 上传目录
func (t *SCPTransfer) uploadDir(dir *os.File, localPath, remotePath string, progress chan<- *types.TransferProgress) error {
	entries, err := dir.ReadDir(-1)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		localFile := filepath.Join(localPath, entry.Name())
		remoteFile := filepath.Join(remotePath, entry.Name())

		if entry.IsDir() {
			// 创建远程目录
			session, err := t.chain.NewSession()
			if err != nil {
				return err
			}
			mkdirCmd := fmt.Sprintf("mkdir -p %s", remoteFile)
			session.Run(mkdirCmd)
			session.Close()

			// 递归上传子目录
			subDir, _ := os.Open(localFile)
			t.uploadDir(subDir, localFile, remoteFile, progress)
			subDir.Close()
		} else {
			file, _ := os.Open(localFile)
			stat, _ := file.Stat()
			t.uploadFile(file, stat.Size(), entry.Name(), remoteFile, progress)
			file.Close()
		}
	}

	return nil
}

// Download 从远程下载文件
func (t *SCPTransfer) Download(remotePath, localPath string, progress chan<- *types.TransferProgress) error {
	if !t.chain.IsConnected() {
		return fmt.Errorf("SSH chain not connected")
	}

	session, err := t.chain.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	// 获取远程文件大小
	stdout, _, err := t.chain.Execute(fmt.Sprintf("stat -f%%z %s 2>/dev/null || stat -c%%s %s 2>/dev/null", remotePath, remotePath))
	if err != nil {
		return fmt.Errorf("failed to get remote file size: %w", err)
	}

	var size int64
	fmt.Sscanf(strings.TrimSpace(stdout), "%d", &size)

	// 创建本地文件
	localFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create local file: %w", err)
	}
	defer localFile.Close()

	// 使用 cat 读取远程文件（比SCP协议更可靠）
	stdoutPipe, err := session.StdoutPipe()
	if err != nil {
		return err
	}

	catCmd := fmt.Sprintf("cat %s", remotePath)
	if err := session.Start(catCmd); err != nil {
		return fmt.Errorf("failed to start cat command: %w", err)
	}

	// 读取文件内容
	buf := make([]byte, 32*1024)
	var received int64
	startTime := time.Now()

	for received < size {
		n, err := stdoutPipe.Read(buf)
		if n > 0 {
			localFile.Write(buf[:n])
			received += int64(n)

			if progress != nil {
				elapsed := time.Since(startTime).Seconds()
				speed := int64(0)
				if elapsed > 0 {
					speed = int64(float64(received) / elapsed)
				}
				eta := time.Duration(0)
				if speed > 0 {
					eta = time.Duration(float64(size-received)/float64(speed)) * time.Second
				}

				progress <- &types.TransferProgress{
					FileName:   filepath.Base(remotePath),
					TotalBytes: size,
					SentBytes:  received,
					Speed:      speed,
					ETA:        eta,
					Status:     "running",
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	if err := session.Wait(); err != nil {
		return fmt.Errorf("remote cat command failed: %w", err)
	}

	if progress != nil {
		progress <- &types.TransferProgress{
			FileName:   filepath.Base(remotePath),
			TotalBytes: size,
			SentBytes:  size,
			Status:     "completed",
		}
	}

	return nil
}
