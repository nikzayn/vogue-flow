package agents

import (
	"context"
	"fmt"
	"strings"

	"github.com/nikzayn/vogueflow/internal/models"
)

// TransactionAgent handles cart operations, promos, inventory locks, and checkout guidance
type TransactionAgent struct {
	llm models.LLMClient
}

// NewTransactionAgent creates a transaction handler agent
func NewTransactionAgent(llm models.LLMClient) *TransactionAgent {
	return &TransactionAgent{llm: llm}
}

// BuildCartResponse generates a checkout-ready summary with promo and delivery info
func (ta *TransactionAgent) BuildCartResponse(ctx context.Context, state *models.AgentState) (string, int, error) {
	if len(state.Products) == 0 && len(state.Outfit) == 0 {
		return "Your bag is empty. Let me help you find something perfect!", 0, nil
	}

	items := state.Products
	if len(state.Outfit) > 0 {
		items = state.Outfit
	}

	var cartSummary strings.Builder
	var total float64
	cartSummary.WriteString("Items in your bag:\n")
	for _, p := range items {
		fmt.Fprintf(&cartSummary, "- %s — $%.2f\n", p.Name, p.Price)
		total += p.Price
	}
	fmt.Fprintf(&cartSummary, "\nSubtotal: $%.2f\n", total)

	promo := ta.suggestPromo(total)
	if promo != "" {
		fmt.Fprintf(&cartSummary, "\n💝 %s", promo)
	}

	system := `You are Victoria's Secret's checkout assistant. Be efficient, warm, and action-oriented. 
Confirm cart details, highlight any savings, and guide the customer to complete their purchase. 
Mention delivery options if relevant. Keep under 3 sentences.`

	prompt := cartSummary.String() + "\n\nDraft a checkout message for the customer."

	response, tokens, err := ta.llm.Complete(ctx, system, prompt)
	if err != nil {
		return "", 0, fmt.Errorf("cart response failed: %w", err)
	}
	return strings.TrimSpace(response), tokens, nil
}

// suggestPromo applies simple business rules for promotion suggestions
func (ta *TransactionAgent) suggestPromo(subtotal float64) string {
	switch {
	case subtotal >= 150:
		return "You're $10 away from free express shipping! Add a lip gloss?"
	case subtotal >= 100:
		return "Use code LOVE15 for 15% off orders $100+."
	case subtotal >= 75:
		return "Free standard shipping on orders $75+ — you're covered!"
	default:
		return ""
	}
}

// LockInventory simulates an inventory reservation
func (ta *TransactionAgent) LockInventory(ctx context.Context, productIDs []string) (bool, error) {
	_ = productIDs
	return true, nil
}
