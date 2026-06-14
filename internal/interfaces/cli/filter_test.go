package cli

import (
	"testing"
)

func TestParseServiceTypes(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{"empty string", "", nil, false},
		{"single valid type", "biz", []string{"biz"}, false},
		{"single valid type infra", "infra", []string{"infra"}, false},
		{"both types", "biz,infra", []string{"biz", "infra"}, false},
		{"with spaces", " biz , infra ", []string{"biz", "infra"}, false},
		{"invalid type", "unknown", nil, true},
		{"mixed valid and invalid", "biz,unknown,infra", nil, true},
		{"only commas", ",,,", nil, false},
		{"trailing comma", "biz,", []string{"biz"}, false},
		{"leading comma", ",biz", []string{"biz"}, false},
		{"spaces only", "   ", nil, false},
		{"multiple invalid", "foo,bar", nil, true},
		{"duplicates", "biz,biz", []string{"biz"}, false},
		{"duplicates mixed", "biz,infra,biz", []string{"biz", "infra"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseServiceTypes(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseServiceTypes(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("parseServiceTypes(%q) unexpected error: %v", tt.input, err)
				return
			}
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if len(got) != len(tt.want) {
				t.Errorf("parseServiceTypes(%q) = %v, want %v", tt.input, got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("parseServiceTypes(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestMatchesFilter(t *testing.T) {
	tests := []struct {
		name   string
		value  string
		filter string
		want   bool
	}{
		{"empty filter matches all", "anything", "", true},
		{"exact match", "server1", "server1", true},
		{"no match", "server1", "server2", false},
		{"match in comma list", "server2", "server1,server2,server3", true},
		{"no match in comma list", "server4", "server1,server2,server3", false},
		{"spaces around filter", "server1", " server1 ", true},
		{"empty value with empty filter", "", "", true},
		{"empty value with non-empty filter", "", "server1", false},
		{"filter with empty segments", "server1", ",server1,", true},
		{"only spaces in filter segment", "server1", "  ", false},
		{"single comma", "a", ",", false},
		{"value with spaces does not match", "server 1", "server1", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesFilter(tt.value, tt.filter)
			if got != tt.want {
				t.Errorf("matchesFilter(%q, %q) = %v, want %v", tt.value, tt.filter, got, tt.want)
			}
		})
	}
}
