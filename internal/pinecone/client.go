package pinecone

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/nikzayn/vogueflow/internal/models"
)

const (
	expectedDimension = 1024

	// Keep this aligned with the Pinecone API version you are using.
	pineconeAPIVersion = "2025-04"
)

// Client wraps the Pinecone REST API.
type Client struct {
	apiKey     string
	indexHost  string
	embedModel string
	httpClient *http.Client
}

// NewClient creates a Pinecone client.
func NewClient(apiKey, indexHost, embedModel string) *Client {
	apiKey = strings.TrimSpace(apiKey)
	indexHost = strings.TrimSpace(indexHost)

	if apiKey == "" {
		panic("PINECONE_API_KEY is required but was empty")
	}

	if indexHost == "" {
		panic("PINECONE_INDEX_HOST is required but was empty")
	}

	// Add https:// if the user supplied only the host.
	if !strings.HasPrefix(indexHost, "http://") &&
		!strings.HasPrefix(indexHost, "https://") {
		indexHost = "https://" + indexHost
	}

	// Remove trailing slash.
	indexHost = strings.TrimRight(indexHost, "/")

	return &Client{
		apiKey:     apiKey,
		indexHost:  indexHost,
		embedModel: embedModel,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        500,
				MaxIdleConnsPerHost: 500,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

// setHeaders applies the headers required by Pinecone.
func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Api-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Pinecone-Api-Version", pineconeAPIVersion)
}

// keyPrefix returns a safe identifier for debugging.
// Never log the complete API key.
func (c *Client) keyPrefix() string {
	if len(c.apiKey) <= 8 {
		return "****"
	}

	return c.apiKey[:8]
}

// queryRequest represents a Pinecone query request.
type queryRequest struct {
	Vector          []float32              `json:"vector"`
	TopK            int                    `json:"topK"`
	Filter          map[string]interface{} `json:"filter,omitempty"`
	IncludeMetadata bool                   `json:"includeMetadata"`
}

// queryResponse represents a Pinecone query response.
type queryResponse struct {
	Matches []struct {
		ID       string                 `json:"id"`
		Score    float64                `json:"score"`
		Metadata map[string]interface{} `json:"metadata"`
	} `json:"matches"`
}

// Query performs a semantic + metadata filtered search.
func (c *Client) Query(
	ctx context.Context,
	embedding []float32,
	filter map[string]interface{},
	topK int,
) ([]models.Product, error) {

	// Your Pinecone index is dimension 1024.
	if len(embedding) != expectedDimension {
		return nil, fmt.Errorf(
			"pinecone query: invalid embedding dimension: got %d, expected %d",
			len(embedding),
			expectedDimension,
		)
	}

	if topK <= 0 {
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
		return nil, fmt.Errorf("pinecone query: marshal request: %w", err)
	}

	url := c.indexHost + "/query"

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		url,
		bytes.NewReader(jsonBody),
	)
	if err != nil {
		return nil, fmt.Errorf("pinecone query: create request: %w", err)
	}

	c.setHeaders(req)

	log.Printf(
		"[Pinecone] query host=%s key_prefix=%s dim=%d topK=%d",
		c.indexHost,
		c.keyPrefix(),
		len(embedding),
		topK,
	)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf(
			"pinecone query request failed: %w",
			err,
		)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf(
			"pinecone query: read response: %w",
			err,
		)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"pinecone query failed: status=%d body=%s host=%s",
			resp.StatusCode,
			strings.TrimSpace(string(body)),
			c.indexHost,
		)
	}

	var response queryResponse

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf(
			"pinecone query: decode response: %w body=%s",
			err,
			string(body),
		)
	}

	products := make([]models.Product, 0, len(response.Matches))

	for _, match := range response.Matches {
		product := models.Product{
			ID:    match.ID,
			Score: match.Score,
		}

		if value, ok := match.Metadata["name"].(string); ok {
			product.Name = value
		}

		if value, ok := match.Metadata["description"].(string); ok {
			product.Description = value
		}

		if value, ok := match.Metadata["category"].(string); ok {
			product.Category = value
		}

		if value, ok := match.Metadata["price"].(float64); ok {
			product.Price = value
		}

		if value, ok := match.Metadata["in_stock"].(bool); ok {
			product.InStock = value
		}

		product.Sizes = stringList(match.Metadata["sizes"])
		product.Colors = stringList(match.Metadata["colors"])

		product.Metadata = map[string]string{}
		for _, key := range []string{"occasion", "image_url", "brand", "fabric"} {
			if value, ok := match.Metadata[key].(string); ok {
				product.Metadata[key] = value
			}
		}

		products = append(products, product)
	}

	log.Printf(
		"[Pinecone] query completed matches=%d",
		len(products),
	)

	return products, nil
}

// Upsert writes vectors to Pinecone.
func (c *Client) Upsert(
	ctx context.Context,
	vectors []Vector,
) error {

	if len(vectors) == 0 {
		return nil
	}

	for i, vector := range vectors {
		if len(vector.Values) != expectedDimension {
			return fmt.Errorf(
				"pinecone upsert: vector[%d] has dimension %d, expected %d",
				i,
				len(vector.Values),
				expectedDimension,
			)
		}
	}

	type upsertRequest struct {
		Vectors []Vector `json:"vectors"`
	}

	reqBody := upsertRequest{
		Vectors: vectors,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf(
			"pinecone upsert: marshal request: %w",
			err,
		)
	}

	url := c.indexHost + "/vectors/upsert"

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		url,
		bytes.NewReader(jsonBody),
	)
	if err != nil {
		return fmt.Errorf(
			"pinecone upsert: create request: %w",
			err,
		)
	}

	c.setHeaders(req)

	log.Printf(
		"[Pinecone] upsert host=%s vectors=%d",
		c.indexHost,
		len(vectors),
	)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf(
			"pinecone upsert request failed: %w",
			err,
		)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf(
			"pinecone upsert: read response: %w",
			err,
		)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf(
			"pinecone upsert failed: status=%d body=%s",
			resp.StatusCode,
			strings.TrimSpace(string(body)),
		)
	}

	return nil
}

// Vector represents a Pinecone vector record.
type Vector struct {
	ID       string                 `json:"id"`
	Values   []float32              `json:"values"`
	Metadata map[string]interface{} `json:"metadata"`
}

// stringList converts a Pinecone list-of-strings metadata value.
func stringList(v interface{}) []string {
	raw, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// Input types for Pinecone hosted embedding models: catalog text is embedded
// as "passage", shopper queries as "query" (asymmetric retrieval).
const (
	InputTypeQuery   = "query"
	InputTypePassage = "passage"
)

// Embed turns texts into vectors with Pinecone Inference, so the same model
// embeds both the catalog and incoming queries.
func (c *Client) Embed(ctx context.Context, texts []string, inputType string) ([][]float32, error) {
	type input struct {
		Text string `json:"text"`
	}
	inputs := make([]input, len(texts))
	for i, t := range texts {
		inputs[i] = input{Text: t}
	}

	jsonBody, err := json.Marshal(map[string]interface{}{
		"model":      c.embedModel,
		"parameters": map[string]string{"input_type": inputType, "truncate": "END"},
		"inputs":     inputs,
	})
	if err != nil {
		return nil, fmt.Errorf("pinecone embed: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.pinecone.io/embed", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("pinecone embed: create request: %w", err)
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pinecone embed request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("pinecone embed: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pinecone embed failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var response struct {
		Data []struct {
			Values []float32 `json:"values"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("pinecone embed: decode response: %w", err)
	}
	if len(response.Data) != len(texts) {
		return nil, fmt.Errorf("pinecone embed: got %d vectors for %d inputs", len(response.Data), len(texts))
	}

	vectors := make([][]float32, len(response.Data))
	for i, d := range response.Data {
		vectors[i] = d.Values
	}
	return vectors, nil
}
