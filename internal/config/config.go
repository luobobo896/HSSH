package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/luobobo896/HSSH/pkg/types"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

const (
	ConfigDirName  = ".gmssh"
	ConfigFileName = "config.yaml"
)

// Manager 配置管理器
type Manager struct {
	config     *types.Config
	configPath string
}

// NewManager 创建配置管理器
func NewManager() (*Manager, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return nil, err
	}

	return &Manager{
		configPath: filepath.Join(configDir, ConfigFileName),
	}, nil
}

// GetConfigDir 获取配置目录
func GetConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ConfigDirName)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create config directory: %w", err)
	}

	return configDir, nil
}

// Load 加载配置
func (m *Manager) Load() (*types.Config, error) {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// 配置文件不存在，创建默认配置
			m.config = m.defaultConfig()
			if err := m.Save(); err != nil {
				return nil, err
			}
			return m.config, nil
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var config types.Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	configDir, _ := GetConfigDir()
	config.ConfigDir = configDir

	// 执行配置迁移
	if NeedsMigration(&config) {
		log.Printf("[Config] Configuration migration needed, current version: %d", config.Version)
		if err := MigrateConfig(&config); err != nil {
			return nil, fmt.Errorf("failed to migrate config: %w", err)
		}
		// 迁移后保存
		m.config = &config
		if err := m.Save(); err != nil {
			log.Printf("[Config] Warning: failed to save migrated config: %v", err)
		}
		log.Printf("[Config] Configuration migrated and saved, new version: %d", config.Version)
	}

	m.config = &config
	return &config, nil
}

// Save 保存配置
func (m *Manager) Save() error {
	data, err := yaml.Marshal(m.config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(m.configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// Get 获取当前配置
func (m *Manager) Get() *types.Config {
	if m.config == nil {
		m.config = m.defaultConfig()
	}
	return m.config
}

// AddHop 添加服务器节点
func (m *Manager) AddHop(hop *types.Hop) error {
	// 生成 ID（如果没有）
	if hop.ID == "" {
		hop.ID = uuid.New().String()
	}

	// 检查 ID 是否已存在（理论上不会发生）
	if existing := m.config.GetHopByID(hop.ID); existing != nil {
		return fmt.Errorf("hop with id '%s' already exists", hop.ID)
	}

	// 检查名称是否已存在（只做提示性警告，允许重复名称）
	if existing := m.config.GetHopByName(hop.Name); existing != nil {
		log.Printf("[Config] Warning: hop with name '%s' already exists", hop.Name)
	}

	m.config.Hops = append(m.config.Hops, hop)
	return m.Save()
}

// UpdateHop 更新服务器节点（通过 ID）
func (m *Manager) UpdateHop(id string, hop *types.Hop) error {
	for i, h := range m.config.Hops {
		if h.ID == id {
			// 保留原 ID
			hop.ID = id
			m.config.Hops[i] = hop
			return m.Save()
		}
	}
	return fmt.Errorf("hop with id '%s' not found", id)
}

// UpdateHopByName 更新服务器节点（通过名称，兼容旧代码）
func (m *Manager) UpdateHopByName(name string, hop *types.Hop) error {
	for i, h := range m.config.Hops {
		if h.Name == name {
			// 保留原 ID
			if hop.ID == "" {
				hop.ID = h.ID
			}
			m.config.Hops[i] = hop
			return m.Save()
		}
	}
	return fmt.Errorf("hop with name '%s' not found", name)
}

// DeleteHop 删除服务器节点（通过 ID）
func (m *Manager) DeleteHop(id string) error {
	// 检查是否有其他服务器引用此服务器作为网关
	for _, h := range m.config.Hops {
		if h.GatewayID == id {
			return fmt.Errorf("cannot delete: server is used as gateway by '%s' (id: %s)", h.Name, h.ID)
		}
	}

	for i, h := range m.config.Hops {
		if h.ID == id {
			m.config.Hops = append(m.config.Hops[:i], m.config.Hops[i+1:]...)
			return m.Save()
		}
	}
	return fmt.Errorf("hop with id '%s' not found", id)
}

// DeleteHopByName 删除服务器节点（通过名称，兼容旧代码）
func (m *Manager) DeleteHopByName(name string) error {
	for i, h := range m.config.Hops {
		if h.Name == name {
			m.config.Hops = append(m.config.Hops[:i], m.config.Hops[i+1:]...)
			return m.Save()
		}
	}
	return fmt.Errorf("hop with name '%s' not found", name)
}

// AddRoute 添加路由偏好
func (m *Manager) AddRoute(route *types.RoutePreference) error {
	m.config.Routes = append(m.config.Routes, route)
	return m.Save()
}

// DeleteRoute 删除路由偏好
func (m *Manager) DeleteRoute(from, to string) error {
	for i, r := range m.config.Routes {
		if r.From == from && r.To == to {
			m.config.Routes = append(m.config.Routes[:i], m.config.Routes[i+1:]...)
			return m.Save()
		}
	}
	return fmt.Errorf("route from '%s' to '%s' not found", from, to)
}

// AddProfile 添加预设配置
func (m *Manager) AddProfile(profile *types.Profile) error {
	// 生成 ID（如果没有）
	if profile.ID == "" {
		profile.ID = uuid.New().String()
	}

	if existing := m.config.GetProfileByID(profile.ID); existing != nil {
		return fmt.Errorf("profile with id '%s' already exists", profile.ID)
	}

	m.config.Profiles = append(m.config.Profiles, profile)
	return m.Save()
}

// DeleteProfile 删除预设配置
func (m *Manager) DeleteProfile(name string) error {
	for i, p := range m.config.Profiles {
		if p.Name == name {
			m.config.Profiles = append(m.config.Profiles[:i], m.config.Profiles[i+1:]...)
			return m.Save()
		}
	}
	return fmt.Errorf("profile with name '%s' not found", name)
}

// defaultConfig 默认配置
func (m *Manager) defaultConfig() *types.Config {
	configDir, _ := GetConfigDir()
	return &types.Config{
		Version:   types.ConfigVersion2, // 新配置默认为最新版本
		Hops:      []*types.Hop{},
		Routes:    []*types.RoutePreference{},
		Profiles:  []*types.Profile{},
		ConfigDir: configDir,
	}
}

// Validate 验证配置有效性
func (m *Manager) Validate() error {
	if m.config == nil {
		return fmt.Errorf("config not loaded")
	}

	// 验证所有 route 引用的 hop 存在（使用 ID）
	for _, route := range m.config.Routes {
		if route.ViaID != "" {
			if m.config.GetHopByID(route.ViaID) == nil {
				return fmt.Errorf("route references unknown hop id: %s", route.ViaID)
			}
		}
	}

	// 验证所有 profile 引用的 hop 存在（使用 ID）
	for _, profile := range m.config.Profiles {
		for _, hopID := range profile.PathIDs {
			if m.config.GetHopByID(hopID) == nil {
				return fmt.Errorf("profile '%s' references unknown hop id: %s", profile.Name, hopID)
			}
		}
	}

	// 验证所有 gateway 引用存在
	for _, hop := range m.config.Hops {
		if hop.GatewayID != "" {
			if m.config.GetHopByID(hop.GatewayID) == nil {
				return fmt.Errorf("hop '%s' references unknown gateway id: %s", hop.Name, hop.GatewayID)
			}
		}
	}

	return nil
}
