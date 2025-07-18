#!/bin/bash
# Docker deployment script for Database Intelligence

set -e

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Source utilities
source "$SCRIPT_DIR/../utils/common.sh"

# Configuration
ACTION=${1:-up}
PROFILE=${2:-production}
COMPOSE_FILE="$ROOT_DIR/deployments/docker/docker-compose.yml"

# Show usage
usage() {
    cat << EOF
Docker Deployment for Database Intelligence

Usage: $0 [action] [profile]

Actions:
  up       Start services (default)
  down     Stop services
  restart  Restart services
  logs     Show logs
  status   Show status
  build    Build images

Profiles:
  minimal     Minimal distribution
  production  Production distribution (default)
  enterprise  Enterprise distribution

Examples:
  $0                    # Start production services
  $0 up minimal         # Start minimal services
  $0 down               # Stop all services
  $0 logs               # Show logs
  $0 build production   # Build production image

Environment Variables Required:
  NEW_RELIC_LICENSE_KEY  New Relic license key
  DB_ENDPOINT           Database connection string
EOF
    exit 0
}

# Check for help
if [[ "$1" == "-h" ]] || [[ "$1" == "--help" ]]; then
    usage
fi

print_header "Docker Deployment"
log_info "Action: $ACTION"
log_info "Profile: $PROFILE"

# Check requirements
check_requirements docker docker-compose || exit 1

# Check environment variables
check_env_vars() {
    local missing=()
    
    if [[ -z "${NEW_RELIC_LICENSE_KEY:-}" ]]; then
        missing+=("NEW_RELIC_LICENSE_KEY")
    fi
    
    if [[ -z "${DB_ENDPOINT:-}" ]] && [[ "$ACTION" == "up" ]]; then
        missing+=("DB_ENDPOINT")
    fi
    
    if [[ ${#missing[@]} -gt 0 ]]; then
        log_error "Missing required environment variables: ${missing[*]}"
        log_info "Please set them or create a .env file"
        return 1
    fi
    
    return 0
}

# Create docker-compose file if needed
create_compose_file() {
    if [[ ! -f "$COMPOSE_FILE" ]]; then
        log_info "Creating docker-compose file..."
        mkdir -p "$(dirname "$COMPOSE_FILE")"
        
        cat > "$COMPOSE_FILE" << 'EOF'
version: '3.8'

services:
  collector:
    image: dbintel:${PROFILE:-production}
    container_name: dbintel-collector
    restart: unless-stopped
    environment:
      - NEW_RELIC_LICENSE_KEY=${NEW_RELIC_LICENSE_KEY}
      - DB_ENDPOINT=${DB_ENDPOINT}
      - SERVICE_NAME=${SERVICE_NAME:-database-intelligence}
      - ENVIRONMENT=${ENVIRONMENT:-production}
      - OTEL_LOG_LEVEL=${OTEL_LOG_LEVEL:-info}
    ports:
      - "8888:8888"   # Prometheus metrics
      - "13133:13133" # Health check
      - "4317:4317"   # OTLP gRPC
      - "4318:4318"   # OTLP HTTP
    volumes:
      - ./configs:/etc/otelcol:ro
      - ./logs:/var/log/otelcol
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:13133/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s
    networks:
      - dbintel

  # Optional: Prometheus for local metrics viewing
  prometheus:
    image: prom/prometheus:latest
    container_name: dbintel-prometheus
    restart: unless-stopped
    ports:
      - "9090:9090"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml:ro
      - prometheus-data:/prometheus
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'
    networks:
      - dbintel
    profiles:
      - monitoring

networks:
  dbintel:
    driver: bridge

volumes:
  prometheus-data:
EOF
    fi
}

# Create Prometheus config if needed
create_prometheus_config() {
    local prom_config="$ROOT_DIR/deployments/docker/prometheus.yml"
    if [[ ! -f "$prom_config" ]]; then
        log_info "Creating Prometheus configuration..."
        cat > "$prom_config" << 'EOF'
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: 'otel-collector'
    static_configs:
      - targets: ['collector:8888']
    scrape_interval: 10s
EOF
    fi
}

# Execute action
case "$ACTION" in
    up)
        check_env_vars || exit 1
        create_compose_file
        create_prometheus_config
        
        # Build image if it doesn't exist
        if ! docker images | grep -q "dbintel:$PROFILE"; then
            log_info "Building Docker image..."
            "$SCRIPT_DIR/../build/build.sh" docker --profile "$PROFILE"
        fi
        
        log_info "Starting services..."
        docker-compose -f "$COMPOSE_FILE" up -d
        
        # Wait for services to be ready
        wait_for_service "Collector" 13133 60
        
        log_success "Services started successfully"
        log_info "Health check: http://localhost:13133/health"
        log_info "Metrics: http://localhost:8888/metrics"
        
        if docker-compose -f "$COMPOSE_FILE" ps | grep -q prometheus; then
            log_info "Prometheus: http://localhost:9090"
        fi
        ;;
        
    down)
        log_info "Stopping services..."
        docker-compose -f "$COMPOSE_FILE" down
        log_success "Services stopped"
        ;;
        
    restart)
        log_info "Restarting services..."
        docker-compose -f "$COMPOSE_FILE" restart
        log_success "Services restarted"
        ;;
        
    logs)
        docker-compose -f "$COMPOSE_FILE" logs -f
        ;;
        
    status)
        log_info "Service status:"
        docker-compose -f "$COMPOSE_FILE" ps
        
        # Check health
        if curl -s http://localhost:13133/health | grep -q "OK"; then
            log_success "Collector is healthy"
        else
            log_warning "Collector health check failed"
        fi
        ;;
        
    build)
        log_info "Building Docker image..."
        "$SCRIPT_DIR/../build/build.sh" docker --profile "$PROFILE"
        ;;
        
    *)
        log_error "Unknown action: $ACTION"
        usage
        ;;
esac