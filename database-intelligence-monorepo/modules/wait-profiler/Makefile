MODULE_NAME = wait-profiler
PORT = 8083

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
	@curl -s http://localhost:$(PORT)/metrics | grep -E "mysql_wait|mysql_mutex|mysql_io|mysql_lock" | head -20 || echo "Module not running"

generate-load:
	@echo "Generating wait events..."
	@docker exec -it $$(docker-compose ps -q mysql-test) mysql -uroot -ptest -e "\
		CREATE DATABASE IF NOT EXISTS load_test; \
		USE load_test; \
		CREATE TABLE IF NOT EXISTS test_table (id INT PRIMARY KEY, data VARCHAR(100)); \
		INSERT INTO test_table VALUES (1, REPEAT('a', 100)) ON DUPLICATE KEY UPDATE data=VALUES(data); \
		SELECT SLEEP(0.1); \
		SELECT * FROM test_table WITH SHARE LOCK; \
	"