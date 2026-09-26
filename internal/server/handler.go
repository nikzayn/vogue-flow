package server

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/nikzayn/vogueflow/internal/agents"
	"github.com/nikzayn/vogueflow/internal/models"
	"github.com/nikzayn/vogueflow/internal/pinecone"
)

// Embedder turns query text into a vector for retrieval and the semantic cache
type Embedder interface {
	Embed(ctx context.Context, texts []string, inputType string) ([][]float32, error)
}

// Handler holds HTTP handlers for the VogueFlow API
type Handler struct {
	orchestrator   *agents.Orchestrator
	embedder       Embedder
	requestTimeout time.Duration
}

// NewHandler creates an HTTP handler
func NewHandler(orch *agents.Orchestrator, embedder Embedder, requestTimeout time.Duration) *Handler {
	return &Handler{orchestrator: orch, embedder: embedder, requestTimeout: requestTimeout}
}

// maxBodyBytes caps request bodies; a shopping query is a few hundred bytes
const maxBodyBytes = 1 << 20

// decodeJSON requires an application/json body and decodes it into dst,
// writing a 415/400 response and returning false on failure
func decodeJSON(w http.ResponseWriter, r *http.Request, dst interface{}) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		http.Error(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
		return false
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return false
	}
	return true
}

// buildQuery validates the request and embeds the query text unless the client sent a vector
func (h *Handler) buildQuery(ctx context.Context, req ShopRequest) (models.Query, error) {
	if strings.TrimSpace(req.Query) == "" {
		return models.Query{}, errors.New("query is required")
	}

	embedding := req.Embedding
	if len(embedding) == 0 {
		vectors, err := h.embedder.Embed(ctx, []string{req.Query}, pinecone.InputTypeQuery)
		if err != nil {
			return models.Query{}, err
		}
		embedding = vectors[0]
	}

	return models.Query{
		SessionID: req.SessionID,
		UserID:    req.UserID,
		Text:      req.Query,
		Embedding: embedding,
		Size:      req.Size,
		Budget:    req.Budget,
		Occasion:  req.Occasion,
	}, nil
}

// ShopRequest is the incoming user query
type ShopRequest struct {
	SessionID string    `json:"session_id"`
	UserID    string    `json:"user_id"`
	Query     string    `json:"query"`
	Embedding []float32 `json:"embedding,omitempty"` // optional; embedded server-side when omitted
	Size      string    `json:"size"`
	Budget    float64   `json:"budget"`
	Occasion  string    `json:"occasion"`
}

// ShopResponse is the structured API response
type ShopResponse struct {
	Intent     string           `json:"intent"`
	Products   []models.Product `json:"products"`
	Outfit     []models.Product `json:"outfit,omitempty"`
	Response   string           `json:"response"`
	TokensUsed int              `json:"tokens_used"`
	LatencyMs  int64            `json:"latency_ms"`
	CacheHit   bool             `json:"cache_hit"`
	ModelTier  int              `json:"model_tier"`
}

// RegisterRoutes sets up HTTP routes
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/shop", h.handleShop)
	mux.HandleFunc("/v1/shop/stream", h.handleShopStream)
	mux.HandleFunc("/health", h.handleHealth)
	mux.HandleFunc("/", h.handleUI)
}

// indexHTML is the browser console (query panel + metrics dashboard), compiled into the binary
//
//go:embed web/index.html
var indexHTML []byte

// handleUI serves the console at "/"; any other unmatched path is a 404
func (h *Handler) handleUI(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write(indexHTML)
}

// handleShop handles synchronous shopping queries
func (h *Handler) handleShop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ShopRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.requestTimeout)
	defer cancel()

	query, err := h.buildQuery(ctx, req)
	if err != nil {
		log.Printf("[shop] build query: %v", err)
		http.Error(w, "Could not process query", http.StatusBadRequest)
		return
	}

	state, err := h.orchestrator.Execute(ctx, query)
	if err != nil {
		log.Printf("[shop] execute: %v", err)
		http.Error(w, "Processing error", http.StatusInternalServerError)
		return
	}

	resp := ShopResponse{
		Intent:     state.Intent,
		Products:   state.Products,
		Outfit:     state.Outfit,
		Response:   state.Response,
		TokensUsed: state.TokensUsed,
		LatencyMs:  state.LatencyMs,
		CacheHit:   state.CacheHit,
		ModelTier:  state.ModelTier,
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache-Hit", strconv.FormatBool(state.CacheHit))
	w.Header().Set("X-Latency-Ms", strconv.FormatInt(state.LatencyMs, 10))
	json.NewEncoder(w).Encode(resp)
}

// handleShopStream handles Server-Sent Events for real-time streaming responses
func (h *Handler) handleShopStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ShopRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.requestTimeout)
	defer cancel()

	query, err := h.buildQuery(ctx, req)
	if err != nil {
		log.Printf("[shop] build query: %v", err)
		http.Error(w, "Could not process query", http.StatusBadRequest)
		return
	}

	// SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	ch := make(chan string)
	go func() {
		if err := h.orchestrator.ExecuteStream(ctx, query, ch); err != nil {
			log.Printf("[shop/stream] execute: %v", err)
		}
	}()

	for token := range ch {
		// SSE format: data: <token>\n\n
		lines := strings.Split(token, "\n")
		for _, line := range lines {
			fmt.Fprintf(w, "data: %s\n", line)
		}
		fmt.Fprint(w, "\n")
		flusher.Flush()
	}

	// Send DONE event
	fmt.Fprint(w, "data: [DONE]\n\n")
	flusher.Flush()
}

// handleHealth returns service health
func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "vogueflow"})
}
