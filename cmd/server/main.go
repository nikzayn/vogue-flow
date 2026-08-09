package main

import (
	"log"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/docker/cli/cli/config"
	"github.com/nikzayn/vogue-flow/internal/pinecone"
)

// import "github.com/docker/cli/cli/config"

func main() {
	cfg := config.Load()

	//initialise redis semantic cache
	semnaticCache := cache.NewSemanticCache(
		cfg.RedisAddr,
		cfg.RedisPasswod,
		cfg.RedisDB,
		cfg.SematicCacheTTL,
	)

	defer semnaticCache.Close()

	//initialise the token cache
	tokenCache := cache.NewTokenCache(
		cfg.RedisAddr,
		cfg.RedisPasswod,
		cfg.RedisDB,
		cfg.TokenCacheTTL,
	)

	defer tokenCache.Close()

	//initialize the pinecone vector store
	pc := pinecone.NewClient(cfg.PineconeAPIKey, cfg.PineconeIndexHost)

	//local llm models for endpoint testing
	tier1 := &llm.MockTier1{}

	//tier 2 and 3: Claude LLM models
	claude := llm.NewClaudeClient(cfg.ClaudeAPIKey, cfg.ClaudeModel, cfg.ClaudeMaxTokens)
	cascade := llm.NewCascade(tier1, claude, claude)

	orchestrator := agents.NewOrchestrator(pc, cascade, semnaticCache, tokenCache)

	//http server
	handler := server.NewHandler(orchestrator)
	mux := http.NewServeMux()
	handler.RegisterRouter(mux)

	srv := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	log.Printf("VogueFlow starting on :%s | GOMAXPROCS=%d", cfg.ServerPort, runtime.GOMAXPROCS(0))
	log.Printf("Redis %s | Pinecone: connected | Claude: %s", cfg.RedisAddr, cfg.ClaudeModel)
	log.Printf("SemanticCacheTTL: %v | TokenCacheTTL: %v", cfg.SematicCacheTTL, cfg.TokenCacheTTL)

	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.SetOutput(os.Stdout)
}
