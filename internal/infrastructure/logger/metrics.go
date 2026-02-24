package logger

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

type Metrics struct {
	operationsTotal   map[string]*atomic.Int64
	operationsFailed  map[string]*atomic.Int64
	operationsLatency map[string]*atomic.Int64
	mu                sync.Mutex
}

var globalMetrics = &Metrics{
	operationsTotal:   make(map[string]*atomic.Int64),
	operationsFailed:  make(map[string]*atomic.Int64),
	operationsLatency: make(map[string]*atomic.Int64),
}

type OperationStats struct {
	Total        int64
	Failed       int64
	AvgLatencyMs float64
}

func RecordOperation(operation string, err error, duration time.Duration) {
	globalMetrics.mu.Lock()

	total, ok := globalMetrics.operationsTotal[operation]
	if !ok {
		total = &atomic.Int64{}
		globalMetrics.operationsTotal[operation] = total
	}

	latency, ok := globalMetrics.operationsLatency[operation]
	if !ok {
		latency = &atomic.Int64{}
		globalMetrics.operationsLatency[operation] = latency
	}

	globalMetrics.mu.Unlock()

	total.Add(1)
	latency.Add(duration.Nanoseconds())

	if err != nil {
		globalMetrics.mu.Lock()
		failed, ok := globalMetrics.operationsFailed[operation]
		if !ok {
			failed = &atomic.Int64{}
			globalMetrics.operationsFailed[operation] = failed
		}
		globalMetrics.mu.Unlock()
		failed.Add(1)
	}
}

func GetMetrics() map[string]OperationStats {
	globalMetrics.mu.Lock()
	defer globalMetrics.mu.Unlock()

	result := make(map[string]OperationStats)
	for op, total := range globalMetrics.operationsTotal {
		stats := OperationStats{
			Total: total.Load(),
		}
		if failed, ok := globalMetrics.operationsFailed[op]; ok {
			stats.Failed = failed.Load()
		}
		if latency, ok := globalMetrics.operationsLatency[op]; ok {
			count := total.Load()
			if count > 0 {
				stats.AvgLatencyMs = float64(latency.Load()) / float64(count) / 1e6
			}
		}
		result[op] = stats
	}
	return result
}

func TimedOperation(ctx context.Context, operation string, fn func() error) error {
	start := time.Now()
	log := FromContext(ctx).With("operation", operation)
	log.Debug("starting operation")

	err := fn()
	duration := time.Since(start)

	RecordOperation(operation, err, duration)

	if err != nil {
		log.Error("operation failed", "error", err, "duration", duration)
	} else {
		log.Debug("operation completed", "duration", duration)
	}

	return err
}

func ResetMetrics() {
	globalMetrics.mu.Lock()
	defer globalMetrics.mu.Unlock()
	globalMetrics.operationsTotal = make(map[string]*atomic.Int64)
	globalMetrics.operationsFailed = make(map[string]*atomic.Int64)
	globalMetrics.operationsLatency = make(map[string]*atomic.Int64)
}
