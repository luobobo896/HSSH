package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// MergeRequest 合并请求
type MergeRequest struct {
	UploadID   string `json:"upload_id"`
	FileName   string `json:"file_name"`
	ChunkCount int    `json:"chunk_count"`
	TotalSize  int64  `json:"total_size"`
	RemoteDir  string `json:"remote_dir"`
}

// UploadStatus 上传状态
type UploadStatus struct {
	UploadID    string    `json:"upload_id"`
	FileName    string    `json:"file_name"`
	ChunkCount  int       `json:"chunk_count"`
	Received    int       `json:"received"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
	Error       string    `json:"error,omitempty"`
	FinalPath   string    `json:"final_path,omitempty"`
}

// Server 网关服务
type Server struct {
	uploadDir string
	chunkTTL  time.Duration
	mu        sync.RWMutex
	uploads   map[string]*UploadStatus
}

func NewServer(uploadDir string) *Server {
	return &Server{
		uploadDir: uploadDir,
		chunkTTL:  24 * time.Hour,
		uploads:   make(map[string]*UploadStatus),
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// CORS
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Logging
	start := time.Now()
	defer func() {
		log.Printf("[%s] %s %s %v", r.Method, r.URL.Path, r.RemoteAddr, time.Since(start))
	}()

	switch r.URL.Path {
	case "/merge":
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleMerge(w, r)
	case "/status":
		s.handleStatus(w, r)
	case "/health":
		s.handleHealth(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleMerge(w http.ResponseWriter, r *http.Request) {
	var req MergeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	// 安全校验
	if !s.isValidPath(req.RemoteDir) {
		http.Error(w, `{"error":"invalid path"}`, http.StatusBadRequest)
		return
	}
	if strings.Contains(req.FileName, "/") || strings.Contains(req.FileName, "..") {
		http.Error(w, `{"error":"invalid filename"}`, http.StatusBadRequest)
		return
	}

	chunkDir := filepath.Join(req.RemoteDir, ".chunks", req.UploadID)
	received, err := s.countChunks(chunkDir)
	if err != nil {
		http.Error(w, `{"error":"chunks not found"}`, http.StatusNotFound)
		return
	}

	if received < req.ChunkCount {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusPartialContent)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":    fmt.Sprintf("incomplete: %d/%d", received, req.ChunkCount),
			"received": received,
			"expected": req.ChunkCount,
		})
		return
	}

	// 记录状态
	s.mu.Lock()
	status := &UploadStatus{
		UploadID:  req.UploadID,
		FileName:  req.FileName,
		ChunkCount: req.ChunkCount,
		Received:  received,
		Status:    "merging",
		CreatedAt: time.Now(),
	}
	s.uploads[req.UploadID] = status
	s.mu.Unlock()

	// 异步合并
	go func() {
		finalPath := filepath.Join(req.RemoteDir, req.FileName)
		if err := s.mergeChunks(chunkDir, finalPath, req.ChunkCount); err != nil {
			s.mu.Lock()
			status.Status = "failed"
			status.Error = err.Error()
			s.mu.Unlock()
			log.Printf("[ERROR] Merge failed %s: %v", req.UploadID, err)
			return
		}

		s.mu.Lock()
		status.Status = "completed"
		status.CompletedAt = time.Now()
		status.FinalPath = finalPath
		s.mu.Unlock()

		log.Printf("[INFO] Merge completed: %s -> %s", req.UploadID, finalPath)
		s.cleanupChunks(chunkDir)
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":     true,
		"upload_id":   req.UploadID,
		"status":      "merging",
		"received":    received,
		"chunk_count": req.ChunkCount,
	})
}

func (s *Server) mergeChunks(chunkDir, finalPath string, count int) error {
	if err := os.MkdirAll(filepath.Dir(finalPath), 0755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}

	out, err := os.Create(finalPath)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer out.Close()

	buf := make([]byte, 32*1024)
	for i := 0; i < count; i++ {
		chunkPath := filepath.Join(chunkDir, fmt.Sprintf("chunk_%04d", i))
		in, err := os.Open(chunkPath)
		if err != nil {
			return fmt.Errorf("open chunk %d: %w", i, err)
		}

		_, err = io.CopyBuffer(out, in, buf)
		in.Close()
		if err != nil {
			return fmt.Errorf("copy chunk %d: %w", i, err)
		}
	}

	return out.Sync()
}

func (s *Server) countChunks(chunkDir string) (int, error) {
	entries, err := os.ReadDir(chunkDir)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "chunk_") {
			count++
		}
	}
	return count, nil
}

func (s *Server) cleanupChunks(chunkDir string) {
	if err := os.RemoveAll(chunkDir); err != nil {
		log.Printf("[WARN] cleanup failed: %s, %v", chunkDir, err)
	}
}

func (s *Server) isValidPath(path string) bool {
	clean := filepath.Clean(path)
	return filepath.IsAbs(clean)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	uploadID := r.URL.Query().Get("id")
	if uploadID == "" {
		http.Error(w, `{"error":"missing id"}`, http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	status, ok := s.uploads[uploadID]
	s.mu.RUnlock()

	if !ok {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"time":   time.Now().Unix(),
	})
}

func (s *Server) cleanupLoop() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for id, st := range s.uploads {
			if now.Sub(st.CreatedAt) > 24*time.Hour {
				delete(s.uploads, id)
			}
		}
		s.mu.Unlock()
	}
}

func main() {
	uploadDir := getEnv("UPLOAD_DIR", "/data/uploads")
	port := getEnv("PORT", "8080")

	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		log.Fatal("Failed to create upload dir:", err)
	}

	server := NewServer(uploadDir)
	go server.cleanupLoop()

	// 注册路由
	http.Handle("/", server)

	log.Printf("[INFO] Server starting on :%s, upload dir: %s", port, uploadDir)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
