# WARNING: DO NOT ADD health check targets to this Makefile!
# Health checks have been intentionally removed from production code.
# Use shared/validation/health-check-all.sh for validation purposes.
# See shared/validation/README-health-check.md for details.

.PHONY: help build up down restart logs clean test
# WARNING: Do NOT add 'health' to .PHONY list!
# Health checks are validation-only, not production targets.

# Default target
help:
	@echo "Business Impact Module - Make targets:"
	@echo "  make build    - Build the Docker image"
	@echo "  make up       - Start the services"
	@echo "  make down     - Stop the services"
	@echo "  make restart  - Restart the services"
	@echo "  make logs     - View service logs"
	@echo "  make clean    - Clean up resources"
	@echo "  make test     - Run integration tests"
	# Health checks are available in shared/validation/ directory
	# Use: ../shared/validation/health-check-all.sh

# Build the Docker image
build:
	docker-compose build

# Start services
up:
	docker-compose up -d
	@echo "Business Impact module started on port 8085"
	@echo "OTLP gRPC: 4317, OTLP HTTP: 4318"
	@echo "Prometheus metrics: http://localhost:8888/metrics"

# Stop services
down:
	docker-compose down

# Restart services
restart: down up

# View logs
logs:
	docker-compose logs -f

# Clean up
clean:
	docker-compose down -v
	docker system prune -f

# Test the module
test:
	@echo "Testing OTLP endpoint..."
	# WARNING: Health check endpoint deprecated - use ../shared/validation/health-check-all.sh
	@curl -f http://localhost:13133/health || echo "Health check failed"
	@echo "\nTesting Prometheus metrics..."
	@curl -s http://localhost:8888/metrics | grep -E "business_impact|business_category" | head -5 || echo "No business metrics found yet"


# Send sample metrics for testing
test-metrics:
	@echo "Sending test metrics with business patterns..."
	@docker run --rm --network intelligence-network \
		-e OTEL_EXPORTER_OTLP_ENDPOINT=http://business-impact:4317 \
		-e OTEL_SERVICE_NAME=test-service \
		otel/opentelemetry-collector-contrib:0.91.0 \
		--config=/dev/null \
		--set=service.telemetry.logs.level=debug \
		--set=receivers.otlp.protocols.grpc.endpoint=0.0.0.0:4317 \
		--set=exporters.logging.loglevel=debug \
		--set=service.pipelines.metrics.receivers=[otlp] \
		--set=service.pipelines.metrics.exporters=[logging]