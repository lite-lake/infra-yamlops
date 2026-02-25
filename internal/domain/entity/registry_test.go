package entity

import (
	"errors"
	"testing"

	"github.com/litelake/yamlops/internal/domain"
	"github.com/litelake/yamlops/internal/domain/valueobject"
)

func TestRegistryCredentials_Validate(t *testing.T) {
	tests := []struct {
		name        string
		credentials RegistryCredentials
		wantErr     error
	}{
		{
			name:        "empty username",
			credentials: RegistryCredentials{Password: *valueobject.NewSecretRefPlain("pass")},
			wantErr:     domain.ErrEmptyValue,
		},
		{
			name:        "empty password",
			credentials: RegistryCredentials{Username: *valueobject.NewSecretRefPlain("user")},
			wantErr:     domain.ErrEmptyValue,
		},
		{
			name: "valid plain credentials",
			credentials: RegistryCredentials{
				Username: *valueobject.NewSecretRefPlain("user"),
				Password: *valueobject.NewSecretRefPlain("pass"),
			},
			wantErr: nil,
		},
		{
			name: "valid secret references",
			credentials: RegistryCredentials{
				Username: *valueobject.NewSecretRefSecret("reg_user"),
				Password: *valueobject.NewSecretRefSecret("reg_pass"),
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.credentials.Validate()
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

func TestRegistry_Validate(t *testing.T) {
	tests := []struct {
		name     string
		registry Registry
		wantErr  error
	}{
		{
			name:     "missing name",
			registry: Registry{},
			wantErr:  domain.ErrInvalidName,
		},
		{
			name:     "missing url",
			registry: Registry{Name: "dockerhub"},
			wantErr:  domain.ErrRequired,
		},
		{
			name: "invalid credentials",
			registry: Registry{
				Name:        "dockerhub",
				URL:         "https://registry.hub.docker.com",
				Credentials: RegistryCredentials{},
			},
			wantErr: domain.ErrEmptyValue,
		},
		{
			name: "valid",
			registry: Registry{
				Name: "dockerhub",
				URL:  "https://registry.hub.docker.com",
				Credentials: RegistryCredentials{
					Username: *valueobject.NewSecretRefPlain("user"),
					Password: *valueobject.NewSecretRefPlain("pass"),
				},
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.registry.Validate()
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
