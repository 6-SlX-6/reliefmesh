# ReliefMesh developer tasks. Run `make help` for an overview.
SHELL := /bin/bash
COMPOSE_DIR := infrastructure/compose
DEV_DB_URL ?= postgres://reliefmesh:reliefmesh@localhost:5432/reliefmesh?sslmode=disable
TEST_DB_URL ?= postgres://reliefmesh:reliefmesh@localhost:5432/reliefmesh_test?sslmode=disable
E2E_DB_URL ?= postgres://reliefmesh:reliefmesh@localhost:5432/reliefmesh_e2e?sslmode=disable

.PHONY: help install dev-db dev-db-down api-run api-seed web-dev build build-api build-web \
        test test-api test-web test-e2e lint fmt openapi docker-build demo-up demo-down

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[1m%-14s\033[0m %s\n", $$1, $$2}'

install: ## Install JavaScript dependencies (pnpm) and download Go modules
	pnpm install
	cd apps/api && go mod download

dev-db: ## Start PostgreSQL for development (also creates test databases)
	docker compose -f $(COMPOSE_DIR)/docker-compose.dev.yml up -d db

dev-db-down: ## Stop the development database
	docker compose -f $(COMPOSE_DIR)/docker-compose.dev.yml down

api-run: ## Run the API on :8080 against the development database
	cd apps/api && set -a && source ../../.env.example && set +a && \
	  RELIEFMESH_DATABASE_URL="$(DEV_DB_URL)" go run ./cmd/reliefmesh-api serve

api-seed: ## Load the flood exercise scenario into the development database
	cd apps/api && set -a && source ../../.env.example && set +a && \
	  RELIEFMESH_DATABASE_URL="$(DEV_DB_URL)" go run ./cmd/reliefmesh-api seed-demo --scenario flood --if-not-seeded

web-dev: ## Run the Nuxt dev server on :3000 (proxies /api to :8080)
	pnpm --filter @reliefmesh/web dev

build: build-api build-web ## Build API binary and web app

build-api: ## Build the API binary into apps/api/bin
	cd apps/api && CGO_ENABLED=0 go build -o bin/reliefmesh-api ./cmd/reliefmesh-api

build-web: ## Build the static web app into apps/web/.output/public
	pnpm --filter @reliefmesh/web build

test: test-api test-web ## Run unit and integration tests

test-api: ## Go unit + integration tests (needs the test database)
	cd apps/api && RELIEFMESH_TEST_DATABASE_URL="$(TEST_DB_URL)" go test -race -count=1 ./...

test-web: ## Frontend and package unit tests
	pnpm -r --if-present test

test-e2e: build-web ## Playwright end-to-end tests against a disposable stack
	cd apps/web && E2E_START_SERVER=1 RELIEFMESH_E2E_DATABASE_URL="$(E2E_DB_URL)" pnpm exec playwright test

lint: ## Lint and type-check everything
	./scripts/lint-all.sh

fmt: ## Format Go code
	cd apps/api && gofmt -w .

openapi: ## Validate the OpenAPI spec against the router
	./scripts/generate-openapi.sh

docker-build: ## Build the production container images
	docker build -t reliefmesh-api:dev apps/api
	docker build -t reliefmesh-web:dev -f apps/web/Dockerfile .

demo-up: ## Start the local demo on http://localhost:8080
	docker compose -f $(COMPOSE_DIR)/docker-compose.demo.yml up -d --build

demo-down: ## Stop the local demo (keeps data volume)
	docker compose -f $(COMPOSE_DIR)/docker-compose.demo.yml down
