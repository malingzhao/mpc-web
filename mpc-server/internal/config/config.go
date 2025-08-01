package config

import (
	"fmt"
)

// ServerConfig 服务器配置
type ServerConfig struct {
	ID            string   `json:"id"`
	Port          int      `json:"port"`
	Name          string   `json:"name"`
	Capabilities  []string `json:"capabilities"`
	Peers         []Peer   `json:"peers"`
	EnableHTTPAPI bool     `json:"enable_http_api"`
	// 连接模式配置
	ConnectionMode string `json:"connection_mode"` // "server" 或 "client"
	ServerURL      string `json:"server_url"`      // 当为client模式时，连接的服务器URL
	AutoDisconnect bool   `json:"auto_disconnect"` // 完成操作后是否自动断开连接
}

// Peer 对等节点配置
type Peer struct {
	ID   string `json:"id"`
	URL  string `json:"url"`
	Name string `json:"name"`
}

// GetServerConfigs 获取所有服务器配置
func GetServerConfigs() map[string]*ServerConfig {
	return map[string]*ServerConfig{
		"third-party": {
			ID:             "third-party",
			Port:           8081,
			Name:           "第三方服务器",
			Capabilities:   []string{"keygen", "reshare"},
			EnableHTTPAPI:  true,
			ConnectionMode: "server",
			Peers: []Peer{
				{ID: "enterprise", URL: "ws://localhost:8082/ws", Name: "企业服务器"},
				{ID: "mobile-app", URL: "ws://localhost:8083/ws", Name: "企业服务器"},
			},
			AutoDisconnect: false,
		},
		"enterprise": {
			ID:             "enterprise",
			Port:           8082,
			Name:           "企业服务器",
			Capabilities:   []string{"keygen", "reshare", "sign"},
			EnableHTTPAPI:  true,
			ConnectionMode: "server",
			Peers: []Peer{
				{ID: "third-party", URL: "ws://localhost:8081/ws", Name: "第三方服务器"},
				{ID: "mobile-app", URL: "ws://localhost:8083/ws", Name: "企业服务器"},
			},
			AutoDisconnect: false,
		},
		"mobile-app": {
			ID:             "mobile-app",
			Port:           8083,
			Name:           "移动应用服务器",
			Capabilities:   []string{"keygen", "reshare", "sign"},
			EnableHTTPAPI:  true,
			ConnectionMode: "server",
			ServerURL:      "ws://localhost:8083/ws",
			Peers: []Peer{
				{ID: "third-party", URL: "ws://localhost:8081/ws", Name: "第三方服务器"},
				{ID: "enterprise", URL: "ws://localhost:8082/ws", Name: "企业服务器"},
			},
			AutoDisconnect: false,
		},
	}
}

// GetServerConfig 获取指定服务器配置
func GetServerConfig(serverID string) (*ServerConfig, error) {
	configs := GetServerConfigs()
	config, exists := configs[serverID]
	if !exists {
		return nil, fmt.Errorf("server config not found for ID: %s", serverID)
	}
	return config, nil
}

// HasCapability 检查服务器是否支持指定能力
func (c *ServerConfig) HasCapability(capability string) bool {
	for _, cap := range c.Capabilities {
		if cap == capability {
			return true
		}
	}
	return false
}
