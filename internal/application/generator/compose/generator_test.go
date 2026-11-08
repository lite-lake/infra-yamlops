package compose

import (
	"strings"
	"testing"
)

func TestGenerateMulti(t *testing.T) {
	gen := NewGenerator()

	main := &ComposeService{
		Name:       "uptime-kuma-node1",
		Image:      "kuma:2",
		Networks:   []string{"yamlops-prod"},
		EnvFiles:   []string{"uptime-kuma-node1.env"},
		Volumes:    []string{"./data:/app/data"},
		ExtraHosts: []string{"host.docker.internal:host-gateway"},
		HealthCheck: &HealthCheck{
			Test:        []string{"CMD", "curl", "-fsS", "http://127.0.0.1:3001"},
			Interval:    "10s",
			Timeout:     "5s",
			Retries:     5,
			StartPeriod: "20s",
		},
	}

	agent := &ComposeService{
		Name:      "uptime-kuma-node1-agent",
		Image:     "sync:dev",
		Networks:  []string{"yamlops-prod"},
		EnvFiles:  []string{"uptime-kuma-node1.env"},
		Volumes:   []string{"./uptime-kuma-node1.kuma.targets.yaml:/targets.yaml:ro"},
		DependsOn: map[string]DependsOnEntry{"uptime-kuma-node1": {Condition: "service_healthy"}},
	}

	content, err := gen.GenerateMulti([]*ComposeService{main, agent}, "prod")
	if err != nil {
		t.Fatalf("GenerateMulti() error = %v", err)
	}
	s := string(content)

	checks := []string{
		"yo-prod-uptime-kuma-node1:",
		"container_name: yo-prod-uptime-kuma-node1",
		"image: kuma:2",
		"yo-prod-uptime-kuma-node1-agent:",
		"container_name: yo-prod-uptime-kuma-node1-agent",
		"image: sync:dev",
		"yamlops-prod:",
		"external: true",
		"- ./data:/app/data",
		"- ./uptime-kuma-node1.kuma.targets.yaml:/targets.yaml:ro",
		"- uptime-kuma-node1.env",
		"aliases:",
		"- uptime-kuma-node1",
		"- uptime-kuma-node1-agent",
		"condition: service_healthy",
		"host.docker.internal:host-gateway",
	}
	for _, c := range checks {
		if !strings.Contains(s, c) {
			t.Errorf("multi compose missing %q\n---\n%s", c, s)
		}
	}
}

func TestGenerateMulti_Empty(t *testing.T) {
	gen := NewGenerator()
	if _, err := gen.GenerateMulti(nil, "prod"); err == nil {
		t.Error("expected error for empty services")
	}
	if _, err := gen.GenerateMulti([]*ComposeService{}, "prod"); err == nil {
		t.Error("expected error for empty services")
	}
}

func TestGenerateMulti_DuplicateName(t *testing.T) {
	gen := NewGenerator()
	dup := []*ComposeService{
		{Name: "svc-a", Image: "img:1", Networks: []string{"yamlops-prod"}},
		{Name: "svc-a", Image: "img:2", Networks: []string{"yamlops-prod"}},
	}
	if _, err := gen.GenerateMulti(dup, "prod"); err == nil {
		t.Error("expected error for duplicate service names")
	}
}

func TestGenerateSingle_UnchangedShape(t *testing.T) {
	gen := NewGenerator()

	svc := &ComposeService{
		Name:       "frps-node5",
		Image:      "frps:prod",
		Ports:      []string{"30070:30070"},
		Volumes:    []string{"./frps-node5.frps.toml:/app/configs/frps.toml:ro"},
		Networks:   []string{"yamlops-prod"},
		ExtraHosts: []string{"host.docker.internal:host-gateway"},
	}

	content, err := gen.Generate(svc, "prod")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	s := string(content)

	// 单服务输出必须保持既有格式：无 depends_on、services 只有一条
	if strings.Contains(s, "depends_on:") {
		t.Errorf("single service must not contain depends_on\n---\n%s", s)
	}
	if strings.Count(s, "container_name: yo-prod-frps-node5") != 1 {
		t.Errorf("expected exactly one container_name\n---\n%s", s)
	}
	for _, c := range []string{"30070:30070", "yamlops-prod:", "external: true", "unless-stopped", "host.docker.internal:host-gateway"} {
		if !strings.Contains(s, c) {
			t.Errorf("single compose missing %q\n---\n%s", c, s)
		}
	}
}

func TestGenerate_DefaultLogRotation(t *testing.T) {
	gen := NewGenerator()
	svc := &ComposeService{Name: "web", Image: "web:1", Networks: []string{"yamlops-dev"}}
	content, err := gen.Generate(svc, "dev")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	s := string(content)
	for _, c := range []string{"logging:", "driver: json-file", "max-size: 10m", "max-file: \"3\""} {
		if !strings.Contains(s, c) {
			t.Errorf("compose missing log rotation %q\n---\n%s", c, s)
		}
	}
}
