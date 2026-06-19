.PHONY: help build run dev clean test tidy

help: ## Zeigt diese Hilfe an
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Baut die Applikation
	@echo "🔨 Building..."
	@go build -o ./tmp/server ./cmd/server
	@echo "✅ Build erfolgreich!"

run: build ## Baut und startet den Server
	@echo "🍻 Starte Zumba WhatsApp-Bot..."
	@./tmp/server

dev: ## Startet den Development Server mit Hot Reload (benötigt Air)
	@echo "🔥 Starting development server with Air..."
	@air

clean: ## Räumt Build-Artefakte auf
	@echo "🧹 Cleaning..."
	@rm -rf ./tmp
	@echo "✅ Clean erfolgreich!"

test: ## Führt Tests aus
	@echo "🧪 Running tests..."
	@go test -v ./...

tidy: ## go mod tidy
	@go mod tidy

install: ## Installiert Dependencies
	@echo "📦 Installing dependencies..."
	@go mod download
	@go install github.com/air-verse/air@latest
	@echo "✅ Dependencies installiert!"

.DEFAULT_GOAL := help
