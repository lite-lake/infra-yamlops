package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"sync"
)

type Logger struct {
	*slog.Logger
}

var (
	defaultLogger *Logger
	once          sync.Once
)

type Config struct {
	Level     slog.Level
	Format    string
	Output    io.Writer
	AddSource bool
}

func DefaultConfig() *Config {
	return &Config{
		Level:     slog.LevelInfo,
		Format:    "text",
		Output:    os.Stderr,
		AddSource: false,
	}
}

func Init(cfg *Config) {
	once.Do(func() {
		if cfg == nil {
			cfg = DefaultConfig()
		}

		output := cfg.Output
		if output == nil {
			output = os.Stderr
		}

		var handler slog.Handler
		opts := &slog.HandlerOptions{
			Level:     cfg.Level,
			AddSource: cfg.AddSource,
		}

		switch cfg.Format {
		case "json":
			handler = slog.NewJSONHandler(output, opts)
		default:
			handler = slog.NewTextHandler(output, opts)
		}

		defaultLogger = &Logger{slog.New(handler)}
	})
}

func L() *Logger {
	if defaultLogger == nil {
		Init(DefaultConfig())
	}
	return defaultLogger
}

func (l *Logger) With(args ...any) *Logger {
	return &Logger{l.Logger.With(args...)}
}

func (l *Logger) WithContext(ctx context.Context) *Logger {
	return l
}

func Debug(msg string, args ...any) { L().Debug(msg, args...) }
func Info(msg string, args ...any)  { L().Info(msg, args...) }
func Warn(msg string, args ...any)  { L().Warn(msg, args...) }
func Error(msg string, args ...any) { L().Error(msg, args...) }

func WithFields(fields ...any) *Logger {
	return L().With(fields...)
}
