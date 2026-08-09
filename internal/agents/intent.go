package agents

import (
	"context"
	"strings"

	"github.com/nikzayn/vogueflow/internal/models"
)

// IntentAgent classifies user queries
type IntentAgent struct {
	llm models.LLMClient
}

// NewIntentAgent creates an intent classifier
func NewIntentAgent(llm models.LLMClient) *IntentAgent {
	return &IntentAgent{llm: llm}
}

// Intent represents classified user goals
type Intent string

const (
	IntentBrowse      Intent = "browse"
	IntentDiscovery   Intent = "discovery"
	IntentSizing      Intent = "sizing"
	IntentStyling     Intent = "styling"
	IntentTransaction Intent = "transaction"
	IntentEscalation  Intent = "escalation"
	IntentGreeting    Intent = "greeting"
)

// Classify uses a fast heuristic + optional LLM fallback
func (ia *IntentAgent) Classify(ctx context.Context, query string) (Intent, error) {
	lower := strings.ToLower(query)

	switch {
	case containsAny(lower, "hello", "hi", "hey"):
		return IntentGreeting, nil
	case containsAny(lower, "size", "fit", "measurement", "bust", "waist"):
		return IntentSizing, nil
	case containsAny(lower, "cart", "checkout", "buy", "purchase", "bag", "payment"):
		return IntentTransaction, nil
	case containsAny(lower, "outfit", "style", "look", "occasion", "wedding", "date"):
		return IntentStyling, nil
	case containsAny(lower, "angry", "frustrated", "human", "representative", "help"):
		return IntentEscalation, nil
	case containsAny(lower, "find", "looking for", "need", "recommend", "suggest"):
		return IntentDiscovery, nil
	}

	return IntentDiscovery, nil
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
