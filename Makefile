GO_TOOLCHAIN := go$(shell sed -n 's/^go //p' go.mod)
GO_CACHE := $(CURDIR)/.cache/go-build
GOLANGCI_LINT_CACHE := $(CURDIR)/.cache/golangci-lint
GOOSE := $(shell go env GOPATH)/bin/goose
GOOSE_TIMEOUT := 100s
API_URL ?= http://localhost:$(or $(PORT),8080)

ifneq (,$(wildcard .env))
DATABASE_URL ?= $(shell sed -n 's/^DATABASE_URL=//p' .env)
MIGRATE_DATABASE_URL ?= $(shell sed -n 's/^MIGRATE_DATABASE_URL=//p' .env)
PORT ?= $(shell sed -n 's/^PORT=//p' .env)
SENTRY_DSN ?= $(shell sed -n 's/^SENTRY_DSN=//p' .env)
SHORT_URL_BASE ?= $(shell sed -n 's/^SHORT_URL_BASE=//p' .env)
export DATABASE_URL PORT SENTRY_DSN SHORT_URL_BASE API_URL
endif

ifeq ($(MIGRATE_DATABASE_URL),)
MIGRATE_DATABASE_URL := $(DATABASE_URL)
endif

GO_ENV := GOTOOLCHAIN=$(GO_TOOLCHAIN) GOCACHE=$(GO_CACHE)
LINT_ENV := $(GO_ENV) GOLANGCI_LINT_CACHE=$(GOLANGCI_LINT_CACHE)

.PHONY: dev backend-dev frontend-dev start build test lint sqlc migrate-up migrate-down migrate-status

dev:
	npm run dev

backend-dev:
	mkdir -p bin $(GO_CACHE)
	$(GO_ENV) air

frontend-dev:
	npm run frontend:dev

start:
	mkdir -p $(GO_CACHE)
	$(GO_ENV) go run .

build:
	mkdir -p bin $(GO_CACHE)
	$(GO_ENV) go build -o bin/link-shortener .

test:
	mkdir -p $(GO_CACHE)
	$(GO_ENV) go test -race ./...

lint:
	mkdir -p $(GO_CACHE) $(GOLANGCI_LINT_CACHE)
	$(LINT_ENV) golangci-lint run

sqlc:
	sqlc generate

migrate-up:
	@$(GOOSE) -timeout $(GOOSE_TIMEOUT) -dir sql/migrations postgres "$(MIGRATE_DATABASE_URL)" up

migrate-down:
	@$(GOOSE) -timeout $(GOOSE_TIMEOUT) -dir sql/migrations postgres "$(MIGRATE_DATABASE_URL)" down

migrate-status:
	@$(GOOSE) -timeout $(GOOSE_TIMEOUT) -dir sql/migrations postgres "$(MIGRATE_DATABASE_URL)" status
