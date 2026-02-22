.PHONY: help dev test verify verify-all build docker-up docker-down lint fmt verify-bundle migrate clean smoke-test

ifeq ($(OS),Windows_NT)
VERIFY_ALL_CMD = powershell -NoProfile -ExecutionPolicy Bypass -File .\\ops\\scripts\\verify-all.ps1
else
VERIFY_ALL_CMD = bash ops/scripts/verify-all.sh
endif

help:
	@echo "AegisRun Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make dev          - Run in development mode"
	@echo "  make test         - Run all tests"
	@echo "  make verify       - Run unified quality gates (api, verifier, ui, ts sdk)"
	@echo "  make verify-all   - Run verification with timestamped evidence logs"
	@echo "  make build        - Build all components"
	@echo "  make docker-up    - Start with Docker Compose"
	@echo "  make docker-down  - Stop Docker Compose"
	@echo "  make smoke-test   - Run smoke tests against running server"
	@echo "  make lint         - Run linters"
	@echo "  make fmt          - Format code"
	@echo "  make verify-bundle - Verify evidence bundle"
	@echo "  make migrate      - Run database migrations"
	@echo "  make clean        - Clean build artifacts"

dev:
	docker-compose -f docker-compose.yml up

test:
	cd api && go test ./...
	cd sdk/python && pytest
	cd sdk/typescript && npm test
	cd verifier && go test ./...

verify:
	cd api && go test ./...
	cd verifier && go test ./...
	cd ui && npm run test -- --run
	cd sdk/typescript && npm test -- --run

verify-all:
	$(VERIFY_ALL_CMD)

build:
	cd api && go build -o bin/server cmd/server/main.go
	cd verifier && go build -o bin/aegis-verify cmd/verify/main.go
	cd ui && npm run build

docker-up:
	docker-compose -f docker-compose.yml up -d

docker-down:
	docker-compose -f docker-compose.yml down

smoke-test:
	@echo "Running smoke tests against $(or $(SMOKE_URL),http://localhost:8080)..."
	@curl -sf "$(or $(SMOKE_URL),http://localhost:8080)/health" > /dev/null && echo "  /health   ✓" || (echo "  /health   ✗ FAILED" && exit 1)
	@curl -sf "$(or $(SMOKE_URL),http://localhost:8080)/ready" > /dev/null && echo "  /ready    ✓" || (echo "  /ready    ✗ FAILED" && exit 1)
	@curl -sf "$(or $(SMOKE_URL),http://localhost:8080)/metrics" > /dev/null && echo "  /metrics  ✓" || (echo "  /metrics  ✗ FAILED" && exit 1)
	@echo "All smoke tests passed."

lint:
	cd api && golangci-lint run
	cd sdk/python && ruff check .
	cd sdk/typescript && npm run lint
	cd ui && npm run lint

fmt:
	cd api && go fmt ./...
	cd sdk/python && black .
	cd sdk/typescript && npm run format
	cd ui && npm run format

verify-bundle:
	cd verifier && go run ./cmd/verify/main.go $(BUNDLE)

migrate:
	cd api && migrate -path migrations -database "postgres://aegisrun:aegisrun@localhost:5432/aegisrun?sslmode=disable" up

migrate-down:
	cd api && migrate -path migrations -database "postgres://aegisrun:aegisrun@localhost:5432/aegisrun?sslmode=disable" down 1

clean:
	rm -rf api/bin
	rm -rf verifier/bin
	rm -rf ui/dist
	rm -rf sdk/python/dist
	rm -rf sdk/typescript/dist
