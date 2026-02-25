package usecase

import (
	"fmt"
	"sync"

	"github.com/litelake/yamlops/internal/application/handler"
	"github.com/litelake/yamlops/internal/domain/interfaces"
	"github.com/litelake/yamlops/internal/infrastructure/ssh"
)

type SSHClientFactory func(info *handler.ServerInfo) (interfaces.SSHClient, error)

type SSHPool struct {
	clients map[string]interfaces.SSHClient
	mu      sync.RWMutex
	factory SSHClientFactory
}

func NewSSHPool() *SSHPool {
	return &SSHPool{
		clients: make(map[string]interfaces.SSHClient),
		factory: func(info *handler.ServerInfo) (interfaces.SSHClient, error) {
			return ssh.NewClient(info.Host, info.Port, info.User, info.Password)
		},
	}
}

func NewSSHPoolWithFactory(factory SSHClientFactory) *SSHPool {
	return &SSHPool{
		clients: make(map[string]interfaces.SSHClient),
		factory: factory,
	}
}

func (p *SSHPool) Get(info *handler.ServerInfo) (interfaces.SSHClient, error) {
	key := fmt.Sprintf("%s:%d:%s", info.Host, info.Port, info.User)

	p.mu.RLock()
	if client, ok := p.clients[key]; ok {
		p.mu.RUnlock()
		return client, nil
	}
	p.mu.RUnlock()

	p.mu.Lock()
	defer p.mu.Unlock()

	if client, ok := p.clients[key]; ok {
		return client, nil
	}

	client, err := p.factory(info)
	if err != nil {
		return nil, err
	}
	p.clients[key] = client
	return client, nil
}

func (p *SSHPool) CloseAll() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, client := range p.clients {
		client.Close()
	}
	p.clients = make(map[string]interfaces.SSHClient)
}

func (p *SSHPool) Size() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.clients)
}
