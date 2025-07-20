# WARNING: DO NOT ADD health check targets to this Makefile!
# Health checks have been intentionally removed from production code.
# Use shared/validation/health-check-all.sh for validation purposes.
# See shared/validation/README-health-check.md for details.

.PHONY: help build up down restart logs shell test clean validate config-test
# WARNING: Do NOT add 'health' to .PHONY list!
# Health checks are validation-only, not production targets.

# Default target
.DEFAULT_GOAL := help

# Service name
SERVICE_NAME := alert-manager
COMPOSE_FILE := docker-compose.yaml

help: ## Show this help message
	@echo "Alert Manager Module - Available targets:"
	# Health checks are available in shared/validation/ directory
	# Use: ../shared/validation/health-check-all.sh
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Build the alert manager container
	@echo "Building $(SERVICE_NAME)..."
	docker-compose -f $(COMPOSE_FILE) build

up: ## Start the alert manager service
	@echo "Starting $(SERVICE_NAME)..."
	docker-compose -f $(COMPOSE_FILE) up -d
	@echo "$(SERVICE_NAME) is running on:"
	@echo "  - OTLP gRPC: http://localhost:8089"
	@echo "  - OTLP HTTP: http://localhost:4318"
	@echo "  - Prometheus: http://localhost:9091/metrics"
	# WARNING: Health check endpoint deprecated - use ../shared/validation/health-check-all.sh
	@echo "  - Health: http://localhost:13134/health"

down: ## Stop the alert manager service
	@echo "Stopping $(SERVICE_NAME)..."
	docker-compose -f $(COMPOSE_FILE) down

restart: down up ## Restart the alert manager service

logs: ## Show logs from the alert manager service
	docker-compose -f $(COMPOSE_FILE) logs -f $(SERVICE_NAME)

shell: ## Open a shell in the alert manager container
	docker-compose -f $(COMPOSE_FILE) exec $(SERVICE_NAME) sh

test: ## Run basic connectivity tests
	@echo "Testing $(SERVICE_NAME) endpoints..."
	@echo -n "Checking health endpoint... "
	# WARNING: Health check endpoint deprecated - use ../shared/validation/health-check-all.sh
	@curl -s -f http://localhost:13134/health > /dev/null && echo "✓ OK" || echo "✗ FAILED"
	@echo -n "Checking OTLP HTTP endpoint... "
	@curl -s -f -X POST http://localhost:4318/v1/metrics -H "Content-Type: application/json" -d '{}' > /dev/null 2>&1 || echo "✓ OK (expected 400)"
	@echo -n "Checking Prometheus endpoint... "
	@curl -s -f http://localhost:9091/metrics > /dev/null && echo "✓ OK" || echo "✗ FAILED"

clean: ## Clean up volumes and generated files
	@echo "Cleaning up $(SERVICE_NAME) volumes..."
	docker-compose -f $(COMPOSE_FILE) down -v
	@echo "Cleanup complete"

validate: ## Validate the collector configuration
	@echo "Validating collector configuration..."
	@docker run --rm -v $(PWD)/config:/config otel/opentelemetry-collector-contrib:0.91.0 validate --config=/config/collector.yaml

config-test: ## Test the configuration by running the collector in dry-run mode
	@echo "Testing collector configuration..."
	@docker run --rm \
		-v $(PWD)/config:/config \
		-e WEBHOOK_ENDPOINT=http://localhost:9999/alerts \
		-e ALERT_FILE_PATH=/tmp/alerts.json \
		otel/opentelemetry-collector-contrib:0.91.0 \
		--config=/config/collector.yaml \
		--dry-run


# Development targets
dev-webhook: ## Start a simple webhook receiver for testing
	@echo "Starting webhook receiver on http://localhost:9999..."
	@python3 -m http.server 9999

send-test-alert: ## Send a test alert to the alert manager
	@echo "Sending test alert..."
	@curl -X POST http://localhost:4318/v1/metrics \
		-H "Content-Type: application/json" \
		-d '{"resourceMetrics":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"test-service"}}]},"scopeMetrics":[{"scope":{"name":"test"},"metrics":[{"name":"alert.database.high_cpu","gauge":{"dataPoints":[{"asDouble":95,"timeUnixNano":"'$$(date +%s%N)'","attributes":[{"key":"alert.severity","value":{"stringValue":"critical"}},{"key":"db.system","value":{"stringValue":"mysql"}},{"key":"alert.type","value":{"stringValue":"performance"}}]}]}}]}]}]}'

view-alerts: ## View the latest alerts from the file exporter
	@echo "Latest alerts:"
	@docker-compose -f $(COMPOSE_FILE) exec $(SERVICE_NAME) tail -n 50 /var/log/alerts/alerts.json 2>/dev/null || echo "No alerts found or service not running"

metrics: ## View Prometheus metrics
	@echo "Alert Manager metrics:"
	@curl -s http://localhost:9091/metrics | grep -E "^alert_manager_" || echo "No metrics available"