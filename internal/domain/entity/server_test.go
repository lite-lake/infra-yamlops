package entity

import (
	"errors"
	"testing"

	"github.com/litelake/yamlops/internal/domain"
	"github.com/litelake/yamlops/internal/domain/valueobject"
)

func TestServerIP_Validate(t *testing.T) {
	tests := []struct {
		name    string
		ip      ServerIP
		wantErr error
	}{
		{
			name:    "empty ips are valid",
			ip:      ServerIP{},
			wantErr: nil,
		},
		{
			name:    "invalid public ip",
			ip:      ServerIP{Public: "invalid"},
			wantErr: domain.ErrInvalidIP,
		},
		{
			name:    "invalid private ip",
			ip:      ServerIP{Private: "invalid"},
			wantErr: domain.ErrInvalidIP,
		},
		{
			name:    "valid ipv4 public only",
			ip:      ServerIP{Public: "192.168.1.1"},
			wantErr: nil,
		},
		{
			name:    "valid ipv4 private only",
			ip:      ServerIP{Private: "10.0.0.1"},
			wantErr: nil,
		},
		{
			name:    "valid both ipv4",
			ip:      ServerIP{Public: "203.0.113.1", Private: "10.0.0.1"},
			wantErr: nil,
		},
		{
			name:    "valid ipv6",
			ip:      ServerIP{Public: "2001:db8::1"},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ip.Validate()
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("Validate() error = %v, want %v", err, tt.wantErr)
				}
			} else if err != nil {
				t.Errorf("Validate() unexpected error = %v", err)
			}
		})
	}
}

func TestServerSSH_Validate(t *testing.T) {
	tests := []struct {
		name    string
		ssh     ServerSSH
		wantErr error
	}{
		{
			name:    "missing host",
			ssh:     ServerSSH{Port: 22, User: "root", Password: valueobject.SecretRef{Plain: "pass"}},
			wantErr: domain.ErrRequired,
		},
		{
			name:    "invalid port zero",
			ssh:     ServerSSH{Host: "192.168.1.1", Port: 0, User: "root", Password: valueobject.SecretRef{Plain: "pass"}},
			wantErr: domain.ErrInvalidPort,
		},
		{
			name:    "invalid port negative",
			ssh:     ServerSSH{Host: "192.168.1.1", Port: -1, User: "root", Password: valueobject.SecretRef{Plain: "pass"}},
			wantErr: domain.ErrInvalidPort,
		},
		{
			name:    "invalid port too large",
			ssh:     ServerSSH{Host: "192.168.1.1", Port: 65536, User: "root", Password: valueobject.SecretRef{Plain: "pass"}},
			wantErr: domain.ErrInvalidPort,
		},
		{
			name:    "missing user",
			ssh:     ServerSSH{Host: "192.168.1.1", Port: 22, Password: valueobject.SecretRef{Plain: "pass"}},
			wantErr: domain.ErrRequired,
		},
		{
			name:    "empty password",
			ssh:     ServerSSH{Host: "192.168.1.1", Port: 22, User: "root", Password: valueobject.SecretRef{}},
			wantErr: domain.ErrEmptyValue,
		},
		{
			name:    "valid with plain password",
			ssh:     ServerSSH{Host: "192.168.1.1", Port: 22, User: "root", Password: valueobject.SecretRef{Plain: "pass"}},
			wantErr: nil,
		},
		{
			name:    "valid with secret reference",
			ssh:     ServerSSH{Host: "192.168.1.1", Port: 22, User: "root", Password: valueobject.SecretRef{Secret: "ssh_pass"}},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ssh.Validate()
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("Validate() error = %v, want %v", err, tt.wantErr)
				}
			} else if err != nil {
				t.Errorf("Validate() unexpected error = %v", err)
			}
		})
	}
}

func TestServer_Validate(t *testing.T) {
	tests := []struct {
		name    string
		server  Server
		wantErr error
	}{
		{
			name:    "missing name",
			server:  Server{},
			wantErr: domain.ErrInvalidName,
		},
		{
			name:    "missing zone",
			server:  Server{Name: "server-1"},
			wantErr: domain.ErrRequired,
		},
		{
			name: "invalid ip",
			server: Server{
				Name: "server-1",
				Zone: "zone-1",
				IP:   ServerIP{Public: "invalid"},
			},
			wantErr: domain.ErrInvalidIP,
		},
		{
			name: "invalid ssh",
			server: Server{
				Name: "server-1",
				Zone: "zone-1",
				IP:   ServerIP{Public: "192.168.1.1"},
				SSH:  ServerSSH{Host: "", Port: 22, User: "root"},
			},
			wantErr: domain.ErrRequired,
		},
		{
			name: "valid minimal",
			server: Server{
				Name: "server-1",
				Zone: "zone-1",
				IP:   ServerIP{},
				SSH: ServerSSH{
					Host:     "192.168.1.1",
					Port:     22,
					User:     "root",
					Password: valueobject.SecretRef{Plain: "pass"},
				},
			},
			wantErr: nil,
		},
		{
			name: "valid full",
			server: Server{
				Name: "server-1",
				Zone: "zone-1",
				ISP:  "isp-1",
				OS:   "ubuntu",
				IP:   ServerIP{Public: "203.0.113.1", Private: "10.0.0.1"},
				SSH: ServerSSH{
					Host:     "203.0.113.1",
					Port:     22,
					User:     "root",
					Password: valueobject.SecretRef{Secret: "ssh_pass"},
				},
				Environment: ServerEnvironment{
					APTSource:  "mirror",
					Registries: []string{"registry-1"},
				},
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.server.Validate()
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("Validate() error = %v, want %v", err, tt.wantErr)
				}
			} else if err != nil {
				t.Errorf("Validate() unexpected error = %v", err)
			}
		})
	}
}
