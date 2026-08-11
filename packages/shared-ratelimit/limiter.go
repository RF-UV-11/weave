// Package ratelimit is Weave's shared defensive rate-limiting building
// block: a Redis-backed fixed-window counter and a gRPC interceptor that
// applies it to every RPC. This is a network-level defense against
// flooding/brute-force/credential-stuffing traffic — distinct from (and
// applied ahead of) any future tenant-plan usage quota, which is a
// business concern layered on top once a caller's identity is known.
package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Config is a limit: at most Limit requests per Window, per key.
type Config struct {
	Limit  int
	Window time.Duration
}

type Limiter struct {
	client *redis.Client
}

// New connects to Redis at redisURL (e.g. "redis://localhost:6379") and
// verifies connectivity before returning.
func New(redisURL string) (*Limiter, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("ratelimit: parse redis url: %w", err)
	}
	client := redis.NewClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ratelimit: connect to redis: %w", err)
	}

	return &Limiter{client: client}, nil
}

// Ping reports whether Redis is currently reachable — for callers that
// want to reflect real dependency health (e.g. a gRPC health check)
// rather than assuming the connection established at New() is still good.
func (l *Limiter) Ping(ctx context.Context) error {
	if err := l.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("ratelimit: ping: %w", err)
	}
	return nil
}

// Allow reports whether one more request under key is permitted by cfg,
// using a fixed-window counter: the first request for a key in a window
// sets the window's expiry, every request within it increments the same
// counter, and the key naturally expires (no cleanup job needed).
func (l *Limiter) Allow(ctx context.Context, key string, cfg Config) (bool, error) {
	count, err := l.client.Incr(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("ratelimit: incr: %w", err)
	}
	if count == 1 {
		// PExpire (millisecond precision) rather than Expire (seconds) —
		// sub-second windows are common in tests and shouldn't silently
		// round up to a full second.
		if err := l.client.PExpire(ctx, key, cfg.Window).Err(); err != nil {
			return false, fmt.Errorf("ratelimit: expire: %w", err)
		}
	}
	return count <= int64(cfg.Limit), nil
}
