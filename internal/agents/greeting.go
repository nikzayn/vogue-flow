package agents

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nikzayn/vogueflow/internal/models"
)

// GreetingAgent handles welcome messages, small talk, and session warm-up.
type GreetingAgent struct {
	llm models.LLMClient
}

// NewGreetingAgent creates a greeting handler
func NewGreetingAgent(llm models.LLMClient) *GreetingAgent {
	return &GreetingAgent{llm: llm}
}

// Greet generates a dynamic, personalized welcome message.
func (ga *GreetingAgent) Greet(ctx context.Context, userID string, query string) (string, int, error) {
	lower := strings.ToLower(strings.TrimSpace(query))
	simpleGreetings := map[string]string{
		"hi":        "Hey gorgeous! 💕 I'm your personal VS stylist. What are you shopping for today?",
		"hello":     "Hello beautiful! ✨ Ready to find something that makes you feel amazing?",
		"hey":       "Hey there! 💋 Tell me what you're in the mood for — lingerie, beauty, or a full outfit?",
		"help":      "I'm here to help! 💕 Looking for sizing advice, outfit ideas, or something specific?",
		"thanks":    "You're so welcome, babe! 💖 Let me know if you need anything else.",
		"thank you": "My pleasure! 🌸 Happy to help anytime.",
	}
	if resp, ok := simpleGreetings[lower]; ok {
		return resp, 0, nil // 0 tokens = no LLM call
	}

	timeOfDay := ga.timeOfDayGreeting()
	system := `You are Victoria's Secret's warm, flirty, and empowering AI stylist. 
Greet the customer naturally. Mention the time of day if relevant. 
Ask what they're shopping for. Keep to 1 sentence. Never be robotic.`

	prompt := fmt.Sprintf("%s The customer said: '%s'. Respond with a warm, brand-aligned greeting.", timeOfDay, query)

	response, tokens, err := ga.llm.Complete(ctx, system, prompt)
	if err != nil {
		// Fallback to static so we never fail on a greeting
		return "Hey gorgeous! 💕 I'm your personal VS stylist. What are you shopping for today?", 0, nil
	}
	return strings.TrimSpace(response), tokens, nil
}

// timeOfDayGreeting returns a contextual prefix based on current hour
func (ga *GreetingAgent) timeOfDayGreeting() string {
	hour := time.Now().Hour()
	switch {
	case hour < 12:
		return "It's morning."
	case hour < 17:
		return "It's afternoon."
	case hour < 21:
		return "It's evening."
	default:
		return "It's late."
	}
}
