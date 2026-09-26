package main

import (
	"log"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/nikzayn/vogueflow/internal/agents"
	"github.com/nikzayn/vogueflow/internal/cache"
	"github.com/nikzayn/vogueflow/internal/config"
	"github.com/nikzayn/vogueflow/internal/llm"
	"github.com/nikzayn/vogueflow/internal/pinecone"
	"github.com/nikzayn/vogueflow/internal/server"
)

func main() {
	cfg := config.Load()

	//initialise redis semantic cache
	semnaticCache := cache.NewSemanticCache(
		cfg.RedisAddr,
		cfg.RedisPassword,
		cfg.RedisDB,
		cfg.SemanticCacheTTL,
	)

	defer semnaticCache.Close()

	//initialise the token cache
	tokenCache := cache.NewTokenCache(
		cfg.RedisAddr,
		cfg.RedisPassword,
		cfg.RedisDB,
		cfg.TokenCacheTTL,
	)

	defer tokenCache.Close()

	//initialize the pinecone vector store
	pc := pinecone.NewClient(cfg.PineconeAPIKey, cfg.PineconeIndexHost, cfg.EmbedModel, cfg.UpstreamTimeout)

	//tier 1: fast/cheap model for greetings and simple lookups
	tier1 := llm.NewClaudeClient(cfg.ClaudeAPIKey, cfg.ClaudeFastModel, 256, cfg.UpstreamTimeout)

	//tier 2: standard model; tier 3: same model with more room for outfit building
	tier2 := llm.NewClaudeClient(cfg.ClaudeAPIKey, cfg.ClaudeModel, cfg.ClaudeMaxTokens, cfg.UpstreamTimeout)
	tier3 := llm.NewClaudeClient(cfg.ClaudeAPIKey, cfg.ClaudeModel, cfg.ClaudeMaxTokens*2, cfg.UpstreamTimeout)
	cascade := llm.NewCascade(tier1, tier2, tier3)

	orchestrator := agents.NewOrchestrator(pc, cascade, semnaticCache, tokenCache)

	//http server; the handler embeds query text via Pinecone when the client sends no embedding
	handler := server.NewHandler(orchestrator, pc, cfg.RequestTimeout)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	srv := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: cfg.RequestTimeout + 10*time.Second, // outlives the request context so errors can still be written
		IdleTimeout:  120 * time.Second,
	}

	log.Printf("VogueFlow starting on :%s | GOMAXPROCS=%d", cfg.ServerPort, runtime.GOMAXPROCS(0))
	log.Printf("Redis %s | Claude tier1=%s tier2/3=%s | Embed: %s", cfg.RedisAddr, cfg.ClaudeFastModel, cfg.ClaudeModel, cfg.EmbedModel)
	log.Printf("SemanticCacheTTL: %v | TokenCacheTTL: %v | RequestTimeout: %v | UpstreamTimeout: %v", cfg.SemanticCacheTTL, cfg.TokenCacheTTL, cfg.RequestTimeout, cfg.UpstreamTimeout)

	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.SetOutput(os.Stdout)
}
