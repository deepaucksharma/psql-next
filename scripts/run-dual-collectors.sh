#!/bin/bash

set -euo pipefail

# Load environment variables
if [ -f .env ]; then
    source .env
fi

COMMAND=${1:-"help"}

show_help() {
    cat << EOF
PostgreSQL Dual Instrumentation Manager

Usage: $0 [COMMAND]

Commands:
    build       Build the collector Docker image
    start       Start all services including dual collectors
    stop        Stop all services
    restart     Restart all services
    status      Show status of all services
    logs-nri    Show logs from NRI collector
    logs-otlp   Show logs from OTLP collector
    logs-all    Show logs from both collectors
    health      Check health of both collectors
    test        Run end-to-end test
    help        Show this help message

Examples:
    $0 build
    $0 start
    $0 status
    $0 logs-nri
    $0 health
EOF
}

build_image() {
    echo "=== Building PostgreSQL Collector Docker Image ==="
    docker-compose build postgres-collector-nri postgres-collector-otlp
}

start_services() {
    echo "=== Starting PostgreSQL Dual Instrumentation Setup ==="
    docker-compose up -d postgres otel-collector postgres-collector-nri postgres-collector-otlp
    
    echo "Waiting for services to start..."
    sleep 10
    
    echo "=== Service Status ==="
    docker-compose ps
}

stop_services() {
    echo "=== Stopping All Services ==="
    docker-compose down
}

restart_services() {
    stop_services
    start_services
}

show_status() {
    echo "=== Docker Compose Services ==="
    docker-compose ps
    
    echo -e "\n=== Container Health ==="
    docker ps --filter "name=postgres" --filter "name=collector" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
}

show_logs_nri() {
    echo "=== NRI Collector Logs ==="
    docker-compose logs -f postgres-collector-nri
}

show_logs_otlp() {
    echo "=== OTLP Collector Logs ==="
    docker-compose logs -f postgres-collector-otlp
}

show_logs_all() {
    echo "=== Both Collector Logs ==="
    docker-compose logs -f postgres-collector-nri postgres-collector-otlp
}

check_health() {
    echo "=== Health Check ==="
    
    echo "1. NRI Collector Health (port 8080):"
    curl -s http://localhost:8080/health || echo "NRI collector health check failed"
    
    echo -e "\n2. OTLP Collector Health (port 8081):"
    curl -s http://localhost:8081/health || echo "OTLP collector health check failed"
    
    echo -e "\n3. PostgreSQL Health:"
    docker-compose exec postgres pg_isready -U postgres || echo "PostgreSQL health check failed"
    
    echo -e "\n4. OTel Collector Health:"
    curl -s http://localhost:13133 || echo "OTel Collector health check failed"
    
    echo -e "\n5. Port Status:"
    netstat -tln | grep -E "(5432|8080|8081|9090|9091|4317|4318)" || echo "Some ports not listening"
}

run_test() {
    echo "=== Running End-to-End Test ==="
    
    # Check if services are running
    if ! docker-compose ps | grep -q "Up"; then
        echo "Services not running. Starting them first..."
        start_services
        sleep 15
    fi
    
    # Run load test
    echo "1. Running PostgreSQL load test..."
    ./scripts/test-load.sh &
    LOAD_PID=$!
    
    # Wait for metrics collection
    echo "2. Waiting for metrics collection (60 seconds)..."
    sleep 60
    
    # Stop load test
    kill $LOAD_PID 2>/dev/null || true
    
    # Check health
    echo "3. Checking collector health..."
    check_health
    
    # Verify metrics
    echo "4. Verifying metrics in New Relic..."
    if [ -f ./scripts/verify-metrics.sh ]; then
        ./scripts/verify-metrics.sh
    else
        echo "Metrics verification script not found"
    fi
    
    echo "Test completed!"
}

case $COMMAND in
    build)
        build_image
        ;;
    start)
        start_services
        ;;
    stop)
        stop_services
        ;;
    restart)
        restart_services
        ;;
    status)
        show_status
        ;;
    logs-nri)
        show_logs_nri
        ;;
    logs-otlp)
        show_logs_otlp
        ;;
    logs-all)
        show_logs_all
        ;;
    health)
        check_health
        ;;
    test)
        run_test
        ;;
    help)
        show_help
        ;;
    *)
        echo "Unknown command: $COMMAND"
        show_help
        exit 1
        ;;
esac