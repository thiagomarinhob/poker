include .env
export

GO      := /usr/local/go/bin/go
GOBIN   := $(HOME)/go/bin
MIGRATE := $(GOBIN)/migrate
SQLC    := $(GOBIN)/sqlc
BACKEND := poker-backend

.PHONY: up down logs db-shell \
        migrate-up migrate-down migrate-create \
        sqlc build test

## ── Docker ──────────────────────────────────────────────────────────────────

up:
	docker compose up -d

down:
	docker compose down

logs:
	docker compose logs -f

## ── Database ─────────────────────────────────────────────────────────────────

db-shell:
	docker compose exec postgres psql -U $(POSTGRES_USER) -d $(POSTGRES_DB)

# Apply all pending migrations
migrate-up:
	$(MIGRATE) -path $(BACKEND)/migrations -database "$(DATABASE_URL)" up

# Rollback last migration
migrate-down:
	$(MIGRATE) -path $(BACKEND)/migrations -database "$(DATABASE_URL)" down 1

# Create a new migration pair: make migrate-create name=add_sessions
migrate-create:
	@test -n "$(name)" || (echo "Usage: make migrate-create name=<migration_name>" && exit 1)
	$(MIGRATE) create -ext sql -dir $(BACKEND)/migrations -seq $(name)

## ── Code generation ──────────────────────────────────────────────────────────

sqlc:
	cd $(BACKEND) && $(SQLC) generate

## ── Backend ──────────────────────────────────────────────────────────────────

build:
	$(GO) build -o $(BACKEND)/tmp/server ./$(BACKEND)/cmd/server

test:
	$(GO) test ./$(BACKEND)/...

## ── Setup ────────────────────────────────────────────────────────────────────

.env:
	@echo "Copying .env.example to .env — fill in your secrets"
	cp .env.example .env
