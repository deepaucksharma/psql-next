.PHONY: help build up down restart logs shell clean test health

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
	@echo "  make health   - Check service health"

# Build the Docker image
build:
	docker-compose build --no-cache

# Start the service
up:
	docker-compose up -d
	@echo "Performance Advisor started on http://localhost:8087"
	@echo "Prometheus metrics available at http://localhost:8888/metrics"
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

# Check health
health:
	@curl -s http://localhost:13133/ > /dev/null && echo "✓ Performance Advisor is healthy" || echo "✗ Performance Advisor is not responding"
	@curl -s http://localhost:8888/metrics > /dev/null && echo "✓ Prometheus metrics endpoint is healthy" || echo "✗ Prometheus metrics endpoint is not responding"

# View current recommendations
recommendations:
	@echo "Fetching current performance recommendations..."
	@curl -s http://localhost:8888/metrics | grep "db_performance_recommendation" || echo "No recommendations available yet"

# Monitor recommendations in real-time
monitor:
	@watch -n 5 'curl -s http://localhost:8888/metrics | grep "db_performance_recommendation" | head -20'