# Database Intelligence MySQL - Monorepo Makefile
# Supports parallel operations for all modules

# WARNING: DO NOT ADD health check targets to this Makefile!
# Health checks have been intentionally removed from production code.
# Use shared/validation/health-check-all.sh for validation purposes.
# See shared/validation/README-health-check.md for details.

MODULES = core-metrics sql-intelligence wait-profiler anomaly-detector \
          business-impact replication-monitor performance-advisor \
          resource-monitor alert-manager canary-tester cross-signal-correlator

# Module groups for logical operations
CORE_MODULES = core-metrics resource-monitor
INTELLIGENCE_MODULES = sql-intelligence wait-profiler anomaly-detector
BUSINESS_MODULES = business-impact performance-advisor
REPLICATION_MODULES = replication-monitor
ENHANCED_MODULES = cross-signal-correlator alert-manager canary-tester

.PHONY: all build test run stop clean help
# WARNING: Do NOT add 'health' to .PHONY list!
# Health checks are validation-only, not production targets.

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
	@echo "  make run-enhanced   - Run all modules with enhanced configs"
	@echo "  make stop-all       - Stop all modules"
	@echo "  make clean          - Clean all modules"
	@echo ""
	@echo "Module group commands:"
	@echo "  make run-core       - Run core modules (metrics, resource)"
	@echo "  make run-intelligence - Run intelligence modules"
	@echo "  make run-business   - Run business modules"
	@echo "  make run-correlation - Run cross-signal correlation"
	@echo ""
	@echo "Individual module commands:"
	@echo "  make build-<module> - Build specific module"
	@echo "  make test-<module>  - Test specific module"
	@echo "  make run-<module>   - Run specific module"
	@echo "  make run-enhanced-<module> - Run with enhanced config"
	@echo "  make stop-<module>  - Stop specific module"
	@echo ""
	@echo "Utility commands:"
	@echo "  make validate       - Validate all modules (see shared/validation/)"
	@echo "  make logs-<module>  - View logs for specific module"
	@echo "  make integration    - Run integration tests"
	@echo "  make validate-configs - Validate all configurations"
	@echo ""
	# Health checks are available in shared/validation/ directory
	# Use: ./shared/validation/health-check-all.sh
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
run-all: validate-env
	@echo "Starting all modules..."
	@cd integration && docker-compose -f docker-compose.all.yaml up -d
	@echo "✓ All modules started"
	@echo "Run './shared/validation/health-check-all.sh' to check module status"

# Run all modules with enhanced configurations
run-enhanced:
	@echo "Starting all modules with enhanced configurations..."
	@cd integration && docker-compose -f docker-compose.enhanced.yaml up -d
	@echo "✓ All modules started with enhanced features"
	@echo "Run './shared/validation/health-check-all.sh' to check module status"

# Stop all modules
stop-all:
	@echo "Stopping all modules..."
	@cd integration && docker-compose -f docker-compose.all.yaml down 2>/dev/null || true
	@cd integration && docker-compose -f docker-compose.enhanced.yaml down 2>/dev/null || true
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

run-correlation:
	@echo "Starting cross-signal correlation..."
	@cd modules/cross-signal-correlator && make run
	@echo "✓ Cross-signal correlation started"

# Pattern rules for individual modules
build-%:
	@echo "Building $*..."
	@if [ -d "modules/$*" ]; then \
		cd modules/$* && make build; \
	else \
		echo "Module $* not found"; \
		exit 1; \
	fi

test-%:
	@echo "Testing $*..."
	@if [ -d "modules/$*" ]; then \
		cd modules/$* && make test; \
	else \
		echo "Module $* not found"; \
		exit 1; \
	fi

run-%:
	@echo "Running $*..."
	@if [ -d "modules/$*" ]; then \
		cd modules/$* && make run; \
	else \
		echo "Module $* not found"; \
		exit 1; \
	fi

run-enhanced-%:
	@echo "Running $* with enhanced configuration..."
	@if [ -d "modules/$*" ]; then \
		cd modules/$* && make run-enhanced 2>/dev/null || make run; \
	else \
		echo "Module $* not found"; \
		exit 1; \
	fi

stop-%:
	@echo "Stopping $*..."
	@if [ -d "modules/$*" ]; then \
		cd modules/$* && make stop; \
	else \
		echo "Module $* not found"; \
		exit 1; \
	fi

logs-%:
	@if [ -d "modules/$*" ]; then \
		cd modules/$* && make logs; \
	else \
		echo "Module $* not found"; \
		exit 1; \
	fi

clean-%:
	@echo "Cleaning $*..."
	@if [ -d "modules/$*" ]; then \
		cd modules/$* && make clean; \
	else \
		echo "Module $* not found"; \
		exit 1; \
	fi

status-%:
	@if [ -d "modules/$*" ]; then \
		cd modules/$* && make status 2>/dev/null || echo "$* is not running"; \
	else \
		echo "Module $* not found"; \
	fi

# Utility commands
validate:
	@echo "Running module validation..."
	@./shared/validation/health-check-all.sh

integration:
	@echo "Running integration tests..."
	@cd integration && docker-compose -f docker-compose.all.yaml up --build --abort-on-container-exit integration-tests
	@cd integration && docker-compose -f docker-compose.all.yaml down

integration-enhanced:
	@echo "Running enhanced integration tests..."
	@cd integration && docker-compose -f docker-compose.enhanced.yaml up --build --abort-on-container-exit
	@sleep 30  # Allow time for all services to stabilize
	@echo "✓ Enhanced integration running. Use 'make logs-integration' to view"

# Validate all configurations
validate-configs:
	@echo "Validating all collector configurations..."
	@for module in $(MODULES); do \
		echo "Checking $$module..."; \
		if [ -f "modules/$$module/config/collector.yaml" ]; then \
			docker run --rm -v $(PWD)/modules/$$module/config:/config \
				otel/opentelemetry-collector-contrib:latest \
				validate --config /config/collector.yaml || exit 1; \
		fi; \
		if [ -f "modules/$$module/config/collector-enhanced.yaml" ]; then \
			echo "Checking $$module enhanced config..."; \
			docker run --rm -v $(PWD)/modules/$$module/config:/config \
				otel/opentelemetry-collector-contrib:latest \
				validate --config /config/collector-enhanced.yaml || exit 1; \
		fi; \
	done
	@echo "✓ All configurations valid"

# Development helpers
dev-setup:
	@echo "Setting up development environment..."
	@chmod +x shared/scripts/*.sh
	@docker network create database-intelligence-monorepo_db-intelligence 2>/dev/null || true
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

run-full-stack: run-enhanced
	@echo "✓ Full enhanced stack started"

# Performance testing
perf-test:
	@echo "Running performance tests..."
	@cd modules/core-metrics && make run
	@sleep 5
	@cd modules/sql-intelligence && make run
	@sleep 5
	@echo "Generating load..."
	@cd modules/wait-profiler && make generate-load 2>/dev/null || echo "Load generation not available"
	@echo "✓ Performance test started"

# CI/CD helpers
ci-build:
	@$(MAKE) build -j$(words $(MODULES))

ci-test:
	@$(MAKE) test -j$(words $(MODULES))

ci-integration:
	@$(MAKE) integration

ci-validate:
	@$(MAKE) validate-configs

# Validate environment variables
validate-env:
	@./scripts/validate-environment.sh

# Docker cleanup
docker-clean:
	@echo "Cleaning up Docker resources..."
	@docker-compose down -v 2>/dev/null || true
	@$(foreach module,$(MODULES),cd modules/$(module) && docker-compose down -v 2>/dev/null || true &&) true
	@docker system prune -f
	@echo "✓ Docker cleanup complete"

# View implementation status
status:
	@cat IMPLEMENTATION_STATUS.md

# Create network if it doesn't exist
network:
	@docker network create database-intelligence-monorepo_db-intelligence 2>/dev/null || echo "Network already exists"