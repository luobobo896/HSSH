package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// Config 配置文件结构
type Config struct {
	// SSH 配置
	SSH struct {
		Username   string `json:"username"`
		PrivateKey string `json:"private_key"`
		// 香港中转服务器（跳板）
		JumpHost string `json:"jump_host"`
		JumpPort int    `json:"jump_port"`
		// 最终目标网关
		GatewayHost string `json:"gateway_host"`
		GatewayPort int    `json:"gateway_port"`
	} `json:"ssh"`

	// 上传配置
	Upload struct {
		ChunkSize   int    `json:"chunk_size"`   // 分片大小（字节）
		Workers     int    `json:"workers"`      // 并发数
		MaxRetries  int    `json:"max_retries"`  // 单分片最大重试次数
		RetryDelay  int    `json:"retry_delay"`  // 重试间隔（秒）
		BufferSize  int    `json:"buffer_size"`  // 读写缓冲区大小
	} `json:"upload"`

	// 服务端配置
	Server struct {
		GatewayURL string `json:"gateway_url"` // HTTP API 地址
	} `json:"server"`

	// 日志配置
	Log struct {
		Level      string `json:"level"`       // debug, info, warn, error
		Progress   bool   `json:"progress"`    // 显示进度条
	} `json:"log"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	c := &Config{}

	// SSH 默认值
	c.SSH.Username = os.Getenv("USER")
	c.SSH.PrivateKey = filepath.Join(os.Getenv("HOME"), ".ssh/id_rsa")
	c.SSH.JumpHost = ""
	c.SSH.JumpPort = 22
	c.SSH.GatewayHost = "localhost"
	c.SSH.GatewayPort = 22

	// 上传默认值
	c.Upload.ChunkSize = 512 * 1024  // 512KB
	c.Upload.Workers = runtime.NumCPU() * 2
	c.Upload.MaxRetries = 3
	c.Upload.RetryDelay = 1
	c.Upload.BufferSize = 32 * 1024  // 32KB

	// 服务端默认值
	c.Server.GatewayURL = "http://localhost:8080"

	// 日志默认值
	c.Log.Level = "info"
	c.Log.Progress = true

	return c
}

// LoadConfig 从文件加载配置
func LoadConfig(path string) (*Config, error) {
	config := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return config, nil
		}
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	if err := json.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 环境变量覆盖
	config.overrideFromEnv()

	return config, nil
}

// overrideFromEnv 从环境变量覆盖配置
func (c *Config) overrideFromEnv() {
	if v := os.Getenv("SSH_USER"); v != "" {
		c.SSH.Username = v
	}
	if v := os.Getenv("SSH_KEY"); v != "" {
		c.SSH.PrivateKey = v
	}
	if v := os.Getenv("HK_HOST"); v != "" {
		c.SSH.JumpHost = v
	}
	if v := os.Getenv("GW_HOST"); v != "" {
		c.SSH.GatewayHost = v
	}
	if v := os.Getenv("GATEWAY_URL"); v != "" {
		c.Server.GatewayURL = v
	}
}

// Validate 验证配置
func (c *Config) Validate() error {
	if c.SSH.Username == "" {
		return fmt.Errorf("SSH 用户名不能为空")
	}
	if c.SSH.GatewayHost == "" {
		return fmt.Errorf("网关地址不能为空")
	}
	if c.Upload.ChunkSize < 64*1024 {
		return fmt.Errorf("分片大小不能小于 64KB")
	}
	if c.Upload.Workers < 1 {
		return fmt.Errorf("并发数不能小于 1")
	}
	return nil
}

// SaveConfig 保存配置到文件
func (c *Config) SaveConfig(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// GetConfigPath 获取配置文件路径
func GetConfigPath() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("APPDATA"), "uploader", "config.json")
	}
	return filepath.Join(os.Getenv("HOME"), ".config", "uploader", "config.json")
}

// ExampleConfig 生成示例配置
func ExampleConfig() string {
	return `{
  "ssh": {
    "username": "fileuser",
    "private_key": "~/.ssh/id_rsa",
    "jump_host": "hk-relay.example.com",
    "jump_port": 22,
    "gateway_host": "gateway.corp.internal",
    "gateway_port": 22
  },
  "upload": {
    "chunk_size": 524288,
    "workers": 8,
    "max_retries": 3,
    "retry_delay": 1,
    "buffer_size": 32768
  },
  "server": {
    "gateway_url": "http://localhost:8080"
  },
  "log": {
    "level": "info",
    "progress": true
  }
}`
}
