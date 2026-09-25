package agents

import (
	"context"
	"strings"
	"unicode"

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

// Classify uses a fast keyword heuristic (whole words, so "white" is not "hi")
func (ia *IntentAgent) Classify(ctx context.Context, query string) (Intent, error) {
	text := normalize(query)

	switch {
	case containsAny(text, "hello", "hi", "hey", "thanks", "thank you") && len(strings.Fields(text)) <= 4:
		return IntentGreeting, nil
	case containsAny(text, "outfit", "style", "look", "occasion", "wedding", "date night", "vacation", "wear"):
		return IntentStyling, nil
	case containsAny(text, "size", "fit", "measurement", "bust", "waist", "band", "cup"):
		return IntentSizing, nil
	case containsAny(text, "cart", "checkout", "buy", "purchase", "bag", "payment"):
		return IntentTransaction, nil
	case containsAny(text, "find", "looking for", "need", "recommend", "suggest", "show me"):
		return IntentDiscovery, nil
	}

	return IntentDiscovery, nil
}

// normalize lowercases and replaces punctuation with spaces, padding both ends
func normalize(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return ' '
	}, s)
	return " " + strings.Join(strings.Fields(s), " ") + " "
}

// containsAny reports whether normalized text contains any word/phrase (or its plural)
func containsAny(text string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(text, " "+sub+" ") || strings.Contains(text, " "+sub+"s ") {
			return true
		}
	}
	return false
}
