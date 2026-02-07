package types

import (
	"encoding/json"
	"fmt"
	"time"
)

// AuthMethod 认证方式
type AuthMethod int

const (
	AuthKey AuthMethod = iota
	AuthPassword
)

func (a AuthMethod) String() string {
	switch a {
	case AuthKey:
		return "key"
	case AuthPassword:
		return "password"
	default:
		return "unknown"
	}
}

// ServerType 服务器类型
type ServerType int

const (
	ServerExternal ServerType = iota // 外网服务器
	ServerInternal                   // 内网服务器
)

func (s ServerType) String() string {
	switch s {
	case ServerExternal:
		return "external"
	case ServerInternal:
		return "internal"
	default:
		return "unknown"
	}
}

// Hop SSH 单跳配置
type Hop struct {
	ID         string     `json:"id" yaml:"id"` // 唯一标识符 (UUID)
	Name       string     `json:"name" yaml:"name"`
	Host       string     `json:"host" yaml:"host"`
	Port       int        `json:"port" yaml:"port"`
	User       string     `json:"user" yaml:"user"`
	AuthType   AuthMethod `json:"auth_type" yaml:"auth"`
	KeyPath    string     `json:"key_path,omitempty" yaml:"key_path,omitempty"`
	Password   string     `json:"password,omitempty" yaml:"password,omitempty"`
	ServerType ServerType `json:"server_type" yaml:"server_type"`    // 服务器类型：0外网, 1内网
	GatewayID  string     `json:"gateway_id,omitempty" yaml:"gateway_id,omitempty"` // 内网服务器的网关ID
	// 兼容旧配置：用于数据迁移
	Gateway string `json:"gateway,omitempty" yaml:"gateway,omitempty"` // Deprecated: 使用 GatewayID
}

// Address 返回主机地址
func (h *Hop) Address() string {
	return fmt.Sprintf("%s:%d", h.Host, h.Port)
}

// Chain 链路定义
type Chain struct {
	Hops []*Hop `json:"hops"`
}

// Path 路径标识
type Path struct {
	From string   `json:"from"`
	To   string   `json:"to"`
	Via  []string `json:"via"` // 中间节点列表
}

// String 返回路径字符串表示
func (p Path) String() string {
	if len(p.Via) == 0 {
		return fmt.Sprintf("%s -> %s (direct)", p.From, p.To)
	}
	return fmt.Sprintf("%s -> %v -> %s", p.From, p.Via, p.To)
}

// Key 返回用于 map 的键
func (p Path) Key() string {
	if len(p.Via) == 0 {
		return fmt.Sprintf("%s->%s", p.From, p.To)
	}
	return fmt.Sprintf("%s->%v->%s", p.From, p.Via, p.To)
}

// LatencyReport 延迟报告
type LatencyReport struct {
	Path      Path          `json:"path"`
	Latency   time.Duration `json:"latency"`
	Timestamp time.Time     `json:"timestamp"`
	Success   bool          `json:"success"`
	Error     string        `json:"error,omitempty"`
}

// RoutePreference 路由偏好配置
type RoutePreference struct {
	FromID string `json:"from_id" yaml:"from_id"` // 起点服务器ID
	ToID   string `json:"to_id" yaml:"to_id"`     // 终点服务器ID
	ViaID  string `json:"via_id,omitempty" yaml:"via_id,omitempty"`
	// 显示用名称（运行时填充，不持久化）
	FromName string `json:"from_name,omitempty" yaml:"-"`
	ToName   string `json:"to_name,omitempty" yaml:"-"`
	ViaName  string `json:"via_name,omitempty" yaml:"-"`
	Threshold int   `json:"threshold_ms" yaml:"threshold"` // 延迟差异阈值(ms)
	// 兼容旧配置
	From string `json:"from,omitempty" yaml:"from,omitempty"` // Deprecated
	To   string `json:"to,omitempty" yaml:"to,omitempty"`     // Deprecated
	Via  string `json:"via,omitempty" yaml:"via,omitempty"`   // Deprecated
}

// Profile 预设配置
type Profile struct {
	ID      string   `json:"id" yaml:"id"` // 唯一标识符
	Name    string   `json:"name" yaml:"name"`
	PathIDs []string `json:"path_ids" yaml:"path_ids"` // 服务器ID列表
	// 显示用路径名称（运行时填充）
	PathNames  []string `json:"path_names,omitempty" yaml:"-"`
	TargetDir  string   `json:"target_dir,omitempty" yaml:"target_dir,omitempty"`
	LocalPort  int      `json:"local_port,omitempty" yaml:"local_port,omitempty"`
	RemoteHost string   `json:"remote_host,omitempty" yaml:"remote_host,omitempty"`
	RemotePort int      `json:"remote_port,omitempty" yaml:"remote_port,omitempty"`
	// 兼容旧配置
	Path []string `json:"path,omitempty" yaml:"path,omitempty"` // Deprecated: 使用 PathIDs
}

// Config 版本常量
const (
	ConfigVersion1 = 1 // 初始版本：使用 name 关联
	ConfigVersion2 = 2 // 当前版本：使用 id 关联
)

// Config 全局配置
type Config struct {
	Version   int                `json:"version" yaml:"version"` // 配置版本，用于迁移
	Hops      []*Hop             `json:"hops" yaml:"hops"`
	Routes    []*RoutePreference `json:"routes" yaml:"routes"`
	Profiles  []*Profile         `json:"profiles" yaml:"profiles"`
	Portal    PortalConfig       `json:"portal,omitempty" yaml:"portal,omitempty"`
	ConfigDir string             `json:"-" yaml:"-"`
}

// GetHopByID 根据ID获取 Hop
func (c *Config) GetHopByID(id string) *Hop {
	for _, h := range c.Hops {
		if h.ID == id {
			return h
		}
	}
	return nil
}

// GetHopByName 根据名称获取 Hop（兼容旧代码）
func (c *Config) GetHopByName(name string) *Hop {
	for _, h := range c.Hops {
		if h.Name == name {
			return h
		}
	}
	return nil
}

// GetProfileByID 根据ID获取 Profile
func (c *Config) GetProfileByID(id string) *Profile {
	for _, p := range c.Profiles {
		if p.ID == id {
			return p
		}
	}
	return nil
}

// GetProfileByName 根据名称获取 Profile
func (c *Config) GetProfileByName(name string) *Profile {
	for _, p := range c.Profiles {
		if p.Name == name {
			return p
		}
	}
	return nil
}

// GetRoutePreference 获取路由偏好
func (c *Config) GetRoutePreference(from, to string) *RoutePreference {
	for _, r := range c.Routes {
		if r.From == from && r.To == to {
			return r
		}
	}
	return nil
}

// UploadRequest 文件上传请求
type UploadRequest struct {
	SourcePath string   `json:"source_path"`
	TargetPath string   `json:"target_path"`
	TargetHost string   `json:"target_host"`
	Via        []string `json:"via,omitempty"`
}

// ProxyRequest 端口转发请求
type ProxyRequest struct {
	LocalAddr  string   `json:"local_addr"`
	RemoteHost string   `json:"remote_host"`
	RemotePort int      `json:"remote_port"`
	Via        []string `json:"via,omitempty"`
}

// TransferProgress 传输进度
type TransferProgress struct {
	TaskID       string        `json:"task_id"`
	FileName     string        `json:"file_name"`
	TotalBytes   int64         `json:"total_bytes"`
	SentBytes    int64         `json:"sent_bytes"`
	Speed        int64         `json:"speed_bytes_per_sec"`
	ETA          time.Duration `json:"eta_seconds"`
	Status       string        `json:"status"` // pending, running, completed, failed
	Error        string        `json:"error,omitempty"`
	Timestamp    time.Time     `json:"timestamp"`
}

// MarshalJSON 自定义 JSON 序列化，添加 percentage 字段
func (tp TransferProgress) MarshalJSON() ([]byte, error) {
	type Alias TransferProgress
	return json.Marshal(&struct {
		Alias
		Percentage float64 `json:"percentage"`
	}{
		Alias:      (Alias)(tp),
		Percentage: tp.Percentage(),
	})
}

// Percentage 返回完成百分比
func (tp *TransferProgress) Percentage() float64 {
	if tp.TotalBytes == 0 {
		return 0
	}
	return float64(tp.SentBytes) * 100 / float64(tp.TotalBytes)
}

// ========== Portal 相关类型 ==========

// PortalProtocol 支持的协议类型
type PortalProtocol string

const (
	PortalProtocolTCP       PortalProtocol = "tcp"
	PortalProtocolHTTP      PortalProtocol = "http"
	PortalProtocolWebSocket PortalProtocol = "websocket"
)

// PortMapping 端口映射配置
type PortMapping struct {
	ID         string         `json:"id" yaml:"id"`
	Name       string         `json:"name" yaml:"name"`
	LocalAddr  string         `json:"local_addr" yaml:"local_addr"`
	RemoteHost string         `json:"remote_host" yaml:"remote_host"`
	RemotePort int            `json:"remote_port" yaml:"remote_port"`
	Via        []string       `json:"via" yaml:"via"`
	Protocol   PortalProtocol `json:"protocol" yaml:"protocol"`
	Enabled    bool           `json:"enabled" yaml:"enabled"`
}

// PortalTokenConfig Token 认证配置
type PortalTokenConfig struct {
	Token          string   `json:"token" yaml:"token"`
	AllowedRemotes []string `json:"allowed_remotes" yaml:"allowed_remotes"`
	MaxMappings    int      `json:"max_mappings" yaml:"max_mappings"`
}

// PortalConnectionConfig 连接配置
type PortalConnectionConfig struct {
	RetryInterval     time.Duration `json:"retry_interval" yaml:"retry_interval"`
	MaxRetries        int           `json:"max_retries" yaml:"max_retries"`
	KeepaliveInterval time.Duration `json:"keepalive_interval" yaml:"keepalive_interval"`
}

// DefaultPortalConnectionConfig 返回默认连接配置
func DefaultPortalConnectionConfig() PortalConnectionConfig {
	return PortalConnectionConfig{
		RetryInterval:     5 * time.Second,
		MaxRetries:        10,
		KeepaliveInterval: 30 * time.Second,
	}
}

// PortalClientConfig 客户端配置
type PortalClientConfig struct {
	Mappings   []PortMapping          `json:"mappings" yaml:"mappings"`
	Connection PortalConnectionConfig `json:"connection" yaml:"connection"`
}

// PortalServerConfig 服务端配置
type PortalServerConfig struct {
	Enabled    bool                `json:"enabled" yaml:"enabled"`
	ListenAddr string              `json:"listen_addr" yaml:"listen_addr"`
	TLSCert    string              `json:"tls_cert" yaml:"tls_cert"`
	TLSKey     string              `json:"tls_key" yaml:"tls_key"`
	AuthTokens []PortalTokenConfig `json:"auth_tokens" yaml:"auth_tokens"`
}

// PortalConfig portal 模块配置
type PortalConfig struct {
	Client PortalClientConfig `json:"client" yaml:"client"`
	Server PortalServerConfig `json:"server" yaml:"server"`
}
