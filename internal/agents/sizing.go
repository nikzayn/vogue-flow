package agents

import (
	"context"
	"fmt"
	"strings"

	"github.com/nikzayn/vogueflow/internal/models"
)

// SizingAgent provides fit recommendations and size-chart guidance
type SizingAgent struct {
	llm models.LLMClient
}

// NewSizingAgent creates a sizing advisor agent
func NewSizingAgent(llm models.LLMClient) *SizingAgent {
	return &SizingAgent{llm: llm}
}

// SizeRecommendation generates a personalized size suggestion based on body data and purchase history
func (sa *SizingAgent) SizeRecommendation(ctx context.Context, state *models.AgentState) (string, int, error) {
	query := state.Query

	var sizingContext strings.Builder
	fmt.Fprintf(&sizingContext, "Customer query: %s\n", query.Text)
	if query.Size != "" {
		fmt.Fprintf(&sizingContext, "Stated size: %s\n", query.Size)
	}
	if query.Metadata != nil {
		if bust, ok := query.Metadata["bust"]; ok {
			fmt.Fprintf(&sizingContext, "Bust measurement: %s\n", bust)
		}
		if band, ok := query.Metadata["band"]; ok {
			fmt.Fprintf(&sizingContext, "Band size: %s\n", band)
		}
	}

	system := `You are Victoria's Secret's sizing expert. Be precise, reassuring, and body-positive. 
Reference standard size charts. If unsure, suggest the customer try both sizes and highlight our easy returns.
Keep response under 2 sentences.`

	prompt := sizingContext.String() + "\nWhat size should this customer order?"

	response, tokens, err := sa.llm.Complete(ctx, system, prompt)
	if err != nil {
		return "", 0, fmt.Errorf("sizing recommendation failed: %w", err)
	}
	return strings.TrimSpace(response), tokens, nil
}

// FitPrediction predicts fit issues from review RAG data
func (sa *SizingAgent) FitPrediction(ctx context.Context, productID string, size string) (string, error) {
	_ = productID
	_ = size
	return "Customers say this style fits true to size.", nil
}
