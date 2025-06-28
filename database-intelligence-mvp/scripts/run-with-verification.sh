#!/bin/bash
# Run Database Intelligence MVP with Integrated Verification
# This script starts the collector with real-time verification and monitoring

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
NC='\033[0m'

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Source common functions
source "${SCRIPT_DIR}/lib/common.sh"

# Configuration
COLLECTOR_CONFIG="${PROJECT_ROOT}/config/collector-with-verification.yaml"
DOCKER_COMPOSE="${PROJECT_ROOT}/deploy/docker/docker-compose.yaml"
VERIFICATION_LOG="${PROJECT_ROOT}/logs/verification-$(date +%Y%m%d-%H%M%S).log"

# Ensure log directory exists
mkdir -p "$(dirname "$VERIFICATION_LOG")"

# ====================
# Pre-flight Checks
# ====================

preflight_checks() {
    echo -e "${PURPLE}=== Running Pre-flight Checks ===${NC}\n"
    
    # Check Docker
    if ! docker info > /dev/null 2>&1; then
        error "Docker is not running. Please start Docker Desktop."
        exit 1
    fi
    success "Docker is running"
    
    # Check environment
    if [[ ! -f "${PROJECT_ROOT}/.env" ]]; then
        error "Environment file not found. Run: ./quickstart.sh configure"
        exit 1
    fi
    success "Environment configured"
    
    # Validate configuration
    if [[ ! -f "$COLLECTOR_CONFIG" ]]; then
        error "Collector configuration not found: $COLLECTOR_CONFIG"
        exit 1
    fi
    success "Configuration file exists"
    
    # Check New Relic credentials
    source "${PROJECT_ROOT}/.env"
    if [[ -z "${NEW_RELIC_LICENSE_KEY:-}" ]]; then
        error "NEW_RELIC_LICENSE_KEY not set in .env"
        exit 1
    fi
    success "New Relic credentials configured"
}

# ====================
# Update Docker Compose
# ====================

update_docker_compose() {
    echo -e "\n${PURPLE}=== Updating Docker Compose Configuration ===${NC}\n"
    
    # Create a temporary docker-compose override
    cat > "${PROJECT_ROOT}/deploy/docker/docker-compose.override.yaml" <<EOF
version: '3.8'

services:
  db-intelligence-primary:
    volumes:
      - ${COLLECTOR_CONFIG}:/etc/otel/config.yaml:ro
    environment:
      - VERIFICATION_ENABLED=true
      - FEEDBACK_LOG_PATH=/var/log/verification.log
    labels:
      - "verification.enabled=true"
      - "verification.version=2.1.0"
EOF
    
    success "Docker Compose updated for verification"
}

# ====================
# Start Collector
# ====================

start_collector() {
    echo -e "\n${PURPLE}=== Starting Collector with Verification ===${NC}\n"
    
    cd "${PROJECT_ROOT}/deploy/docker"
    
    # Stop any existing containers
    docker-compose down 2>/dev/null || true
    
    # Start with verification config
    docker-compose up -d db-intelligence-primary
    
    echo "Waiting for collector to start..."
    sleep 10
    
    # Check if started
    if docker ps | grep -q db-intel-primary; then
        success "Collector started successfully"
    else
        error "Failed to start collector"
        docker logs db-intel-primary
        exit 1
    fi
}

# ====================
# Monitor Verification
# ====================

monitor_verification() {
    echo -e "\n${PURPLE}=== Monitoring Verification Status ===${NC}\n"
    
    # Start log streaming in background
    docker logs -f db-intel-primary 2>&1 | grep -E "(verification|feedback|VERIFICATION)" > "$VERIFICATION_LOG" &
    local LOG_PID=$!
    
    # Wait for initial verification
    echo "Waiting for initial verification..."
    sleep 20
    
    # Check health endpoint
    echo -e "\n${BLUE}Checking Health Endpoints:${NC}"
    
    # Basic health
    if curl -sf http://localhost:13133/health > /dev/null; then
        success "Basic health check: OK"
    else
        warning "Basic health check: FAILED"
    fi
    
    # Detailed health
    echo -e "\n${BLUE}Detailed Health Status:${NC}"
    curl -s http://localhost:13133/health/detailed | jq '.' 2>/dev/null || echo "Not available yet"
    
    # Verification status
    echo -e "\n${BLUE}Verification Status:${NC}"
    curl -s http://localhost:13133/health/verification | jq '.' 2>/dev/null || echo "Not available yet"
    
    # Remediation suggestions
    echo -e "\n${BLUE}Remediation Suggestions:${NC}"
    curl -s http://localhost:13133/health/remediation | jq '.remediations' 2>/dev/null || echo "None"
    
    # Stop log streaming
    kill $LOG_PID 2>/dev/null || true
}

# ====================
# Run Verification Tests
# ====================

run_verification_tests() {
    echo -e "\n${PURPLE}=== Running Verification Tests ===${NC}\n"
    
    # Test 1: Check data ingestion
    echo -e "${BLUE}Test 1: Data Ingestion${NC}"
    local metrics=$(curl -s http://localhost:8888/metrics)
    local received=$(echo "$metrics" | grep "otelcol_receiver_accepted_log_records_total" | tail -1 | awk '{print $2}' || echo "0")
    
    if [[ "$received" != "0" && "$received" != "" ]]; then
        success "Data ingestion verified: $received records"
    else
        warning "No data ingested yet"
    fi
    
    # Test 2: Check for errors
    echo -e "\n${BLUE}Test 2: Integration Errors${NC}"
    local errors=$(docker logs db-intel-primary 2>&1 | grep -c "ERROR" || echo "0")
    
    if [[ "$errors" -eq 0 ]]; then
        success "No errors detected"
    else
        warning "Found $errors errors in logs"
    fi
    
    # Test 3: Entity synthesis
    echo -e "\n${BLUE}Test 3: Entity Synthesis${NC}"
    local feedback=$(curl -s http://localhost:13133/health/feedback | jq -r '.[] | select(.category == "entity_synthesis") | .message' 2>/dev/null)
    
    if [[ -n "$feedback" ]]; then
        echo "Entity synthesis feedback: $feedback"
    else
        success "No entity synthesis issues"
    fi
    
    # Test 4: Circuit breaker
    echo -e "\n${BLUE}Test 4: Circuit Breaker Status${NC}"
    local cb_status=$(curl -s http://localhost:13133/health/detailed | jq -r '.databases | to_entries[] | "\(.key): \(.value.circuit_breaker_state)"' 2>/dev/null)
    
    if [[ -n "$cb_status" ]]; then
        echo "$cb_status"
    else
        echo "No database connections established yet"
    fi
}

# ====================
# Show Dashboard Info
# ====================

show_dashboard_info() {
    echo -e "\n${PURPLE}=== Verification Dashboard Information ===${NC}\n"
    
    echo -e "${YELLOW}Available Endpoints:${NC}"
    echo "  Health Check:       http://localhost:13133/health"
    echo "  Detailed Health:    http://localhost:13133/health/detailed"
    echo "  Verification:       http://localhost:13133/health/verification"
    echo "  Feedback History:   http://localhost:13133/health/feedback"
    echo "  Remediation:        http://localhost:13133/health/remediation"
    echo "  Metrics:            http://localhost:8888/metrics"
    echo "  Debug UI:           http://localhost:55679"
    
    echo -e "\n${YELLOW}New Relic Dashboard:${NC}"
    echo "  1. Go to: https://one.newrelic.com/dashboards"
    echo "  2. Import: ${PROJECT_ROOT}/monitoring/verification-dashboard.json"
    
    echo -e "\n${YELLOW}Key Verification Queries:${NC}"
    echo "  # Check for silent failures"
    echo "  SELECT count(*) FROM NrIntegrationError"
    echo "  WHERE newRelicFeature = 'Metrics'"
    echo "  SINCE 5 minutes ago"
    echo ""
    echo "  # Verify entity creation"
    echo "  SELECT uniques(entity.guid) FROM Log"
    echo "  WHERE entity.type = 'DATABASE'"
    echo "  SINCE 1 hour ago"
    echo ""
    echo "  # Check verification feedback"
    echo "  SELECT * FROM Log"
    echo "  WHERE service.name = 'database-intelligence-verification'"
    echo "  SINCE 30 minutes ago"
}

# ====================
# Continuous Monitoring
# ====================

continuous_monitoring() {
    echo -e "\n${PURPLE}=== Starting Continuous Monitoring ===${NC}"
    echo -e "${YELLOW}Press Ctrl+C to stop${NC}\n"
    
    while true; do
        echo -e "\n${BLUE}$(date '+%Y-%m-%d %H:%M:%S') - Health Check${NC}"
        
        # Quick health check
        if curl -sf http://localhost:13133/health > /dev/null; then
            echo -e "${GREEN}✓${NC} Collector healthy"
        else
            echo -e "${RED}✗${NC} Collector unhealthy"
        fi
        
        # Check for recent feedback
        local recent_feedback=$(curl -s http://localhost:13133/health/feedback | \
            jq -r '.[-3:] | .[] | "\(.timestamp | split("T")[1] | split(".")[0]) [\(.level)] \(.message)"' 2>/dev/null)
        
        if [[ -n "$recent_feedback" ]]; then
            echo -e "\n${YELLOW}Recent Feedback:${NC}"
            echo "$recent_feedback"
        fi
        
        # Check metrics
        local metrics=$(curl -s http://localhost:8888/metrics 2>/dev/null)
        local rate=$(echo "$metrics" | grep "otelcol_receiver_accepted_log_records_total" | tail -1 | awk '{print $2}' || echo "0")
        echo -e "\n${BLUE}Records processed:${NC} $rate"
        
        sleep 30
    done
}

# ====================
# Main Execution
# ====================

main() {
    echo -e "${GREEN}=== Database Intelligence MVP with Verification ===${NC}"
    echo -e "Version: 2.1.0\n"
    
    # Run checks
    preflight_checks
    
    # Update configuration
    update_docker_compose
    
    # Start collector
    start_collector
    
    # Monitor verification
    monitor_verification
    
    # Run tests
    run_verification_tests
    
    # Show dashboard info
    show_dashboard_info
    
    echo -e "\n${GREEN}=== Verification System Active ===${NC}"
    echo "Verification log: $VERIFICATION_LOG"
    echo ""
    
    # Ask if user wants continuous monitoring
    read -p "Start continuous monitoring? (y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        continuous_monitoring
    else
        echo -e "\n${YELLOW}To monitor manually:${NC}"
        echo "  docker logs -f db-intel-primary"
        echo "  curl http://localhost:13133/health/detailed"
        echo "  tail -f $VERIFICATION_LOG"
    fi
}

# Handle cleanup on exit
cleanup() {
    echo -e "\n${YELLOW}Cleaning up...${NC}"
    # Remove override file
    rm -f "${PROJECT_ROOT}/deploy/docker/docker-compose.override.yaml"
}

trap cleanup EXIT

# Run main
main "$@"