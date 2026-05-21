package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSendRateLimiter_Wait(t *testing.T) {
	lim := newSendRateLimiter(10, 2)
	start := time.Now()
	lim.Wait()
	lim.Wait()
	lim.Wait()
	elapsed := time.Since(start)
	assert.GreaterOrEqual(t, elapsed, 80*time.Millisecond)
}

func TestSendRateLimiter_Disabled(t *testing.T) {
	lim := newSendRateLimiter(0, 0)
	start := time.Now()
	lim.Wait()
	lim.Wait()
	assert.Less(t, time.Since(start), 20*time.Millisecond)
}
