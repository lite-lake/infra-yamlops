package entity

import (
	"errors"
	"testing"

	"github.com/lite-lake/infra-yamlops/internal/domain"
)

func TestZone_Validate(t *testing.T) {
	tests := []struct {
		name    string
		zone    Zone
		wantErr error
	}{
		{
			name:    "missing name",
			zone:    Zone{},
			wantErr: domain.ErrInvalidName,
		},
		{
			name:    "missing region",
			zone:    Zone{Name: "zone-1"},
			wantErr: domain.ErrRequired,
		},
		{
			name:    "valid minimal",
			zone:    Zone{Name: "zone-1", Region: "us-east-1"},
			wantErr: nil,
		},
		{
			name:    "valid full",
			zone:    Zone{Name: "zone-1", Description: "Primary zone", ISP: "aws", Region: "us-east-1"},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.zone.Validate()
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
