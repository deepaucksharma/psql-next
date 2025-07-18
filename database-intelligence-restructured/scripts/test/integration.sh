#!/bin/bash
# Integration test runner for Database Intelligence

set -e

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Source utilities
source "$SCRIPT_DIR/../utils/common.sh"

# Configuration
DATABASE=${1:-all}
TEST_DURATION=${TEST_DURATION:-30}
DOCKER_COMPOSE_FILE="$ROOT_DIR/deployments/docker/docker-compose.test.yml"

print_header "Integration Tests"
log_info "Database: $DATABASE"
log_info "Test Duration: ${TEST_DURATION}s"

# Check requirements
check_requirements docker docker-compose curl || exit 1

# Function to test a database
test_database() {
    local db_name=$1
    local collector_name="otel-collector-$db_name"
    local db_container="${db_name}-test"
    
    print_separator
    log_info "Testing $db_name integration..."
    
    # Start database
    log_info "Starting $db_name database..."
    docker-compose -f "$DOCKER_COMPOSE_FILE" up -d "$db_container"
    
    # Wait for database to be ready
    case "$db_name" in
        postgresql)
            wait_for_service "PostgreSQL" 5432 60 || return 1
            ;;
        mysql)
            wait_for_service "MySQL" 3306 60 || return 1
            ;;
        mongodb)
            wait_for_service "MongoDB" 27017 60 || return 1
            ;;
        redis)
            wait_for_service "Redis" 6379 30 || return 1
            ;;
    esac
    
    # Prepare environment
    export NEW_RELIC_LICENSE_KEY="${NEW_RELIC_LICENSE_KEY:-dummy-key-for-testing}"
    
    # Start collector
    log_info "Starting collector for $db_name..."
    local config_file="$ROOT_DIR/configs/examples/config-only-${db_name}.yaml"
    
    if [[ ! -f "$config_file" ]]; then
        log_warning "Config file not found: $config_file"
        config_file="$ROOT_DIR/configs/modes/config-only.yaml"
    fi
    
    # Run collector in background
    "$ROOT_DIR/distributions/production/dbintel" \
        --config="$config_file" \
        > "$ROOT_DIR/collector-${db_name}.log" 2>&1 &
    
    local collector_pid=$!
    
    # Wait for collector to start
    wait_for_service "Collector health" 13133 30 || {
        kill $collector_pid 2>/dev/null
        return 1
    }
    
    # Test health endpoint
    log_info "Testing health endpoint..."
    if curl -s http://localhost:13133/health | grep -q "OK"; then
        log_success "Health check passed"
    else
        log_error "Health check failed"
        kill $collector_pid 2>/dev/null
        return 1
    fi
    
    # Test metrics endpoint
    log_info "Testing metrics endpoint..."
    sleep 5  # Give time for metrics to be collected
    
    local metrics_count=$(curl -s http://localhost:8888/metrics | grep -c "^${db_name}_" || true)
    if [[ $metrics_count -gt 0 ]]; then
        log_success "Found $metrics_count ${db_name} metrics"
    else
        log_warning "No ${db_name} metrics found"
    fi
    
    # Test for specific metrics
    case "$db_name" in
        postgresql)
            check_metric "postgresql_backends"
            check_metric "postgresql_database_size"
            ;;
        mysql)
            check_metric "mysql_connections"
            check_metric "mysql_queries"
            ;;
        mongodb)
            check_metric "mongodb_connections"
            check_metric "mongodb_operations"
            ;;
        redis)
            check_metric "redis_connections"
            check_metric "redis_commands"
            ;;
    esac
    
    # Let it run for test duration
    log_info "Running for ${TEST_DURATION}s..."
    sleep "$TEST_DURATION"
    
    # Check for errors in logs
    if grep -i "error\|panic" "$ROOT_DIR/collector-${db_name}.log" | grep -v "level=debug"; then
        log_warning "Errors found in collector logs"
    else
        log_success "No errors in collector logs"
    fi
    
    # Stop collector
    log_info "Stopping collector..."
    kill $collector_pid 2>/dev/null
    wait $collector_pid 2>/dev/null || true
    
    # Stop database
    log_info "Stopping $db_name database..."
    docker-compose -f "$DOCKER_COMPOSE_FILE" down "$db_container"
    
    log_success "$db_name integration test completed"
    return 0
}

# Function to check if a metric exists
check_metric() {
    local metric_name=$1
    if curl -s http://localhost:8888/metrics | grep -q "^$metric_name"; then
        log_success "Metric found: $metric_name"
    else
        log_warning "Metric not found: $metric_name"
    fi
}

# Main test execution
if [[ "$DATABASE" == "all" ]]; then
    DATABASES=("postgresql" "mysql" "mongodb" "redis")
else
    DATABASES=("$DATABASE")
fi

# Create docker-compose file if it doesn't exist
if [[ ! -f "$DOCKER_COMPOSE_FILE" ]]; then
    log_info "Creating test docker-compose file..."
    mkdir -p "$(dirname "$DOCKER_COMPOSE_FILE")"
    cat > "$DOCKER_COMPOSE_FILE" << 'EOF'
version: '3.8'

services:
  postgresql-test:
    image: postgres:15
    environment:
      POSTGRES_PASSWORD: testpass
      POSTGRES_USER: testuser
      POSTGRES_DB: testdb
    ports:
      - "5432:5432"

  mysql-test:
    image: mysql:8.0
    environment:
      MYSQL_ROOT_PASSWORD: testpass
      MYSQL_DATABASE: testdb
      MYSQL_USER: testuser
      MYSQL_PASSWORD: testpass
    ports:
      - "3306:3306"

  mongodb-test:
    image: mongo:6
    ports:
      - "27017:27017"

  redis-test:
    image: redis:7
    ports:
      - "6379:6379"
EOF
fi

# Test selected databases
FAILED_TESTS=0
for db in "${DATABASES[@]}"; do
    if test_database "$db"; then
        log_success "$db test passed"
    else
        log_error "$db test failed"
        ((FAILED_TESTS++))
    fi
done

# Cleanup
log_info "Cleaning up..."
docker-compose -f "$DOCKER_COMPOSE_FILE" down

# Summary
print_separator
if [[ $FAILED_TESTS -eq 0 ]]; then
    log_success "All integration tests passed"
    exit 0
else
    log_error "$FAILED_TESTS integration tests failed"
    exit 1
fi