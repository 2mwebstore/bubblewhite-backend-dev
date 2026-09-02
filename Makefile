.PHONY: run build tidy fmt vet test up down logs seed-check

# Run the API locally (loads .env automatically).
run:
	go run ./cmd/server

# Build a release binary into ./bin/server.
build:
	go build -o bin/server ./cmd/server

# Resolve/update dependencies.
tidy:
	go mod tidy

fmt:
	gofmt -w .

vet:
	go vet ./...

test:
	go test ./...

# Start MySQL + phpMyAdmin for local dev.
up:
	docker compose up -d

down:
	docker compose down

logs:
	docker compose logs -f mysql

# Quick sanity check: does the API respond?
seed-check:
	curl -fsS http://localhost:8080/health && echo
