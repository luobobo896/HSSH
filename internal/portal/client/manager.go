package client

import (
	"log"
	"sync"

	"github.com/gmssh/gmssh/pkg/portal"
)

// Manager manages multiple portal clients (one per server)
type Manager struct {
	clients map[string]*Client // server_addr -> client
	mu      sync.RWMutex
}

// NewManager creates a new client manager
func NewManager() *Manager {
	return &Manager{
		clients: make(map[string]*Client),
	}
}

// AddClient adds a client for a server
func (m *Manager) AddClient(serverAddr string, client *Client) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clients[serverAddr] = client
}

// GetClient gets a client for a server
func (m *Manager) GetClient(serverAddr string) (*Client, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	client, ok := m.clients[serverAddr]
	return client, ok
}

// RemoveClient removes a client
func (m *Manager) RemoveClient(serverAddr string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.clients, serverAddr)
}

// StopAll stops all clients
func (m *Manager) StopAll() {
	m.mu.Lock()
	clients := make([]*Client, 0, len(m.clients))
	for _, c := range m.clients {
		clients = append(clients, c)
	}
	m.mu.Unlock()

	for _, client := range clients {
		if err := client.Close(); err != nil {
			log.Printf("[Manager] Error closing client: %v", err)
		}
	}
}

// GetAllStatus returns status of all mappings across all clients
func (m *Manager) GetAllStatus() map[string][]portal.MappingStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string][]portal.MappingStatus)
	for addr, client := range m.clients {
		result[addr] = client.GetMappingStatus()
	}
	return result
}
