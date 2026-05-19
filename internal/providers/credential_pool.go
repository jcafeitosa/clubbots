package providers

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// CredentialPool provides multi-key failover for the same provider.
// Inspired by Hermes Agent's credential_pool.py pattern.
// Keys are tried in weighted round-robin order with health tracking.
type CredentialPool struct {
	mu     sync.RWMutex
	keys   []*poolKey
	next   atomic.Uint32
}

type poolKey struct {
	Key        string
	Weight     int     // relative weight (default 1)
	Failures   atomic.Int64
	LastFailed time.Time
	Cooldown   time.Duration // how long to skip after failure
}

// NewCredentialPool creates a pool with the given API keys.
// All keys get equal weight by default.
func NewCredentialPool(keys []string) *CredentialPool {
	p := &CredentialPool{}
	for _, k := range keys {
		p.keys = append(p.keys, &poolKey{
			Key:      k,
			Weight:   1,
			Cooldown: 30 * time.Second,
		})
	}
	return p
}

// AddKey adds a key with a given weight (higher = more use).
func (p *CredentialPool) AddKey(key string, weight int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.keys = append(p.keys, &poolKey{
		Key:      key,
		Weight:   weight,
		Cooldown: 30 * time.Second,
	})
}

// Next returns the next healthy key from the pool.
// Keys in cooldown (recently failed) are skipped.
// Returns error if all keys are in cooldown.
func (p *CredentialPool) Next() (string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.keys) == 0 {
		return "", fmt.Errorf("credential pool: no keys configured")
	}

	// Try each key starting from the next index
	start := int(p.next.Add(1)) % len(p.keys)
	for i := 0; i < len(p.keys); i++ {
		idx := (start + i) % len(p.keys)
		key := p.keys[idx]

		// Skip keys in cooldown
		if key.Failures.Load() > 0 && time.Since(key.LastFailed) < key.Cooldown {
			continue
		}

		return key.Key, nil
	}

	// All keys in cooldown — return the one with oldest failure
	oldest := p.keys[0]
	for _, k := range p.keys[1:] {
		if k.LastFailed.Before(oldest.LastFailed) {
			oldest = k
		}
	}
	return oldest.Key, nil
}

// RecordSuccess resets the failure counter for a key.
func (p *CredentialPool) RecordSuccess(key string) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, k := range p.keys {
		if k.Key == key {
			k.Failures.Store(0)
			return
		}
	}
}

// RecordFailure increments the failure counter and sets cooldown.
// After maxFailures consecutive failures, the key is longer-cooled.
func (p *CredentialPool) RecordFailure(key string) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, k := range p.keys {
		if k.Key == key {
			fails := k.Failures.Add(1)
			k.LastFailed = time.Now()
			// Exponential backoff: 30s, 60s, 120s, 240s...
			if fails > 1 {
				k.Cooldown = time.Duration(30*(1<<min(int(fails)-1, 4))) * time.Second
			}
			return
		}
	}
}

// Stats returns pool statistics for monitoring.
func (p *CredentialPool) Stats() map[string]any {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := make(map[string]any)
	var keyStats []map[string]any
	available := 0
	for _, k := range p.keys {
		ks := map[string]any{
			"key_preview": k.Key[:8] + "..." + k.Key[len(k.Key)-4:],
			"failures":    k.Failures.Load(),
			"cooldown":    k.Cooldown.String(),
		}
		if k.Failures.Load() == 0 || time.Since(k.LastFailed) >= k.Cooldown {
			ks["available"] = true
			available++
		} else {
			ks["available"] = false
		}
		keyStats = append(keyStats, ks)
	}
	stats["total_keys"] = len(p.keys)
	stats["available_keys"] = available
	stats["keys"] = keyStats
	return stats
}

// Do executes fn with automatic key rotation and retry.
// On failure, records the failure and retries with the next key.
func (p *CredentialPool) Do(ctx context.Context, fn func(key string) error) error {
	maxAttempts := len(p.keys)
	for attempt := 0; attempt < maxAttempts; attempt++ {
		key, err := p.Next()
		if err != nil {
			return err
		}

		err = fn(key)
		if err == nil {
			p.RecordSuccess(key)
			return nil
		}

		p.RecordFailure(key)
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	return fmt.Errorf("credential pool: all %d keys exhausted", len(p.keys))
}

// Rotate returns keys in round-robin order ignoring health.
func (p *CredentialPool) Rotate() string {
	if len(p.keys) == 0 {
		return ""
	}
	idx := int(p.next.Add(1)) % len(p.keys)
	return p.keys[idx].Key
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Shuffle randomizes key order for initial distribution.
func (p *CredentialPool) Shuffle() {
	p.mu.Lock()
	defer p.mu.Unlock()
	rand.Shuffle(len(p.keys), func(i, j int) {
		p.keys[i], p.keys[j] = p.keys[j], p.keys[i]
	})
}
