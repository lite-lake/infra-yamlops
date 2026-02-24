package ssh

import "testing"

func TestShellEscape(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "'hello'"},
		{"hello'world", "'hello'\\''world'"},
		{"file.txt", "'file.txt'"},
		{"/path/to/file", "'/path/to/file'"},
		{"$HOME", "'$HOME'"},
		{"`whoami`", "'`whoami`'"},
		{"; rm -rf /", "'; rm -rf /'"},
		{"$(cat /etc/passwd)", "'$(cat /etc/passwd)'"},
		{"a'b'c", "'a'\\''b'\\''c'"},
		{"", "''"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := ShellEscape(tt.input); got != tt.expected {
				t.Errorf("ShellEscape(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
