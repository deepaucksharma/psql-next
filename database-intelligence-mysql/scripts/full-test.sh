#!/bin/bash

set -euo pipefail

echo "🧪 Full MySQL OpenTelemetry Test Suite"
echo "====================================="
echo

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_step() {
    echo -e "${BLUE}[STEP]${NC} $1"
}

# Function to wait for service to be ready
wait_for_service() {
    local service=$1
    local max_attempts=30
    local attempt=1
    
    log_step "Waiting for $service to be ready..."
    
    while [ $attempt -le $max_attempts ]; do
        if docker compose ps | grep -q "$service.*running.*healthy"; then
            log_info "✅ $service is ready"
            return 0
        fi
        
        echo -n "."
        sleep 2
        ((attempt++))
    done
    
    log_error "❌ $service failed to start within timeout"
    docker compose logs $service --tail=20
    return 1
}

# Function to test MySQL connection
test_mysql_connection() {
    local host=$1
    local user=$2
    local password=$3
    local service=$4
    
    log_step "Testing MySQL connection to $service..."
    
    if docker compose exec -T $service mysql -h localhost -u$user -p$password -e "SELECT 'Connection successful' as status;" 2>/dev/null; then
        log_info "✅ MySQL connection to $service successful"
        return 0
    else
        log_error "❌ MySQL connection to $service failed"
        return 1
    fi
}

# Function to test OTel collector
test_otel_collector() {
    log_step "Testing OpenTelemetry Collector..."
    
    # Test health endpoint
    if curl -s http://localhost:13133/ | grep -q "Server available"; then
        log_info "✅ OTel Collector health check passed"
    else
        log_error "❌ OTel Collector health check failed"
        return 1
    fi
    
    # Check if collector is receiving metrics
    sleep 5
    local logs=$(docker compose logs otel-collector --tail=50 2>&1)
    
    if echo "$logs" | grep -q "mysql"; then
        log_info "✅ OTel Collector is processing MySQL metrics"
    else
        log_warn "⚠️  No MySQL metrics detected in collector logs yet"
    fi
    
    # Check for export attempts (will fail with test credentials)
    if echo "$logs" | grep -q "otlp/newrelic"; then
        log_info "✅ OTel Collector is attempting to export to New Relic"
        
        if echo "$logs" | grep -qi "unauthorized\|401\|403"; then
            log_warn "⚠️  Export failing due to invalid API key (expected with test credentials)"
        fi
    fi
}

# Function to test replication
test_replication() {
    log_step "Testing MySQL replication..."
    
    # Check slave status
    local slave_io=$(docker compose exec -T mysql-replica mysql -uroot -prootpassword -e "SHOW SLAVE STATUS\G" 2>/dev/null | grep "Slave_IO_Running:" | awk '{print $2}')
    local slave_sql=$(docker compose exec -T mysql-replica mysql -uroot -prootpassword -e "SHOW SLAVE STATUS\G" 2>/dev/null | grep "Slave_SQL_Running:" | awk '{print $2}')
    
    if [ "$slave_io" = "Yes" ] && [ "$slave_sql" = "Yes" ]; then
        log_info "✅ MySQL replication is working"
        
        # Get lag
        local lag=$(docker compose exec -T mysql-replica mysql -uroot -prootpassword -e "SHOW SLAVE STATUS\G" 2>/dev/null | grep "Seconds_Behind_Master:" | awk '{print $2}')
        log_info "   Replication lag: ${lag}s"
    else
        log_error "❌ MySQL replication not working"
        log_error "   Slave_IO_Running: $slave_io"
        log_error "   Slave_SQL_Running: $slave_sql"
        return 1
    fi
}

# Function to generate test data
generate_test_data() {
    log_step "Generating test data..."
    
    # Create some orders
    docker compose exec -T mysql-primary mysql -uappuser -papppassword ecommerce -e "CALL generate_orders(10);" 2>/dev/null
    
    # Run analytics query
    docker compose exec -T mysql-primary mysql -uappuser -papppassword ecommerce -e "
        SELECT COUNT(*) as order_count FROM orders;
        SELECT COUNT(*) as customer_count FROM customers;
    " 2>/dev/null
    
    log_info "✅ Test data generated"
}

# Function to show monitoring endpoints
show_endpoints() {
    log_step "Monitoring Endpoints"
    echo "📊 Available endpoints:"
    echo "   MySQL Primary:       localhost:3306"
    echo "   MySQL Replica:       localhost:3307" 
    echo "   OTel Health Check:   http://localhost:13133/"
    echo "   OTel zPages:         http://localhost:55679/"
    echo "   OTel Profiling:      http://localhost:1777/"
    echo ""
    echo "🔑 Default credentials:"
    echo "   Root:     root/rootpassword"
    echo "   App:      appuser/apppassword"
    echo "   Monitor:  otel_monitor/otelmonitorpass"
}

# Main test execution
main() {
    # Step 1: Check prerequisites
    log_step "Running diagnostics..."
    if ! ./scripts/diagnose.sh > /dev/null 2>&1; then
        log_error "Diagnostics failed. Run: ./scripts/diagnose.sh"
        exit 1
    fi
    
    # Step 2: Start services if not running
    if ! docker compose ps | grep -q "running"; then
        log_step "Starting services..."
        docker compose up -d
    fi
    
    # Step 3: Wait for services
    wait_for_service "mysql-primary"
    wait_for_service "mysql-replica"
    
    # Step 4: Test MySQL connections
    test_mysql_connection "localhost" "otel_monitor" "otelmonitorpass" "mysql-primary"
    test_mysql_connection "localhost" "otel_monitor" "otelmonitorpass" "mysql-replica"
    
    # Step 5: Setup replication if needed
    log_step "Setting up replication..."
    # Get master status and configure replica
    local master_file=$(docker compose exec -T mysql-primary mysql -uroot -prootpassword -e "SHOW MASTER STATUS\G" | grep "File:" | awk '{print $2}')
    local master_pos=$(docker compose exec -T mysql-primary mysql -uroot -prootpassword -e "SHOW MASTER STATUS\G" | grep "Position:" | awk '{print $2}')
    
    if [ -n "$master_file" ] && [ -n "$master_pos" ]; then
        docker compose exec -T mysql-replica mysql -uroot -prootpassword <<EOF
STOP SLAVE;
CHANGE MASTER TO
    MASTER_HOST='mysql-primary',
    MASTER_USER='root',
    MASTER_PASSWORD='rootpassword',
    MASTER_AUTO_POSITION=1;
START SLAVE;
EOF
        log_info "✅ Replication configured"
    fi
    
    # Step 6: Test replication
    sleep 3
    test_replication
    
    # Step 7: Test OTel collector
    test_otel_collector
    
    # Step 8: Generate test data
    generate_test_data
    
    # Step 9: Show endpoints
    show_endpoints
    
    # Step 10: Final status
    echo ""
    log_info "🎉 All tests completed!"
    echo ""
    echo "Next steps:"
    echo "1. Replace NEW_RELIC_API_KEY in .env with your real API key"
    echo "2. Check New Relic for MySQL metrics: https://one.newrelic.com/"
    echo "3. Generate more load: ./scripts/generate-load.sh"
    echo "4. View logs: docker compose logs -f otel-collector"
}

# Handle errors
trap 'log_error "Test failed at line $LINENO"; exit 1' ERR

# Run main function
main "$@"