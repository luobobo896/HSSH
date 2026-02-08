package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/luobobo896/HSSH"
	"github.com/luobobo896/HSSH/internal/config"
	"github.com/luobobo896/HSSH/internal/profiler"
	"github.com/luobobo896/HSSH/internal/proxy"
	"github.com/luobobo896/HSSH/internal/ssh"
	"github.com/luobobo896/HSSH/internal/transfer"
	"github.com/luobobo896/HSSH/pkg/types"
)

// Helper functions
func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func firstNonZero(a, b int) int {
	if a != 0 {
		return a
	}
	return b
}

// buildHopChainWithGateways 递归构建包含所有必要网关的链路
// 展开每个节点的 gateway 链，避免重复，检测循环
// via 参数现在是服务器 ID 列表
func (s *Server) buildHopChainWithGateways(via []string) []*types.Hop {
	var hops []*types.Hop
	visited := make(map[string]bool) // 防止循环，存储的是 ID

	// 递归添加节点及其网关
	var addHopWithGateway func(hopID string)
	addHopWithGateway = func(hopID string) {
		if visited[hopID] {
			return // 已添加，避免循环
		}

		hop := s.config.GetHopByID(hopID)
		if hop == nil {
			log.Printf("[UPLOAD] Warning: hop with id '%s' not found", hopID)
			return
		}

		// 先添加该节点的网关（如果有且不是它自己）
		if hop.GatewayID != "" && hop.GatewayID != hop.ID {
			addHopWithGateway(hop.GatewayID)
		}

		// 再添加该节点本身
		if !visited[hop.ID] {
			hops = append(hops, hop)
			visited[hop.ID] = true
			log.Printf("[UPLOAD] Added hop to chain: %s (id: %s, gateway_id: %s)", hop.Name, hop.ID, hop.GatewayID)
		}
	}

	// 处理每个 via 节点
	for _, hopID := range via {
		addHopWithGateway(hopID)
	}

	return hops
}

// Server HTTP API 服务器
type Server struct {
	config        *types.Config
	manager       *config.Manager
	profiler      *profiler.NetworkProfiler
	proxies       *proxy.ForwarderManager
	uploads       map[string]*types.TransferProgress
	mu            sync.RWMutex
	portalForwarders map[string]*proxy.PortForwarder // mapping_id -> forwarder
	portalMu         sync.RWMutex
}

// NewServer 创建新的 API 服务器
func NewServer() (*Server, error) {
	mgr, err := config.NewManager()
	if err != nil {
		return nil, err
	}

	cfg, err := mgr.Load()
	if err != nil {
		return nil, err
	}

	return &Server{
		config:           cfg,
		manager:          mgr,
		profiler:         profiler.NewNetworkProfiler(0),
		proxies:          proxy.NewForwarderManager(),
		uploads:          make(map[string]*types.TransferProgress),
		portalForwarders: make(map[string]*proxy.PortForwarder),
	}, nil
}

// RegisterRoutes 注册路由
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	// 服务器管理
	mux.HandleFunc("/api/servers", s.handleServers)
	mux.HandleFunc("/api/servers/", s.handleServerDetail)

	// 路由配置
	mux.HandleFunc("/api/routes", s.handleRoutes)

	// 文件上传
	mux.HandleFunc("/api/upload", s.handleUpload)

	// 端口转发
	mux.HandleFunc("/api/proxy", s.handleProxies)
	mux.HandleFunc("/api/proxy/", s.handleProxyDetail)

	// 性能指标
	mux.HandleFunc("/api/metrics/latency", s.handleLatencyProbe)

	// WebSocket 进度推送
	mux.HandleFunc("/api/ws/progress/", s.handleProgressWebSocket)

	// WebSocket 终端
	mux.HandleFunc("/api/terminal", s.handleTerminal)

	// 目录浏览
	mux.HandleFunc("/api/browse/", s.handleBrowse)

	// Portal 端口转发管理
	mux.HandleFunc("/api/portal", s.handlePortal)
	mux.HandleFunc("/api/portal/mappings", s.handlePortalMappings)
	mux.HandleFunc("/api/portal/mappings/", s.handlePortalMappingDetail)

	// 静态文件（前端）- 使用嵌入的文件系统
	staticFS, err := fs.Sub(gmssh.WebDist, "web/dist")
	if err != nil {
		log.Printf("Warning: failed to load embedded web assets: %v", err)
	} else {
		mux.Handle("/", http.FileServer(http.FS(staticFS)))
	}
}

// Start 启动服务器
func (s *Server) Start(addr string) error {
	mux := http.NewServeMux()
	s.RegisterRoutes(mux)

	// CORS 中间件
	handler := corsMiddleware(mux)

	log.Printf("Starting API server on %s", addr)
	return http.ListenAndServe(addr, handler)
}

// corsMiddleware CORS 中间件
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// jsonResponse 发送 JSON 响应
func jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Error encoding JSON: %v", err)
	}
}

// errorResponse 发送错误响应
func errorResponse(w http.ResponseWriter, status int, message string) {
	jsonResponse(w, status, map[string]string{"error": message})
}

// CreateServerRequest 创建服务器请求
type CreateServerRequest struct {
	Name       string `json:"name"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	User       string `json:"user"`
	AuthType   string `json:"auth_type"`
	KeyPath    string `json:"key_path,omitempty"`
	Password   string `json:"password,omitempty"`
	ServerType string `json:"server_type"`          // "external" | "internal"
	GatewayID  string `json:"gateway_id,omitempty"` // 内网服务器的网关ID
}

// handleServers 处理服务器列表
func (s *Server) handleServers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		jsonResponse(w, http.StatusOK, s.config.Hops)
	case http.MethodPost:
		var req CreateServerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			errorResponse(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
			return
		}

		// 验证必填字段
		if req.Name == "" || req.Host == "" || req.User == "" {
			errorResponse(w, http.StatusBadRequest, "name, host, and user are required")
			return
		}

		// 转换 auth_type
		var authMethod types.AuthMethod
		switch req.AuthType {
		case "key":
			authMethod = types.AuthKey
		case "password":
			authMethod = types.AuthPassword
		default:
			errorResponse(w, http.StatusBadRequest, "auth_type must be 'key' or 'password'")
			return
		}

		// 转换 server_type (支持数字和字符串两种格式)
		var serverType types.ServerType
		switch req.ServerType {
		case "external", "0":
			serverType = types.ServerExternal
		case "internal", "1":
			serverType = types.ServerInternal
		default:
			serverType = types.ServerExternal // 默认外网
		}

		// 内网服务器必须配置网关
		if serverType == types.ServerInternal && req.GatewayID == "" {
			errorResponse(w, http.StatusBadRequest, "internal server requires a gateway")
			return
		}

		// 验证 gateway_id 存在且有效
		if req.GatewayID != "" {
			if gateway := s.config.GetHopByID(req.GatewayID); gateway == nil {
				errorResponse(w, http.StatusBadRequest, "invalid gateway_id: gateway not found")
				return
			}
		}

		// 设置默认端口
		if req.Port == 0 {
			req.Port = 22
		}

		// 设置默认密钥路径
		if authMethod == types.AuthKey && req.KeyPath == "" {
			req.KeyPath = "~/.ssh/id_rsa"
		}

		hop := &types.Hop{
			Name:       req.Name,
			Host:       req.Host,
			Port:       req.Port,
			User:       req.User,
			AuthType:   authMethod,
			KeyPath:    req.KeyPath,
			Password:   req.Password,
			ServerType: serverType,
			GatewayID:  req.GatewayID,
		}

		if err := s.manager.AddHop(hop); err != nil {
			errorResponse(w, http.StatusConflict, err.Error())
			return
		}

		jsonResponse(w, http.StatusCreated, hop)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// TestConnectionResponse 连接测试结果响应
type TestConnectionResponse struct {
	Success   bool   `json:"success"`
	LatencyMs int64  `json:"latency_ms"`
	Error     string `json:"error,omitempty"`
}

// handleServerDetail 处理单个服务器
func (s *Server) handleServerDetail(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path[len("/api/servers/"):]
	parts := strings.SplitN(path, "/", 2)
	id := parts[0] // UUID 是 URL-safe 的，无需解码
	subPath := ""
	if len(parts) > 1 {
		subPath = parts[1]
	}

	// 查找服务器
	hop := s.config.GetHopByID(id)
	if hop == nil {
		errorResponse(w, http.StatusNotFound, "Server not found")
		return
	}

	// 处理测试连接请求 /api/servers/:id/test
	if subPath == "test" && r.Method == http.MethodPost {
		s.handleTestConnection(w, r, hop) // 传入 hop 对象而不是 name
		return
	}

	switch r.Method {
	case http.MethodGet:
		jsonResponse(w, http.StatusOK, hop)
	case http.MethodPut:
		var req CreateServerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Printf("[DEBUG] PUT /api/servers/%s - JSON decode error: %v", id, err)
			errorResponse(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
			return
		}
		log.Printf("[DEBUG] PUT /api/servers/%s - request: %+v", id, req)
		log.Printf("[DEBUG] PUT /api/servers/%s - existing: server_type=%d, gateway_id=%s", id, hop.ServerType, hop.GatewayID)

		// 转换 auth_type
		var authMethod types.AuthMethod
		if req.AuthType != "" {
			switch req.AuthType {
			case "key":
				authMethod = types.AuthKey
			case "password":
				authMethod = types.AuthPassword
			default:
				errorResponse(w, http.StatusBadRequest, "auth_type must be 'key' or 'password'")
				return
			}
		} else {
			authMethod = hop.AuthType
		}

		// 转换 server_type (支持数字和字符串两种格式)
		var serverType types.ServerType
		if req.ServerType != "" {
			switch req.ServerType {
			case "external", "0":
				serverType = types.ServerExternal
			case "internal", "1":
				serverType = types.ServerInternal
			default:
				serverType = hop.ServerType
			}
		} else {
			serverType = hop.ServerType
		}

		// 验证 gateway_id（如果提供）
		gatewayID := hop.GatewayID
		if req.GatewayID != "" {
			if gateway := s.config.GetHopByID(req.GatewayID); gateway == nil {
				errorResponse(w, http.StatusBadRequest, "invalid gateway_id: gateway not found")
				return
			}
			gatewayID = req.GatewayID
		}

		// 内网服务器必须配置网关
		log.Printf("[DEBUG] PUT /api/servers/%s - serverType=%d, req.GatewayID=%s, existing.GatewayID=%s", id, serverType, req.GatewayID, hop.GatewayID)
		if serverType == types.ServerInternal && gatewayID == "" {
			log.Printf("[DEBUG] PUT /api/servers/%s - rejected: internal server requires gateway", id)
			errorResponse(w, http.StatusBadRequest, "internal server requires a gateway")
			return
		}

		// 使用现有值或新值
		updatedHop := &types.Hop{
			ID:         hop.ID, // 保留原 ID
			Name:       firstNonEmpty(req.Name, hop.Name),
			Host:       firstNonEmpty(req.Host, hop.Host),
			Port:       firstNonZero(req.Port, hop.Port),
			User:       firstNonEmpty(req.User, hop.User),
			AuthType:   authMethod,
			KeyPath:    firstNonEmpty(req.KeyPath, hop.KeyPath),
			Password:   firstNonEmpty(req.Password, hop.Password),
			ServerType: serverType,
			GatewayID:  gatewayID,
		}

		if err := s.manager.UpdateHop(id, updatedHop); err != nil {
			errorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}

		jsonResponse(w, http.StatusOK, updatedHop)
	case http.MethodDelete:
		if err := s.manager.DeleteHop(id); err != nil {
			errorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		jsonResponse(w, http.StatusNoContent, nil)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// CreateRouteRequest 创建路由请求
type CreateRouteRequest struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Via       string `json:"via,omitempty"`
	Threshold int    `json:"threshold_ms"`
}

// handleRoutes 处理路由配置
func (s *Server) handleRoutes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		jsonResponse(w, http.StatusOK, s.config.Routes)
	case http.MethodPost:
		var req CreateRouteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			errorResponse(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
			return
		}

		if req.From == "" || req.To == "" {
			errorResponse(w, http.StatusBadRequest, "from and to are required")
			return
		}

		route := &types.RoutePreference{
			From:      req.From,
			To:        req.To,
			Via:       req.Via,
			Threshold: req.Threshold,
		}

		if err := s.manager.AddRoute(route); err != nil {
			errorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}

		jsonResponse(w, http.StatusCreated, route)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// handleUpload 处理文件上传（支持单文件和文件夹）
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// 解析 multipart form (100MB max memory)
	if err := r.ParseMultipartForm(100 << 20); err != nil {
		errorResponse(w, http.StatusBadRequest, "Failed to parse form: "+err.Error())
		return
	}

	targetPath := r.FormValue("target_path")
	targetHost := r.FormValue("target_host")
	viaStr := r.FormValue("via")
	isDir := r.FormValue("is_dir") == "true"

	if targetPath == "" || targetHost == "" {
		errorResponse(w, http.StatusBadRequest, "target_path and target_host are required")
		return
	}

	// 创建上传任务
	taskID := fmt.Sprintf("upload-%d", time.Now().UnixNano())

	// 保存到临时目录
	tempDir, err := os.MkdirTemp("", "gmssh-upload-*")
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, "Failed to create temp dir: "+err.Error())
		return
	}

	var totalSize int64
	var displayName string

	if isDir {
		// 文件夹上传：处理多个文件
		files := r.MultipartForm.File["files"]
		if len(files) == 0 {
			errorResponse(w, http.StatusBadRequest, "No files in directory upload")
			return
		}

		log.Printf("[UPLOAD] Directory upload: %d files", len(files))
		displayName = files[0].Filename
		// 从第一个文件名提取文件夹名
		if idx := strings.Index(displayName, "/"); idx > 0 {
			displayName = displayName[:idx]
		}

		for _, header := range files {
			file, err := header.Open()
			if err != nil {
				log.Printf("[UPLOAD] Failed to open file %s: %v", header.Filename, err)
				continue
			}

			// 创建目录结构
			filePath := filepath.Join(tempDir, header.Filename)
			dir := filepath.Dir(filePath)
			if err := os.MkdirAll(dir, 0755); err != nil {
				log.Printf("[UPLOAD] Failed to create dir %s: %v", dir, err)
				file.Close()
				continue
			}

			f, err := os.Create(filePath)
			if err != nil {
				log.Printf("[UPLOAD] Failed to create file %s: %v", filePath, err)
				file.Close()
				continue
			}

			size, err := io.Copy(f, file)
			file.Close()
			f.Close()
			
			if err != nil {
				log.Printf("[UPLOAD] Failed to save file %s: %v", header.Filename, err)
				continue
			}
			totalSize += size
		}
	} else {
		// 单文件上传
		file, header, err := r.FormFile("file")
		if err != nil {
			errorResponse(w, http.StatusBadRequest, "Failed to get file: "+err.Error())
			return
		}
		defer file.Close()

		displayName = header.Filename
		tempFile := filepath.Join(tempDir, header.Filename)
		f, err := os.Create(tempFile)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, "Failed to create temp file: "+err.Error())
			return
		}

		size, err := io.Copy(f, file)
		f.Close()
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, "Failed to save file: "+err.Error())
			return
		}
		totalSize = size
	}

	// 创建传输进度记录
	progress := &types.TransferProgress{
		TaskID:     taskID,
		FileName:   displayName,
		TotalBytes: totalSize,
		SentBytes:  0,
		Status:     "pending",
		Timestamp:  time.Now(),
	}

	s.mu.Lock()
	s.uploads[taskID] = progress
	s.mu.Unlock()

	// 解析 via 链
	var via []string
	if viaStr != "" {
		via = strings.Split(viaStr, ",")
	}

	// 异步执行上传
	go func() {
		s.executeUpload(taskID, tempDir, targetHost, targetPath, via, isDir)
	}()

	jsonResponse(w, http.StatusOK, map[string]string{"task_id": taskID})
}

// executeUpload 执行实际上传
func (s *Server) executeUpload(taskID, localPath, targetHost, targetPath string, via []string, isDir bool) {
	log.Printf("[UPLOAD] Starting upload: taskID=%s, localPath=%s, targetHost=%s, targetPath=%s, via=%v, isDir=%v", 
		taskID, localPath, targetHost, targetPath, via, isDir)
	
	s.mu.Lock()
	progress := s.uploads[taskID]
	progress.Status = "running"
	s.mu.Unlock()

	// 查找目标服务器配置（优先通过 ID，然后是 name 或 host）
	var targetHop *types.Hop
	configuredHop := s.config.GetHopByID(targetHost)
	if configuredHop == nil {
		configuredHop = s.config.GetHopByName(targetHost)
	}
	if configuredHop == nil {
		// 尝试通过主机地址匹配
		for _, h := range s.config.Hops {
			if h.Host == targetHost {
				configuredHop = h
				break
			}
		}
	}

	if configuredHop != nil {
		log.Printf("[UPLOAD] Using configured hop for target: %s (id: %s, host: %s, type: %v, gateway_id: %s)",
			configuredHop.Name, configuredHop.ID, configuredHop.Host, configuredHop.ServerType, configuredHop.GatewayID)
		targetHop = configuredHop
	} else {
		log.Printf("[UPLOAD] Using default root@ target (no config found for %s)", targetHost)
		targetHop = &types.Hop{
			Name:       targetHost,
			Host:       targetHost,
			Port:       22,
			User:       "root",
			ServerType: types.ServerExternal, // 默认为外网
		}
	}

	// 构建 hop 链
	var hops []*types.Hop

	// 添加中转节点（递归展开每个节点的网关链）
	if len(via) > 0 {
		hops = s.buildHopChainWithGateways(via)
	}

	// 如果目标是内网服务器，确保其网关链被添加（避免重复）
	if targetHop.ServerType == types.ServerInternal {
		if targetHop.GatewayID == "" {
			log.Printf("[UPLOAD] ERROR: Internal server %s has no gateway configured", targetHost)
			s.mu.Lock()
			progress.Status = "failed"
			progress.Error = fmt.Sprintf("内网服务器 %s 未配置网关", targetHost)
			s.mu.Unlock()
			os.RemoveAll(filepath.Dir(localPath))
			return
		}
		// 展开目标服务器的网关链并添加（避免重复）
		gatewayChain := s.buildHopChainWithGateways([]string{targetHop.GatewayID})
		existingHops := make(map[string]bool)
		for _, h := range hops {
			existingHops[h.ID] = true
		}
		for _, h := range gatewayChain {
			if !existingHops[h.ID] {
				hops = append(hops, h)
				existingHops[h.ID] = true
				log.Printf("[UPLOAD] Adding target gateway hop: %s (id: %s)", h.Name, h.ID)
			}
		}
	}

	// 添加目标主机
	hops = append(hops, targetHop)

	log.Printf("[UPLOAD] Total hops in chain: %d", len(hops))

	// 创建进度通道
	progressChan := make(chan *types.TransferProgress, 100)
	
	// 启动进度更新 goroutine
	go func() {
		for p := range progressChan {
			s.mu.Lock()
			if existing, ok := s.uploads[taskID]; ok {
				existing.SentBytes = p.SentBytes
				existing.Speed = p.Speed
				existing.ETA = p.ETA
				if p.Status != "" {
					existing.Status = p.Status
				}
			}
			s.mu.Unlock()
		}
	}()

	// 构建 SSH 链并连接
	log.Printf("[UPLOAD] Connecting SSH chain...")
	chain := ssh.NewChain(hops)
	if err := chain.Connect(); err != nil {
		log.Printf("[UPLOAD] ERROR: SSH connection failed: %v", err)
		s.mu.Lock()
		progress.Status = "failed"
		progress.Error = fmt.Sprintf("SSH connection failed: %v", err)
		s.mu.Unlock()
		close(progressChan)
		os.RemoveAll(filepath.Dir(localPath))
		return
	}
	log.Printf("[UPLOAD] SSH chain connected successfully")
	defer chain.Disconnect()

	// 创建 SCP 传输器
	transfer := transfer.NewSCPTransfer(chain)
	
	// 执行上传
	log.Printf("[UPLOAD] Starting file transfer: %s -> %s", localPath, targetPath)
	if err := transfer.Upload(localPath, targetPath, progressChan); err != nil {
		log.Printf("[UPLOAD] ERROR: Upload failed: %v", err)
		s.mu.Lock()
		progress.Status = "failed"
		progress.Error = fmt.Sprintf("Upload failed: %v", err)
		s.mu.Unlock()
		close(progressChan)
		os.RemoveAll(filepath.Dir(localPath))
		return
	}

	close(progressChan)

	log.Printf("[UPLOAD] Upload completed successfully: %s -> %s", localPath, targetPath)
	
	s.mu.Lock()
	progress.SentBytes = progress.TotalBytes
	progress.Status = "completed"
	s.mu.Unlock()

	// 清理临时文件
	os.RemoveAll(filepath.Dir(localPath))
}

// CreateProxyRequest 创建代理请求
type CreateProxyRequest struct {
	LocalAddr  string   `json:"local_addr"`
	RemoteHost string   `json:"remote_host"`
	RemotePort int      `json:"remote_port"`
	Via        []string `json:"via,omitempty"`
}

// ProxyInfo 代理信息响应
type ProxyInfo struct {
	ID                string `json:"id"`
	LocalAddr         string `json:"local_addr"`
	RemoteHost        string `json:"remote_host"`
	RemotePort        int    `json:"remote_port"`
	Active            bool   `json:"active"`
	ConnectionCount   int    `json:"connection_count"`
}

// handleProxies 处理代理列表
func (s *Server) handleProxies(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		proxies := s.proxies.List()
		jsonResponse(w, http.StatusOK, proxies)
	case http.MethodPost:
		var req CreateProxyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			errorResponse(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
			return
		}

		if req.RemoteHost == "" || req.RemotePort == 0 {
			errorResponse(w, http.StatusBadRequest, "remote_host and remote_port are required")
			return
		}

		// 构建 SSH 链（via 参数现在是 ID 列表）
		var hops []*types.Hop
		for _, hopID := range req.Via {
			hop := s.config.GetHopByID(hopID)
			if hop == nil {
				// 兼容：尝试通过 name 查找
				hop = s.config.GetHopByName(hopID)
			}
			if hop == nil {
				errorResponse(w, http.StatusBadRequest, fmt.Sprintf("Unknown hop: %s", hopID))
				return
			}
			hops = append(hops, hop)
		}

		// 添加目标主机
		targetHop := &types.Hop{
			Host: req.RemoteHost,
			Port: req.RemotePort,
		}
		hops = append(hops, targetHop)

		chain := ssh.NewChain(hops)
		if err := chain.Connect(); err != nil {
			errorResponse(w, http.StatusInternalServerError, fmt.Sprintf("Failed to connect: %v", err))
			return
		}

		// 创建端口转发器
		localAddr := req.LocalAddr
		if localAddr == "" || localAddr == ":0" {
			localAddr = ":0" // 自动分配端口
		}

		forwarder := proxy.NewPortForwarder(chain, localAddr, req.RemoteHost, req.RemotePort)
		if err := forwarder.Start(); err != nil {
			chain.Disconnect()
			errorResponse(w, http.StatusInternalServerError, fmt.Sprintf("Failed to start forwarder: %v", err))
			return
		}

		// 生成唯一ID并添加到管理器
		id := fmt.Sprintf("proxy-%d", time.Now().UnixNano())
		if err := s.proxies.Add(id, forwarder); err != nil {
			forwarder.Stop()
			chain.Disconnect()
			errorResponse(w, http.StatusInternalServerError, fmt.Sprintf("Failed to add proxy: %v", err))
			return
		}

		info := ProxyInfo{
			ID:         id,
			LocalAddr:  forwarder.GetLocalAddr(),
			RemoteHost: req.RemoteHost,
			RemotePort: req.RemotePort,
			Active:     true,
		}

		jsonResponse(w, http.StatusCreated, info)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// handleProxyDetail 处理单个代理
func (s *Server) handleProxyDetail(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/api/proxy/"):]

	switch r.Method {
	case http.MethodGet:
		fwd := s.proxies.Get(id)
		if fwd == nil {
			errorResponse(w, http.StatusNotFound, "Proxy not found")
			return
		}
		jsonResponse(w, http.StatusOK, fwd.GetInfo(id))
	case http.MethodDelete:
		if err := s.proxies.Remove(id); err != nil {
			errorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		jsonResponse(w, http.StatusNoContent, nil)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// LatencyProbeRequest 延迟探测请求
type LatencyProbeRequest struct {
	Target string   `json:"target"`
	Via    []string `json:"via,omitempty"`
}

// handleLatencyProbe 处理延迟探测
func (s *Server) handleLatencyProbe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req LatencyProbeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	if req.Target == "" {
		errorResponse(w, http.StatusBadRequest, "target is required")
		return
	}

	// 构建 hop 链（via 参数现在是 ID 列表）
	var hops []*types.Hop
	for _, hopID := range req.Via {
		hop := s.config.GetHopByID(hopID)
		if hop == nil {
			// 兼容：尝试通过 name 查找
			hop = s.config.GetHopByName(hopID)
		}
		if hop == nil {
			errorResponse(w, http.StatusBadRequest, fmt.Sprintf("Unknown hop: %s", hopID))
			return
		}
		hops = append(hops, hop)
	}

	// 添加目标主机（优先通过 ID 查找，然后是 name 或 host）
	targetHop := s.config.GetHopByID(req.Target)
	if targetHop == nil {
		targetHop = s.config.GetHopByName(req.Target)
	}
	if targetHop == nil {
		// 尝试通过主机地址匹配
		for _, h := range s.config.Hops {
			if h.Host == req.Target {
				targetHop = h
				break
			}
		}
	}
	if targetHop == nil {
		// 目标不在配置中，创建一个临时 hop
		targetHop = &types.Hop{
			Name: req.Target,
			Host: req.Target,
			Port: 22,
		}
	}
	hops = append(hops, targetHop)

	// 执行探测
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	report, err := s.profiler.Probe(ctx, hops)
	if err != nil {
		jsonResponse(w, http.StatusOK, map[string]interface{}{
			"latency_ms": 0,
			"success":    false,
			"error":      err.Error(),
			"path":       buildPath(hops),
		})
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"latency_ms": report.Latency.Milliseconds(),
		"success":    report.Success,
		"error":      report.Error,
		"path":       buildPath(hops),
	})
}

// buildPath 构建路径信息（返回 ID 列表，前端通过 ID 查找名称）
func buildPath(hops []*types.Hop) []map[string]string {
	path := make([]map[string]string, len(hops))
	for i, hop := range hops {
		path[i] = map[string]string{
			"id":   hop.ID,
			"name": hop.Name,
			"host": hop.Host,
		}
	}
	return path
}

// handleTestConnection 处理连接测试
func (s *Server) handleTestConnection(w http.ResponseWriter, r *http.Request, hop *types.Hop) {
	// 构建 hop 链
	hops := []*types.Hop{hop}

	// 执行探测
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	start := time.Now()
	report, err := s.profiler.Probe(ctx, hops)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		jsonResponse(w, http.StatusOK, TestConnectionResponse{
			Success:   false,
			LatencyMs: latency,
			Error:     err.Error(),
		})
		return
	}

	jsonResponse(w, http.StatusOK, TestConnectionResponse{
		Success:   report.Success,
		LatencyMs: report.Latency.Milliseconds(),
		Error:     report.Error,
	})
}

// handleProgressWebSocket 处理进度查询 (改为 HTTP 轮询)
func (s *Server) handleProgressWebSocket(w http.ResponseWriter, r *http.Request) {
	// 提取 task ID
	path := r.URL.Path[len("/api/ws/progress/"):]
	taskID := strings.TrimSpace(path)
	if taskID == "" {
		errorResponse(w, http.StatusBadRequest, "task_id is required")
		return
	}

	s.mu.RLock()
	progress, exists := s.uploads[taskID]
	s.mu.RUnlock()

	if !exists {
		errorResponse(w, http.StatusNotFound, "Task not found")
		return
	}

	jsonResponse(w, http.StatusOK, progress)
}

// BrowseResponse 目录浏览响应
type BrowseResponse struct {
	Path    string       `json:"path"`
	Entries []DirEntry   `json:"entries"`
	Success bool         `json:"success"`
	Error   string       `json:"error,omitempty"`
}

// DirEntry 目录项
type DirEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size,omitempty"`
}

// CommonPaths 常用路径
type CommonPaths struct {
	Paths []string `json:"paths"`
}

// handleBrowse 处理目录浏览请求
func (s *Server) handleBrowse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// 提取路径: /api/browse/server_id/path
	path := r.URL.Path[len("/api/browse/"):]
	parts := strings.SplitN(path, "/", 2)
	if len(parts) == 0 || parts[0] == "" {
		errorResponse(w, http.StatusBadRequest, "server id is required")
		return
	}

	serverID := parts[0]
	browsePath := "/"
	if len(parts) > 1 {
		browsePath = "/" + parts[1]
	}

	// 特殊路径: 获取常用路径列表
	if browsePath == "/__common_paths__" {
		s.handleCommonPaths(w, r)
		return
	}

	// 查找服务器配置（优先通过 ID，然后是 name 或 host）
	server := s.config.GetHopByID(serverID)
	if server == nil {
		server = s.config.GetHopByName(serverID)
	}
	if server == nil {
		// 尝试通过主机地址匹配
		for _, h := range s.config.Hops {
			if h.Host == serverID {
				server = h
				break
			}
		}
	}

	if server == nil {
		errorResponse(w, http.StatusNotFound, "Server not found")
		return
	}

	// 构建 hop 链
	var hops []*types.Hop

	// 如果目标是内网服务器，添加网关
	if server.ServerType == types.ServerInternal {
		if server.GatewayID == "" {
			errorResponse(w, http.StatusBadRequest, "Internal server has no gateway configured")
			return
		}
		gatewayHop := s.config.GetHopByID(server.GatewayID)
		if gatewayHop == nil {
			errorResponse(w, http.StatusBadRequest, "Gateway not found")
			return
		}
		hops = append(hops, gatewayHop)
	}
	hops = append(hops, server)

	// 连接 SSH
	chain := ssh.NewChain(hops)
	if err := chain.Connect(); err != nil {
		jsonResponse(w, http.StatusOK, BrowseResponse{
			Path:    browsePath,
			Success: false,
			Error:   fmt.Sprintf("SSH connection failed: %v", err),
		})
		return
	}
	defer chain.Disconnect()

	// 执行 ls 命令获取目录内容
	cmd := fmt.Sprintf("ls -la %s 2>/dev/null || ls -l %s 2>/dev/null || echo 'ERROR'", 
		shellEscape(browsePath), shellEscape(browsePath))
	
	stdout, stderr, err := chain.Execute(cmd)
	if err != nil || strings.TrimSpace(stdout) == "ERROR" {
		errMsg := stderr
		if err != nil {
			errMsg = fmt.Sprintf("%s: %v", stderr, err)
		}
		jsonResponse(w, http.StatusOK, BrowseResponse{
			Path:    browsePath,
			Success: false,
			Error:   fmt.Sprintf("Failed to list directory: %s", errMsg),
		})
		return
	}

	entries := parseLsOutput(browsePath, stdout)

	jsonResponse(w, http.StatusOK, BrowseResponse{
		Path:    browsePath,
		Entries: entries,
		Success: true,
	})
}

// handleCommonPaths 返回常用路径列表
func (s *Server) handleCommonPaths(w http.ResponseWriter, r *http.Request) {
	commonPaths := []string{
		"/tmp/",
		"/root/",
		"/home/",
		"/var/www/",
		"/opt/",
		"/data/",
		"/usr/local/",
		"/var/log/",
		"/etc/",
	}
	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"paths": commonPaths,
	})
}

// shellEscape 转义 shell 特殊字符
func shellEscape(s string) string {
	// 简单的转义，处理常见的特殊字符
	if strings.ContainsAny(s, "'\";|&$`<>!*?[]{}\\") {
		return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
	}
	return s
}

// parseLsOutput 解析 ls 输出
func parseLsOutput(basePath, output string) []DirEntry {
	var entries []DirEntry
	lines := strings.Split(output, "\n")
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "total ") {
			continue
		}
		
		// 解析 ls -la 输出格式: drwxr-xr-x 2 user group 4096 Jan 1 12:00 name
		parts := strings.Fields(line)
		if len(parts) < 9 {
			continue
		}
		
		perms := parts[0]
		name := parts[len(parts)-1]
		
		// 跳过 . 和 ..
		if name == "." || name == ".." {
			continue
		}
		
		isDir := strings.HasPrefix(perms, "d")
		
		// 构建完整路径
		fullPath := filepath.Join(basePath, name)
		if isDir && !strings.HasSuffix(fullPath, "/") {
			fullPath += "/"
		}
		
		entries = append(entries, DirEntry{
			Name:  name,
			Path:  fullPath,
			IsDir: isDir,
		})
	}
	
	// 按目录在前、文件在后排序
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return entries[i].Name < entries[j].Name
	})
	
	return entries
}
