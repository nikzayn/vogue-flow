package pinecone

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/nikzayn/vogueflow/internal/models"
)

// Pinecone Client Setup
type Client struct {
	apiKey     string
	indexHost  string
	httpClient *http.Client
}

func NewClient(apiKey, indexHost string) *Client {
	return &Client{
		apiKey:    apiKey,
		indexHost: indexHost,
		httpClient: &http.Client{
			Timeout: 50 * time.Millisecond,
			Transport: &http.Transport{
				MaxIdleConns:        500,
				MaxIdleConnsPerHost: 500,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

// Pinecode request payload
type queryRequest struct {
	Vector          []float32              `json:"vector"`
	TopK            int                    `json:"topK"`
	Filter          map[string]interface{} `json:"filter,omitempty"`
	IncludeMetadata bool                   `json:"includeMetadata"`
}

type queryResponse struct {
	Matches []struct {
		ID       string                 `json:"id"`
		Score    float64                `json:"score"`
		Metadata map[string]interface{} `json:"metadata"`
	} `json:"matches"`
}

// Query performs a hybrid semantic + metadata filtered search
func (c *Client) Query(ctx context.Context, embedding []float32, filter map[string]interface{}, topK int) ([]models.Product, error) {
	if topK == 0 {
		topK = 20
	}

	reqBody := queryRequest{
		Vector:          embedding,
		TopK:            topK,
		Filter:          filter,
		IncludeMetadata: true,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal query: %w", err)
	}

	url := fmt.Sprintf("%s/query", c.indexHost)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Api-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Pinecone-API-Version", "2024-07")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pinecone request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("pinecone status %d: %s", resp.StatusCode, string(body))
	}

	var qr queryResponse
	if err := json.NewDecoder(resp.Body).Decode(&qr); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	products := make([]models.Product, 0, len(qr.Matches))
	for _, m := range qr.Matches {
		p := models.Product{
			ID:    m.ID,
			Score: m.Score,
		}
		// Extract metadata fields safely
		if v, ok := m.Metadata["name"].(string); ok {
			p.Name = v
		}
		if v, ok := m.Metadata["description"].(string); ok {
			p.Description = v
		}
		if v, ok := m.Metadata["category"].(string); ok {
			p.Category = v
		}
		if v, ok := m.Metadata["price"].(float64); ok {
			p.Price = v
		}
		if v, ok := m.Metadata["in_stock"].(bool); ok {
			p.InStock = v
		}
		products = append(products, p)
	}

	return products, nil
}
