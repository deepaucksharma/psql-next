# WARNING: DO NOT ADD health check targets to this Makefile!
# Health checks have been intentionally removed from production code.
# Use shared/validation/health-check-all.sh for validation purposes.
# See shared/validation/README-health-check.md for details.

.PHONY: help build up down restart logs shell clean test
# WARNING: Do NOT add 'health' to .PHONY list!
# Health checks are validation-only, not production targets.

# Default target
help:
	@echo "Performance Advisor Module - Available commands:"
	@echo "  make build    - Build the Docker image"
	@echo "  make up       - Start the performance advisor service"
	@echo "  make down     - Stop the performance advisor service"
	@echo "  make restart  - Restart the performance advisor service"
	@echo "  make logs     - View service logs"
	@echo "  make shell    - Open a shell in the container"
	@echo "  make clean    - Remove containers and volumes"
	@echo "  make test     - Test the configuration"
	# Health checks are available in shared/validation/ directory
	# Use: ../shared/validation/health-check-all.sh

# Build the Docker image
build:
	docker-compose build --no-cache

# Start the service
up:
	docker-compose up -d
	@echo "Performance Advisor started on http://localhost:8087"
	@echo "Prometheus metrics available at http://localhost:8888/metrics"
	# WARNING: Health check endpoint deprecated - use ../shared/validation/health-check-all.sh
	@echo "Health check at http://localhost:13133/"

# Stop the service
down:
	docker-compose down

# Restart the service
restart: down up

# View logs
logs:
	docker-compose logs -f performance-advisor

# Open shell in container
shell:
	docker-compose exec performance-advisor /bin/sh

# Clean up
clean:
	docker-compose down -v
	docker rmi -f performance-advisor:latest || true

# Test configuration
test:
	@echo "Testing collector configuration..."
	docker run --rm \
		-v $(PWD)/config:/etc/otelcol-contrib \
		otel/opentelemetry-collector-contrib:0.91.0 \
		validate --config=/etc/otelcol-contrib/collector.yaml


# View current recommendations
recommendations:
	@echo "Fetching current performance recommendations..."
	@curl -s http://localhost:8888/metrics | grep "db_performance_recommendation" || echo "No recommendations available yet"

# Monitor recommendations in real-time
monitor:
	@watch -n 5 'curl -s http://localhost:8888/metrics | grep "db_performance_recommendation" | head -20'