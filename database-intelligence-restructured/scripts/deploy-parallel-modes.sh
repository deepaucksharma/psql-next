#!/bin/bash

# Deploy Parallel Modes Script
# Deploys both Config-Only and Custom/Enhanced modes in parallel

set -euo pipefail

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
DOCKER_COMPOSE_DIR="$PROJECT_ROOT/deployments/docker/compose"

# Functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    # Check Docker
    if ! command -v docker &> /dev/null; then
        log_error "Docker is required but not installed"
        exit 1
    fi
    
    # Check Docker Compose
    if ! docker compose version &> /dev/null; then
        log_error "Docker Compose is required but not installed"
        exit 1
    fi
    
    # Check environment variables
    if [[ -z "${NEW_RELIC_LICENSE_KEY:-}" ]]; then
        log_error "NEW_RELIC_LICENSE_KEY environment variable is required"
        exit 1
    fi
    
    log_success "Prerequisites check passed"
}

# Build custom collector image
build_custom_image() {
    log_info "Building custom collector image..."
    
    cd "$PROJECT_ROOT"
    
    # Build enterprise collector
    docker build -t newrelic/database-intelligence-enterprise:latest \
        -f deployments/docker/Dockerfile.enterprise .
    
    # Build load generator
    docker build -t newrelic/database-intelligence-loadgen:latest \
        -f deployments/docker/Dockerfile.loadgen .
    
    log_success "Custom images built successfully"
}

# Deploy services
deploy_services() {
    log_info "Deploying services..."
    
    cd "$DOCKER_COMPOSE_DIR"
    
    # Stop any existing services
    docker compose -f docker-compose-parallel.yaml down || true
    
    # Start services
    docker compose -f docker-compose-parallel.yaml up -d
    
    log_success "Services deployed successfully"
}

# Wait for services to be healthy
wait_for_health() {
    log_info "Waiting for services to be healthy..."
    
    local max_attempts=30
    local attempt=0
    
    while [ $attempt -lt $max_attempts ]; do
        if docker compose -f "$DOCKER_COMPOSE_DIR/docker-compose-parallel.yaml" ps | grep -q "unhealthy"; then
            log_info "Waiting for services to become healthy... ($attempt/$max_attempts)"
            sleep 10
            ((attempt++))
        else
            log_success "All services are healthy"
            return 0
        fi
    done
    
    log_error "Services failed to become healthy"
    return 1
}

# Deploy dashboards
deploy_dashboards() {
    log_info "Deploying dashboards to New Relic..."
    
    # Deploy Unified Parallel dashboard
    log_info "Deploying Unified Parallel Monitoring dashboard..."
    "$SCRIPT_DIR/migrate-dashboard.sh" deploy "$PROJECT_ROOT/dashboards/newrelic/unified-parallel-dashboard.json"
    
    # Optional: Deploy individual dashboards
    read -p "Deploy individual mode dashboards as well? (y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        log_info "Deploying Config-Only mode dashboard..."
        "$SCRIPT_DIR/migrate-dashboard.sh" deploy "$PROJECT_ROOT/dashboards/newrelic/config-only-dashboard.json"
        
        log_info "Deploying Custom/Enhanced mode dashboard..."
        "$SCRIPT_DIR/migrate-dashboard.sh" deploy "$PROJECT_ROOT/dashboards/newrelic/custom-mode-dashboard.json"
        
        log_info "Deploying Comparison dashboard..."
        "$SCRIPT_DIR/migrate-dashboard.sh" deploy "$PROJECT_ROOT/dashboards/newrelic/comparison-dashboard.json"
    fi
    
    log_success "Dashboards deployed successfully"
}

# Show status
show_status() {
    log_info "Deployment Status:"
    echo ""
    
    # Show running containers
    docker compose -f "$DOCKER_COMPOSE_DIR/docker-compose-parallel.yaml" ps
    
    echo ""
    log_info "Access points:"
    echo "  - Config-Only Collector: http://localhost:4318 (OTLP HTTP)"
    echo "  - Custom Mode Collector: http://localhost:5318 (OTLP HTTP)"
    echo "  - PostgreSQL: localhost:5432"
    echo "  - MySQL: localhost:3306"
    echo ""
    
    log_info "View logs:"
    echo "  - Config-Only: docker logs db-intel-collector-config-only"
    echo "  - Custom Mode: docker logs db-intel-collector-custom"
    echo "  - PostgreSQL: docker logs db-intel-postgres"
    echo "  - MySQL: docker logs db-intel-mysql"
    echo ""
}

# Main execution
main() {
    log_info "Starting parallel mode deployment..."
    
    check_prerequisites
    build_custom_image
    deploy_services
    wait_for_health
    
    # Optional: Deploy dashboards
    read -p "Deploy dashboards to New Relic? (y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        deploy_dashboards
    fi
    
    show_status
    
    log_success "Parallel mode deployment completed!"
    log_info "Both Config-Only and Custom modes are now running"
}

# Handle cleanup on exit
cleanup() {
    if [[ "${1:-}" == "error" ]]; then
        log_error "Deployment failed. Cleaning up..."
        docker compose -f "$DOCKER_COMPOSE_DIR/docker-compose-parallel.yaml" down || true
    fi
}

trap 'cleanup error' ERR

# Run main function
main "$@"