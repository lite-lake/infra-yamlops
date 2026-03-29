package handler

import (
	"fmt"
	"sync"
	"time"

	"github.com/lite-lake/infra-yamlops/internal/domain/contract"
	"github.com/lite-lake/infra-yamlops/internal/infrastructure/logger"
	"github.com/lite-lake/infra-yamlops/internal/infrastructure/ssh"
)

type SSHClientFactory func(info *ServerInfo) (contract.SSHClient, error)

type poolEntry struct {
	client     contract.SSHClient
	createdAt  time.Time
	lastUsedAt time.Time
}

const (
	defaultConnectionTTL = 30 * time.Minute
)

type SSHPool struct {
	clients map[string]*poolEntry
	mu      sync.RWMutex
	factory SSHClientFactory
	connTTL time.Duration
}

func NewSSHPool() *SSHPool {
	return NewSSHPoolWithTTL(defaultConnectionTTL)
}

func NewSSHPoolWithTTL(ttl time.Duration) *SSHPool {
	return &SSHPool{
		clients: make(map[string]*poolEntry),
		connTTL: ttl,
		factory: func(info *ServerInfo) (contract.SSHClient, error) {
			cfg := &ssh.SSHConfig{
				StrictHostKeyChecking: info.StrictHostKeyChecking,
			}
			return ssh.NewClientWithConfig(info.Host, info.Port, info.User, info.Password, cfg)
		},
	}
}

func NewSSHPoolWithFactory(factory SSHClientFactory) *SSHPool {
	return &SSHPool{
		clients: make(map[string]*poolEntry),
		connTTL: defaultConnectionTTL,
		factory: factory,
	}
}

func (p *SSHPool) Get(info *ServerInfo) (contract.SSHClient, error) {
	key := fmt.Sprintf("%s:%d:%s:%s:%t", info.Host, info.Port, info.User, info.Password, info.StrictHostKeyChecking)

	p.mu.RLock()
	if entry, ok := p.clients[key]; ok {
		valid := p.isEntryValid(entry)
		p.mu.RUnlock()
		if valid {
			entry.lastUsedAt = time.Now()
			return entry.client, nil
		}
		p.invalidateEntry(key)
	} else {
		p.mu.RUnlock()
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if entry, ok := p.clients[key]; ok {
		valid := p.isEntryValid(entry)
		if valid {
			entry.lastUsedAt = time.Now()
			return entry.client, nil
		}
		p.invalidateEntryLocked(key)
	}

	client, err := p.factory(info)
	if err != nil {
		return nil, err
	}
	entry := &poolEntry{
		client:     client,
		createdAt:  time.Now(),
		lastUsedAt: time.Now(),
	}
	p.clients[key] = entry
	return entry.client, nil
}

func (p *SSHPool) isEntryValid(entry *poolEntry) bool {
	if time.Since(entry.createdAt) > p.connTTL {
		return false
	}
	if checker, ok := entry.client.(contract.SSHHealthChecker); ok {
		return checker.Healthy()
	}
	return true
}

func (p *SSHPool) invalidateEntry(key string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.invalidateEntryLocked(key)
}

func (p *SSHPool) invalidateEntryLocked(key string) {
	if entry, ok := p.clients[key]; ok {
		logger.Debug("invalidating SSH pool entry", "key", key)
		entry.client.Close()
		delete(p.clients, key)
	}
}

func (p *SSHPool) CloseAll() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for key, entry := range p.clients {
		entry.client.Close()
		delete(p.clients, key)
	}
	p.clients = make(map[string]*poolEntry)
}

func (p *SSHPool) Size() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.clients)
}
