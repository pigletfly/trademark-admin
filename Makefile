.PHONY: help install dev api web up down logs build test fmt tidy up-gotenberg

help:
	@echo "Targets:"
	@echo "  install       pnpm install (web deps)"
	@echo "  dev           start web dev server"
	@echo "  api           run api locally (requires postgres)"
	@echo "  up            docker compose up -d (postgres+api+web)"
	@echo "  down          docker compose down"
	@echo "  logs          tail compose logs"
	@echo "  build         build all docker images"
	@echo "  test          run all tests (unit)"
	@echo "  fmt           format frontend code"
	@echo "  tidy          go mod tidy"
	@echo "  up-gotenberg  start only the gotenberg service (for local api dev)"

install:
	pnpm install

dev:
	pnpm -C apps/web dev

api:
	cd apps/api && go run ./cmd/server

up:
	docker compose up -d

down:
	docker compose down

logs:
	docker compose logs -f --tail=200

build:
	docker compose build

test:
	pnpm -C apps/web test
	cd apps/api && go test ./...

fmt:
	pnpm format

tidy:
	cd apps/api && go mod tidy

up-gotenberg:
	docker compose up -d gotenberg
