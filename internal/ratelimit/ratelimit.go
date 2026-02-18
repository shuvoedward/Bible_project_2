package ratelimit

import (
	"context"
	_ "embed"
	"fmt"
	"shuvoedward/Bible_project/internal/cache"
	"time"
)

//go:embed scripts/slidingWindow.lua
var slidingWindowScript string

type Limiters struct {
	Enabled bool
	IP      *rateLimiter
	Note    *rateLimiter
	Auth    *rateLimiter
}

func NewLimiters(enabled bool, ipLimit, noteLimit, authLimit int, cache *cache.RedisClient, window time.Duration) *Limiters {
	return &Limiters{
		Enabled: enabled,
		IP:      newRateLimiter(ipLimit, window, cache, "ip"),
		Note:    newRateLimiter(noteLimit, window, cache, "note"),
		Auth:    newRateLimiter(authLimit, window, cache, "auth"),
	}
}

type rateLimiter struct {
	cache  *cache.RedisClient
	action string
	limit  int
	window time.Duration
}

func newRateLimiter(limit int, window time.Duration, cache *cache.RedisClient, action string) *rateLimiter {
	rl := &rateLimiter{
		cache:  cache,
		action: action,
		limit:  limit,
		window: window,
	}

	return rl
}

func (rl *rateLimiter) Allow(ip string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	key := fmt.Sprintf("rl:%s:%s", rl.action, ip)
	windowMs := rl.window.Milliseconds()

	result, err := rl.cache.Eval(ctx, slidingWindowScript, []string{key}, rl.limit, windowMs)
	if err != nil {
		return false, err
	}

	vals := result.([]interface{})
	allowed := vals[0].(int64) == 1

	return allowed, nil
}
