# Makefile for Database Intelligence E2E Tests

.PHONY: all help setup verify test clean coverage report

# Default target
all: help

# Help target
help:
	@echo "Database Intelligence E2E Test Makefile"
	@echo ""
	@echo "Available targets:"
	@echo "  setup              - Set up test environment and build collector"
	@echo "  verify             - Verify New Relic connection"
	@echo "  test               - Run all E2E tests"
	@echo "  test-comprehensive - Run comprehensive test suite"
	@echo "  test-verification  - Run New Relic verification tests"
	@echo "  test-adapters      - Run adapter integration tests"
	@echo "  test-database      - Run database verification tests"
	@echo "  test-performance   - Run performance tests"
	@echo "  coverage           - Run tests with coverage"
	@echo "  report             - Generate test report"
	@echo "  clean              - Clean up test artifacts"
	@echo "  docker-up          - Start test infrastructure"
	@echo "  docker-down        - Stop test infrastructure"
	@echo "  docker-logs        - Show docker logs"
	@echo ""
	@echo "Environment variables:"
	@echo "  Copy .env.example to .env and configure your credentials"
	@echo ""

# Load environment variables
ifneq (,$(wildcard ./.env))
    include .env
    export
endif

# Setup target
setup:
	@echo "Setting up E2E test environment..."
	@if [ ! -f .env ]; then \
		echo "Creating .env file from example..."; \
		cp .env.example .env; \
		echo "Please edit .env file with your credentials"; \
	fi
	@echo "Building collector..."
	@cd ../../core/cmd/collector && go build -o collector .
	@echo "Setup complete!"

# Verify New Relic connection
verify:
	@echo "Verifying New Relic connection..."
	@go run verify_newrelic.go

# Test targets
test:
	@./run_e2e_tests.sh all

test-comprehensive:
	@./run_e2e_tests.sh comprehensive

test-verification:
	@./run_e2e_tests.sh verification

test-adapters:
	@./run_e2e_tests.sh adapters

test-database:
	@./run_e2e_tests.sh database

test-performance:
	@./run_e2e_tests.sh performance

# Coverage target
coverage:
	@COVERAGE_ENABLED=true ./run_e2e_tests.sh all
	@echo "Coverage report available at: coverage/coverage.html"

# Report target
report:
	@if [ -f coverage/e2e.coverage ]; then \
		go tool cover -html=coverage/e2e.coverage -o coverage/coverage.html; \
		echo "Coverage report generated: coverage/coverage.html"; \
		go tool cover -func=coverage/e2e.coverage | grep total; \
	else \
		echo "No coverage file found. Run 'make coverage' first."; \
	fi

# Docker targets
docker-up:
	@echo "Starting test infrastructure..."
	@docker-compose up -d

docker-down:
	@echo "Stopping test infrastructure..."
	@docker-compose down -v

docker-logs:
	@docker-compose logs -f

# Clean target
clean:
	@echo "Cleaning up test artifacts..."
	@rm -rf coverage/
	@rm -rf test-results/
	@rm -f test-report-*.txt
	@docker-compose down -v || true
	@echo "Cleanup complete!"

# Quick test with local databases
quick-test:
	@echo "Running quick E2E test with local databases..."
	@docker-compose up -d postgres
	@sleep 10  # Wait for postgres to be ready
	@go test -v -short -timeout=5m ./suites/...
	@docker-compose down

# Development helpers
dev-setup: setup docker-up
	@echo "Development environment ready!"
	@echo "Run 'make test' to start testing"

dev-teardown: docker-down clean
	@echo "Development environment cleaned up!"

# CI/CD targets
ci-test:
	@echo "Running CI tests..."
	@./run_e2e_tests.sh all

# Watch for changes and re-run tests
watch:
	@echo "Watching for changes..."
	@while true; do \
		inotifywait -e modify,create,delete -r . --exclude '\.git|coverage|test-results' 2>/dev/null || true; \
		clear; \
		echo "Changes detected, running tests..."; \
		go test -v -short -timeout=2m ./suites/... || true; \
		echo ""; \
		echo "Waiting for changes..."; \
	done

# Specific processor tests
test-processor-%:
	@echo "Testing $* processor..."
	@go test -v -timeout=10m -run "Test.*/$*" ./suites/...

# Database-specific tests
test-postgres:
	@echo "Running PostgreSQL-specific tests..."
	@MYSQL_ENABLED=false go test -v -timeout=15m -run "TestPostgreSQL\|TestDatabase" ./suites/...

test-mysql:
	@echo "Running MySQL-specific tests..."
	@go test -v -timeout=15m -run "TestMySQL" ./suites/...

# Performance profiling
profile:
	@echo "Running tests with profiling..."
	@go test -v -timeout=30m -cpuprofile=cpu.prof -memprofile=mem.prof -run TestPerformance ./suites/...
	@echo "CPU profile: cpu.prof"
	@echo "Memory profile: mem.prof"
	@echo "View with: go tool pprof cpu.prof"

# Stress test
stress-test:
	@echo "Running stress tests..."
	@STRESS_TEST=true QPS=1000 DURATION=10m ./run_e2e_tests.sh performance

# Generate test data
generate-data:
	@echo "Generating test data..."
	@go run -tags=tools ./tools/generate_test_data.go

# Validate configuration
validate-config:
	@echo "Validating test configuration..."
	@go run -tags=tools ./tools/validate_config.go -config e2e-test-config.yaml

.SILENT: help