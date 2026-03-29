package entity

import (
	"fmt"

	"github.com/lite-lake/infra-yamlops/internal/domain"
)

func ValidatePort(port int) error {
	if port <= 0 || port > MaxPortNumber {
		return fmt.Errorf("%w: must be between 1 and %d", domain.ErrInvalidPort, MaxPortNumber)
	}
	return nil
}
