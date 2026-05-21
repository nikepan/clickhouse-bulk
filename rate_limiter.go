package main

import (
	"sync"
	"time"
)

// sendRateLimiter limits HTTP posts to ClickHouse (queue worker and dump replay).
type sendRateLimiter struct {
	interval time.Duration
	burst    int
	mu       sync.Mutex
	tokens   float64
	last     time.Time
}

func newSendRateLimiter(rps, burst int) *sendRateLimiter {
	if rps <= 0 {
		return nil
	}
	if burst <= 0 {
		burst = rps
	}
	if burst < 1 {
		burst = 1
	}
	return &sendRateLimiter{
		interval: time.Second / time.Duration(rps),
		burst:    burst,
		tokens:   float64(burst),
		last:     time.Now(),
	}
}

// Wait blocks until a send token is available.
func (r *sendRateLimiter) Wait() {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	elapsed := now.Sub(r.last)
	r.last = now
	r.tokens += float64(elapsed) / float64(r.interval)
	if r.tokens > float64(r.burst) {
		r.tokens = float64(r.burst)
	}
	for r.tokens < 1 {
		sleep := time.Duration((1 - r.tokens) * float64(r.interval))
		if sleep < time.Millisecond {
			sleep = time.Millisecond
		}
		r.mu.Unlock()
		time.Sleep(sleep)
		r.mu.Lock()
		now = time.Now()
		elapsed = now.Sub(r.last)
		r.last = now
		r.tokens += float64(elapsed) / float64(r.interval)
		if r.tokens > float64(r.burst) {
			r.tokens = float64(r.burst)
		}
	}
	r.tokens--
}
