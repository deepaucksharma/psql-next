#!/bin/bash

# Deploy MySQL Wait-Based Monitoring
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}=== MySQL Wait-Based Monitoring Deployment ===${NC}"
echo "Project root: $PROJECT_ROOT"

# Function to print status
print_status() {
    local status=$1
    local message=$2
    case $status in
        "success")
            echo -e "${GREEN}✓${NC} $message"
            ;;
        "error")
            echo -e "${RED}✗${NC} $message"
            ;;
        "info")
            echo -e "${YELLOW}ℹ${NC} $message"
            ;;
        "step")
            echo -e "${BLUE}►${NC} $message"
            ;;
    esac
}

# Function to check prerequisites
check_prerequisites() {
    print_status "step" "Checking prerequisites..."
    
    # Check Docker
    if ! command -v docker &> /dev/null; then
        print_status "error" "Docker is not installed"
        exit 1
    fi
    print_status "success" "Docker is installed"
    
    # Check Docker Compose
    if ! command -v docker-compose &> /dev/null && ! docker compose version &> /dev/null; then
        print_status "error" "Docker Compose is not installed"
        exit 1
    fi
    print_status "success" "Docker Compose is installed"
    
    # Check if Docker is running
    if ! docker info &> /dev/null; then
        print_status "error" "Docker daemon is not running"
        exit 1
    fi
    print_status "success" "Docker daemon is running"
    
    # Check for required environment variables
    if [ -z "$NEW_RELIC_LICENSE_KEY" ]; then
        print_status "error" "NEW_RELIC_LICENSE_KEY is not set"
        print_status "info" "Please export NEW_RELIC_LICENSE_KEY=your-key"
        exit 1
    fi
    print_status "success" "New Relic license key is configured"
}

# Function to validate configuration
validate_configuration() {
    print_status "step" "Validating configuration files..."
    
    # Check required config files
    local configs=(
        "config/edge-collector-wait.yaml"
        "config/gateway-advisory.yaml"
        "mysql/init/04-enable-wait-analysis.sql"
        "mysql/init/05-create-test-workload.sql"
    )
    
    for config in "${configs[@]}"; do
        if [ ! -f "$PROJECT_ROOT/$config" ]; then
            print_status "error" "Missing configuration file: $config"
            exit 1
        fi
        print_status "success" "Found: $config"
    done
}

# Function to deploy services
deploy_services() {
    print_status "step" "Deploying services..."
    
    cd "$PROJECT_ROOT"
    
    # Stop any existing services
    print_status "info" "Stopping existing services..."
    docker-compose down --remove-orphans
    
    # Pull latest images
    print_status "info" "Pulling Docker images..."
    docker-compose pull
    
    # Start services
    print_status "info" "Starting services..."
    docker-compose up -d
    
    # Wait for services to be healthy
    print_status "info" "Waiting for services to be healthy..."
    local services=("mysql-primary" "mysql-replica" "otel-collector-edge" "otel-gateway")
    
    for service in "${services[@]}"; do
        local retries=30
        while [ $retries -gt 0 ]; do
            if docker-compose ps | grep $service | grep -q "healthy\|Up"; then
                print_status "success" "$service is healthy"
                break
            fi
            retries=$((retries - 1))
            sleep 2
        done
        
        if [ $retries -eq 0 ]; then
            print_status "error" "$service failed to become healthy"
            docker-compose logs $service
            exit 1
        fi
    done
}

# Function to verify deployment
verify_deployment() {
    print_status "step" "Verifying deployment..."
    
    # Check edge collector
    if curl -s -f http://localhost:13133/ > /dev/null; then
        print_status "success" "Edge collector health check passed"
    else
        print_status "error" "Edge collector health check failed"
        exit 1
    fi
    
    # Check gateway
    if curl -s -f http://localhost:13134/ > /dev/null; then
        print_status "success" "Gateway health check passed"
    else
        print_status "error" "Gateway health check failed"
        exit 1
    fi
    
    # Check MySQL connectivity
    if docker exec mysql-primary mysqladmin ping -h localhost -u root -prootpassword &> /dev/null; then
        print_status "success" "MySQL primary is accessible"
    else
        print_status "error" "MySQL primary is not accessible"
        exit 1
    fi
    
    # Check Performance Schema
    local ps_enabled=$(docker exec mysql-primary mysql -u root -prootpassword -sN -e "SELECT @@performance_schema")
    if [ "$ps_enabled" = "1" ]; then
        print_status "success" "Performance Schema is enabled"
    else
        print_status "error" "Performance Schema is not enabled"
        exit 1
    fi
}

# Function to generate initial workload
generate_initial_workload() {
    print_status "step" "Generating initial workload..."
    
    # Enable event schedulers for continuous load
    docker exec mysql-primary mysql -u root -prootpassword wait_analysis_test -e "
        ALTER EVENT e_generate_io_load ENABLE;
        ALTER EVENT e_generate_lock_load ENABLE;
    "
    
    print_status "success" "Workload generation enabled"
    
    # Generate some immediate load
    docker exec mysql-primary mysql -u root -prootpassword wait_analysis_test -e "
        CALL generate_io_waits();
        CALL generate_lock_waits();
        CALL generate_temp_table_waits();
    "
    
    print_status "success" "Initial workload generated"
}

# Function to display next steps
display_next_steps() {
    echo ""
    echo -e "${GREEN}=== Deployment Successful! ===${NC}"
    echo ""
    echo "Next steps:"
    echo "1. View metrics locally:"
    echo "   - Edge collector metrics: http://localhost:8888/metrics"
    echo "   - Gateway metrics: http://localhost:8889/metrics"
    echo "   - Prometheus endpoint: http://localhost:9091/metrics"
    echo ""
    echo "2. Import New Relic dashboards:"
    echo "   - Wait Analysis Dashboard: dashboards/newrelic/wait-analysis-dashboard.json"
    echo "   - Query Detail Dashboard: dashboards/newrelic/query-detail-dashboard.json"
    echo ""
    echo "3. Configure alerts in New Relic:"
    echo "   - Use dashboards/newrelic/wait-based-alerts.yaml as reference"
    echo ""
    echo "4. Run validation tests:"
    echo "   cd $PROJECT_ROOT"
    echo "   make test-e2e"
    echo ""
    echo "5. Monitor collector logs:"
    echo "   docker-compose logs -f otel-collector-edge"
    echo "   docker-compose logs -f otel-gateway"
}

# Main execution
main() {
    cd "$PROJECT_ROOT"
    
    # Parse command line arguments
    case "${1:-}" in
        "--skip-checks")
            print_status "info" "Skipping prerequisite checks"
            ;;
        *)
            check_prerequisites
            ;;
    esac
    
    validate_configuration
    deploy_services
    verify_deployment
    
    # Optional: generate workload
    if [ "${2:-}" = "--with-workload" ]; then
        generate_initial_workload
    fi
    
    display_next_steps
}

# Run main function
main "$@"