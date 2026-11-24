package entity

import "fmt"

type Endpoint struct {
	ServiceName string
	Hostname    string
	Protocol    string
	Path        string
	Server      string
}

func (e Endpoint) URL() string {
	path := e.Path
	if path == "" {
		path = "/"
	}
	return fmt.Sprintf("%s://%s%s", e.Protocol, e.Hostname, path)
}
