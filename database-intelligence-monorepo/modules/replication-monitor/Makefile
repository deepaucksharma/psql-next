.PHONY: help build up up-enhanced down restart logs status clean test-replication monitor-lag generate-load check-gtid backup-master restore-replica validate-enhanced

# Default target
help:
	@echo "MySQL Replication Monitor - Available Commands:"
	@echo "  make build          - Build Docker images"
	@echo "  make up             - Start all services with standard config"
	@echo "  make up-enhanced    - Start all services with enhanced config"
	@echo "  make down           - Stop all services"
	@echo "  make restart        - Restart all services"
	@echo "  make logs           - View logs from all services"
	@echo "  make status         - Check replication status"
	@echo "  make clean          - Clean up volumes and containers"
	@echo "  make test-replication - Test replication is working"
	@echo "  make monitor-lag    - Monitor replication lag in real-time"
	@echo "  make generate-load  - Generate load on master to test lag"
	@echo "  make check-gtid     - Check GTID status"
	@echo "  make backup-master  - Create backup from master"
	@echo "  make restore-replica - Restore backup to replica"
	@echo "  make validate-enhanced - Validate enhanced collector config"

# Build Docker images
build:
	docker-compose build

# Start all services
up:
	docker-compose up -d
	@echo "Waiting for services to be healthy..."
	@sleep 20
	@echo "Services started. MySQL replica is available on port 8086"
	@make status

# Start all services with enhanced configuration
up-enhanced:
	@echo "Starting services with enhanced configuration..."
	COLLECTOR_CONFIG=collector-enhanced.yaml docker-compose up -d
	@echo "Waiting for services to be healthy..."
	@sleep 20
	@echo "Services started with enhanced configuration. MySQL replica is available on port 8086"
	@make status

# Stop all services
down:
	docker-compose down

# Restart all services
restart: down up

# View logs
logs:
	docker-compose logs -f

# Check replication status
status:
	@echo "=== Master Status ==="
	@docker exec mysql-master mysql -uroot -prootpassword -e "SHOW MASTER STATUS\G" 2>/dev/null || echo "Master not ready"
	@echo ""
	@echo "=== Replica Status ==="
	@docker exec mysql-replica mysql -uroot -prootpassword -e "SHOW SLAVE STATUS\G" 2>/dev/null | grep -E "Slave_IO_Running|Slave_SQL_Running|Seconds_Behind_Master|Last_Error" || echo "Replica not ready"
	@echo ""
	@echo "=== Data Counts ==="
	@docker exec mysql-master mysql -uroot -prootpassword -e "SELECT 'Master' as Server, COUNT(*) as UserCount FROM test_db.users" 2>/dev/null || echo "Cannot count master data"
	@docker exec mysql-replica mysql -uroot -prootpassword -e "SELECT 'Replica' as Server, COUNT(*) as UserCount FROM test_db.users" 2>/dev/null || echo "Cannot count replica data"

# Clean up
clean:
	docker-compose down -v
	@echo "Cleaned up containers and volumes"

# Test replication
test-replication:
	@echo "=== Testing Replication ==="
	@echo "Inserting test data on master..."
	@docker exec mysql-master mysql -uroot -prootpassword test_db -e "INSERT INTO users (username, email) VALUES ('test_user_$$(date +%s)', 'test_$$(date +%s)@example.com');"
	@echo "Waiting for replication..."
	@sleep 3
	@echo "Checking if data replicated..."
	@docker exec mysql-master mysql -uroot -prootpassword test_db -e "SELECT username, email FROM users ORDER BY id DESC LIMIT 1;" 2>/dev/null
	@docker exec mysql-replica mysql -uroot -prootpassword test_db -e "SELECT username, email FROM users ORDER BY id DESC LIMIT 1;" 2>/dev/null

# Monitor replication lag
monitor-lag:
	@echo "Monitoring replication lag (Press Ctrl+C to stop)..."
	@while true; do \
		clear; \
		echo "=== Replication Lag Monitor - $$(date) ==="; \
		docker exec mysql-replica mysql -uroot -prootpassword -e "SHOW SLAVE STATUS\G" 2>/dev/null | grep -E "Slave_IO_Running|Slave_SQL_Running|Seconds_Behind_Master|Master_Log_File|Exec_Master_Log_Pos"; \
		sleep 2; \
	done

# Generate load on master
generate-load:
	@echo "Generating load on master database..."
	@docker exec -d mysql-master mysql -uroot -prootpassword test_db -e "CALL generate_continuous_data();"
	@echo "Load generation started. Use 'make monitor-lag' to watch replication lag"
	@echo "To stop load generation, restart the master: docker restart mysql-master"

# Check GTID status
check-gtid:
	@echo "=== GTID Status ==="
	@echo "Master GTID Executed:"
	@docker exec mysql-master mysql -uroot -prootpassword -e "SELECT @@gtid_executed\G" 2>/dev/null
	@echo ""
	@echo "Replica GTID Executed:"
	@docker exec mysql-replica mysql -uroot -prootpassword -e "SELECT @@gtid_executed\G" 2>/dev/null

# Create backup from master
backup-master:
	@echo "Creating backup from master..."
	@docker exec mysql-master mysqldump -uroot -prootpassword --all-databases --single-transaction --flush-logs --master-data=2 > master_backup.sql
	@echo "Backup saved to master_backup.sql"

# Restore backup to replica (use with caution)
restore-replica:
	@echo "WARNING: This will reset the replica. Continue? [y/N]"
	@read confirm && [ "$$confirm" = "y" ] || exit 1
	@echo "Stopping replica..."
	@docker exec mysql-replica mysql -uroot -prootpassword -e "STOP SLAVE;"
	@echo "Restoring backup..."
	@docker cp master_backup.sql mysql-replica:/tmp/
	@docker exec mysql-replica mysql -uroot -prootpassword < /tmp/master_backup.sql
	@echo "Restarting replication..."
	@docker exec mysql-replica mysql -uroot -prootpassword -e "START SLAVE;"
	@echo "Restore complete"

# View collector metrics
metrics:
	@echo "=== Prometheus Metrics ==="
	@curl -s http://localhost:8889/metrics | grep -E "mysql_replication_" | head -20
	@echo ""
	@echo "For all metrics, visit: http://localhost:8889/metrics"

# Health check
health:
	@echo "=== Service Health Status ==="
	@docker-compose ps
	@echo ""
	@echo "=== Collector Health ==="
	@curl -s http://localhost:13133/health || echo "Collector health check failed"

# View collector logs
collector-logs:
	docker logs replication-collector -f

# Connect to master
connect-master:
	docker exec -it mysql-master mysql -uroot -prootpassword test_db

# Connect to replica
connect-replica:
	docker exec -it mysql-replica mysql -uroot -prootpassword test_db

# Initialize sample data
init-data:
	@echo "Initializing more sample data..."
	@docker exec mysql-master mysql -uroot -prootpassword test_db -e "CALL generate_test_data(50, 200);"
	@echo "Sample data initialized"

# Validate enhanced configuration
validate-enhanced:
	@echo "Validating enhanced collector configuration..."
	@docker run --rm -v $$(pwd)/config:/config otel/opentelemetry-collector-contrib:latest validate --config=/config/collector-enhanced.yaml