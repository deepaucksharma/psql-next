#!/bin/bash
# PostgreSQL Unified Collector - Master Run Script
# Consolidated script for all operations

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Load environment variables
load_env() {
    if [ -f "$PROJECT_ROOT/.env" ]; then
        export $(cat "$PROJECT_ROOT/.env" | grep -v '^#' | xargs)
    else
        echo -e "${RED}Error: .env file not found!${NC}"
        echo "Copy .env.example to .env and configure it"
        exit 1
    fi
}

# Validate environment
validate_env() {
    local required_vars=(
        "NEW_RELIC_LICENSE_KEY"
        "NEW_RELIC_ACCOUNT_ID"
        "POSTGRES_HOST"
        "POSTGRES_PORT"
        "POSTGRES_USER"
        "POSTGRES_PASSWORD"
        "POSTGRES_DATABASE"
    )
    
    for var in "${required_vars[@]}"; do
        if [ -z "${!var:-}" ]; then
            echo -e "${RED}Error: $var not set in .env${NC}"
            exit 1
        fi
    done
}

# Show help
show_help() {
    cat << EOF
PostgreSQL Unified Collector - Master Run Script

Usage: $0 [COMMAND] [OPTIONS]

Commands:
    build               Build the collector binary
    start               Start collector (local binary)
    stop                Stop all containers
    test                Run all tests
    test-nri            Test NRI output format
    test-otel           Test OTLP output format
    docker-build        Build Docker image
    docker-run          Run collector in Docker
    docker-dual         Run dual collectors in Docker
    k8s-deploy          Deploy to Kubernetes
    k8s-remove          Remove from Kubernetes
    e2e                 Run end-to-end test
    verify              Verify metrics in New Relic
    load-test           Generate PostgreSQL load
    clean               Clean build artifacts
    help                Show this help

Options:
    --mode MODE         Set collection mode (nri|otel|hybrid)
    --quick             Skip builds and checks (for k8s-deploy)
    --with-infra        Start infrastructure containers
    --debug             Enable debug logging

Examples:
    $0 build
    $0 start --mode hybrid --with-infra
    $0 docker-run --mode nri
    $0 k8s-deploy --quick
    $0 e2e
    $0 verify

EOF
}

# Build collector binary
build_collector() {
    echo -e "${GREEN}Building PostgreSQL Unified Collector...${NC}"
    cd "$PROJECT_ROOT"
    
    # Update dependencies if needed
    if [ "${1:-}" == "--update" ]; then
        cargo update
    fi
    
    cargo build --release --all-features
    echo -e "${GREEN}Build complete!${NC}"
}

# Start collector locally
start_collector() {
    local mode="${COLLECTOR_MODE:-hybrid}"
    local with_infra=false
    local debug=false
    
    # Parse options
    while [[ $# -gt 0 ]]; do
        case $1 in
            --mode)
                mode="$2"
                shift 2
                ;;
            --with-infra)
                with_infra=true
                shift
                ;;
            --debug)
                debug=true
                shift
                ;;
            *)
                shift
                ;;
        esac
    done
    
    # Start infrastructure if requested
    if [ "$with_infra" = true ]; then
        echo -e "${BLUE}Starting infrastructure containers...${NC}"
        docker-compose up -d postgres otel-collector
        echo "Waiting for services to start..."
        sleep 10
    fi
    
    # Build if needed
    if [ ! -f "$PROJECT_ROOT/target/release/postgres-unified-collector" ]; then
        build_collector
    fi
    
    # Run collector
    echo -e "${GREEN}Starting collector in $mode mode...${NC}"
    local debug_flag=""
    if [ "$debug" = true ]; then
        debug_flag="--debug"
    fi
    
    cd "$PROJECT_ROOT"
    ./target/release/postgres-unified-collector \
        --config configs/collector-config-env.toml \
        --mode "$mode" \
        $debug_flag
}

# Stop all containers
stop_all() {
    echo -e "${YELLOW}Stopping all containers...${NC}"
    cd "$PROJECT_ROOT"
    docker-compose down
    echo -e "${GREEN}All containers stopped${NC}"
}

# Test NRI format
test_nri() {
    echo -e "${BLUE}Testing NRI output format...${NC}"
    cd "$PROJECT_ROOT"
    
    # Build if needed
    if [ ! -f "target/release/postgres-unified-collector" ]; then
        build_collector
    fi
    
    # Run in NRI mode for 1 collection
    ./target/release/postgres-unified-collector \
        --config configs/collector-config-env.toml \
        --mode nri \
        --dry-run | head -50
}

# Test OTLP format
test_otel() {
    echo -e "${BLUE}Testing OTLP output format...${NC}"
    cd "$PROJECT_ROOT"
    
    # Build if needed
    if [ ! -f "target/release/postgres-unified-collector" ]; then
        build_collector
    fi
    
    # Run in OTLP mode for 1 collection
    ./target/release/postgres-unified-collector \
        --config configs/collector-config-env.toml \
        --mode otel \
        --dry-run | head -50
}

# Build Docker image
docker_build() {
    echo -e "${GREEN}Building Docker image...${NC}"
    cd "$PROJECT_ROOT"
    docker build -t postgres-unified-collector:latest .
    echo -e "${GREEN}Docker build complete!${NC}"
}

# Run in Docker
docker_run() {
    local mode="${1:-hybrid}"
    echo -e "${GREEN}Running collector in Docker (mode: $mode)...${NC}"
    cd "$PROJECT_ROOT"
    
    # Ensure image exists
    if ! docker images | grep -q "postgres-unified-collector"; then
        docker_build
    fi
    
    # Start all services
    docker-compose up -d postgres otel-collector
    
    # Run collector
    docker run --rm \
        --network psql-next_default \
        -e POSTGRES_HOST=postgres \
        -e POSTGRES_PORT=5432 \
        -e POSTGRES_USER="$POSTGRES_USER" \
        -e POSTGRES_PASSWORD="$POSTGRES_PASSWORD" \
        -e POSTGRES_DATABASE="$POSTGRES_DATABASE" \
        -e NEW_RELIC_LICENSE_KEY="$NEW_RELIC_LICENSE_KEY" \
        -e COLLECTOR_MODE="$mode" \
        -v "$PROJECT_ROOT/configs:/app/configs:ro" \
        postgres-unified-collector:latest \
        --config /app/configs/collector-config-env.toml \
        --mode "$mode"
}

# Run dual collectors in Docker
docker_dual() {
    echo -e "${GREEN}Running dual collectors in Docker...${NC}"
    cd "$PROJECT_ROOT"
    "$SCRIPT_DIR/run-dual-collectors.sh" "$@"
}

# Deploy to Kubernetes
k8s_deploy() {
    local quick=false
    
    # Parse options
    while [[ $# -gt 0 ]]; do
        case $1 in
            --quick)
                quick=true
                shift
                ;;
            *)
                shift
                ;;
        esac
    done
    
    echo -e "${GREEN}Deploying to Kubernetes...${NC}"
    cd "$PROJECT_ROOT"
    
    # Build image if not quick mode
    if [ "$quick" = false ]; then
        docker_build
        
        # Tag and push if registry is configured
        if [ -n "${DOCKER_REGISTRY:-}" ]; then
            docker tag postgres-unified-collector:latest "$DOCKER_REGISTRY/postgres-unified-collector:latest"
            docker push "$DOCKER_REGISTRY/postgres-unified-collector:latest"
        fi
    fi
    
    # Apply Kubernetes manifests
    kubectl apply -k deployments/kubernetes/overlays/production/
    
    # Wait for deployment
    echo "Waiting for deployment to be ready..."
    kubectl wait --for=condition=available --timeout=300s \
        deployment/postgres-collector -n postgres-monitoring
    
    echo -e "${GREEN}Kubernetes deployment complete!${NC}"
}

# Remove from Kubernetes
k8s_remove() {
    echo -e "${YELLOW}Removing from Kubernetes...${NC}"
    cd "$PROJECT_ROOT"
    kubectl delete -k deployments/kubernetes/overlays/production/
    echo -e "${GREEN}Kubernetes resources removed${NC}"
}

# Run end-to-end test
run_e2e() {
    echo -e "${GREEN}Running end-to-end test...${NC}"
    cd "$PROJECT_ROOT"
    
    # Start infrastructure
    echo "Starting infrastructure..."
    docker-compose up -d postgres otel-collector
    sleep 10
    
    # Run load generator in background
    echo "Starting load generator..."
    "$SCRIPT_DIR/run-load-test.sh" &
    LOAD_PID=$!
    
    # Run collector
    echo "Starting collector in hybrid mode..."
    start_collector --mode hybrid &
    COLLECTOR_PID=$!
    
    # Wait for metrics collection
    echo "Waiting for metrics collection (60 seconds)..."
    sleep 60
    
    # Stop processes
    kill $LOAD_PID 2>/dev/null || true
    kill $COLLECTOR_PID 2>/dev/null || true
    
    # Verify metrics
    echo "Verifying metrics..."
    "$SCRIPT_DIR/verify-metrics.sh"
    
    echo -e "${GREEN}End-to-end test complete!${NC}"
}

# Verify metrics in New Relic
verify_metrics() {
    echo -e "${BLUE}Verifying metrics in New Relic...${NC}"
    cd "$PROJECT_ROOT"
    "$SCRIPT_DIR/verify-metrics.sh"
}

# Generate PostgreSQL load
load_test() {
    echo -e "${BLUE}Generating PostgreSQL load...${NC}"
    cd "$PROJECT_ROOT"
    "$SCRIPT_DIR/run-load-test.sh" "$@"
}

# Clean build artifacts
clean() {
    echo -e "${YELLOW}Cleaning build artifacts...${NC}"
    cd "$PROJECT_ROOT"
    cargo clean
    rm -rf target/
    docker-compose down -v
    echo -e "${GREEN}Clean complete!${NC}"
}

# Run all tests
run_tests() {
    echo -e "${GREEN}Running all tests...${NC}"
    cd "$PROJECT_ROOT"
    
    # Rust tests
    echo "Running Rust tests..."
    cargo test --all-features
    
    # Integration tests
    echo "Running integration tests..."
    test_nri
    test_otel
    
    echo -e "${GREEN}All tests passed!${NC}"
}

# Main execution
main() {
    load_env
    
    case "${1:-help}" in
        build)
            validate_env
            build_collector "${@:2}"
            ;;
        start)
            validate_env
            start_collector "${@:2}"
            ;;
        stop)
            stop_all
            ;;
        test)
            validate_env
            run_tests
            ;;
        test-nri)
            validate_env
            test_nri
            ;;
        test-otel)
            validate_env
            test_otel
            ;;
        docker-build)
            docker_build
            ;;
        docker-run)
            validate_env
            docker_run "${2:-hybrid}"
            ;;
        docker-dual)
            validate_env
            docker_dual "${@:2}"
            ;;
        k8s-deploy)
            validate_env
            k8s_deploy "${@:2}"
            ;;
        k8s-remove)
            k8s_remove
            ;;
        e2e)
            validate_env
            run_e2e
            ;;
        verify)
            validate_env
            verify_metrics
            ;;
        load-test)
            validate_env
            load_test "${@:2}"
            ;;
        clean)
            clean
            ;;
        help|--help|-h)
            show_help
            ;;
        *)
            echo -e "${RED}Unknown command: $1${NC}"
            show_help
            exit 1
            ;;
    esac
}

# Run main function
main "$@"