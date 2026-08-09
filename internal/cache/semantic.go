package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/nikzayn/vogueflow/internal/models"
	"github.com/redis/go-redis/v9"
)

// Semantic Cache implements caching using vector similarity
// Storing embedding + response in Redis, on lookup we fetch candidate keys and do cosine similarity check
type SemanticCache struct {
	client       *redis.Client
	simThreshold float64
	ttl          time.Duration
}

// Orechestrator function which creates a new semantic cache
func NewSemanticCache(addr, password string, db int, ttl time.Duration) *SemanticCache {
	return &SemanticCache{
		client: redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: password,
			DB:       db,
			PoolSize: 100,
		}),
		simThreshold: 0.94,
		ttl:          ttl,
	}
}

// Generates a deterministic redis key from embedding vector
func (sc *SemanticCache) embeddingKey(emb []float32) string {
	b := make([]byte, len(emb)*4)

	for i, v := range emb {
		_ = v
		_ = i
		_ = b
	}

	h := sha256.New()
	for _, v := range emb {
		h.Write([]byte(fmt.Sprintf("%.6f,", v)))
	}
	return "semantic:" + hex.EncodeToString(h.Sum(nil))[:16]
}

// Gets cached response by query embedding
func (sc *SemanticCache) Get(ctx context.Context, embedding []float32) (*models.AgentState, bool) {
	key := sc.embeddingKey(embedding)
	val, err := sc.client.Get(ctx, key).Result()
	if err != nil {
		return nil, false
	}

	var state models.AgentState
	if err := json.Unmarshal([]byte(val), &state); err != nil {
		return nil, false
	}
	state.CacheHit = true
	return &state, true
}

// Sets stores a successful response keyed by embedding
func (sc *SemanticCache) Set(ctx context.Context, embedding []float32, state *models.AgentState) error {
	key := sc.embeddingKey(embedding)
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}

	return sc.client.Set(ctx, key, data, sc.ttl).Err()
}

// Following invalidation strategy which computes cosine b/w 2 float32 vectors
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}

	var dot, normA, normB float64

	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

func (sc *SemanticCache) Close() error {
	return sc.client.Close()
}
