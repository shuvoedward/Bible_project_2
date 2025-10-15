package ratelimit

import (
	"sync"
	"time"
)

type RateLimiter struct {
	requests map[int64][]time.Time
	mu       sync.RWMutex
	limit    int
	window   time.Duration
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		requests: make(map[int64][]time.Time),
		limit:    limit,
		window:   window,
	}

	go rl.cleanup()

	return rl
}

func (rl *RateLimiter) Allow(id int64) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	timestamps := rl.requests[id]

	valid := []time.Time{}
	for _, ts := range timestamps {
		if ts.After(cutoff) {
			valid = append(valid, ts)
		}
	}

	if len(valid) >= rl.limit {
		rl.requests[id] = valid
		return false
	}

	valid = append(valid, now)
	rl.requests[id] = valid

	return true
}

func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		cutoff := now.Add(-rl.window)

		for id, timestamps := range rl.requests {

			valid := []time.Time{}
			for _, ts := range timestamps {
				if ts.After(cutoff) {
					valid = append(valid, ts)
				}
			}

			// Remove entry if no recent requests
			if len(valid) == 0 {
				delete(rl.requests, id)
			} else {
				rl.requests[id] = valid
			}

		}

		rl.mu.Unlock()
	}
}
