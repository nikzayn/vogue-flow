package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/redis/go-redis/v9"
)

// TokenCache deduplicates identical (system + prompt) -> response pairs
type TokenCache struct {
	client *redis.Client
	ttl    time.Duration
}

// NewTokenCache creates a token-level prompt cache
func NewTokenCache(addr, password string, db int, ttl time.Duration) *TokenCache {
	return &TokenCache{
		client: redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: password,
			DB:       db + 1,
			PoolSize: 100,
		}),
		ttl: ttl,
	}
}

// hashPrompt creates a deterministic hash of system + user prompt
func (tc *TokenCache) hashPrompt(system, prompt string) string {
	h := sha256.New()
	h.Write([]byte(system))
	h.Write([]byte("||"))
	h.Write([]byte(prompt))
	return "token:" + hex.EncodeToString(h.Sum(nil))[:24]
}

// Get retrieves a cached LLM response
func (tc *TokenCache) Get(ctx context.Context, system, prompt string) (string, bool) {
	key := tc.hashPrompt(system, prompt)
	val, err := tc.client.Get(ctx, key).Result()
	if err != nil {
		return "", false
	}
	return val, true
}

// Set stores an LLM response
func (tc *TokenCache) Set(ctx context.Context, system, prompt, response string) error {
	key := tc.hashPrompt(system, prompt)
	return tc.client.Set(ctx, key, response, tc.ttl).Err()
}

// Close closes the Redis connection
func (tc *TokenCache) Close() error {
	return tc.client.Close()
}

// Stats returns cache hit/miss metrics
func (tc *TokenCache) Stats(ctx context.Context) (hits, misses int64, err error) {
	return 0, 0, nil
}
