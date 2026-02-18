package entity

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/litelake/yamlops/internal/domain"
)

type Domain struct {
	Name    string      `yaml:"name"`
	ISP     string      `yaml:"isp,omitempty"`
	DNSISP  string      `yaml:"dns_isp"`
	Parent  string      `yaml:"parent,omitempty"`
	Records []DNSRecord `yaml:"records,omitempty"`
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
	for i, r := range d.Records {
		if err := r.Validate(); err != nil {
			return fmt.Errorf("records[%d]: %w", i, err)
		}
	}
	return nil
}

func (d *Domain) FlattenRecords() []DNSRecord {
	var result []DNSRecord
	for _, r := range d.Records {
		record := r
		record.Domain = d.Name
		result = append(result, record)
	}
	return result
}
