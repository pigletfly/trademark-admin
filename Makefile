.PHONY: help install dev api web up down logs build test fmt tidy up-gotenberg e2e

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
	@echo "  e2e           run Playwright E2E (requires docker compose up -d)"

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

e2e:
	@echo "→ Checking api health at http://localhost:8080/health"
	@curl -fsS http://localhost:8080/health > /dev/null 2>&1 || ( \
		echo "API not responding. Run 'docker compose up -d' first."; exit 1 \
	)
	@echo "→ Checking web at http://localhost:5173"
	@curl -fsS http://localhost:5173/ > /dev/null 2>&1 || ( \
		echo "Web not responding. Run 'docker compose up -d' first."; exit 1 \
	)
	pnpm -C packages/e2e test
