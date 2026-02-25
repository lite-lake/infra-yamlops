package domain

import "time"

const MaxPortNumber = 65535

const (
	DefaultRetryMaxAttempts    = 3
	DefaultRetryInitialDelayMs = 100
	DefaultRetryMaxDelaySec    = 30
	DefaultRetryMultiplier     = 2.0
)

var (
	DefaultRetryInitialDelay = DefaultRetryInitialDelayMs * time.Millisecond
	DefaultRetryMaxDelay     = DefaultRetryMaxDelaySec * time.Second
)
