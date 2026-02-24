package logger

import (
	"context"
	"crypto/rand"
	"encoding/hex"
)

type ctxKey struct{}

func ContextWithLogger(ctx context.Context, logger *Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, logger)
}

func FromContext(ctx context.Context) *Logger {
	if ctx == nil {
		return L()
	}
	if l, ok := ctx.Value(ctxKey{}).(*Logger); ok {
		return l
	}
	return L()
}

func WithOperation(ctx context.Context, operation string) context.Context {
	opID := generateShortID()
	logger := FromContext(ctx).With(
		"operation", operation,
		"op_id", opID,
	)
	return ContextWithLogger(ctx, logger)
}

func WithTraceID(ctx context.Context, traceID string) context.Context {
	logger := FromContext(ctx).With("trace_id", traceID)
	return ContextWithLogger(ctx, logger)
}

func generateShortID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}
