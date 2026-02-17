package handler

import (
	"context"

	"github.com/litelake/yamlops/internal/domain/valueobject"
)

type CertificateHandler struct{}

func NewCertificateHandler() *CertificateHandler {
	return &CertificateHandler{}
}

func (h *CertificateHandler) EntityType() string {
	return "certificate"
}

func (h *CertificateHandler) Apply(ctx context.Context, change *valueobject.Change, deps *Deps) (*Result, error) {
	return &Result{
		Change:  change,
		Success: true,
		Output:  "skipped (not a deployable entity)",
	}, nil
}
