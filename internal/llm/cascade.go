package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/nikzayn/vogueflow/internal/models"
)

// Cascade implements a tiered LLM strategy:
// Tier 1: Fast local model (Llama-3.1-8B via vLLM) for simple queries
// Tier 2: Claude for complex reasoning and agentic workflows
// Tier 3: Claude-3.5-Sonnet for high-value multi-step tasks
type Cascade struct {
	tier1 models.LLMClient // Local
	tier2 models.LLMClient // Claude (standard)
	tier3 models.LLMClient // Claude (high-quality)
}

// NewCascade creates a model cascade
func NewCascade(tier1, tier2, tier3 models.LLMClient) *Cascade {
	return &Cascade{tier1: tier1, tier2: tier2, tier3: tier3}
}

// ClassifyTier determines which model tier to use based on query complexity
func (c *Cascade) ClassifyTier(query string, intent string) int {
	lower := strings.ToLower(query)

	// Tier 1: Simple factual lookups, greetings, inventory checks
	simplePatterns := []string{"in stock", "price", "color", "size", "hello", "hi "}
	for _, p := range simplePatterns {
		if strings.Contains(lower, p) {
			return 1
		}
	}

	// Tier 3: Complex styling, outfit building, emotional/sentiment heavy
	complexPatterns := []string{"outfit", "wedding", "date night", "flattering", "broad shoulders", "stylist"}
	for _, p := range complexPatterns {
		if strings.Contains(lower, p) {
			return 3
		}
	}

	// Default to Tier 2 for sizing, recommendations, comparisons
	return 2
}

// Complete routes to the appropriate tier and returns (text, tokensUsed, tier, error)
func (c *Cascade) Complete(ctx context.Context, system, prompt string, tier int) (string, int, int, error) {
	switch tier {
	case 1:
		text, tokens, err := c.tier1.Complete(ctx, system, prompt)
		return text, tokens, 1, err
	case 3:
		text, tokens, err := c.tier3.Complete(ctx, system, prompt)
		return text, tokens, 3, err
	default:
		text, tokens, err := c.tier2.Complete(ctx, system, prompt)
		return text, tokens, 2, err
	}
}

// Stream routes streaming to the appropriate tier
func (c *Cascade) Stream(ctx context.Context, system, prompt string, tier int, ch chan<- string) error {
	switch tier {
	case 1:
		return c.tier1.Stream(ctx, system, prompt, ch)
	case 3:
		return c.tier3.Stream(ctx, system, prompt, ch)
	default:
		return c.tier2.Stream(ctx, system, prompt, ch)
	}
}

// MockTier1 is a placeholder for a local Llama model. In production, replace with vLLM client.
type MockTier1 struct{}

func (m *MockTier1) Complete(ctx context.Context, system, prompt string) (string, int, error) {
	return fmt.Sprintf("[Tier1] Based on your query, here are the top matches."), 15, nil
}
func (m *MockTier1) Stream(ctx context.Context, system, prompt string, ch chan<- string) error {
	defer close(ch)
	ch <- "[Tier1] Quick response from edge model."
	return nil
}
