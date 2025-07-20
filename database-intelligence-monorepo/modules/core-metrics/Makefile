MODULE_NAME = core-metrics
PORT = 8081

# WARNING: DO NOT ADD health check targets to this Makefile!
# Health checks have been intentionally removed from production code.
# Use shared/validation/health-check-all.sh for validation purposes.
# See shared/validation/README-health-check.md for details.

.PHONY: build test run run-enhanced stop logs clean test-unit test-integration validate-enhanced help
# WARNING: Do NOT add 'health' to .PHONY list!
# Health checks are validation-only, not production targets.

help:
	@echo "Core Metrics Module - Available Commands:"
	@echo "  make build           - Build Docker images"
	@echo "  make run             - Run with standard configuration"
	@echo "  make run-enhanced    - Run with enhanced configuration"
	@echo "  make stop            - Stop all services"
	@echo "  make logs            - View service logs"
	@echo "  make clean           - Clean up containers and volumes"
	@echo "  make test            - Run all tests"
	@echo "  make validate-enhanced - Validate enhanced collector config"
	# Health checks are available in shared/validation/ directory
	# Use: ../shared/validation/health-check-all.sh

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
	@sleep 10
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

stop:
	docker-compose down

logs:
	docker-compose logs -f $(MODULE_NAME)

clean:
	docker-compose down -v
	docker rmi $(MODULE_NAME):latest 2>/dev/null || true

status:
	@curl -s http://localhost:$(PORT)/metrics | head -20 || echo "Module not running"

validate-enhanced:
	@echo "Validating enhanced collector configuration..."
	@docker run --rm -v $$(pwd)/config:/config otel/opentelemetry-collector-contrib:latest validate --config=/config/collector-enhanced.yaml