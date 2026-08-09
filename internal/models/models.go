package models

import "context"

// Represents user shopping intent
type Query struct {
	SessionID string            `json:"session_id"`
	UserID    string            `json:"user_id"`
	Text      string            `json:"text"`
	Embedding []float32         `json:"-"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Budget    float64           `json:"budget,omitempty"`
	Size      string            `json:"size,omitempty"`
	Occasion  string            `json:"ocassion,omitempty"`
}

// Product which actually represnets the catalog item in Pinecone
type Product struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Category    string            `json:"category"`
	Price       float64           `json:"price"`
	Colors      []string          `json:"colors"`
	Sizes       []string          `json:"sizes"`
	InStock     bool              `json:"in_stock"`
	Score       float64           `json:"score"`
	Metadata    map[string]string `json:"metadata"`
}

// AgentState is a state object passed through langgraph as a part of workflows
type AgentState struct {
	Query      Query     `json:"query"`
	Intent     string    `json:"intent"`
	Products   []Product `json:"product"`
	Outfit     []Product `json:"outfit"`
	Response   string    `json:"response"`
	TokensUsed int       `json:"tokens_used"`
	LatencyMs  int64     `json:"latency_ms"`
	CacheHit   bool      `json:"cache_hit"`
	ModelTier  int       `json:"model_tier"`
	Completed  bool      `json:"completed"`
}

// An interface for the LLMClient for various kinds of LLMs
type LLMClient interface {
	Complete(ctx context.Context, prompt, system string) (string, int, error)
	Stream(ctx context.Context, prompt, system string, ch chan<- string) error
}

// Vector Store defines the interface for vector database operations
type VectorStore interface {
	Query(ctx context.Context, embedding []float32, filter map[string]interface{}, topK int) ([]Product, error)
}

// Cache defines generic caching ops
type Cache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string, ttlSeconds int) error
}
