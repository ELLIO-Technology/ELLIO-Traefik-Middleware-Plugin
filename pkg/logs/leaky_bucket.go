package logs

import (
	"sync"
	"time"
)

// LeakyBucket implements a token bucket rate limiter
type LeakyBucket struct {
	capacity   int64
	tokens     int64
	refillRate int64 // tokens per second
	lastRefill time.Time
	mu         sync.Mutex
}

// NewLeakyBucket creates a new leaky bucket rate limiter
func NewLeakyBucket(capacity, refillRate int64) *LeakyBucket {
	return &LeakyBucket{
		capacity:   capacity,
		tokens:     capacity,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

// Allow checks if n tokens are available and consumes them if so
func (lb *LeakyBucket) Allow(tokens int64) bool {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	lb.refill()

	if lb.tokens >= tokens {
		lb.tokens -= tokens
		return true
	}

	return false
}

// refill adds tokens based on time elapsed
func (lb *LeakyBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(lb.lastRefill)
	tokensToAdd := int64(elapsed.Seconds() * float64(lb.refillRate))

	if tokensToAdd > 0 {
		lb.tokens = minInt64(lb.capacity, lb.tokens+tokensToAdd)
		lb.lastRefill = now
	}
}

// WaitTime returns how long to wait for n tokens to be available
func (lb *LeakyBucket) WaitTime(tokens int64) time.Duration {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	lb.refill()

	if lb.tokens >= tokens {
		return 0
	}

	tokensNeeded := tokens - lb.tokens
	secondsToWait := float64(tokensNeeded) / float64(lb.refillRate)
	return time.Duration(secondsToWait * float64(time.Second))
}

// minInt64 returns the minimum of two int64 values
func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
