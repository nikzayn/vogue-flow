# Loads .env into the environment for every target that talks to external services.
ENV = set -a; [ -f .env ] && . ./.env; set +a;

.PHONY: up down run ingest test build loadtest

up:        ## start Redis
	docker compose up -d redis

down:
	docker compose down

run:       ## run the API on :8080
	$(ENV) go run ./cmd/server

ingest:    ## embed data/catalog.json and upsert into Pinecone
	$(ENV) go run ./cmd/ingest -file data/catalog.json

test:
	go test ./...

build:
	go build -o bin/server ./cmd/server && go build -o bin/ingest ./cmd/ingest

# Every request can hit the Claude API and cost money; keep RPS low.
loadtest:
	python3 scripts/loadtest.py --rps 2 --duration 10 --workers 4
