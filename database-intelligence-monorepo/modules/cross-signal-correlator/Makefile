# Cross-Signal Correlator Module Makefile

# WARNING: DO NOT ADD health check targets to this Makefile!
# Health checks have been intentionally removed from production code.
# Use shared/validation/health-check-all.sh for validation purposes.
# See shared/validation/README-health-check.md for details.

MODULE_NAME := cross-signal-correlator
IMAGE_NAME := database-intelligence/$(MODULE_NAME)
IMAGE_TAG := latest
EXPORT_PORT := 8892
HEALTH_PORT := 13137

.PHONY: help build test run stop logs clean
# WARNING: Do NOT add 'health' to .PHONY list!
# Health checks are validation-only, not production targets.

help:
	@echo "Cross-Signal Correlator Module Commands:"
	@echo "  make build    - Build the Docker image"
	@echo "  make test     - Run module tests"
	@echo "  make run      - Run the module"
	@echo "  make stop     - Stop the module"
	@echo "  make logs     - View module logs"
	@echo "  make clean    - Clean up resources"
	# Health checks are available in shared/validation/ directory
	# Use: ../shared/validation/health-check-all.sh

build:
	@echo "Building $(MODULE_NAME) image..."
	docker build -t $(IMAGE_NAME):$(IMAGE_TAG) .

test:
	@echo "Running $(MODULE_NAME) tests..."
	@if [ -f tests/test_correlation.py ]; then \
		python -m pytest tests/test_correlation.py -v; \
	else \
		echo "No tests found, skipping..."; \
	fi

run:
	@echo "Starting $(MODULE_NAME)..."
	docker-compose up -d
	@echo "$(MODULE_NAME) started. Metrics available at http://localhost:$(EXPORT_PORT)"

run-enhanced:
	@echo "Starting $(MODULE_NAME) with enhanced configuration..."
	COLLECTOR_CONFIG=collector-enhanced.yaml docker-compose up -d
	@echo "$(MODULE_NAME) started with enhanced features. Metrics available at http://localhost:$(EXPORT_PORT)"

stop:
	@echo "Stopping $(MODULE_NAME)..."
	docker-compose down

logs:
	@echo "Showing $(MODULE_NAME) logs..."
	docker-compose logs -f

clean:
	@echo "Cleaning up $(MODULE_NAME) resources..."
	docker-compose down -v
	docker rmi $(IMAGE_NAME):$(IMAGE_TAG) || true


metrics:
	@echo "Fetching $(MODULE_NAME) metrics..."
	@curl -s http://localhost:$(EXPORT_PORT)/metrics | head -20

# Development commands
shell:
	@echo "Opening shell in $(MODULE_NAME) container..."
	docker-compose exec $(MODULE_NAME) sh

validate-config:
	@echo "Validating collector configuration..."
	docker run --rm -v $(PWD)/config:/config $(IMAGE_NAME):$(IMAGE_TAG) validate --config /config/collector.yaml

validate-enhanced:
	@echo "Validating enhanced collector configuration..."
	docker run --rm -v $(PWD)/config:/config $(IMAGE_NAME):$(IMAGE_TAG) validate --config /config/collector-enhanced.yaml