package features

import (
	"testing"
	"time"

	"github.com/stockyard-dev/stockyard/internal/provider"
)

func TestCacheKey(t *testing.T) {
	msgs1 := []provider.Message{{Role: "user", Content: "hello"}}
	msgs2 := []provider.Message{{Role: "user", Content: "hello"}}
	msgs3 := []provider.Message{{Role: "user", Content: "world"}}

	key1 := CacheKey("gpt-4o-mini", msgs1)
	key2 := CacheKey("gpt-4o-mini", msgs2)
	key3 := CacheKey("gpt-4o-mini", msgs3)
	key4 := CacheKey("gpt-4o", msgs1)

	// Same input = same key
	if key1 != key2 {
		t.Errorf("identical inputs produced different keys: %s != %s", key1, key2)
	}
	// Different content = different key
	if key1 == key3 {
		t.Error("different content produced same key")
	}
	// Different model = different key
	if key1 == key4 {
		t.Error("different model produced same key")
	}
	// Key is hex-encoded SHA256 = 64 chars
	if len(key1) != 64 {
		t.Errorf("key length = %d, want 64", len(key1))
	}
}

func TestCacheGetSet(t *testing.T) {
	c := NewCache(CacheConfig{
		Enabled:    true,
		TTL:        1 * time.Hour,
		MaxEntries: 100,
	})

	key := "test-key"
	entry := &CacheEntry{
		Response:  &provider.Response{ID: "cached-1"},
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}

	// Miss before set
	if got := c.Get(key); got != nil {
		t.Error("expected nil before set")
	}

	// Set and hit
	c.Set(key, entry)
	got := c.Get(key)
	if got == nil {
		t.Fatal("expected cache hit after set")
	}
	if got.Response.ID != "cached-1" {
		t.Errorf("got ID = %q, want %q", got.Response.ID, "cached-1")
	}

	// Clear and miss
	c.Clear()
	if got := c.Get(key); got != nil {
		t.Error("expected nil after clear")
	}
}

func TestCacheExpiration(t *testing.T) {
	c := NewCache(CacheConfig{Enabled: true, TTL: 1 * time.Hour, MaxEntries: 100})

	key := "expired"
	c.Set(key, &CacheEntry{
		Response:  &provider.Response{ID: "old"},
		CreatedAt: time.Now().Add(-2 * time.Hour),
		ExpiresAt: time.Now().Add(-1 * time.Hour), // Already expired
	})

	if got := c.Get(key); got != nil {
		t.Error("expected nil for expired entry")
	}
}

func TestCircuitBreaker(t *testing.T) {
	cb := NewCircuitBreaker(3, 100*time.Millisecond)

	// Starts closed
	if !cb.Allow() {
		t.Error("circuit breaker should allow initially")
	}
	if cb.State() != "closed" {
		t.Errorf("state = %q, want closed", cb.State())
	}

	// Record failures up to threshold
	cb.RecordFailure()
	cb.RecordFailure()
	if !cb.Allow() {
		t.Error("should still allow before threshold")
	}

	// Third failure trips the breaker
	cb.RecordFailure()
	if cb.Allow() {
		t.Error("should block after threshold")
	}
	if cb.State() != "open" {
		t.Errorf("state = %q, want open", cb.State())
	}

	// Wait for recovery timeout
	time.Sleep(150 * time.Millisecond)

	// Should transition to half-open and allow one request
	if !cb.Allow() {
		t.Error("should allow after recovery timeout (half-open)")
	}

	// Success resets to closed
	cb.RecordSuccess()
	if cb.State() != "closed" {
		t.Errorf("state = %q, want closed after success", cb.State())
	}
}

func TestRateLimiter(t *testing.T) {
	limiter := NewRateLimiter(RateLimitConfig{
		Enabled:           true,
		RequestsPerMinute: 60,
		Burst:             3,
	})

	// Should allow burst
	for i := 0; i < 3; i++ {
		if !limiter.Allow("test-key") {
			t.Errorf("request %d should be allowed (within burst)", i+1)
		}
	}

	// Fourth request should be blocked (burst exhausted, not enough time to refill)
	if limiter.Allow("test-key") {
		t.Error("request 4 should be blocked (burst exhausted)")
	}

	// Different key should be allowed
	if !limiter.Allow("other-key") {
		t.Error("different key should be allowed")
	}
}
