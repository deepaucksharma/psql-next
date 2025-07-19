MODULE_NAME = anomaly-detector
PORT = 8084

.PHONY: build test run stop logs clean test-unit test-integration status check-dependencies

build:
	docker-compose build

test: test-unit test-integration

test-unit:
	@echo "Running unit tests for $(MODULE_NAME)..."
	@if [ -d "tests" ] && [ -n "$$(ls -A tests/*.py 2>/dev/null)" ]; then \
		docker-compose run --rm $(MODULE_NAME) pytest tests/; \
	else \
		echo "No unit tests found"; \
	fi

test-integration:
	@echo "Running integration tests for $(MODULE_NAME)..."
	@echo "Checking if dependent modules are running..."
	@make check-dependencies
	docker-compose up -d
	@sleep 15  # Extra time for federation setup
	@curl -f http://localhost:$(PORT)/metrics || (docker-compose logs && exit 1)
	@echo "Checking anomaly detection metrics..."
	@curl -s http://localhost:$(PORT)/metrics | grep -E "anomaly_score|anomaly_alert" || echo "No anomalies detected yet"
	docker-compose down

check-dependencies:
	@echo "Checking if dependent modules are accessible..."
	@curl -f http://localhost:8081/metrics > /dev/null 2>&1 || echo "WARNING: core-metrics (port 8081) not accessible"
	@curl -f http://localhost:8082/metrics > /dev/null 2>&1 || echo "WARNING: sql-intelligence (port 8082) not accessible"
	@curl -f http://localhost:8083/metrics > /dev/null 2>&1 || echo "WARNING: wait-profiler (port 8083) not accessible"

run:
	@make check-dependencies
	docker-compose up -d
	@echo "$(MODULE_NAME) is running on port $(PORT)"
	@echo "Metrics available at: http://localhost:$(PORT)/metrics"
	@echo "Anomaly alerts will be written to container's /tmp/anomaly_alerts.json"

stop:
	docker-compose down

logs:
	docker-compose logs -f $(MODULE_NAME)

clean:
	docker-compose down -v
	docker rmi $(MODULE_NAME):latest 2>/dev/null || true

status:
	@echo "=== Module Status ==="
	@curl -s http://localhost:$(PORT)/metrics > /dev/null 2>&1 && echo "✓ $(MODULE_NAME) is running" || echo "✗ $(MODULE_NAME) is not running"
	@echo ""
	@echo "=== Anomaly Metrics ==="
	@curl -s http://localhost:$(PORT)/metrics | grep -E "^anomaly_score" | head -10 || echo "No anomaly scores available"
	@echo ""
	@echo "=== Active Alerts ==="
	@curl -s http://localhost:$(PORT)/metrics | grep -E "^anomaly_alert" | head -10 || echo "No active alerts"

alerts:
	@echo "Fetching recent anomaly alerts..."
	@docker-compose exec $(MODULE_NAME) cat /tmp/anomaly_alerts.json 2>/dev/null | jq . || echo "No alerts found"