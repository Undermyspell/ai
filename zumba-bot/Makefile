# Lokale Dev-Umgebung: Postgres + Evolution API laufen im Cluster (siehe
# whatsapp-bot/.env) — alles andere startet lokal: Renderer (docker-compose)
# plus Bot, Admin-UI, Wrapped und Classifier via dev-local.sh.
.PHONY: help dev stack up down renderer logs test

help: ## Zeigt diese Hilfe an
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

dev: stack ## Alias für `stack` (voller Stack)

stack: up ## Startet alles: Renderer (Docker) + Bot, Admin-UI, Wrapped, Classifier (Strg-C beendet)
	@./dev-local.sh

up: ## Startet den Renderer (docker compose, detached)
	@echo "🐳 Starte Renderer..."
	@docker compose up -d renderer

down: ## Stoppt die lokalen Compose-Services
	@docker compose down

renderer: ## Baut das Renderer-Image neu (nach Änderungen an renderer-service/)
	@docker compose up -d --build renderer

logs: ## Compose-Logs verfolgen
	@docker compose logs -f

test: ## Tests aller Go-Module
	@echo "🧪 whatsapp-bot..." && cd whatsapp-bot && go test ./...
	@echo "🧪 zumba-admin-ui..." && cd zumba-admin-ui && go test ./...
	@echo "🧪 renderer-service..." && cd renderer-service && go test ./...
	@echo "🧪 shared..." && cd shared && go test ./...
	@echo "🧪 wrapped..." && cd wrapped && go test ./...
	@echo "🧪 classifier-service..." && cd classifier-service && go test ./...

.DEFAULT_GOAL := help
