# VogueFlow

A conversational shopping assistant for a lingerie / beauty catalog, written in Go.
A shopper sends a natural-language message ("strapless bra for a wedding dress under $80");
VogueFlow classifies the intent, retrieves matching products from Pinecone (RAG),
routes to a specialist agent, and answers with Claude — using a Haiku/Sonnet model
cascade and two Redis caches to keep cost and latency down.

## Request flow

```
POST /v1/shop  {query, size?, budget?, occasion?}
  │
  ├─ handler: embed query text (Pinecone Inference, llama-text-embed-v2, 1024-d, input_type=query)
  │
  ├─ Orchestrator.Execute
  │    1. Semantic cache (Redis): cosine ≥ 0.94 within the same {size,budget,occasion} bucket → return
  │    2. Escalation check (frustration keywords) → human-handoff message
  │    3. Intent agent (keyword rules) → greeting | sizing | styling | transaction | discovery
  │    4. Cascade.ClassifyTier → 1 (Haiku) | 2 (Sonnet) | 3 (Sonnet, larger budget)
  │    5. Route:
  │         greeting    → static reply or Haiku
  │         sizing      → Sizing agent (LLM)
  │         transaction → Transaction agent (cart summary + promo rules + LLM)
  │         discovery / styling →
  │              Discovery agent: Pinecone query with metadata filters (in_stock, price ≤ budget,
  │                               sizes ∈ [size], occasion) → business re-rank → top 10
  │              Stylist agent:   one item per category → outfit + 1-line styling tip (LLM)
  │              Final answer:    token cache (exact prompt hash) → else Claude at chosen tier
  │    6. Store in semantic cache
  └─ JSON: intent, products, outfit, response, tokens_used, latency_ms, cache_hit, model_tier
```

Both endpoints take a raw JSON body and require `Content-Type: application/json`
(other content types get `415 Unsupported Media Type`).

`POST /v1/shop/stream` runs the same steps and streams the final Claude answer as SSE.

## Layout

| Path | What it does |
|---|---|
| `cmd/server` | wires config → caches → Pinecone → Claude cascade → orchestrator → HTTP |
| `cmd/ingest` | embeds `data/catalog.json` (input_type=passage) and upserts to Pinecone |
| `internal/agents` | orchestrator + intent, discovery, sizing, stylist, transaction, escalation, greeting agents |
| `internal/llm` | Claude Messages API client (sync + SSE streaming) and the 3-tier cascade |
| `internal/pinecone` | REST client: query, upsert, embed |
| `internal/cache` | semantic cache (embedding similarity) and token cache (exact prompt hash) |
| `internal/server` | HTTP handlers, plus the embedded browser console (`web/index.html`) served at `/` |
| `scripts/loadtest.py` | async load generator (keep RPS low — cache misses cost money) |

## Run locally

Prereqs: Go 1.26+, Docker (for Redis), a Pinecone index with dimension 1024 / cosine,
an Anthropic API key.

```bash
cp .env.example .env      # fill in keys and the index host
make up                   # Redis
make ingest               # load the sample catalog into Pinecone
make run                  # API on :8080
```

```bash
curl -s -X POST localhost:8080/v1/shop \
  -H 'Content-Type: application/json' \
  -d '{"query":"strapless bra for a wedding dress","budget":80}'

curl -N -X POST localhost:8080/v1/shop/stream \
  -H 'Content-Type: application/json' \
  -d '{"query":"help me build a date night outfit"}'
make test
```

### Browser console

With the server running, open http://localhost:8080/ for a small UI (served by the Go
binary from `internal/server/web/index.html`, no build step):

- send queries in JSON or streaming mode, or click a preset scenario ("Run all scenarios" fires each once)
- see the answer, intent / tier / tokens / cache badges, and the ranked products (outfit items highlighted)
- charts of latency and tokens per request (cache hits in orange), requests by model tier and by intent,
  KPI tiles (cache hit rate, avg / p95 latency, total tokens) and a request log you can export as JSON

History is kept in your browser's localStorage; "Clear history" resets it.

The catalog in `data/catalog.json` is synthetic. To use other data, produce a JSON array
with the same fields (`id, name, description, category, price, colors, sizes, in_stock,
occasion, fabric`) and run `go run ./cmd/ingest -file your.json`.
