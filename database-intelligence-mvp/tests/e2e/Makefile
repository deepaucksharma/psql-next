# E2E Test Makefile

.PHONY: all test test-unit test-integration test-performance test-benchmark clean help docker-up docker-down

# Default target
all: test

# Run all E2E tests
test:
	@echo "Running all E2E tests..."
	./run_e2e_tests.sh --mode all

# Run only unit tests (fast)
test-unit:
	@echo "Running unit tests..."
	go test -v -short ./...

# Run integration tests
test-integration:
	@echo "Running integration tests..."
	./run_e2e_tests.sh --mode integration

# Run performance tests
test-performance:
	@echo "Running performance tests..."
	./run_e2e_tests.sh --mode performance

# Run benchmarks
test-benchmark:
	@echo "Running benchmarks..."
	./run_e2e_tests.sh --mode benchmark

# Run specific test
test-specific:
	@echo "Running specific test: $(TEST)"
	go test -v -run $(TEST) -timeout 10m ./...

# Start test environment
docker-up:
	@echo "Starting test environment..."
	docker-compose -f testdata/docker-compose.test.yml up -d

# Stop test environment
docker-down:
	@echo "Stopping test environment..."
	docker-compose -f testdata/docker-compose.test.yml down -v

# Clean test artifacts
clean:
	@echo "Cleaning test artifacts..."
	rm -rf test-results/
	docker-compose -f testdata/docker-compose.test.yml down -v
	go clean -testcache

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Lint tests
lint:
	@echo "Linting test code..."
	golangci-lint run ./...

# Format test code
fmt:
	@echo "Formatting test code..."
	go fmt ./...

# Run tests in CI mode
ci:
	@echo "Running tests in CI mode..."
	./run_e2e_tests.sh --mode all --timeout 20m

# Generate test report
report:
	@echo "Generating test report..."
	@if [ -d "test-results/current" ]; then \
		echo "Test Summary:"; \
		echo "============="; \
		cat test-results/current/summary.txt 2>/dev/null || echo "No summary found"; \
	else \
		echo "No test results found. Run tests first."; \
	fi

# Watch mode - run tests on file changes
watch:
	@echo "Running tests in watch mode..."
	@which reflex >/dev/null || (echo "Installing reflex..." && go install github.com/cespare/reflex@latest)
	reflex -r '\.go$$' -s -- make test-unit

# Help
help:
	@echo "Database Intelligence Collector E2E Tests"
	@echo "========================================"
	@echo ""
	@echo "Available targets:"
	@echo "  make test              - Run all E2E tests"
	@echo "  make test-unit         - Run unit tests only (fast)"
	@echo "  make test-integration  - Run integration tests"
	@echo "  make test-performance  - Run performance tests"
	@echo "  make test-benchmark    - Run benchmarks"
	@echo "  make test-specific TEST=TestName - Run specific test"
	@echo "  make test-coverage     - Run tests with coverage report"
	@echo "  make docker-up         - Start test environment"
	@echo "  make docker-down       - Stop test environment"
	@echo "  make clean            - Clean test artifacts"
	@echo "  make lint             - Lint test code"
	@echo "  make fmt              - Format test code"
	@echo "  make ci               - Run tests in CI mode"
	@echo "  make report           - Generate test report"
	@echo "  make watch            - Run tests in watch mode"
	@echo "  make help             - Show this help message"
	@echo ""
	@echo "Environment variables:"
	@echo "  TEST_TIMEOUT          - Test timeout (default: 30m)"
	@echo "  TEST_MODE             - Test mode (all, unit, integration, performance, benchmark)"
	@echo "  BUILD_CONTAINERS      - Build containers before running (true/false)"

# Shortcuts
i: test-integration
p: test-performance
b: test-benchmark
u: test-unit