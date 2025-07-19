.PHONY: build start stop restart logs clean status test help

# Default target
.DEFAULT_GOAL := help

# Variables
COMPOSE_FILE := docker-compose.yaml
CONTAINER_NAME := resource-monitor-collector
IMAGE_NAME := resource-monitor:latest

## help: Display this help message
help:
	@echo "Resource Monitor Module Commands:"
	@echo ""
	@grep -E '^##' Makefile | sed 's/## //'

## build: Build the Docker image
build:
	@echo "Building Resource Monitor image..."
	docker-compose -f $(COMPOSE_FILE) build

## start: Start the resource monitor
start:
	@echo "Starting Resource Monitor..."
	docker-compose -f $(COMPOSE_FILE) up -d
	@echo "Resource Monitor is running on:"
	@echo "  - OTLP gRPC: localhost:8088"
	@echo "  - OTLP HTTP: localhost:8089"
	@echo "  - Prometheus metrics: http://localhost:8090/metrics"
	@echo "  - Health check: http://localhost:8091/"

## stop: Stop the resource monitor
stop:
	@echo "Stopping Resource Monitor..."
	docker-compose -f $(COMPOSE_FILE) down

## restart: Restart the resource monitor
restart: stop start

## logs: View collector logs
logs:
	docker-compose -f $(COMPOSE_FILE) logs -f

## logs-tail: View last 100 lines of logs
logs-tail:
	docker-compose -f $(COMPOSE_FILE) logs --tail=100

## status: Check the status of the resource monitor
status:
	@echo "Checking Resource Monitor status..."
	@docker ps -a | grep $(CONTAINER_NAME) || echo "Container not found"
	@echo ""
	@echo "Health check:"
	@curl -s http://localhost:8091/ && echo "Collector is healthy" || echo "Collector is not responding"

## metrics: Display current metrics
metrics:
	@echo "Fetching current metrics..."
	@curl -s http://localhost:8090/metrics | head -50
	@echo ""
	@echo "For full metrics, visit: http://localhost:8090/metrics"

## clean: Remove containers and volumes
clean:
	@echo "Cleaning up Resource Monitor..."
	docker-compose -f $(COMPOSE_FILE) down -v
	@rm -f /tmp/resource-metrics.json
	@rm -f /tmp/otel-collector.log

## test: Test the configuration
test:
	@echo "Testing collector configuration..."
	docker run --rm \
		-v $(PWD)/config/collector.yaml:/etc/otel-collector/config.yaml \
		otel/opentelemetry-collector-contrib:0.91.0 \
		validate --config=/etc/otel-collector/config.yaml

## shell: Open a shell in the running container
shell:
	docker exec -it $(CONTAINER_NAME) /bin/bash

## config-reload: Reload the configuration
config-reload:
	@echo "Reloading configuration..."
	docker-compose -f $(COMPOSE_FILE) restart

## monitor-mysql: Show MySQL-specific resource metrics
monitor-mysql:
	@echo "MySQL Process Resource Usage:"
	@docker exec $(CONTAINER_NAME) sh -c "ps aux | grep mysql | grep -v grep" || echo "No MySQL processes found in container"
	@echo ""
	@echo "MySQL-related metrics:"
	@curl -s http://localhost:8090/metrics | grep -E "(mysql|process)" | head -20

## export-metrics: Export current metrics to file
export-metrics:
	@echo "Exporting metrics to metrics-export.txt..."
	@curl -s http://localhost:8090/metrics > metrics-export-$(shell date +%Y%m%d-%H%M%S).txt
	@echo "Metrics exported successfully"

## debug: Enable debug mode
debug:
	@echo "Viewing debug output..."
	@curl -s http://localhost:55679/debug/tracez
	@echo ""
	@echo "For more debug info, visit:"
	@echo "  - Traces: http://localhost:55679/debug/tracez"
	@echo "  - Extensions: http://localhost:55679/debug/extensionz"
	@echo "  - Features: http://localhost:55679/debug/featurez"