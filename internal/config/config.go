package config

import (
	"errors"
	"io/fs"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	ServerPort        string
	RedisAddr         string
	RedisPassword     string
	RedisDB           int
	PineconeAPIKey    string
	PineconeIndexHost string
	ClaudeAPIKey      string
	ClaudeModel       string
	ClaudeFastModel   string
	EmbedModel        string
	ClaudeMaxTokens   int
	SemanticCacheTTL  time.Duration
	TokenCacheTTL     time.Duration
	MaxRPS            int
	RequestTimeout    time.Duration
	UpstreamTimeout   time.Duration
	EnableStreaming   bool
}

// Load reads configuration from the environment. Secrets have no defaults:
// they must come from the environment (see .env.example), never from source.
//
// For local development, a .env file in the working directory is read on every
// start; variables already set in the environment take precedence. The debugger
// launch config deliberately does not inject .env (a long-lived dlv would pin the
// values it read at launch), so each restart picks up the latest .env.
func Load() *Config {
	if err := godotenv.Load(); err == nil {
		log.Printf("config: loaded .env")
	} else if !errors.Is(err, fs.ErrNotExist) {
		log.Fatalf("config: read .env: %v", err)
	}

	cfg := &Config{
		ServerPort:        getEnv("SERVER_PORT", "8080"),
		RedisAddr:         getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:     getEnv("REDIS_PASSWORD", ""),
		RedisDB:           getEnvInt("REDIS_DB", 0),
		PineconeAPIKey:    getEnv("PINECONE_API_KEY", ""),
		PineconeIndexHost: getEnv("PINECONE_INDEX_HOST", ""),
		ClaudeAPIKey:      getEnv("CLAUDE_API_KEY", ""),
		ClaudeModel:       getEnv("CLAUDE_MODEL", "claude-sonnet-5"),
		ClaudeFastModel:   getEnv("CLAUDE_FAST_MODEL", "claude-haiku-4-5"),
		EmbedModel:        getEnv("PINECONE_EMBED_MODEL", "llama-text-embed-v2"),
		ClaudeMaxTokens:   getEnvInt("CLAUDE_MAX_TOKENS", 1024),
		SemanticCacheTTL:  getEnvDuration("SEMANTIC_CACHE_TTL", 300),
		TokenCacheTTL:     getEnvDuration("TOKEN_CACHE_TTL", 3600),
		MaxRPS:            getEnvInt("MAX_RPS", 20000),
		RequestTimeout:    getEnvDuration("REQUEST_TIMEOUT", 20),
		UpstreamTimeout:   getEnvDuration("UPSTREAM_TIMEOUT", 30),
		EnableStreaming:   getEnvBool("ENABLE_STREAMING", true),
	}

	for name, v := range map[string]string{
		"PINECONE_API_KEY":    cfg.PineconeAPIKey,
		"PINECONE_INDEX_HOST": cfg.PineconeIndexHost,
		"CLAUDE_API_KEY":      cfg.ClaudeAPIKey,
	} {
		if v == "" {
			log.Fatalf("config: %s is required (copy .env.example to .env and fill it in)", name)
		}
	}
	return cfg
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

func getEnvDuration(key string, fallbackSec int) time.Duration {
	return time.Duration(getEnvInt(key, fallbackSec)) * time.Second
}
