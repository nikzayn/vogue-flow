package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/nikzayn/vogueflow/internal/models"
	"github.com/redis/go-redis/v9"
)

// SemanticCache returns a previous answer when a new query means (nearly) the same thing.
// Entries are bucketed by the structured filters (size, budget, occasion) so a cached
// answer is never served for a different filter set; within a bucket, the most recent
// entries are scanned and the best cosine match above simThreshold wins.
type SemanticCache struct {
	client       *redis.Client
	simThreshold float64
	maxEntries   int64
	ttl          time.Duration
}

type semanticEntry struct {
	Embedding []float32         `json:"embedding"`
	State     models.AgentState `json:"state"`
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
		maxEntries:   200,
		ttl:          ttl,
	}
}

// bucketKey scopes cache entries to the query's structured filters
func (sc *SemanticCache) bucketKey(q models.Query) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s|%.2f|%s", strings.ToLower(q.Size), q.Budget, strings.ToLower(q.Occasion))
	return "semantic:" + hex.EncodeToString(h.Sum(nil))[:16]
}

// Get returns the most similar cached response in the query's bucket, if any clears the threshold
func (sc *SemanticCache) Get(ctx context.Context, q models.Query) (*models.AgentState, bool) {
	if len(q.Embedding) == 0 {
		return nil, false
	}

	raw, err := sc.client.LRange(ctx, sc.bucketKey(q), 0, sc.maxEntries-1).Result()
	if err != nil {
		return nil, false
	}

	var best *models.AgentState
	bestSim := sc.simThreshold
	for _, item := range raw {
		var entry semanticEntry
		if err := json.Unmarshal([]byte(item), &entry); err != nil {
			continue
		}
		if sim := cosineSimilarity(q.Embedding, entry.Embedding); sim >= bestSim {
			bestSim = sim
			state := entry.State
			best = &state
		}
	}
	if best == nil {
		return nil, false
	}
	best.CacheHit = true
	return best, true
}

// Set stores a successful response alongside its query embedding
func (sc *SemanticCache) Set(ctx context.Context, q models.Query, state *models.AgentState) error {
	if len(q.Embedding) == 0 {
		return nil
	}

	data, err := json.Marshal(semanticEntry{Embedding: q.Embedding, State: *state})
	if err != nil {
		return err
	}

	key := sc.bucketKey(q)
	pipe := sc.client.TxPipeline()
	pipe.LPush(ctx, key, data)
	pipe.LTrim(ctx, key, 0, sc.maxEntries-1)
	pipe.Expire(ctx, key, sc.ttl)
	_, err = pipe.Exec(ctx)
	return err
}

// cosineSimilarity computes cosine similarity between 2 float32 vectors
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
