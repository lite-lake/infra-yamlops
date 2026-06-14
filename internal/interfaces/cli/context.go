package cli

import "strings"

type Context struct {
	Env         string
	ConfigDir   string
	Concurrency int
}

func NewContext() *Context {
	return &Context{
		Env:         "dev",
		ConfigDir:   ".",
		Concurrency: 5,
	}
}

type Filters struct {
	Domain  string
	Zone    string
	Server  string
	Service string
}

func splitAndTrim(s, sep string) []string {
	parts := strings.Split(s, sep)
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
