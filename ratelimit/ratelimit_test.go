package ratelimit

import (
	"context"
	"errors"
	"testing"
)

func TestMemoryAllowsConfiguredLimit(t *testing.T) {
	t.Parallel()
	limiter, err := NewMemory(Policy{Limit: 2}, 1)
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		allowed, err := limiter.Allow(context.Background(), "usr_123")
		if err != nil || !allowed {
			t.Fatalf("Allow() = %t, %v", allowed, err)
		}
	}
	allowed, err := limiter.Allow(context.Background(), "usr_123")
	if err != nil || allowed {
		t.Fatalf("Allow() = %t, %v", allowed, err)
	}
}

func TestMemoryRejectsUnboundedNewKeys(t *testing.T) {
	t.Parallel()
	limiter, err := NewMemory(Policy{Limit: 1}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := limiter.Allow(context.Background(), "one"); err != nil {
		t.Fatal(err)
	}
	_, err = limiter.Allow(context.Background(), "two")
	if !errors.Is(err, ErrCapacity) {
		t.Fatalf("Allow() error = %v", err)
	}
}
