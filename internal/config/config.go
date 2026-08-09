package config

import (
	"os"
	"strconv"
	"time"
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
	ClaudeMaxTokens   int
	SemanticCacheTTL  time.Duration
	TokenCacheTTL     time.Duration
	MaxRPS            int
	EnableStreaming   bool
}

func Load() *Config {
	return &Config{
		ServerPort:        getEnv("SERVER_PORT", "8080"),
		RedisAddr:         getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:     getEnv("REDIS_PASSWORD", ""),
		RedisDB:           getEnvInt("REDIS_DB", 0),
		PineconeAPIKey:    getEnv("PINECONE_API_KEY", "pcsk_46axUa_FVrrebnUZQXCotNVvB5SoPodcQhsDKvVv2phJ1FNwXtTfUZN5AQAa3Jk8u53vZj"),
		PineconeIndexHost: getEnv("PINECONE_INDEX_HOST", "https://llama-text-embed-v2-index-4ndumhq.svc.aped-4627-b74a.pinecone.io"),
		ClaudeAPIKey:      getEnv("CLAUDE_API_KEY", ""),
		ClaudeModel:       getEnv("CLAUDE_MODEL", "claude-sonnet-4-5-20250929"),
		ClaudeMaxTokens:   getEnvInt("CLAUDE_MAX_TOKENS", 1024),
		SemanticCacheTTL:  getEnvDuration("SEMANTIC_CACHE_TTL", 300),
		TokenCacheTTL:     getEnvDuration("TOKEN_CACHE_TTL", 3600),
		MaxRPS:            getEnvInt("MAX_RPS", 20000),
		EnableStreaming:   getEnvBool("ENABLE_STREAMING", true),
	}
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
