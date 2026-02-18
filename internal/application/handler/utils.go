package handler

import (
	"fmt"
	"os"

	"github.com/litelake/yamlops/internal/domain/valueobject"
)

func ExtractServerFromChange(ch *valueobject.Change) string {
	if ch.OldState != nil {
		if svc, ok := ch.OldState.(map[string]interface{}); ok {
			if server, ok := svc["server"].(string); ok {
				return server
			}
		}
		switch v := ch.OldState.(type) {
		case interface{ GetServer() string }:
			return v.GetServer()
		}
	}
	if ch.NewState != nil {
		if svc, ok := ch.NewState.(map[string]interface{}); ok {
			if server, ok := svc["server"].(string); ok {
				return server
			}
		}
		switch v := ch.NewState.(type) {
		case interface{ GetServer() string }:
			return v.GetServer()
		}
	}
	return ""
}

func SyncContent(client SSHClient, content, remotePath string) error {
	tmpFile, err := os.CreateTemp("", "yamlops-*.yml")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.WriteString(content); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	tmpFile.Close()

	return client.UploadFileSudo(tmpFile.Name(), remotePath)
}
