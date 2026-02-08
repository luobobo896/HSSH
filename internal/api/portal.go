package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/luobobo896/HSSH/internal/proxy"
	"github.com/luobobo896/HSSH/internal/ssh"
	"github.com/luobobo896/HSSH/pkg/types"
	"github.com/google/uuid"
)

// CreatePortalMappingRequest 创建端口映射请求
type CreatePortalMappingRequest struct {
	Name         string   `json:"name"`
	LocalAddr    string   `json:"local_addr"`
	RemoteHost   string   `json:"remote_host"`
	RemotePort   int      `json:"remote_port"`
	Via          []string `json:"via"`
	Protocol     string   `json:"protocol"`
	PortalServer string   `json:"portal_server,omitempty"`
}

// PortalMappingStatus 端口映射状态
type PortalMappingStatus struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	LocalAddr        string `json:"local_addr"`
	RemoteHost       string `json:"remote_host"`
	RemotePort       int    `json:"remote_port"`
	Protocol         string `json:"protocol"`
	Enabled          bool   `json:"enabled"`
	Active           bool   `json:"active"`
	ConnectionCount  int    `json:"connection_count"`
	BytesTransferred int64  `json:"bytes_transferred"`
}

// PortalStatusResponse Portal 状态响应
type PortalStatusResponse struct {
	Active     bool                  `json:"active"`
	Mappings   []PortalMappingStatus `json:"mappings"`
	ServerAddr string                `json:"server_addr,omitempty"`
}

// handlePortal 处理 /api/portal 请求
func (s *Server) handlePortal(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handlePortalStatus(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// handlePortalStatus 获取 Portal 状态
func (s *Server) handlePortalStatus(w http.ResponseWriter, r *http.Request) {
	// Build response from config
	response := PortalStatusResponse{
		Active:   len(s.config.Portal.Client.Mappings) > 0,
		Mappings: make([]PortalMappingStatus, 0, len(s.config.Portal.Client.Mappings)),
	}

	for _, m := range s.config.Portal.Client.Mappings {
		response.Mappings = append(response.Mappings, PortalMappingStatus{
			ID:         m.ID,
			Name:       m.Name,
			LocalAddr:  m.LocalAddr,
			RemoteHost: m.RemoteHost,
			RemotePort: m.RemotePort,
			Protocol:   string(m.Protocol),
			Enabled:    m.Enabled,
			Active:     m.Enabled, // TODO: Check actual runtime status
		})
	}

	jsonResponse(w, http.StatusOK, response)
}

// handlePortalMappings 处理 /api/portal/mappings 请求
func (s *Server) handlePortalMappings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleListPortalMappings(w, r)
	case http.MethodPost:
		s.handleCreatePortalMapping(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// handleListPortalMappings 列出所有端口映射
func (s *Server) handleListPortalMappings(w http.ResponseWriter, r *http.Request) {
	mappings := make([]PortalMappingStatus, 0, len(s.config.Portal.Client.Mappings))

	for _, m := range s.config.Portal.Client.Mappings {
		// 检查实际运行状态
		s.portalMu.RLock()
		forwarder, isActive := s.portalForwarders[m.ID]
		s.portalMu.RUnlock()

		status := PortalMappingStatus{
			ID:         m.ID,
			Name:       m.Name,
			LocalAddr:  m.LocalAddr,
			RemoteHost: m.RemoteHost,
			RemotePort: m.RemotePort,
			Protocol:   string(m.Protocol),
			Enabled:    m.Enabled,
			Active:     isActive,
		}

		if isActive {
			status.ConnectionCount = forwarder.GetConnectionCount()
		}

		mappings = append(mappings, status)
	}

	jsonResponse(w, http.StatusOK, mappings)
}

// handleCreatePortalMapping 创建新的端口映射
func (s *Server) handleCreatePortalMapping(w http.ResponseWriter, r *http.Request) {
	var req CreatePortalMappingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Validation
	if req.Name == "" {
		errorResponse(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.LocalAddr == "" {
		errorResponse(w, http.StatusBadRequest, "local_addr is required")
		return
	}
	if req.RemoteHost == "" || req.RemotePort == 0 {
		errorResponse(w, http.StatusBadRequest, "remote_host and remote_port are required")
		return
	}

	// Create mapping
	protocol := types.PortalProtocolTCP
	if req.Protocol != "" {
		protocol = types.PortalProtocol(req.Protocol)
	}

	mapping := types.PortMapping{
		ID:           uuid.New().String(),
		Name:         req.Name,
		LocalAddr:    req.LocalAddr,
		RemoteHost:   req.RemoteHost,
		RemotePort:   req.RemotePort,
		Via:          req.Via,
		Protocol:     protocol,
		Enabled:      true,
		PortalServer: req.PortalServer,
	}

	// Add to config
	s.config.Portal.Client.Mappings = append(s.config.Portal.Client.Mappings, mapping)

	// Save config
	if err := s.manager.Save(); err != nil {
		errorResponse(w, http.StatusInternalServerError, "Failed to save config: "+err.Error())
		return
	}

	// Return created mapping
	status := PortalMappingStatus{
		ID:         mapping.ID,
		Name:       mapping.Name,
		LocalAddr:  mapping.LocalAddr,
		RemoteHost: mapping.RemoteHost,
		RemotePort: mapping.RemotePort,
		Protocol:   string(mapping.Protocol),
		Enabled:    mapping.Enabled,
		Active:     false,
	}

	jsonResponse(w, http.StatusCreated, status)
}

// handlePortalMappingDetail 处理单个映射操作
func (s *Server) handlePortalMappingDetail(w http.ResponseWriter, r *http.Request) {
	// Extract ID from path: /api/portal/mappings/:id
	path := r.URL.Path[len("/api/portal/mappings/"):]
	parts := strings.SplitN(path, "/", 2)
	id := parts[0]
	subPath := ""
	if len(parts) > 1 {
		subPath = parts[1]
	}

	switch r.Method {
	case http.MethodGet:
		s.handleGetPortalMapping(w, r, id)
	case http.MethodPut:
		s.handleUpdatePortalMapping(w, r, id)
	case http.MethodDelete:
		s.handleDeletePortalMapping(w, r, id)
	case http.MethodPost:
		// Handle start/stop actions: /api/portal/mappings/:id/start or /stop
		switch subPath {
		case "start":
			s.handleStartPortalMapping(w, r, id)
		case "stop":
			s.handleStopPortalMapping(w, r, id)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// handleGetPortalMapping 获取单个映射
func (s *Server) handleGetPortalMapping(w http.ResponseWriter, r *http.Request, id string) {
	for _, m := range s.config.Portal.Client.Mappings {
		if m.ID == id {
			// 检查实际运行状态
			s.portalMu.RLock()
			forwarder, isActive := s.portalForwarders[m.ID]
			s.portalMu.RUnlock()

			status := PortalMappingStatus{
				ID:         m.ID,
				Name:       m.Name,
				LocalAddr:  m.LocalAddr,
				RemoteHost: m.RemoteHost,
				RemotePort: m.RemotePort,
				Protocol:   string(m.Protocol),
				Enabled:    m.Enabled,
				Active:     isActive,
			}

			if isActive {
				status.ConnectionCount = forwarder.GetConnectionCount()
			}

			jsonResponse(w, http.StatusOK, status)
			return
		}
	}
	errorResponse(w, http.StatusNotFound, "Mapping not found")
}

// handleUpdatePortalMapping 更新端口映射
func (s *Server) handleUpdatePortalMapping(w http.ResponseWriter, r *http.Request, id string) {
	var req CreatePortalMappingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	// Find mapping
	for i, m := range s.config.Portal.Client.Mappings {
		if m.ID == id {
			// Update fields if provided
			if req.Name != "" {
				s.config.Portal.Client.Mappings[i].Name = req.Name
			}
			if req.LocalAddr != "" {
				s.config.Portal.Client.Mappings[i].LocalAddr = req.LocalAddr
			}
			if req.RemoteHost != "" {
				s.config.Portal.Client.Mappings[i].RemoteHost = req.RemoteHost
			}
			if req.RemotePort != 0 {
				s.config.Portal.Client.Mappings[i].RemotePort = req.RemotePort
			}
			if req.Protocol != "" {
				s.config.Portal.Client.Mappings[i].Protocol = types.PortalProtocol(req.Protocol)
			}
			if req.Via != nil {
				s.config.Portal.Client.Mappings[i].Via = req.Via
			}
			if req.PortalServer != "" {
				s.config.Portal.Client.Mappings[i].PortalServer = req.PortalServer
			}

			// Save config
			if err := s.manager.Save(); err != nil {
				errorResponse(w, http.StatusInternalServerError, "Failed to save config: "+err.Error())
				return
			}

			// Return updated mapping
			status := PortalMappingStatus{
				ID:         s.config.Portal.Client.Mappings[i].ID,
				Name:       s.config.Portal.Client.Mappings[i].Name,
				LocalAddr:  s.config.Portal.Client.Mappings[i].LocalAddr,
				RemoteHost: s.config.Portal.Client.Mappings[i].RemoteHost,
				RemotePort: s.config.Portal.Client.Mappings[i].RemotePort,
				Protocol:   string(s.config.Portal.Client.Mappings[i].Protocol),
				Enabled:    s.config.Portal.Client.Mappings[i].Enabled,
				Active:     s.config.Portal.Client.Mappings[i].Enabled,
			}
			jsonResponse(w, http.StatusOK, status)
			return
		}
	}
	errorResponse(w, http.StatusNotFound, "Mapping not found")
}

// handleDeletePortalMapping 删除端口映射
func (s *Server) handleDeletePortalMapping(w http.ResponseWriter, r *http.Request, id string) {
	// 先停止运行中的转发
	s.portalMu.Lock()
	if forwarder, exists := s.portalForwarders[id]; exists {
		forwarder.Stop()
		delete(s.portalForwarders, id)
	}
	s.portalMu.Unlock()

	for i, m := range s.config.Portal.Client.Mappings {
		if m.ID == id {
			// Remove from slice
			s.config.Portal.Client.Mappings = append(
				s.config.Portal.Client.Mappings[:i],
				s.config.Portal.Client.Mappings[i+1:]...,
			)

			// Save config
			if err := s.manager.Save(); err != nil {
				errorResponse(w, http.StatusInternalServerError, "Failed to save config: "+err.Error())
				return
			}

			w.WriteHeader(http.StatusNoContent)
			return
		}
	}
	errorResponse(w, http.StatusNotFound, "Mapping not found")
}

// buildHopChainForMapping 构建映射的 SSH 链
func (s *Server) buildHopChainForMapping(mapping *types.PortMapping) ([]*types.Hop, error) {
	var hops []*types.Hop
	visited := make(map[string]bool)

	// 递归添加节点及其网关
	var addHopWithGateway func(hopID string)
	addHopWithGateway = func(hopID string) {
		if visited[hopID] {
			return
		}

		hop := s.config.GetHopByID(hopID)
		if hop == nil {
			hop = s.config.GetHopByName(hopID)
		}
		if hop == nil {
			log.Printf("[Portal] Warning: hop '%s' not found", hopID)
			return
		}

		// 先添加网关
		if hop.GatewayID != "" && hop.GatewayID != hop.ID {
			addHopWithGateway(hop.GatewayID)
		}

		// 再添加节点本身
		if !visited[hop.ID] {
			hops = append(hops, hop)
			visited[hop.ID] = true
		}
	}

	// 处理 Via 链
	for _, hopID := range mapping.Via {
		addHopWithGateway(hopID)
	}

	// 查找目标主机配置
	targetHop := s.config.GetHopByID(mapping.RemoteHost)
	if targetHop == nil {
		targetHop = s.config.GetHopByName(mapping.RemoteHost)
	}
	if targetHop == nil {
		// 尝试通过 host 匹配
		for _, h := range s.config.Hops {
			if h.Host == mapping.RemoteHost {
				targetHop = h
				break
			}
		}
	}

	if targetHop != nil {
		// 如果目标是内网服务器，添加其网关
		if targetHop.ServerType == types.ServerInternal && targetHop.GatewayID != "" {
			addHopWithGateway(targetHop.GatewayID)
		}
		// 目标服务器已在配置中，不需要再次添加（转发会通过 SSH 链 Dial 到目标）
	}

	return hops, nil
}

// handleStartPortalMapping 启动端口转发（使用 SSH 隧道）
func (s *Server) handleStartPortalMapping(w http.ResponseWriter, r *http.Request, id string) {
	// 1. 从 config 中找到对应 mapping
	var mapping *types.PortMapping
	for i := range s.config.Portal.Client.Mappings {
		if s.config.Portal.Client.Mappings[i].ID == id {
			mapping = &s.config.Portal.Client.Mappings[i]
			break
		}
	}

	if mapping == nil {
		errorResponse(w, http.StatusNotFound, "Mapping not found")
		return
	}

	// 检查是否已经在运行
	s.portalMu.RLock()
	_, exists := s.portalForwarders[id]
	s.portalMu.RUnlock()

	if exists {
		errorResponse(w, http.StatusConflict, "Mapping is already running")
		return
	}

	// 2. 构建 SSH 链
	hops, err := s.buildHopChainForMapping(mapping)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, "Failed to build hop chain: "+err.Error())
		return
	}

	// 如果没有配置 hops，创建一个默认的（使用 Via 的第一个）
	if len(hops) == 0 {
		// 尝试从 Via 获取第一个服务器
		if len(mapping.Via) > 0 {
			hop := s.config.GetHopByID(mapping.Via[0])
			if hop == nil {
				hop = s.config.GetHopByName(mapping.Via[0])
			}
			if hop != nil {
				hops = append(hops, hop)
			}
		}
	}

	if len(hops) == 0 {
		errorResponse(w, http.StatusBadRequest, "No valid SSH hops configured. Please configure Via hops.")
		return
	}

	log.Printf("[Portal] Starting mapping %s with %d hops", mapping.ID, len(hops))

	// 3. 建立 SSH 连接链
	chain := ssh.NewChain(hops)
	if err := chain.Connect(); err != nil {
		errorResponse(w, http.StatusInternalServerError, "Failed to connect SSH chain: "+err.Error())
		return
	}

	// 4. 创建端口转发器
	forwarder := proxy.NewPortForwarder(chain, mapping.LocalAddr, mapping.RemoteHost, mapping.RemotePort)
	if err := forwarder.Start(); err != nil {
		chain.Disconnect()
		errorResponse(w, http.StatusInternalServerError, "Failed to start port forwarder: "+err.Error())
		return
	}

	// 5. 保存转发器到运行时管理
	s.portalMu.Lock()
	s.portalForwarders[id] = forwarder
	s.portalMu.Unlock()

	// 更新 mapping 状态为启用
	mapping.Enabled = true
	s.manager.Save()

	log.Printf("[Portal] Mapping %s started successfully on %s", mapping.ID, forwarder.GetLocalAddr())

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success":     true,
		"message":     "Mapping started",
		"id":          id,
		"local_addr":  forwarder.GetLocalAddr(),
		"active":      true,
	})
}

// handleStopPortalMapping 停止端口转发
func (s *Server) handleStopPortalMapping(w http.ResponseWriter, r *http.Request, id string) {
	// 1. 找到运行中的 forwarder
	s.portalMu.Lock()
	forwarder, exists := s.portalForwarders[id]
	if exists {
		delete(s.portalForwarders, id)
	}
	s.portalMu.Unlock()

	// 2. 如果 forwarder 存在，停止它
	if exists {
		if err := forwarder.Stop(); err != nil {
			// 即使出错也记录日志，继续处理
			log.Printf("[Portal] Error stopping forwarder for mapping %s: %v", id, err)
		}
		log.Printf("[Portal] Mapping %s stopped", id)
	} else {
		log.Printf("[Portal] Mapping %s was not running (forwarder not found)", id)
	}

	// 3. 更新 mapping 状态为禁用（无论 forwarder 是否存在，都更新配置）
	for i := range s.config.Portal.Client.Mappings {
		if s.config.Portal.Client.Mappings[i].ID == id {
			s.config.Portal.Client.Mappings[i].Enabled = false
			if err := s.manager.Save(); err != nil {
				log.Printf("[Portal] Error saving config after stopping mapping %s: %v", id, err)
			}
			break
		}
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Mapping stopped",
		"id":      id,
	})
}
