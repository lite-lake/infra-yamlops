package handler

import (
	"context"
	"sync"
	"testing"

	"github.com/litelake/yamlops/internal/domain/valueobject"
)

func TestRegistry_New(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("expected non-nil registry")
	}
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()
	h := NewDNSHandler()

	r.Register(h)

	got, ok := r.Get("dns_record")
	if !ok {
		t.Error("expected to find handler")
	}
	if got.EntityType() != "dns_record" {
		t.Errorf("expected dns_record, got %s", got.EntityType())
	}
}

func TestRegistry_Get_NotFound(t *testing.T) {
	r := NewRegistry()

	_, ok := r.Get("nonexistent")
	if ok {
		t.Error("expected not to find handler")
	}
}

func TestRegistry_All(t *testing.T) {
	r := NewRegistry()
	r.Register(NewDNSHandler())
	r.Register(NewServiceHandler())

	all := r.All()
	if len(all) != 2 {
		t.Errorf("expected 2 handlers, got %d", len(all))
	}
	if _, ok := all["dns_record"]; !ok {
		t.Error("expected dns_record handler")
	}
	if _, ok := all["service"]; !ok {
		t.Error("expected service handler")
	}
}

func TestRegistry_All_Empty(t *testing.T) {
	r := NewRegistry()

	all := r.All()
	if all == nil {
		t.Error("expected non-nil map")
	}
	if len(all) != 0 {
		t.Errorf("expected 0 handlers, got %d", len(all))
	}
}

func TestRegistry_Overwrite(t *testing.T) {
	r := NewRegistry()

	h1 := NewDNSHandler()
	h2 := NewDNSHandler()

	r.Register(h1)
	r.Register(h2)

	got, _ := r.Get("dns_record")
	if got != h2 {
		t.Error("expected second handler to overwrite first")
	}
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
	r := NewRegistry()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(2)

		go func(id int) {
			defer wg.Done()
			h := NewDNSHandler()
			r.Register(h)
		}(i)

		go func(id int) {
			defer wg.Done()
			_, _ = r.Get("dns_record")
		}(i)
	}

	wg.Wait()
}

func TestRegistry_ConcurrentAll(t *testing.T) {
	r := NewRegistry()
	r.Register(NewDNSHandler())
	r.Register(NewServiceHandler())

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			all := r.All()
			if len(all) != 2 {
				t.Errorf("expected 2 handlers, got %d", len(all))
			}
		}()
	}
	wg.Wait()
}

func TestRegistry_All_ReturnsCopy(t *testing.T) {
	r := NewRegistry()
	r.Register(NewDNSHandler())

	all := r.All()
	all["new_handler"] = NewServiceHandler()

	_, ok := r.Get("new_handler")
	if ok {
		t.Error("All() should return a copy, not the internal map")
	}
}

type testHandler struct {
	entityType string
}

func (h *testHandler) EntityType() string {
	return h.entityType
}

func (h *testHandler) Apply(ctx context.Context, change *valueobject.Change, deps DepsProvider) (*Result, error) {
	return nil, nil
}
