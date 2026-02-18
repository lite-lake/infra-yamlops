package entity

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/litelake/yamlops/internal/domain"
)

type Domain struct {
	Name   string `yaml:"name"`
	ISP    string `yaml:"isp,omitempty"`
	DNSISP string `yaml:"dns_isp"`
	Parent string `yaml:"parent,omitempty"`
}

var domainRegex = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`)

func (d *Domain) Validate() error {
	if d.Name == "" {
		return fmt.Errorf("%w: domain name is required", domain.ErrInvalidDomain)
	}
	name := d.Name
	if strings.HasPrefix(name, "*.") {
		name = name[2:]
	}
	if !domainRegex.MatchString(name) {
		return fmt.Errorf("%w: invalid domain format %s", domain.ErrInvalidDomain, d.Name)
	}
	if d.DNSISP == "" {
		return errors.New("dns_isp is required")
	}
	return nil
}
