package main

import (
	"log/slog"
	"os"

	"github.com/lite-lake/infra-yamlops/internal/infrastructure/logger"
	"github.com/lite-lake/infra-yamlops/internal/interfaces/cli"
)

func main() {
	logLevel := slog.LevelInfo
	if os.Getenv("YAMLOPS_DEBUG") != "" {
		logLevel = slog.LevelDebug
	}

	logFormat := os.Getenv("YAMLOPS_LOG_FORMAT")

	logger.Init(&logger.Config{
		Level:     logLevel,
		Format:    logFormat,
		AddSource: os.Getenv("YAMLOPS_DEBUG") != "",
	})

	cli.Execute()
}
