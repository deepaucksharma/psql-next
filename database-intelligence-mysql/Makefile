.PHONY: help test test-unit test-integration test-e2e validate-metrics clean docker-up docker-down

# Default target
help:
	@echo "MySQL Monitoring Test Targets:"
	@echo "  make test              - Run all tests"
	@echo "  make test-unit         - Run unit tests"
	@echo "  make test-integration  - Run integration tests"
	@echo "  make test-e2e          - Run end-to-end tests"
	@echo "  make validate-metrics  - Validate metrics collection"
	@echo "  make docker-up         - Start Docker environment"
	@echo "  make docker-down       - Stop Docker environment"
	@echo "  make clean             - Clean test artifacts"

# Test targets
test:
	./scripts/run-tests.sh all

test-unit:
	./scripts/run-tests.sh unit

test-integration:
	./scripts/run-tests.sh integration

test-e2e:
	./scripts/run-tests.sh e2e

validate-metrics:
	./scripts/validate-metrics.sh

# Docker targets
docker-up:
	docker-compose up -d
	@echo "Waiting for services to be ready..."
	@sleep 10

docker-down:
	docker-compose down -v

# Development targets
deps:
	cd tests && go mod download
	cd tests && go mod tidy

fmt:
	cd tests && go fmt ./...

lint:
	cd tests && golangci-lint run

# Clean target
clean:
	rm -f tests/coverage.out tests/coverage.html
	rm -f /tmp/mysql-metrics.json
	find . -name "*.test" -delete