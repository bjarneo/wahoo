// Package ratelimit provides bounded local rate limiting primitives.
package ratelimit

import (
	"context"
	"errors"
	"sync"
	"time"
)

const (
	// DefaultWindow is used when Policy.Window is zero.
	DefaultWindow = time.Minute
	// MaxWindow prevents accidental long-lived in-memory buckets.
	MaxWindow = 24 * time.Hour
	// DefaultMaxKeys bounds the default local limiter memory use.
	DefaultMaxKeys = 10_000
	// MaxKeys caps local limiter memory use.
	MaxKeys     = 1_000_000
	maxKeyBytes = 256
)

var (
	// ErrInvalidPolicy reports an unsafe local rate-limit configuration.
	ErrInvalidPolicy = errors.New("invalid rate limit policy")
	// ErrInvalidKey reports a missing or oversized limiter key.
	ErrInvalidKey = errors.New("invalid rate limit key")
	// ErrCapacity reports a full local limiter with no expired key to evict.
	ErrCapacity = errors.New("rate limit capacity reached")
)

// Policy limits one key to Limit requests per fixed Window. Fixed windows are
// process-local and should not be used as a cross-instance security boundary.
type Policy struct {
	Limit  int
	Window time.Duration
}

// Memory is a bounded process-local fixed-window limiter. Use a shared adapter
// in multi-instance deployments.
type Memory struct {
	mu      sync.Mutex
	policy  Policy
	maxKeys int
	entries map[string]entry
}

type entry struct {
	count   int
	started time.Time
}

// NewMemory creates a validated local limiter.
func NewMemory(policy Policy, maxKeys int) (*Memory, error) {
	if policy.Window == 0 {
		policy.Window = DefaultWindow
	}
	if policy.Limit < 1 || policy.Window < time.Millisecond || policy.Window > MaxWindow {
		return nil, ErrInvalidPolicy
	}
	if maxKeys == 0 {
		maxKeys = DefaultMaxKeys
	}
	if maxKeys < 1 || maxKeys > MaxKeys {
		return nil, ErrInvalidPolicy
	}
	return &Memory{policy: policy, maxKeys: maxKeys, entries: make(map[string]entry)}, nil
}

// Allow reports whether key can make another request at the current time.
func (m *Memory) Allow(ctx context.Context, key string) (bool, error) {
	if ctx == nil || m == nil {
		return false, ErrInvalidPolicy
	}
	if err := validKey(key); err != nil {
		return false, err
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	current, found := m.entries[key]
	if !found || now.Sub(current.started) >= m.policy.Window {
		if !found && len(m.entries) >= m.maxKeys {
			m.evictExpired(now)
			if len(m.entries) >= m.maxKeys {
				return false, ErrCapacity
			}
		}
		m.entries[key] = entry{count: 1, started: now}
		return true, nil
	}
	if current.count >= m.policy.Limit {
		return false, nil
	}
	current.count++
	m.entries[key] = current
	return true, nil
}

func (m *Memory) evictExpired(now time.Time) {
	for key, current := range m.entries {
		if now.Sub(current.started) >= m.policy.Window {
			delete(m.entries, key)
		}
	}
}

func validKey(key string) error {
	if len(key) == 0 || len(key) > maxKeyBytes {
		return ErrInvalidKey
	}
	return nil
}
