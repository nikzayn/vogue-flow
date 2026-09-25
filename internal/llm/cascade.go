package llm

import (
	"context"
	"strings"

	"github.com/nikzayn/vogueflow/internal/models"
)

// Cascade implements a tiered LLM strategy:
// Tier 1: Claude Haiku — fast, cheap, for simple lookups and greetings
// Tier 2: Claude Sonnet — standard reasoning for sizing, recommendations
// Tier 3: Claude Sonnet (higher max_tokens) — complex styling, outfit building
//
// All tiers use real Claude models. Tier 1 uses a lighter model for cost/speed.
type Cascade struct {
	tier1 models.LLMClient // Claude Haiku (fast/cheap)
	tier2 models.LLMClient // Claude Sonnet (standard)
	tier3 models.LLMClient // Claude Sonnet (premium config)
}

// NewCascade creates a model cascade with real Claude clients for all tiers.
func NewCascade(tier1, tier2, tier3 models.LLMClient) *Cascade {
	return &Cascade{tier1: tier1, tier2: tier2, tier3: tier3}
}

// ClassifyTier determines which model tier to use based on query complexity.
// Order matters: check Tier 3 (complex) first, then Tier 1 (simple), default to Tier 2.
func (c *Cascade) ClassifyTier(query string, intent string) int {
	lower := strings.ToLower(query)

	// Tier 3: Complex styling, outfit building, emotional/sentiment heavy, multi-step
	complexPatterns := []string{
		"outfit", "wedding", "date night", "flattering", "broad shoulders",
		"stylist", "put together", "what should i wear", "help me choose",
	}
	for _, p := range complexPatterns {
		if strings.Contains(lower, p) {
			return 3
		}
	}

	// Tier 1: Ultra-simple factual lookups, greetings, one-word inventory checks
	simplePatterns := []string{
		"hello", "hi ", "hey ", "in stock", "price of ", "how much",
		"what color", "available", "shipping cost",
	}
	for _, p := range simplePatterns {
		if strings.Contains(lower, p) {
			return 1
		}
	}

	// Intent-based overrides: sizing and transaction need reasoning (Tier 2)
	switch intent {
	case "sizing", "transaction":
		return 2
	}

	// Default to Tier 2 for sizing, recommendations, comparisons, and everything else
	return 2
}

// Complete implements models.LLMClient — auto-classifies tier and delegates
func (c *Cascade) Complete(ctx context.Context, system, prompt string) (string, int, error) {
	tier := c.ClassifyTier(prompt, "")
	text, tokens, _, err := c.CompleteWithTier(ctx, system, prompt, tier)
	return text, tokens, err
}

// Stream implements models.LLMClient — auto-classifies tier and delegates
func (c *Cascade) Stream(ctx context.Context, system, prompt string, ch chan<- string) error {
	tier := c.ClassifyTier(prompt, "")
	return c.StreamWithTier(ctx, system, prompt, tier, ch)
}

// CompleteWithTier routes to the appropriate tier and returns (text, tokensUsed, tier, error)
func (c *Cascade) CompleteWithTier(ctx context.Context, system, prompt string, tier int) (string, int, int, error) {
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

// StreamWithTier routes streaming to the appropriate tier
func (c *Cascade) StreamWithTier(ctx context.Context, system, prompt string, tier int, ch chan<- string) error {
	switch tier {
	case 1:
		return c.tier1.Stream(ctx, system, prompt, ch)
	case 3:
		return c.tier3.Stream(ctx, system, prompt, ch)
	default:
		return c.tier2.Stream(ctx, system, prompt, ch)
	}
}
