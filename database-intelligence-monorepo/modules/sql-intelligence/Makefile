MODULE_NAME = sql-intelligence
PORT = 8082

# WARNING: DO NOT ADD health check targets to this Makefile!
# Health checks have been intentionally removed from production code.
# Use shared/validation/health-check-all.sh for validation purposes.
# See shared/validation/README-health-check.md for details.

.PHONY: build test run run-enhanced stop logs clean test-unit test-integration test-integration-cleanup validate-enhanced help
# WARNING: Do NOT add 'health' to .PHONY list!
# Health checks are validation-only, not production targets.

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

test-integration: test-integration-cleanup
	@echo "=== SQL Intelligence Integration Test ==="
	
	# Start MySQL and wait for it
	docker-compose up -d mysql-test
	@echo "Waiting for MySQL to be ready..."
	@for i in {1..30}; do \
		docker-compose exec mysql-test mysqladmin ping -h localhost --silent && break || \
		(echo "Waiting for MySQL... $$i/30" && sleep 2); \
	done
	
	# Generate test load
	@echo "Generating test query patterns..."
	@chmod +x scripts/generate-test-load.sh
	@MYSQL_HOST=localhost MYSQL_PORT=$$(docker-compose port mysql-test 3306 | cut -d: -f2) \
		./scripts/generate-test-load.sh || true
	
	# Start sql-intelligence module
	docker-compose up -d sql-intelligence
	@echo "Waiting for sql-intelligence to collect metrics..."
	@sleep 45
	
	# Validate metrics
	@echo "Validating intelligence metrics..."
	@METRICS=$$(curl -s localhost:$(PORT)/metrics); \
	echo "$$METRICS" | grep -q "mysql_query_cost_score" || \
		(echo "FAIL: Query cost scores not found" && exit 1); \
	echo "$$METRICS" | grep -q "mysql_table_iops_estimate" || \
		(echo "FAIL: Table access patterns not found" && exit 1); \
	echo "$$METRICS" | grep -q "mysql_index_effectiveness_score" || \
		(echo "FAIL: Index effectiveness not found" && exit 1); \
	echo "$$METRICS" | grep -q "recommendation_level" || \
		(echo "FAIL: Recommendations not generated" && exit 1); \
	echo "$$METRICS" | grep -q 'business_impact_score="[0-9]' || \
		(echo "FAIL: Business impact scores not calculated" && exit 1)
	
	@echo "✓ Integration test passed!"
	@docker-compose down

test-integration-cleanup:
	@docker-compose down -v 2>/dev/null || true

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