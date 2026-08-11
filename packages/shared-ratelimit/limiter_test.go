package ratelimit

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

func newTestLimiter(t *testing.T) *Limiter {
	l, _ := newTestLimiterWithMiniredis(t)
	return l
}

func newTestLimiterWithMiniredis(t *testing.T) (*Limiter, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	t.Cleanup(mr.Close)

	l, err := New("redis://" + mr.Addr())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return l, mr
}

func TestNewRejectsUnreachableRedis(t *testing.T) {
	if _, err := New("redis://127.0.0.1:1"); err == nil {
		t.Fatal("expected error for an unreachable redis")
	}
}

func TestNewRejectsInvalidURL(t *testing.T) {
	if _, err := New("not-a-url"); err == nil {
		t.Fatal("expected error for an invalid redis URL")
	}
}

func TestAllowsWithinLimit(t *testing.T) {
	l := newTestLimiter(t)
	cfg := Config{Limit: 3, Window: time.Minute}

	for i := 0; i < 3; i++ {
		allowed, err := l.Allow(t.Context(), "test-key", cfg)
		if err != nil {
			t.Fatalf("Allow: %v", err)
		}
		if !allowed {
			t.Fatalf("expected request %d to be allowed within limit 3", i+1)
		}
	}
}

func TestRejectsOverLimit(t *testing.T) {
	l := newTestLimiter(t)
	cfg := Config{Limit: 2, Window: time.Minute}

	for i := 0; i < 2; i++ {
		if allowed, err := l.Allow(t.Context(), "over-key", cfg); err != nil || !allowed {
			t.Fatalf("expected request %d to be allowed, allowed=%v err=%v", i+1, allowed, err)
		}
	}
	allowed, err := l.Allow(t.Context(), "over-key", cfg)
	if err != nil {
		t.Fatalf("Allow: %v", err)
	}
	if allowed {
		t.Fatal("expected the 3rd request to exceed a limit of 2")
	}
}

func TestDifferentKeysHaveIndependentLimits(t *testing.T) {
	l := newTestLimiter(t)
	cfg := Config{Limit: 1, Window: time.Minute}

	if allowed, err := l.Allow(t.Context(), "key-a", cfg); err != nil || !allowed {
		t.Fatalf("key-a: allowed=%v err=%v", allowed, err)
	}
	if allowed, err := l.Allow(t.Context(), "key-b", cfg); err != nil || !allowed {
		t.Fatalf("key-b should be independent of key-a: allowed=%v err=%v", allowed, err)
	}
}

func TestWindowResetsAfterExpiry(t *testing.T) {
	l, mr := newTestLimiterWithMiniredis(t)
	cfg := Config{Limit: 1, Window: 50 * time.Millisecond}

	if allowed, err := l.Allow(t.Context(), "expiring-key", cfg); err != nil || !allowed {
		t.Fatalf("first request: allowed=%v err=%v", allowed, err)
	}
	if allowed, _ := l.Allow(t.Context(), "expiring-key", cfg); allowed {
		t.Fatal("expected the 2nd request within the window to be rejected")
	}

	// miniredis uses a virtual clock for key expiry — advance it directly
	// rather than sleeping real wall-clock time.
	mr.FastForward(100 * time.Millisecond)

	if allowed, err := l.Allow(t.Context(), "expiring-key", cfg); err != nil || !allowed {
		t.Fatalf("expected a new window to allow again: allowed=%v err=%v", allowed, err)
	}
}
