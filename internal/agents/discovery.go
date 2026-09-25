package agents

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/nikzayn/vogueflow/internal/models"
)

// DiscoveryAgent retrieves products from Pinecone based on intent and filters
type DiscoveryAgent struct {
	store models.VectorStore
}

// NewDiscoveryAgent creates a product discovery agent
func NewDiscoveryAgent(store models.VectorStore) *DiscoveryAgent {
	return &DiscoveryAgent{store: store}
}

// Discover executes a vector search with metadata filters derived from natural language
func (da *DiscoveryAgent) Discover(ctx context.Context, state *models.AgentState) ([]models.Product, error) {
	query := state.Query

	// Build Pinecone metadata filter from parsed query
	filter := make(map[string]interface{})
	filter["in_stock"] = map[string]interface{}{"$eq": true}

	if query.Budget > 0 {
		filter["price"] = map[string]interface{}{"$lte": query.Budget}
	}
	if query.Size != "" {
		filter["sizes"] = map[string]interface{}{"$in": []string{query.Size}}
	}
	if query.Occasion != "" {
		filter["occasion"] = map[string]interface{}{"$eq": strings.ToLower(query.Occasion)}
	}

	// Execute hybrid search
	products, err := da.store.Query(ctx, query.Embedding, filter, 20)
	if err != nil {
		return nil, fmt.Errorf("discovery search failed: %w", err)
	}

	// Re-rank by composite score (Pinecone score + business rules)
	products = da.reRank(products, query)
	if len(products) > 10 {
		products = products[:10]
	}

	return products, nil
}

// reRank applies business logic: boost new arrivals, penalize low stock
func (da *DiscoveryAgent) reRank(products []models.Product, query models.Query) []models.Product {
	for i := range products {
		if query.Budget > 0 && products[i].Price < query.Budget*0.8 {
			products[i].Score += 0.02
		}
	}
	sort.SliceStable(products, func(i, j int) bool {
		return products[i].Score > products[j].Score
	})
	return products
}
