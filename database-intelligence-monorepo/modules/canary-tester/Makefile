.PHONY: help build run run-enhanced stop clean logs test init dev prod health metrics validate-enhanced

# Default target
help:
	@echo "Canary Tester Module - Available Commands:"
	@echo "  make build           - Build Docker images"
	@echo "  make run             - Run all services with standard config"
	@echo "  make run-enhanced    - Run all services with enhanced config"
	@echo "  make stop            - Stop all services"
	@echo "  make clean           - Stop services and remove volumes"
	@echo "  make logs            - Show service logs"
	@echo "  make test            - Run canary tests"
	@echo "  make init            - Initialize Go module"
	@echo "  make dev             - Run in development mode"
	@echo "  make prod            - Run in production mode"
	@echo "  make health          - Check service health"
	@echo "  make metrics         - View Prometheus metrics"
	@echo "  make validate-enhanced - Validate enhanced collector config"

# Initialize Go module
init:
	@echo "Initializing Go module..."
	@go mod init github.com/deepaksharma/canary-tester || true
	@go mod tidy

# Build Docker images
build:
	@echo "Building canary-tester image..."
	docker-compose build

# Run all services
run:
	@echo "Starting canary-tester services..."
	docker-compose up -d
	@echo "Canary tester running on http://localhost:8090"
	@echo "Prometheus metrics available at http://localhost:8888/metrics"

# Run all services with enhanced configuration
run-enhanced:
	@echo "Starting canary-tester services with enhanced configuration..."
	COLLECTOR_CONFIG=collector-enhanced.yaml docker-compose up -d
	@echo "Canary tester running on http://localhost:8090 with enhanced configuration"
	@echo "Prometheus metrics available at http://localhost:8888/metrics"

# Run in development mode with logs
dev:
	@echo "Starting in development mode..."
	docker-compose up

# Run in production mode
prod:
	@echo "Starting in production mode..."
	docker-compose up -d --build
	@echo "Waiting for services to be healthy..."
	@sleep 10
	@make health

# Stop all services
stop:
	@echo "Stopping canary-tester services..."
	docker-compose down

# Clean up (stop and remove volumes)
clean:
	@echo "Cleaning up canary-tester..."
	docker-compose down -v
	@rm -f go.mod go.sum

# View logs
logs:
	docker-compose logs -f

# View specific service logs
logs-canary:
	docker-compose logs -f canary-tester

logs-mysql:
	docker-compose logs -f mysql

logs-collector:
	docker-compose logs -f otel-collector

# Run canary tests manually
test:
	@echo "Running canary tests..."
	@docker-compose exec canary-tester sh -c "mysql -h mysql -uroot -prootpassword test < /app/queries/canary-queries.sql"

# Check service health
health:
	@echo "Checking service health..."
	@echo "MySQL status:"
	@docker-compose exec mysql mysqladmin -uroot -prootpassword status || echo "MySQL not ready"
	@echo "\nOTel Collector health:"
	@curl -s http://localhost:13133/health | jq . || echo "Collector not ready"
	@echo "\nCanary metrics:"
	@curl -s http://localhost:8090/metrics | grep canary || echo "Canary metrics not available"

# View Prometheus metrics
metrics:
	@echo "Prometheus metrics endpoint: http://localhost:8888/metrics"
	@echo "\nCanary metrics:"
	@curl -s http://localhost:8090/metrics | grep -E "(canary_|mysql_)" | head -20

# Connect to MySQL
mysql-cli:
	docker-compose exec mysql mysql -uroot -prootpassword test

# View canary test results
results:
	@echo "Recent canary test results:"
	@docker-compose exec mysql mysql -uroot -prootpassword -e "SELECT * FROM test.canary_test ORDER BY created_at DESC LIMIT 10;"

# Monitor canary performance
monitor:
	@echo "Monitoring canary performance..."
	@watch -n 5 'curl -s http://localhost:8090/metrics | grep -E "(canary_query_duration|canary_query_errors|mysql_up)"'

# Export metrics to file
export-metrics:
	@echo "Exporting metrics..."
	@mkdir -p exports
	@curl -s http://localhost:8888/metrics > exports/metrics-$$(date +%Y%m%d-%H%M%S).txt
	@echo "Metrics exported to exports/"

# Restart specific service
restart-canary:
	docker-compose restart canary-tester

restart-collector:
	docker-compose restart otel-collector

restart-mysql:
	docker-compose restart mysql

# View configuration
show-config:
	@echo "Collector configuration:"
	@cat config/collector.yaml

# Validate configuration
validate:
	@echo "Validating collector configuration..."
	@docker run --rm -v $$(pwd)/config:/config otel/opentelemetry-collector-contrib:latest validate --config=/config/collector.yaml

# Validate enhanced configuration
validate-enhanced:
	@echo "Validating enhanced collector configuration..."
	@docker run --rm -v $$(pwd)/config:/config otel/opentelemetry-collector-contrib:latest validate --config=/config/collector-enhanced.yaml