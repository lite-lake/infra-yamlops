package usecase

import (
	"sync"
	"testing"

	"github.com/litelake/yamlops/internal/application/handler"
	"github.com/litelake/yamlops/internal/domain/interfaces"
)

type mockSSHClient struct {
	closed bool
}

func (m *mockSSHClient) Run(cmd string) (stdout, stderr string, err error) {
	return "", "", nil
}

func (m *mockSSHClient) RunWithStdin(stdin string, cmd string) (stdout, stderr string, err error) {
	return "", "", nil
}

func (m *mockSSHClient) MkdirAllSudoWithPerm(path, perm string) error {
	return nil
}

func (m *mockSSHClient) UploadFileSudo(localPath, remotePath string) error {
	return nil
}

func (m *mockSSHClient) UploadFileSudoWithPerm(localPath, remotePath, perm string) error {
	return nil
}

func (m *mockSSHClient) Close() error {
	m.closed = true
	return nil
}

func TestSSHPool_Get(t *testing.T) {
	pool := NewSSHPool()

	t.Run("basic get", func(t *testing.T) {
		if pool.Size() != 0 {
			t.Errorf("expected empty pool, got %d", pool.Size())
		}
	})
}

func TestSSHPool_CloseAll(t *testing.T) {
	pool := NewSSHPool()
	pool.CloseAll()

	if pool.Size() != 0 {
		t.Errorf("expected empty pool after CloseAll, got %d", pool.Size())
	}
}

func TestSSHPool_Concurrency(t *testing.T) {
	pool := NewSSHPool()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = pool.Size()
		}()
	}

	wg.Wait()
}

func TestSSHPool_GetWithFactory(t *testing.T) {
	pool := NewSSHPoolWithFactory(func(info *handler.ServerInfo) (interfaces.SSHClient, error) {
		return &mockSSHClient{}, nil
	})

	info := &handler.ServerInfo{Host: "test.example.com", Port: 22, User: "test", Password: "test"}

	client1, err := pool.Get(info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	client2, err := pool.Get(info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if client1 != client2 {
		t.Error("expected same client for same host")
	}

	if pool.Size() != 1 {
		t.Errorf("expected pool size 1, got %d", pool.Size())
	}
}
