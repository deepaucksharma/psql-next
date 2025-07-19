MODULE_NAME = core-metrics
PORT = 8081

.PHONY: build test run stop logs clean test-unit test-integration

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
	docker-compose up -d
	@sleep 10
	@curl -f http://localhost:$(PORT)/metrics || (docker-compose logs && exit 1)
	docker-compose down

run:
	docker-compose up -d
	@echo "$(MODULE_NAME) is running on port $(PORT)"
	@echo "Metrics available at: http://localhost:$(PORT)/metrics"

stop:
	docker-compose down

logs:
	docker-compose logs -f $(MODULE_NAME)

clean:
	docker-compose down -v
	docker rmi $(MODULE_NAME):latest 2>/dev/null || true

status:
	@curl -s http://localhost:$(PORT)/metrics | head -20 || echo "Module not running"