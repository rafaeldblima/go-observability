.PHONY: help build up down logs test clean health

# Default target
help: ## Show this help message
	@echo "Available commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Build all services
	@echo "🔨 Building services..."
	docker-compose build

up: ## Start all services
	@echo "🚀 Starting all services..."
	docker-compose up -d
	@echo "✅ Services started!"
	@echo "📊 Zipkin UI: http://localhost:9411"
	@echo "🌡️  Service A: http://localhost:8080"
	@echo "🌍 Service B: http://localhost:8081"

down: ## Stop all services
	@echo "🛑 Stopping all services..."
	docker-compose down

logs: ## Show logs from all services
	docker-compose logs -f

logs-a: ## Show logs from Service A
	docker-compose logs -f service-a

logs-b: ## Show logs from Service B
	docker-compose logs -f service-b

logs-zipkin: ## Show logs from Zipkin
	docker-compose logs -f zipkin

test: ## Run system tests
	@echo "🧪 Running system tests..."
	@./test-system.sh

health: ## Check health of all services
	@echo "🏥 Checking service health..."
	@echo "Service A:"
	@curl -s http://localhost:8080/health | jq . || echo "Service A not responding"
	@echo "\nService B:"
	@curl -s http://localhost:8081/health | jq . || echo "Service B not responding"
	@echo "\nZipkin:"
	@curl -s http://localhost:9411/health || echo "Zipkin not responding"

clean: ## Clean up containers, images, and volumes
	@echo "🧹 Cleaning up..."
	docker-compose down -v --rmi all --remove-orphans
	docker system prune -f

dev-a: ## Run Service A in development mode
	@echo "🔧 Running Service A in development mode..."
	cd service-a && go run main.go

dev-b: ## Run Service B in development mode
	@echo "🔧 Running Service B in development mode..."
	cd service-b && WEATHER_API_KEY=${WEATHER_API_KEY} go run main.go

install-deps: ## Install Go dependencies for both services
	@echo "📦 Installing dependencies..."
	cd service-a && go mod tidy
	cd service-b && go mod tidy

zipkin: ## Open Zipkin UI in browser (macOS)
	open http://localhost:9411

# Example requests
example-valid: ## Send example request with valid CEP
	@echo "📮 Sending valid CEP request..."
	curl -X POST http://localhost:8080/ \
		-H "Content-Type: application/json" \
		-d '{"cep": "01310100"}' | jq .

example-invalid: ## Send example request with invalid CEP
	@echo "📮 Sending invalid CEP request..."
	curl -X POST http://localhost:8080/ \
		-H "Content-Type: application/json" \
		-d '{"cep": "123"}' | jq .
