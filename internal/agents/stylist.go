package agents

import (
	"context"
	"fmt"
	"strings"

	"github.com/nikzayn/vogueflow/internal/models"
)

// StylistAgent assembles outfits and generates personalized styling advice
type StylistAgent struct {
	llm models.LLMClient
}

// NewStylistAgent creates a stylist agent
func NewStylistAgent(llm models.LLMClient) *StylistAgent {
	return &StylistAgent{llm: llm}
}

// BuildOutfit creates complementary product combinations from discovered items
func (sa *StylistAgent) BuildOutfit(ctx context.Context, state *models.AgentState) ([]models.Product, string, error) {
	if len(state.Products) < 2 {
		return state.Products, "", nil
	}

	// Simple rule-based outfit building
	outfit := sa.ruleBasedAssembly(state.Products, state.Query)

	// Generate styling tip via LLM (cached via token cache)
	system := `You are a Victoria's Secret stylist. Write a 1-sentence styling tip. Be concise, sexy, and brand-aligned.`
	prompt := fmt.Sprintf("Occasion: %s. Products: %s. Give one styling tip.",
		state.Query.Occasion, productNames(outfit))

	tip, tokens, err := sa.llm.Complete(ctx, system, prompt)
	if err != nil {
		return outfit, "", err
	}
	state.TokensUsed += tokens

	return outfit, strings.TrimSpace(tip), nil
}

// ruleBasedAssembly creates outfits by category complementarity
func (sa *StylistAgent) ruleBasedAssembly(products []models.Product, query models.Query) []models.Product {
	// Pick one item from each major category to build a cohesive look
	categories := map[string]bool{}
	var outfit []models.Product
	for _, p := range products {
		cat := strings.ToLower(p.Category)
		if !categories[cat] {
			outfit = append(outfit, p)
			categories[cat] = true
		}
		if len(outfit) >= 3 {
			break
		}
	}
	return outfit
}

func productNames(products []models.Product) string {
	names := make([]string, len(products))
	for i, p := range products {
		names[i] = p.Name
	}
	return strings.Join(names, ", ")
}
