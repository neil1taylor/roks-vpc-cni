package vpc

import (
	"context"
	"fmt"
)

// RateLimiter controls concurrent access to the VPC API using a semaphore pattern.
type RateLimiter struct {
	sem chan struct{}
}

// NewRateLimiter creates a rate limiter allowing maxConcurrent simultaneous VPC API calls.
func NewRateLimiter(maxConcurrent int) *RateLimiter {
	return &RateLimiter{
		sem: make(chan struct{}, maxConcurrent),
	}
}

// Acquire blocks until a slot is available or the context is cancelled.
func (r *RateLimiter) Acquire(ctx context.Context) error {
	select {
	case r.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("rate limiter: context cancelled: %w", ctx.Err())
	}
}

// Release returns a slot to the pool. Must be called after Acquire.
func (r *RateLimiter) Release() {
	<-r.sem
}
