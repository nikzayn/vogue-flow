// Command ingest loads a product catalog (JSON array of models.Product plus
// "occasion"/"fabric"), embeds each product with the same Pinecone model the
// server uses for queries, and upserts the vectors with filterable metadata.
//
//	go run ./cmd/ingest -file data/catalog.json
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/nikzayn/vogueflow/internal/pinecone"
)

// catalogItem mirrors the catalog file; occasion and fabric are stored as metadata
type catalogItem struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Price       float64  `json:"price"`
	Colors      []string `json:"colors"`
	Sizes       []string `json:"sizes"`
	InStock     bool     `json:"in_stock"`
	Occasion    string   `json:"occasion"`
	Fabric      string   `json:"fabric"`
}

// embedBatchSize stays under Pinecone Inference's per-request input limit
const embedBatchSize = 90

func main() {
	file := flag.String("file", "data/catalog.json", "path to catalog JSON")
	flag.Parse()

	apiKey, host := os.Getenv("PINECONE_API_KEY"), os.Getenv("PINECONE_INDEX_HOST")
	model := os.Getenv("PINECONE_EMBED_MODEL")
	if model == "" {
		model = "llama-text-embed-v2"
	}
	if apiKey == "" || host == "" {
		log.Fatal("PINECONE_API_KEY and PINECONE_INDEX_HOST are required (run via `make ingest` to load .env)")
	}

	raw, err := os.ReadFile(*file)
	if err != nil {
		log.Fatalf("read catalog: %v", err)
	}
	var items []catalogItem
	if err := json.Unmarshal(raw, &items); err != nil {
		log.Fatalf("parse catalog: %v", err)
	}

	pc := pinecone.NewClient(apiKey, host, model)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	for start := 0; start < len(items); start += embedBatchSize {
		end := min(start+embedBatchSize, len(items))
		batch := items[start:end]

		texts := make([]string, len(batch))
		for i, it := range batch {
			texts[i] = embeddingText(it)
		}
		vectors, err := pc.Embed(ctx, texts, pinecone.InputTypePassage)
		if err != nil {
			log.Fatalf("embed batch %d-%d: %v", start, end, err)
		}

		records := make([]pinecone.Vector, len(batch))
		for i, it := range batch {
			records[i] = pinecone.Vector{
				ID:     it.ID,
				Values: vectors[i],
				Metadata: map[string]interface{}{
					"name":        it.Name,
					"description": it.Description,
					"category":    strings.ToLower(it.Category),
					"price":       it.Price,
					"colors":      it.Colors,
					"sizes":       it.Sizes,
					"in_stock":    it.InStock,
					"occasion":    strings.ToLower(it.Occasion),
					"fabric":      it.Fabric,
				},
			}
		}
		if err := pc.Upsert(ctx, records); err != nil {
			log.Fatalf("upsert batch %d-%d: %v", start, end, err)
		}
		log.Printf("upserted %d/%d products", end, len(items))
	}
}

// embeddingText is what the model "reads" for each product; include every
// attribute a shopper might describe in natural language.
func embeddingText(it catalogItem) string {
	return fmt.Sprintf("%s. Category: %s. %s Colors: %s. Fabric: %s. Occasion: %s.",
		it.Name, it.Category, it.Description, strings.Join(it.Colors, ", "), it.Fabric, it.Occasion)
}
