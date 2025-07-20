MODULE_NAME = sql-intelligence
PORT = 8082

.PHONY: build test run run-enhanced stop logs clean test-unit test-integration validate-enhanced help

help:
	@echo "SQL Intelligence Module - Available Commands:"
	@echo "  make build           - Build Docker images"
	@echo "  make run             - Run with standard configuration"
	@echo "  make run-enhanced    - Run with enhanced configuration"
	@echo "  make run-with-core   - Run with core-metrics integration"
	@echo "  make stop            - Stop all services"
	@echo "  make logs            - View service logs"
	@echo "  make clean           - Clean up containers and volumes"
	@echo "  make test            - Run all tests"
	@echo "  make validate-enhanced - Validate enhanced collector config"

build:
	docker-compose build

test: test-unit test-integration

test-unit:
	@echo "Running unit tests for $(MODULE_NAME)..."
	@if [ -d "tests" ] && [ -n "$$(ls -A tests/*.py 2>/dev/null)" ]; then \
		docker-compose run --rm $(MODULE_NAME) pytest tests/; \
	else \
		echo "No unit tests found"; \
	fi

test-integration:
	@echo "Running integration tests for $(MODULE_NAME)..."
	docker-compose up -d
	@sleep 15
	@curl -f http://localhost:$(PORT)/metrics || (docker-compose logs && exit 1)
	docker-compose down

run:
	docker-compose up -d
	@echo "$(MODULE_NAME) is running on port $(PORT)"
	@echo "Metrics available at: http://localhost:$(PORT)/metrics"

run-enhanced:
	@echo "Running $(MODULE_NAME) with enhanced configuration..."
	COLLECTOR_CONFIG=collector-enhanced.yaml docker-compose up -d
	@echo "$(MODULE_NAME) is running on port $(PORT) with enhanced configuration"
	@echo "Metrics available at: http://localhost:$(PORT)/metrics"

run-with-core:
	@echo "Running with core-metrics integration..."
	METRICS_ENDPOINT=host.docker.internal:8081 docker-compose up -d

stop:
	docker-compose down

logs:
	docker-compose logs -f $(MODULE_NAME)

clean:
	docker-compose down -v
	docker rmi $(MODULE_NAME):latest 2>/dev/null || true

status:
	@curl -s http://localhost:$(PORT)/metrics | grep -E "mysql_query|mysql_table" | head -20 || echo "Module not running"

validate-enhanced:
	@echo "Validating enhanced collector configuration..."
	@docker run --rm -v $$(pwd)/config:/config otel/opentelemetry-collector-contrib:latest validate --config=/config/collector-enhanced.yaml