#!/bin/bash

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored messages
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if Docker and Docker Compose are installed
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    if ! command -v docker &> /dev/null; then
        log_error "Docker is not installed. Please install Docker first."
        exit 1
    fi
    
    if ! docker compose version &> /dev/null; then
        log_error "Docker Compose is not installed. Please install Docker Compose V2."
        exit 1
    fi
    
    log_info "Prerequisites check passed."
}

# Create .env file from example if it doesn't exist
setup_env_file() {
    if [ ! -f .env ]; then
        log_info "Creating .env file from .env.example..."
        cp .env.example .env
        log_warn "Please edit .env file and add your New Relic API key and account ID."
        log_warn "You can get your API key from: https://one.newrelic.com/api-keys"
        read -p "Press Enter to continue after updating .env file..."
    else
        log_info ".env file already exists."
    fi
}

# Check if New Relic credentials are set
check_newrelic_credentials() {
    if [ -f .env ]; then
        source .env
        if [ -z "${NEW_RELIC_API_KEY:-}" ] || [ "${NEW_RELIC_API_KEY}" = "your_new_relic_ingest_license_key_here" ]; then
            log_error "NEW_RELIC_API_KEY is not set in .env file"
            log_info "Get your API key from: https://one.newrelic.com/api-keys"
            exit 1
        fi
        if [ -z "${NEW_RELIC_ACCOUNT_ID:-}" ] || [ "${NEW_RELIC_ACCOUNT_ID}" = "your_new_relic_account_id_here" ]; then
            log_error "NEW_RELIC_ACCOUNT_ID is not set in .env file"
            log_info "Find your account ID in New Relic UI (top right corner)"
            exit 1
        fi
        log_info "New Relic credentials found."
    fi
}

# Create required directories
create_directories() {
    log_info "Creating required directories..."
    mkdir -p mysql/data
    mkdir -p logs
    chmod 755 mysql/init/*.sql
}

# Start the services
start_services() {
    log_info "Starting MySQL and OpenTelemetry Collector..."
    docker compose up -d
    
    log_info "Waiting for services to be healthy..."
    sleep 10
    
    # Check service health
    if docker compose ps | grep -q "unhealthy"; then
        log_error "Some services are unhealthy. Checking logs..."
        docker compose logs --tail=50
        exit 1
    fi
}

# Configure MySQL replication
setup_replication() {
    log_info "Setting up MySQL replication..."
    
    # Wait for MySQL to be fully ready
    sleep 5
    
    # Get master status
    MASTER_STATUS=$(docker compose exec -T mysql-primary mysql -uroot -prootpassword -e "SHOW MASTER STATUS\G" | grep -E "File:|Position:" | sed 's/: /=/g' | tr '\n' ' ')
    
    if [ -z "$MASTER_STATUS" ]; then
        log_error "Failed to get master status"
        return 1
    fi
    
    # Extract file and position
    eval $MASTER_STATUS
    
    # Configure replica
    docker compose exec -T mysql-replica mysql -uroot -prootpassword <<EOF
STOP SLAVE;
CHANGE MASTER TO
    MASTER_HOST='mysql-primary',
    MASTER_USER='root',
    MASTER_PASSWORD='rootpassword',
    MASTER_AUTO_POSITION=1;
START SLAVE;
EOF
    
    # Check replication status
    sleep 2
    SLAVE_STATUS=$(docker compose exec -T mysql-replica mysql -uroot -prootpassword -e "SHOW SLAVE STATUS\G" | grep "Slave_IO_Running:" | awk '{print $2}')
    
    if [ "$SLAVE_STATUS" = "Yes" ]; then
        log_info "Replication setup successful!"
    else
        log_warn "Replication setup may have issues. Check with: docker compose exec mysql-replica mysql -uroot -prootpassword -e 'SHOW SLAVE STATUS\\G'"
    fi
}

# Display service information
show_info() {
    log_info "Services are running!"
    echo ""
    echo "MySQL Primary: localhost:3306"
    echo "MySQL Replica: localhost:3307"
    echo "OTel Collector Health: http://localhost:13133/"
    echo "OTel Collector zPages: http://localhost:55679/debug/tracez"
    echo ""
    echo "Default MySQL credentials:"
    echo "  Root: root/rootpassword"
    echo "  App: appuser/apppassword"
    echo "  Monitor: otel_monitor/otelmonitorpass"
    echo ""
    echo "To view logs:"
    echo "  docker compose logs -f otel-collector"
    echo ""
    echo "To stop services:"
    echo "  docker compose down"
    echo ""
    log_info "Check your New Relic account for MySQL metrics!"
    log_info "Dashboard URL: https://one.newrelic.com/"
}

# Main execution
main() {
    log_info "Starting MySQL OpenTelemetry Monitoring Setup..."
    
    check_prerequisites
    setup_env_file
    check_newrelic_credentials
    create_directories
    start_services
    setup_replication
    show_info
    
    log_info "Setup completed successfully!"
}

# Handle errors
trap 'log_error "Setup failed. Check the error messages above."; exit 1' ERR

# Run main function
main "$@"