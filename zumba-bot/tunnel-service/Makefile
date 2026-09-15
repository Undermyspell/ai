.PHONY: help build run dev test tidy

help: ## Zeigt diese Hilfe an
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Baut die Applikation
	@go build -o ./tmp/server ./cmd/server

run: build ## Baut und startet den Service
	@./tmp/server

dev: build ## Startet mit Attrappe statt echtem ngrok-Agent (kein Authtoken nötig)
	@MOCK=true TUNNEL_TARGETS='[{"name":"wrapped","label":"Wrapped","addr":"localhost:8081"},{"name":"admin","label":"Admin-UI","addr":"localhost:8080"}]' ./tmp/server

test: ## Führt Tests aus
	@go test ./...

tidy: ## go mod tidy
	@go mod tidy

.DEFAULT_GOAL := help
