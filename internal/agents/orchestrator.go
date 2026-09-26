package agents

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nikzayn/vogueflow/internal/cache"
	"github.com/nikzayn/vogueflow/internal/llm"
	"github.com/nikzayn/vogueflow/internal/models"
	"github.com/nikzayn/vogueflow/internal/pinecone"
)

// Orchestrator manages the LangGraph-like workflow execution
type Orchestrator struct {
	intentAgent      *IntentAgent
	discoveryAgent   *DiscoveryAgent
	sizingAgent      *SizingAgent
	stylistAgent     *StylistAgent
	transactionAgent *TransactionAgent
	escalationAgent  *EscalationAgent
	greetingAgent    *GreetingAgent
	cascade          *llm.Cascade
	semanticCache    *cache.SemanticCache
	tokenCache       *cache.TokenCache
}

// NewOrchestrator wires all 6 agents and caches
func NewOrchestrator(
	pc *pinecone.Client,
	cascade *llm.Cascade,
	semCache *cache.SemanticCache,
	tokCache *cache.TokenCache,
) *Orchestrator {
	return &Orchestrator{
		intentAgent:      NewIntentAgent(cascade),
		discoveryAgent:   NewDiscoveryAgent(pc),
		sizingAgent:      NewSizingAgent(cascade),
		stylistAgent:     NewStylistAgent(cascade),
		transactionAgent: NewTransactionAgent(cascade),
		escalationAgent:  NewEscalationAgent(),
		greetingAgent:    NewGreetingAgent(cascade),
		cascade:          cascade,
		semanticCache:    semCache,
		tokenCache:       tokCache,
	}
}

// Execute runs the full agentic workflow with caching and model cascading
func (o *Orchestrator) Execute(ctx context.Context, query models.Query) (*models.AgentState, error) {
	start := time.Now()

	// L1: Semantic Cache Check
	if cached, hit := o.semanticCache.Get(ctx, query); hit {
		cached.LatencyMs = time.Since(start).Milliseconds()
		cached.TokensUsed = 0 // no LLM call was made for this request
		return cached, nil
	}

	state, needsGeneration, err := o.run(ctx, query)
	if err != nil {
		return nil, err
	}

	// Final step: generate the response via the selected model tier
	if needsGeneration {
		response, tokens, _, err := o.generateResponse(ctx, state, state.ModelTier)
		if err != nil {
			return nil, fmt.Errorf("response generation: %w", err)
		}
		state.Response = response
		state.TokensUsed += tokens
	}

	// Cache the result
	state.LatencyMs = time.Since(start).Milliseconds()
	state.Completed = true
	_ = o.semanticCache.Set(ctx, query, state)

	return state, nil
}

// run executes every workflow step except final response generation. It returns
// needsGeneration=true when the final answer should come from the LLM (discovery/styling);
// otherwise state.Response already holds the answer.
func (o *Orchestrator) run(ctx context.Context, query models.Query) (*models.AgentState, bool, error) {
	state := &models.AgentState{Query: query}

	// Check escalation first (frustration / human request)
	if o.escalationAgent.ShouldEscalate(query.Text) {
		state.Intent = string(IntentEscalation)
		state.Response = o.escalationAgent.EscalationMessage()
		return state, false, nil
	}

	// Step 1: Intent Classification
	intent, err := o.intentAgent.Classify(ctx, query.Text)
	if err != nil {
		return nil, false, fmt.Errorf("intent classification: %w", err)
	}
	state.Intent = string(intent)

	// Step 2: Model Tier Selection
	state.ModelTier = o.cascade.ClassifyTier(query.Text, state.Intent)

	// Route to specialized agent based on intent
	switch intent {
	case IntentSizing:
		resp, tokens, err := o.sizingAgent.SizeRecommendation(ctx, state)
		if err != nil {
			return nil, false, fmt.Errorf("sizing: %w", err)
		}
		state.Response = resp
		state.TokensUsed += tokens

	case IntentTransaction:
		resp, tokens, err := o.transactionAgent.BuildCartResponse(ctx, state)
		if err != nil {
			return nil, false, fmt.Errorf("transaction: %w", err)
		}
		state.Response = resp
		state.TokensUsed += tokens

	case IntentBrowse, IntentDiscovery, IntentStyling:
		// Step 3: Product Discovery (RAG via Pinecone)
		products, err := o.discoveryAgent.Discover(ctx, state)
		if err != nil {
			return nil, false, fmt.Errorf("discovery: %w", err)
		}
		state.Products = products

		// Step 4: Stylist Agent adds an outfit and a styling note for the final prompt
		if intent == IntentStyling || intent == IntentDiscovery {
			outfit, tip, err := o.stylistAgent.BuildOutfit(ctx, state)
			if err == nil {
				state.Outfit = outfit
				state.Response = tip
			}
		}
		return state, true, nil

	case IntentGreeting:
		resp, tokens, err := o.greetingAgent.Greet(ctx, query.UserID, query.Text)
		if err != nil {
			return nil, false, fmt.Errorf("greeting: %w", err)
		}
		state.Response = resp
		state.TokensUsed += tokens
		state.ModelTier = 1 // greetings are always Tier 1 (Haiku or static)

	default:
		state.Response = "I'd love to help! Can you tell me more about what you're looking for?"
		state.ModelTier = 1
	}

	return state, false, nil
}

// ExecuteStream runs the workflow and streams the final LLM response via SSE
func (o *Orchestrator) ExecuteStream(ctx context.Context, query models.Query, ch chan<- string) error {
	defer close(ch)
	start := time.Now()

	// Fast path: if semantic cache hit, stream cached response immediately
	if cached, hit := o.semanticCache.Get(ctx, query); hit {
		ch <- cached.Response
		return nil
	}

	state, needsGeneration, err := o.run(ctx, query)
	if err != nil {
		ch <- "Sorry, I couldn't process your request. Please try again."
		return err
	}

	// Agents that already produced a final answer (greeting, sizing, cart, escalation): emit it
	if !needsGeneration {
		ch <- state.Response
		state.LatencyMs = time.Since(start).Milliseconds()
		_ = o.semanticCache.Set(ctx, query, state)
		return nil
	}

	// Stream the final response using the selected tier (single LLM call)
	system := o.buildSystemPrompt(state.Intent)
	prompt := o.buildUserPrompt(state)

	streamCh := make(chan string)
	errCh := make(chan error, 1)
	go func() {
		errCh <- o.cascade.StreamWithTier(ctx, system, prompt, state.ModelTier, streamCh)
	}()

	var full strings.Builder
	for token := range streamCh {
		full.WriteString(token)
		select {
		case ch <- token:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if err := <-errCh; err != nil {
		return fmt.Errorf("stream generation: %w", err)
	}

	state.Response = full.String()
	state.LatencyMs = time.Since(start).Milliseconds()
	state.Completed = true
	_ = o.semanticCache.Set(ctx, query, state)
	return nil
}

// generateResponse uses token cache to avoid redundant LLM calls
func (o *Orchestrator) generateResponse(ctx context.Context, state *models.AgentState, tier int) (string, int, int, error) {
	system := o.buildSystemPrompt(state.Intent)
	prompt := o.buildUserPrompt(state)

	// Check token cache: identical (system + prompt) -> cached response
	if cached, hit := o.tokenCache.Get(ctx, system, prompt); hit {
		return cached, 0, tier, nil
	}

	text, tokens, usedTier, err := o.cascade.CompleteWithTier(ctx, system, prompt, tier)
	if err != nil {
		return "", 0, tier, err
	}

	// Store in token cache for future deduplication
	_ = o.tokenCache.Set(ctx, system, prompt, text)
	return text, tokens, usedTier, nil
}

// buildSystemPrompt returns brand voice and safety instructions per intent
func (o *Orchestrator) buildSystemPrompt(intent string) string {
	base := "You are Victoria's Secret's AI stylist. Be confident, empowering, and concise. "
	switch intent {
	case string(IntentSizing):
		return base + "Focus on fit accuracy. Reference size charts. Be reassuring about body diversity."
	case string(IntentStyling):
		return base + "Create aspirational but achievable looks. Mention fabric and occasion."
	case string(IntentTransaction):
		return base + "Be efficient. Confirm details. Highlight promotions and delivery options."
	default:
		return base + "Help the customer discover products that make them feel confident."
	}
}

// buildUserPrompt constructs the final prompt from agent state
func (o *Orchestrator) buildUserPrompt(state *models.AgentState) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Customer query: %s\n", state.Query.Text)
	if state.Query.Size != "" {
		fmt.Fprintf(&b, "Size: %s\n", state.Query.Size)
	}
	if state.Query.Budget > 0 {
		fmt.Fprintf(&b, "Budget: $%.2f\n", state.Query.Budget)
	}
	if state.Query.Occasion != "" {
		fmt.Fprintf(&b, "Occasion: %s\n", state.Query.Occasion)
	}
	if len(state.Products) > 0 {
		b.WriteString("\nTop products (only recommend from this list):\n")
		for i, p := range state.Products {
			if i >= 5 {
				break
			}
			fmt.Fprintf(&b, "- %s ($%.2f, %s): %s\n", p.Name, p.Price, p.Category, p.Description)
		}
	} else {
		b.WriteString("\nNo matching products were found in the catalog. Say so and ask a clarifying question.\n")
	}
	if state.Response != "" {
		fmt.Fprintf(&b, "\nStylist note: %s\n", state.Response)
	}
	b.WriteString("\nRespond with a helpful, brand-aligned message. Keep under 3 sentences.")
	return b.String()
}
