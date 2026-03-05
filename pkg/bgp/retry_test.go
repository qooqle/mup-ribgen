package bgp

// Task 8.4: Error handling unit tests for GoBGP retry (req 9.5).

import (
	"context"
	"errors"
	"testing"
	"time"
)

var errSimulated = errors.New("simulated gRPC error")

// TestWithRetry_SucceedsImmediately verifies that a successful fn is called exactly once.
func TestWithRetry_SucceedsImmediately(t *testing.T) {
	c := &Client{cfg: Config{MaxRetries: 3}}
	calls := 0
	err := c.withRetry(context.Background(), func() error {
		calls++
		return nil
	})
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

// TestWithRetry_SucceedsOnRetry verifies that fn is retried after a transient error.
// Uses context cancellation to cut short the inter-retry backoff.
func TestWithRetry_SucceedsOnRetry(t *testing.T) {
	c := &Client{cfg: Config{MaxRetries: 3}}
	calls := 0
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := c.withRetry(ctx, func() error {
		calls++
		if calls == 1 {
			return errSimulated // fail first call
		}
		return nil
	})
	if err != nil {
		t.Errorf("expected success on retry, got %v", err)
	}
	if calls < 2 {
		t.Errorf("expected at least 2 calls, got %d", calls)
	}
}

// TestWithRetry_ExhaustsRetries verifies that error is returned after MaxRetries (req 9.5).
func TestWithRetry_ExhaustsRetries(t *testing.T) {
	// MaxRetries=0: exactly 1 attempt, then fail immediately (no backoff).
	c := &Client{cfg: Config{MaxRetries: 0}}
	calls := 0
	err := c.withRetry(context.Background(), func() error {
		calls++
		return errSimulated
	})
	if err == nil {
		t.Error("expected error after retry exhaustion")
	}
	if calls != 1 {
		t.Errorf("expected 1 call (MaxRetries=0), got %d", calls)
	}
	if !errors.Is(err, errSimulated) {
		t.Errorf("expected sentinel error in chain, got %v", err)
	}
}

// TestWithRetry_ContextCancelled verifies that context cancellation stops retries (req 9.5).
func TestWithRetry_ContextCancelled(t *testing.T) {
	c := &Client{cfg: Config{MaxRetries: 10}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := c.withRetry(ctx, func() error {
		return errSimulated
	})
	if err == nil {
		t.Error("expected error on cancelled context")
	}
}
