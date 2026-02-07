package portal

import "time"

// Protocol 支持的协议类型
type Protocol string

const (
	ProtocolTCP       Protocol = "tcp"
	ProtocolHTTP      Protocol = "http"
	ProtocolWebSocket Protocol = "websocket"
)

// PortMapping 端口映射配置
type PortMapping struct {
	ID         string   `json:"id" yaml:"id"`
	Name       string   `json:"name" yaml:"name"`
	LocalAddr  string   `json:"local_addr" yaml:"local_addr"`
	RemoteHost string   `json:"remote_host" yaml:"remote_host"`
	RemotePort int      `json:"remote_port" yaml:"remote_port"`
	Via        []string `json:"via" yaml:"via"`
	Protocol   Protocol `json:"protocol" yaml:"protocol"`
	Enabled    bool     `json:"enabled" yaml:"enabled"`
}

// PortalConfig portal 模块配置
type PortalConfig struct {
	Client ClientConfig `json:"client" yaml:"client"`
	Server ServerConfig `json:"server" yaml:"server"`
}

// ClientConfig 客户端配置
type ClientConfig struct {
	Mappings   []PortMapping    `json:"mappings" yaml:"mappings"`
	Connection ConnectionConfig `json:"connection" yaml:"connection"`
}

// ServerConfig 服务端配置
type ServerConfig struct {
	Enabled    bool          `json:"enabled" yaml:"enabled"`
	ListenAddr string        `json:"listen_addr" yaml:"listen_addr"`
	TLSCert    string        `json:"tls_cert" yaml:"tls_cert"`
	TLSKey     string        `json:"tls_key" yaml:"tls_key"`
	AuthTokens []TokenConfig `json:"auth_tokens" yaml:"auth_tokens"`
}

// TokenConfig Token 认证配置
type TokenConfig struct {
	Token          string   `json:"token" yaml:"token"`
	AllowedRemotes []string `json:"allowed_remotes" yaml:"allowed_remotes"`
	MaxMappings    int      `json:"max_mappings" yaml:"max_mappings"`
}

// ConnectionConfig 连接配置
type ConnectionConfig struct {
	RetryInterval     time.Duration `json:"retry_interval" yaml:"retry_interval"`
	MaxRetries        int           `json:"max_retries" yaml:"max_retries"`
	KeepaliveInterval time.Duration `json:"keepalive_interval" yaml:"keepalive_interval"`
}

// MappingStatus 运行时映射状态
type MappingStatus struct {
	PortMapping
	Active           bool      `json:"active"`
	ConnectionCount  int       `json:"connection_count"`
	BytesTransferred int64     `json:"bytes_transferred"`
	LastActive       time.Time `json:"last_active"`
	Error            string    `json:"error,omitempty"`
}

// DefaultConnectionConfig 返回默认连接配置
func DefaultConnectionConfig() ConnectionConfig {
	return ConnectionConfig{
		RetryInterval:     5 * time.Second,
		MaxRetries:        10,
		KeepaliveInterval: 30 * time.Second,
	}
}
