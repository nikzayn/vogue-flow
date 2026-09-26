package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ClaudeClient struct
type ClaudeClient struct {
	apiKey     string
	model      string
	maxTokens  int
	endpoint   string
	httpClient *http.Client
}

// Claude API client
func NewClaudeClient(apiKey, model string, maxTokens int, timeout time.Duration) *ClaudeClient {
	return &ClaudeClient{
		apiKey:    apiKey,
		model:     model,
		maxTokens: maxTokens,
		endpoint:  "https://api.anthropic.com/v1/messages",
		httpClient: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:        200,
				MaxIdleConnsPerHost: 200,
				IdleConnTimeout:     120 * time.Second,
			},
		},
	}
}

// MessageRequest is the Anthropic Messages API payload
type MessageRequest struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	System    string    `json:"system,omitempty"`
	Messages  []Message `json:"messages"`
	Stream    bool      `json:"stream,omitempty"`
}

// Message represents a single turn
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// MessageResponse
type MessageResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// Complete performs a blocking request to Claude
func (c *ClaudeClient) Complete(ctx context.Context, system, prompt string) (string, int, error) {
	reqBody := MessageRequest{
		Model:     c.model,
		MaxTokens: c.maxTokens,
		System:    system,
		Messages:  []Message{{Role: "user", Content: prompt}},
		Stream:    false,
	}

	jsonBody, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("claude api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", 0, fmt.Errorf("claude status %d: %s", resp.StatusCode, string(body))
	}

	var msgResp MessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&msgResp); err != nil {
		return "", 0, err
	}

	var text string
	for _, c := range msgResp.Content {
		if c.Type == "text" {
			text += c.Text
		}
	}
	totalTokens := msgResp.Usage.InputTokens + msgResp.Usage.OutputTokens
	return text, totalTokens, nil
}

// Streams send token to the channel
func (c *ClaudeClient) Stream(ctx context.Context, system, prompt string, ch chan<- string) error {
	defer close(ch)

	reqBody := MessageRequest{
		Model:     c.model,
		MaxTokens: c.maxTokens,
		System:    system,
		Messages:  []Message{{Role: "user", Content: prompt}},
		Stream:    true,
	}

	jsonBody, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("claude stream status %d: %s", resp.StatusCode, string(body))
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var event struct {
			Type  string `json:"type"`
			Delta struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"delta"`
		}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}
		if event.Type == "content_block_delta" && event.Delta.Type == "text_delta" {
			select {
			case ch <- event.Delta.Text:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	return scanner.Err()
}
