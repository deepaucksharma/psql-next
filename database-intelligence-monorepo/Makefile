# Database Intelligence MySQL - Monorepo Makefile
# Supports parallel operations for all modules

MODULES = core-metrics sql-intelligence wait-profiler anomaly-detector \
          business-impact replication-monitor performance-advisor \
          resource-monitor

# Module groups for logical operations
CORE_MODULES = core-metrics resource-monitor
INTELLIGENCE_MODULES = sql-intelligence wait-profiler anomaly-detector
BUSINESS_MODULES = business-impact performance-advisor
REPLICATION_MODULES = replication-monitor

.PHONY: all build test run stop clean help

# Default target
all: build

# Help target
help:
	@echo "Database Intelligence MySQL - Monorepo Commands"
	@echo "=============================================="
	@echo "Global commands:"
	@echo "  make build          - Build all modules in parallel"
	@echo "  make test           - Test all modules in parallel"
	@echo "  make run-all        - Run all modules"
	@echo "  make stop-all       - Stop all modules"
	@echo "  make clean          - Clean all modules"
	@echo ""
	@echo "Module group commands:"
	@echo "  make run-core       - Run core modules (metrics, resource)"
	@echo "  make run-intelligence - Run intelligence modules"
	@echo "  make run-business   - Run business modules"
	@echo ""
	@echo "Individual module commands:"
	@echo "  make build-<module> - Build specific module"
	@echo "  make test-<module>  - Test specific module"
	@echo "  make run-<module>   - Run specific module"
	@echo "  make stop-<module>  - Stop specific module"
	@echo ""
	@echo "Utility commands:"
	@echo "  make health         - Check health of all modules"
	@echo "  make logs-<module>  - View logs for specific module"
	@echo "  make integration    - Run integration tests"
	@echo ""
	@echo "Available modules: $(MODULES)"

# Build all modules in parallel
build:
	@echo "Building all modules in parallel..."
	@$(MAKE) $(foreach module,$(MODULES),build-$(module)) -j$(words $(MODULES))
	@echo "✓ All modules built successfully"

# Test all modules in parallel
test:
	@echo "Testing all modules in parallel..."
	@$(MAKE) $(foreach module,$(MODULES),test-$(module)) -j$(words $(MODULES))
	@echo "✓ All module tests passed"

# Run all modules
run-all:
	@echo "Starting all modules..."
	@cd integration && docker-compose -f docker-compose.all.yaml up -d
	@echo "✓ All modules started"
	@echo "Run 'make health' to check module status"

# Stop all modules
stop-all:
	@echo "Stopping all modules..."
	@cd integration && docker-compose -f docker-compose.all.yaml down
	@echo "✓ All modules stopped"

# Clean all modules
clean:
	@echo "Cleaning all modules..."
	@$(MAKE) $(foreach module,$(MODULES),clean-$(module)) -j$(words $(MODULES))
	@echo "✓ All modules cleaned"

# Module group operations
run-core:
	@echo "Starting core modules..."
	@$(foreach module,$(CORE_MODULES),cd modules/$(module) && make run &&) true
	@echo "✓ Core modules started"

run-intelligence:
	@echo "Starting intelligence modules..."
	@$(foreach module,$(INTELLIGENCE_MODULES),cd modules/$(module) && make run &&) true
	@echo "✓ Intelligence modules started"

run-business:
	@echo "Starting business modules..."
	@$(foreach module,$(BUSINESS_MODULES),cd modules/$(module) && make run &&) true
	@echo "✓ Business modules started"

# Pattern rules for individual modules
build-%:
	@echo "Building $*..."
	@cd modules/$* && make build

test-%:
	@echo "Testing $*..."
	@cd modules/$* && make test

run-%:
	@echo "Running $*..."
	@cd modules/$* && make run

stop-%:
	@echo "Stopping $*..."
	@cd modules/$* && make stop

logs-%:
	@cd modules/$* && make logs

clean-%:
	@echo "Cleaning $*..."
	@cd modules/$* && make clean

status-%:
	@cd modules/$* && make status 2>/dev/null || echo "$* is not running"

# Utility commands
health:
	@./shared/scripts/health-check.sh

integration:
	@echo "Running integration tests..."
	@cd integration && docker-compose -f docker-compose.all.yaml up --build --abort-on-container-exit integration-tests
	@cd integration && docker-compose -f docker-compose.all.yaml down

# Development helpers
dev-setup:
	@echo "Setting up development environment..."
	@chmod +x shared/scripts/*.sh
	@echo "✓ Development environment ready"

# Quick test of a single module
quick-test-%:
	@./shared/scripts/test-module.sh $*

# Run specific module combinations
run-monitoring: run-core run-resource-monitor
	@echo "✓ Monitoring stack started"

run-analysis: run-sql-intelligence run-wait-profiler run-anomaly-detector
	@echo "✓ Analysis stack started"

run-advisory: run-performance-advisor run-business-impact
	@echo "✓ Advisory stack started"

# Performance testing
perf-test:
	@echo "Running performance tests..."
	@cd modules/core-metrics && make run
	@sleep 5
	@cd modules/sql-intelligence && make run
	@sleep 5
	@echo "Generating load..."
	@cd modules/wait-profiler && make generate-load
	@echo "✓ Performance test started"

# CI/CD helpers
ci-build:
	@$(MAKE) build -j$(words $(MODULES))

ci-test:
	@$(MAKE) test -j$(words $(MODULES))

ci-integration:
	@$(MAKE) integration

# Docker cleanup
docker-clean:
	@echo "Cleaning up Docker resources..."
	@docker-compose down -v 2>/dev/null || true
	@docker system prune -f
	@echo "✓ Docker cleanup complete"