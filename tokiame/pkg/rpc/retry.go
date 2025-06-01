package rpc

import (
	"math/rand"
	"sync"
	"time"
)

// RetryPolicy manages exponential backoff for reconnections.
type RetryPolicy struct {
	InitialInterval time.Duration
	MaxInterval     time.Duration
	Multiplier      float64
	currentInterval time.Duration
	mu              sync.Mutex
}

func NewRetryPolicy(initial, max time.Duration, multiplier float64) *RetryPolicy {
	if multiplier < 1.0 {
		multiplier = 1.0
	}
	return &RetryPolicy{
		InitialInterval: initial,
		MaxInterval:     max,
		Multiplier:      multiplier,
		currentInterval: initial,
	}
}

// NextInterval calculates the next retry interval with jitter.
func (rp *RetryPolicy) NextInterval() time.Duration {
	rp.mu.Lock()
	intervalToUse := rp.currentInterval
	rp.currentInterval = min(time.Duration(float64(rp.currentInterval)*rp.Multiplier), rp.MaxInterval)
	rp.mu.Unlock()

	if intervalToUse <= 0 {
		intervalToUse = rp.InitialInterval
	}
	jitterRange := float64(intervalToUse) * 0.1
	// Use global rand.Float64()
	jitter := (rand.Float64() * 2 * jitterRange) - jitterRange

	return intervalToUse + time.Duration(jitter)
}

func (rp *RetryPolicy) Reset() {
	rp.mu.Lock()
	rp.currentInterval = rp.InitialInterval
	rp.mu.Unlock()
}
