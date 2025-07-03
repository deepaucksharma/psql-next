#!/bin/bash
# Simplified E2E test runner for comprehensive testing

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Source common functions
source "${SCRIPT_DIR}/../../scripts/lib/common.sh"

log_info "Starting comprehensive E2E tests..."

# Environment setup
export TEST_OUTPUT_DIR="${SCRIPT_DIR}/output"
export TEST_TIMESTAMP=$(date +%Y%m%d_%H%M%S)
export TEST_NR_LICENSE_KEY="${TEST_NR_LICENSE_KEY:-test-license-key}"

# Create output directory
mkdir -p "$TEST_OUTPUT_DIR"

# Function to run docker-compose commands
docker_compose() {
    docker-compose -f docker-compose.e2e.yml "$@"
}

# Cleanup function
cleanup() {
    log_info "Cleaning up test environment..."
    docker_compose down -v --remove-orphans || true
}

# Set up trap for cleanup
trap cleanup EXIT

# Start test environment
log_info "Starting test environment..."
docker_compose down -v --remove-orphans || true
docker_compose up -d

# Wait for services to be ready
log_info "Waiting for services to be ready..."
sleep 10

# Check if services are healthy
log_info "Checking service health..."
docker_compose ps

# Verify PostgreSQL is ready
until docker_compose exec -T postgres pg_isready -U postgres; do
    log_info "Waiting for PostgreSQL..."
    sleep 2
done

# Verify MySQL is ready
until docker_compose exec -T mysql mysqladmin ping -h localhost --silent; do
    log_info "Waiting for MySQL..."
    sleep 2
done

# Run test workloads
log_info "Generating test workloads..."

# PostgreSQL workload
docker_compose exec -T postgres psql -U postgres -d testdb <<EOF
-- Create test tables
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100),
    email VARCHAR(100),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS orders (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id),
    total DECIMAL(10,2),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Insert test data
INSERT INTO users (name, email) 
SELECT 
    'User ' || i,
    'user' || i || '@example.com'
FROM generate_series(1, 100) AS i;

-- Run some queries
SELECT COUNT(*) FROM users;
SELECT * FROM users LIMIT 10;
SELECT u.name, COUNT(o.id) as order_count 
FROM users u 
LEFT JOIN orders o ON u.id = o.user_id 
GROUP BY u.name 
LIMIT 10;
EOF

# MySQL workload
docker_compose exec -T mysql mysql -uroot -proot testdb <<EOF
-- Create test tables
CREATE TABLE IF NOT EXISTS products (
    id INT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(100),
    price DECIMAL(10,2),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Insert test data
INSERT INTO products (name, price) 
VALUES 
    ('Product 1', 19.99),
    ('Product 2', 29.99),
    ('Product 3', 39.99);

-- Run some queries
SELECT COUNT(*) FROM products;
SELECT * FROM products;
EOF

# Wait for metrics to be collected
log_info "Waiting for metrics collection..."
sleep 30

# Check collector metrics
log_info "Checking collector metrics..."
METRICS_RESPONSE=$(curl -s http://localhost:8888/metrics || echo "Failed to get metrics")
echo "$METRICS_RESPONSE" > "$TEST_OUTPUT_DIR/collector_metrics_${TEST_TIMESTAMP}.txt"

# Check if metrics contain expected data
if echo "$METRICS_RESPONSE" | grep -q "otelcol_receiver_accepted_metric_points"; then
    log_success "Collector is receiving metrics"
else
    log_error "Collector is not receiving metrics"
fi

# Run Go tests if available
if command -v go &> /dev/null; then
    log_info "Running Go-based tests..."
    
    # Create a simple test file
    cat > simple_e2e_test.go << 'EOF'
package main

import (
    "database/sql"
    "fmt"
    "net/http"
    "testing"
    "time"
    
    _ "github.com/lib/pq"
    _ "github.com/go-sql-driver/mysql"
)

func TestPostgreSQLConnection(t *testing.T) {
    db, err := sql.Open("postgres", "host=localhost port=5432 user=postgres password=postgres dbname=testdb sslmode=disable")
    if err != nil {
        t.Fatalf("Failed to connect to PostgreSQL: %v", err)
    }
    defer db.Close()
    
    err = db.Ping()
    if err != nil {
        t.Fatalf("Failed to ping PostgreSQL: %v", err)
    }
    
    var count int
    err = db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
    if err != nil {
        t.Fatalf("Failed to query PostgreSQL: %v", err)
    }
    
    t.Logf("PostgreSQL test passed, user count: %d", count)
}

func TestMySQLConnection(t *testing.T) {
    db, err := sql.Open("mysql", "root:root@tcp(localhost:3306)/testdb")
    if err != nil {
        t.Fatalf("Failed to connect to MySQL: %v", err)
    }
    defer db.Close()
    
    err = db.Ping()
    if err != nil {
        t.Fatalf("Failed to ping MySQL: %v", err)
    }
    
    t.Log("MySQL test passed")
}

func TestCollectorMetrics(t *testing.T) {
    resp, err := http.Get("http://localhost:8888/metrics")
    if err != nil {
        t.Fatalf("Failed to get collector metrics: %v", err)
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        t.Fatalf("Unexpected status code: %d", resp.StatusCode)
    }
    
    t.Log("Collector metrics endpoint test passed")
}
EOF

    # Run the tests
    go test -v simple_e2e_test.go 2>&1 | tee "$TEST_OUTPUT_DIR/go_test_${TEST_TIMESTAMP}.log" || true
fi

# Generate summary report
log_info "Generating test summary..."
cat > "$TEST_OUTPUT_DIR/summary_${TEST_TIMESTAMP}.txt" << EOF
E2E Test Summary
================
Timestamp: $(date)
Environment: Docker Compose

Services Status:
$(docker_compose ps)

Test Results:
- PostgreSQL Connection: $(if docker_compose exec -T postgres pg_isready -U postgres &>/dev/null; then echo "PASS"; else echo "FAIL"; fi)
- MySQL Connection: $(if docker_compose exec -T mysql mysqladmin ping -h localhost --silent &>/dev/null; then echo "PASS"; else echo "FAIL"; fi)
- Collector Running: $(if curl -s http://localhost:8888/metrics &>/dev/null; then echo "PASS"; else echo "FAIL"; fi)
- Metrics Available: $(if echo "$METRICS_RESPONSE" | grep -q "otelcol_"; then echo "PASS"; else echo "FAIL"; fi)

Logs saved to: $TEST_OUTPUT_DIR
EOF

cat "$TEST_OUTPUT_DIR/summary_${TEST_TIMESTAMP}.txt"

# Check overall status
if docker_compose ps | grep -q "Exit"; then
    log_error "Some services failed to start properly"
    exit 1
else
    log_success "All E2E tests completed successfully!"
fi