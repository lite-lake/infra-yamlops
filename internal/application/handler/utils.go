package handler

import (
	"fmt"
	"os"

	"github.com/litelake/yamlops/internal/constants"
	domainerr "github.com/litelake/yamlops/internal/domain"
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
	tmpFile, err := os.CreateTemp("", constants.TempFilePattern)
	if err != nil {
		return fmt.Errorf("syncing to %s: %w: %w", remotePath, domainerr.ErrTempFileFailed, err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		return fmt.Errorf("syncing to %s: %w: %w", remotePath, domainerr.ErrTempFileFailed, err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("syncing to %s: %w: %w", remotePath, domainerr.ErrTempFileFailed, err)
	}

	return client.UploadFileSudo(tmpFile.Name(), remotePath)
}
