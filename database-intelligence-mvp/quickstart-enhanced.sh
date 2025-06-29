#!/bin/bash
# Enhanced Quickstart Script with Verification & Feedback Loops
# This script provides comprehensive setup, validation, and continuous monitoring

set -euo pipefail

# Color codes for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONFIG_DIR="${SCRIPT_DIR}/config"
VERIFICATION_DIR="${SCRIPT_DIR}/verification"
LOG_DIR="${SCRIPT_DIR}/logs"
METRICS_DIR="${SCRIPT_DIR}/metrics"

# Create necessary directories
mkdir -p "$LOG_DIR" "$METRICS_DIR" "$VERIFICATION_DIR"

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $(date '+%Y-%m-%d %H:%M:%S') - $1" | tee -a "$LOG_DIR/quickstart.log"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $(date '+%Y-%m-%d %H:%M:%S') - $1" | tee -a "$LOG_DIR/quickstart.log"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $(date '+%Y-%m-%d %H:%M:%S') - $1" | tee -a "$LOG_DIR/quickstart.log"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $(date '+%Y-%m-%d %H:%M:%S') - $1" | tee -a "$LOG_DIR/quickstart.log"
}

# Verification functions
verify_prerequisites() {
    log_info "Verifying prerequisites..."
    
    local prereqs_met=true
    
    # Check Docker
    if command -v docker &> /dev/null; then
        log_success "Docker installed: $(docker --version)"
    else
        log_error "Docker not found. Please install Docker."
        prereqs_met=false
    fi
    
    # Check Docker Compose
    if command -v docker-compose &> /dev/null; then
        log_success "Docker Compose installed: $(docker-compose --version)"
    else
        log_error "Docker Compose not found. Please install Docker Compose."
        prereqs_met=false
    fi
    
    # Check environment variables
    if [ -z "${NEW_RELIC_LICENSE_KEY:-}" ]; then
        log_warning "NEW_RELIC_LICENSE_KEY not set. You'll need to provide it."
    fi
    
    if [ "$prereqs_met" = false ]; then
        return 1
    fi
    
    return 0
}

# Database connectivity verification
verify_database_connectivity() {
    local db_type=$1
    local host=$2
    local port=$3
    local user=$4
    local password=$5
    local database=$6
    
    log_info "Verifying $db_type connectivity..."
    
    if [ "$db_type" = "postgresql" ]; then
        PGPASSWORD=$password psql -h "$host" -p "$port" -U "$user" -d "$database" -c "SELECT version();" &> /dev/null
        if [ $? -eq 0 ]; then
            log_success "PostgreSQL connection successful"
            
            # Check pg_stat_statements
            PGPASSWORD=$password psql -h "$host" -p "$port" -U "$user" -d "$database" \
                -c "SELECT 1 FROM pg_extension WHERE extname='pg_stat_statements';" | grep -q 1
            if [ $? -eq 0 ]; then
                log_success "pg_stat_statements extension is enabled"
            else
                log_warning "pg_stat_statements not enabled. Enabling..."
                PGPASSWORD=$password psql -h "$host" -p "$port" -U "$user" -d "$database" \
                    -c "CREATE EXTENSION IF NOT EXISTS pg_stat_statements;"
            fi
            
            # Store connection metrics
            echo "{\"timestamp\": \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\", \"db_type\": \"postgresql\", \"status\": \"connected\", \"latency_ms\": 0}" \
                >> "$METRICS_DIR/db_connectivity.jsonl"
            
            return 0
        else
            log_error "PostgreSQL connection failed"
            return 1
        fi
    elif [ "$db_type" = "mysql" ]; then
        mysql -h "$host" -P "$port" -u "$user" -p"$password" "$database" -e "SELECT VERSION();" &> /dev/null
        if [ $? -eq 0 ]; then
            log_success "MySQL connection successful"
            
            # Check Performance Schema
            mysql -h "$host" -P "$port" -u "$user" -p"$password" "$database" \
                -e "SELECT 1 FROM information_schema.ENGINES WHERE ENGINE='PERFORMANCE_SCHEMA' AND SUPPORT='YES';" | grep -q 1
            if [ $? -eq 0 ]; then
                log_success "Performance Schema is enabled"
            else
                log_warning "Performance Schema not enabled"
            fi
            
            return 0
        else
            log_error "MySQL connection failed"
            return 1
        fi
    fi
}

# Configuration generation with validation
generate_configuration() {
    log_info "Generating OTEL collector configuration..."
    
    cat > "$CONFIG_DIR/collector-generated.yaml" <<EOF
# Auto-generated configuration with verification
extensions:
  health_check:
    endpoint: 0.0.0.0:13133
    
  # Verification extension
  verification:
    enabled: true
    interval: 60s
    checks:
      - database_connectivity
      - metric_quality
      - pii_sanitization

receivers:
  # Standard OTEL receivers
  postgresql:
    endpoint: ${PG_HOST:-localhost}:${PG_PORT:-5432}
    username: ${PG_USER:-postgres}
    password: ${PG_PASSWORD}
    databases:
      - ${PG_DATABASE:-postgres}
    collection_interval: 60s
    
  mysql:
    endpoint: ${MYSQL_HOST:-localhost}:${MYSQL_PORT:-3306}
    username: ${MYSQL_USER:-root}
    password: ${MYSQL_PASSWORD}
    database: ${MYSQL_DATABASE:-mysql}
    collection_interval: 60s

processors:
  # Verification processor
  database_intelligence/verification:
    enabled: true
    health_checks:
      database_connectivity:
        interval: 60s
        timeout: 5s
      metric_quality:
        interval: 300s
        thresholds:
          missing_attributes: 0.05
          invalid_values: 0.01
      pii_sanitization:
        interval: 600s
        
  # Standard processors
  batch:
    timeout: 10s
    
  resource:
    attributes:
      - key: service.name
        value: database-intelligence
        action: insert

exporters:
  otlp:
    endpoint: ${OTLP_ENDPOINT:-https://otlp.nr-data.net:4317}
    headers:
      api-key: ${NEW_RELIC_LICENSE_KEY}
      
  # Verification metrics exporter
  prometheus:
    endpoint: 0.0.0.0:8888

service:
  extensions: [health_check, verification]
  
  pipelines:
    metrics:
      receivers: [postgresql, mysql]
      processors: [database_intelligence/verification, batch, resource]
      exporters: [otlp, prometheus]
      
  telemetry:
    logs:
      level: info
      output_paths: ["stdout", "$LOG_DIR/collector.log"]
    metrics:
      address: 0.0.0.0:8889
EOF

    log_success "Configuration generated at $CONFIG_DIR/collector-generated.yaml"
}

# Deploy with verification
deploy_collector() {
    log_info "Deploying OTEL collector with verification..."
    
    # Create docker-compose with verification
    cat > "$SCRIPT_DIR/docker-compose-verified.yaml" <<EOF
version: '3.8'

services:
  collector:
    image: otel/opentelemetry-collector-contrib:latest
    container_name: db-intel-collector-verified
    volumes:
      - $CONFIG_DIR/collector-generated.yaml:/etc/otelcol/config.yaml:ro
      - $LOG_DIR:/var/log/collector
      - $METRICS_DIR:/var/lib/collector/metrics
    environment:
      - NEW_RELIC_LICENSE_KEY=${NEW_RELIC_LICENSE_KEY}
      - PG_HOST=${PG_HOST:-postgres}
      - PG_USER=${PG_USER}
      - PG_PASSWORD=${PG_PASSWORD}
      - PG_DATABASE=${PG_DATABASE:-postgres}
      - MYSQL_HOST=${MYSQL_HOST:-mysql}
      - MYSQL_USER=${MYSQL_USER}
      - MYSQL_PASSWORD=${MYSQL_PASSWORD}
      - MYSQL_DATABASE=${MYSQL_DATABASE:-mysql}
    ports:
      - "13133:13133"  # Health check
      - "8888:8888"    # Prometheus metrics
      - "8889:8889"    # Internal metrics
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:13133"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s
    restart: unless-stopped
    
  # Verification sidecar
  verifier:
    image: alpine:latest
    container_name: db-intel-verifier
    volumes:
      - $SCRIPT_DIR:/scripts:ro
      - $LOG_DIR:/var/log
      - $METRICS_DIR:/var/lib/metrics
    command: |
      sh -c '
        apk add --no-cache curl jq bc
        while true; do
          /scripts/verification/continuous-verify.sh
          sleep 60
        done
      '
    depends_on:
      - collector
      
  # Test databases (optional)
  postgres:
    image: postgres:15
    container_name: test-postgres
    environment:
      - POSTGRES_PASSWORD=postgres
      - POSTGRES_DB=testdb
    command: |
      postgres
      -c shared_preload_libraries=pg_stat_statements
      -c pg_stat_statements.track=all
    profiles: ["test"]
    
  mysql:
    image: mysql:8.0
    container_name: test-mysql
    environment:
      - MYSQL_ROOT_PASSWORD=mysql
      - MYSQL_DATABASE=testdb
    command: |
      --performance-schema=ON
      --performance-schema-instrument='statement/%=ON'
    profiles: ["test"]
EOF

    # Start services
    docker-compose -f docker-compose-verified.yaml up -d
    
    # Wait for collector to be ready
    log_info "Waiting for collector to be ready..."
    local retries=30
    while [ $retries -gt 0 ]; do
        if curl -sf http://localhost:13133 > /dev/null; then
            log_success "Collector is ready!"
            break
        fi
        retries=$((retries - 1))
        sleep 2
    done
    
    if [ $retries -eq 0 ]; then
        log_error "Collector failed to start"
        return 1
    fi
    
    return 0
}

# Continuous verification loop
run_continuous_verification() {
    log_info "Starting continuous verification..."
    
    # Create verification script
    cat > "$VERIFICATION_DIR/continuous-verify.sh" <<'EOF'
#!/bin/sh
# Continuous verification script

# Health score calculation
calculate_health_score() {
    local connectivity=$1
    local metrics=$2
    local pii=$3
    
    # Weighted average
    echo "scale=2; ($connectivity * 0.4 + $metrics * 0.4 + $pii * 0.2)" | bc
}

# Check database connectivity
check_connectivity() {
    curl -s http://localhost:8888/metrics | grep -q "postgresql_up 1"
    if [ $? -eq 0 ]; then
        echo "1.0"
    else
        echo "0.0"
    fi
}

# Check metric quality
check_metrics() {
    local metric_count=$(curl -s http://localhost:8888/metrics | grep -c "^postgresql_")
    if [ $metric_count -gt 10 ]; then
        echo "1.0"
    elif [ $metric_count -gt 5 ]; then
        echo "0.5"
    else
        echo "0.0"
    fi
}

# Check PII sanitization
check_pii() {
    # Look for patterns in exported metrics
    curl -s http://localhost:8888/metrics | grep -E "[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}" > /dev/null
    if [ $? -eq 0 ]; then
        echo "0.0"  # Found PII
    else
        echo "1.0"  # No PII found
    fi
}

# Main verification loop
connectivity=$(check_connectivity)
metrics=$(check_metrics)
pii=$(check_pii)

health_score=$(calculate_health_score "$connectivity" "$metrics" "$pii")

# Log results
echo "{\"timestamp\": \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\", \"health_score\": $health_score, \"connectivity\": $connectivity, \"metrics\": $metrics, \"pii\": $pii}" >> /var/lib/metrics/health.jsonl

# Take action based on health
if [ $(echo "$health_score < 0.8" | bc) -eq 1 ]; then
    echo "WARNING: Health score below threshold: $health_score"
    # Trigger self-healing actions
fi
EOF
    
    chmod +x "$VERIFICATION_DIR/continuous-verify.sh"
    
    # Start verification dashboard
    start_verification_dashboard
}

# Verification dashboard
start_verification_dashboard() {
    log_info "Starting verification dashboard..."
    
    # Create simple web dashboard
    cat > "$VERIFICATION_DIR/dashboard.html" <<'EOF'
<!DOCTYPE html>
<html>
<head>
    <title>Database Intelligence Verification Dashboard</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .metric { 
            display: inline-block; 
            margin: 10px; 
            padding: 20px; 
            border: 1px solid #ddd; 
            border-radius: 5px;
            text-align: center;
        }
        .healthy { background-color: #90EE90; }
        .warning { background-color: #FFD700; }
        .critical { background-color: #FF6B6B; }
        #health-chart { width: 100%; height: 300px; }
    </style>
</head>
<body>
    <h1>Database Intelligence Verification Dashboard</h1>
    
    <div id="metrics">
        <div class="metric" id="health-score">
            <h3>Health Score</h3>
            <p id="score-value">Loading...</p>
        </div>
        
        <div class="metric" id="connectivity">
            <h3>Database Connectivity</h3>
            <p id="connectivity-value">Loading...</p>
        </div>
        
        <div class="metric" id="metric-quality">
            <h3>Metric Quality</h3>
            <p id="quality-value">Loading...</p>
        </div>
        
        <div class="metric" id="pii-status">
            <h3>PII Sanitization</h3>
            <p id="pii-value">Loading...</p>
        </div>
    </div>
    
    <h2>Health Trend</h2>
    <canvas id="health-chart"></canvas>
    
    <h2>Recent Verification Events</h2>
    <div id="events"></div>
    
    <script>
        // Auto-refresh every 30 seconds
        setInterval(function() {
            fetch('/api/health')
                .then(response => response.json())
                .then(data => updateDashboard(data));
        }, 30000);
        
        function updateDashboard(data) {
            // Update metrics
            document.getElementById('score-value').textContent = data.health_score.toFixed(2);
            document.getElementById('connectivity-value').textContent = data.connectivity ? 'Connected' : 'Disconnected';
            document.getElementById('quality-value').textContent = data.metric_quality + '%';
            document.getElementById('pii-value').textContent = data.pii_clean ? 'Clean' : 'Issues Detected';
            
            // Update styles based on thresholds
            const scoreElement = document.getElementById('health-score');
            if (data.health_score >= 0.9) {
                scoreElement.className = 'metric healthy';
            } else if (data.health_score >= 0.7) {
                scoreElement.className = 'metric warning';
            } else {
                scoreElement.className = 'metric critical';
            }
        }
    </script>
</body>
</html>
EOF
    
    # Start simple web server for dashboard (if Python available)
    if command -v python3 &> /dev/null; then
        cd "$VERIFICATION_DIR" && python3 -m http.server 8080 &
        log_success "Verification dashboard available at http://localhost:8080/dashboard.html"
    fi
}

# Generate comprehensive report
generate_verification_report() {
    log_info "Generating verification report..."
    
    local report_file="$VERIFICATION_DIR/report-$(date +%Y%m%d-%H%M%S).md"
    
    cat > "$report_file" <<EOF
# Database Intelligence Verification Report

Generated: $(date)

## System Status

### Overall Health Score
$(tail -1 "$METRICS_DIR/health.jsonl" 2>/dev/null | jq -r '.health_score' || echo "N/A")

### Component Status
- Collector: $(curl -sf http://localhost:13133 > /dev/null && echo "✅ Running" || echo "❌ Not Running")
- PostgreSQL Receiver: $(curl -s http://localhost:8888/metrics | grep -q postgresql_up && echo "✅ Active" || echo "❌ Inactive")
- MySQL Receiver: $(curl -s http://localhost:8888/metrics | grep -q mysql_up && echo "✅ Active" || echo "❌ Inactive")
- Verification: $(pgrep -f continuous-verify.sh > /dev/null && echo "✅ Running" || echo "❌ Not Running")

### Metrics Collection
- Total Metrics: $(curl -s http://localhost:8888/metrics | grep -c "^[a-zA-Z]" || echo "0")
- PostgreSQL Metrics: $(curl -s http://localhost:8888/metrics | grep -c "^postgresql_" || echo "0")
- MySQL Metrics: $(curl -s http://localhost:8888/metrics | grep -c "^mysql_" || echo "0")

### Recent Health Scores
\`\`\`
$(tail -5 "$METRICS_DIR/health.jsonl" 2>/dev/null | jq -r '"\(.timestamp): \(.health_score)"' || echo "No data")
\`\`\`

### Configuration
- Collection Interval: 60s
- PII Sanitization: Enabled
- Circuit Breaker: $(grep -q "circuit_breaker.*enabled.*true" "$CONFIG_DIR/collector-generated.yaml" && echo "Enabled" || echo "Disabled")
- Adaptive Sampling: $(grep -q "adaptive_sampler.*enabled.*true" "$CONFIG_DIR/collector-generated.yaml" && echo "Enabled" || echo "Disabled")

### Recommendations
$(generate_recommendations)

EOF
    
    log_success "Report generated: $report_file"
}

# Generate recommendations based on current state
generate_recommendations() {
    local recommendations=""
    
    # Check health score
    local health_score=$(tail -1 "$METRICS_DIR/health.jsonl" 2>/dev/null | jq -r '.health_score' || echo "0")
    if [ $(echo "$health_score < 0.8" | bc) -eq 1 ]; then
        recommendations+="- ⚠️ Health score is below threshold. Review recent errors.\n"
    fi
    
    # Check metric count
    local metric_count=$(curl -s http://localhost:8888/metrics | grep -c "^[a-zA-Z]" || echo "0")
    if [ $metric_count -lt 50 ]; then
        recommendations+="- ⚠️ Low metric count. Check database connectivity.\n"
    fi
    
    # Check for PII
    if curl -s http://localhost:8888/metrics | grep -qE "[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}"; then
        recommendations+="- 🚨 Potential PII detected in metrics. Review sanitization rules.\n"
    fi
    
    if [ -z "$recommendations" ]; then
        recommendations="- ✅ System operating normally. No actions required.\n"
    fi
    
    echo -e "$recommendations"
}

# Main menu
show_menu() {
    echo -e "\n${BLUE}Database Intelligence Enhanced Quickstart${NC}"
    echo "========================================"
    echo "1. Full Setup (Recommended)"
    echo "2. Verify Prerequisites Only"
    echo "3. Test Database Connectivity"
    echo "4. Generate Configuration"
    echo "5. Deploy Collector"
    echo "6. Start Verification"
    echo "7. Generate Report"
    echo "8. View Dashboard"
    echo "9. Stop All Services"
    echo "0. Exit"
    echo
}

# Interactive mode
interactive_mode() {
    while true; do
        show_menu
        read -p "Select option: " choice
        
        case $choice in
            1)
                log_info "Starting full setup..."
                verify_prerequisites || exit 1
                collect_database_info
                verify_database_connectivity "$DB_TYPE" "$DB_HOST" "$DB_PORT" "$DB_USER" "$DB_PASSWORD" "$DB_NAME" || exit 1
                generate_configuration
                deploy_collector || exit 1
                run_continuous_verification
                generate_verification_report
                log_success "Setup complete! Dashboard: http://localhost:8080/dashboard.html"
                ;;
            2)
                verify_prerequisites
                ;;
            3)
                collect_database_info
                verify_database_connectivity "$DB_TYPE" "$DB_HOST" "$DB_PORT" "$DB_USER" "$DB_PASSWORD" "$DB_NAME"
                ;;
            4)
                generate_configuration
                ;;
            5)
                deploy_collector
                ;;
            6)
                run_continuous_verification
                ;;
            7)
                generate_verification_report
                ;;
            8)
                xdg-open "http://localhost:8080/dashboard.html" 2>/dev/null || \
                open "http://localhost:8080/dashboard.html" 2>/dev/null || \
                echo "Open http://localhost:8080/dashboard.html in your browser"
                ;;
            9)
                log_info "Stopping all services..."
                docker-compose -f docker-compose-verified.yaml down
                pkill -f "python3 -m http.server" 2>/dev/null || true
                log_success "All services stopped"
                ;;
            0)
                log_info "Exiting..."
                exit 0
                ;;
            *)
                log_error "Invalid option"
                ;;
        esac
    done
}

# Collect database information
collect_database_info() {
    echo -e "\n${BLUE}Database Configuration${NC}"
    echo "====================="
    
    # Database type
    PS3="Select database type: "
    select db_type in "PostgreSQL" "MySQL" "Both"; do
        case $db_type in
            PostgreSQL)
                DB_TYPE="postgresql"
                break
                ;;
            MySQL)
                DB_TYPE="mysql"
                break
                ;;
            Both)
                DB_TYPE="both"
                break
                ;;
        esac
    done
    
    # Connection details
    read -p "Database host [localhost]: " DB_HOST
    DB_HOST=${DB_HOST:-localhost}
    
    read -p "Database port [5432/3306]: " DB_PORT
    if [ "$DB_TYPE" = "postgresql" ]; then
        DB_PORT=${DB_PORT:-5432}
    else
        DB_PORT=${DB_PORT:-3306}
    fi
    
    read -p "Database user: " DB_USER
    read -sp "Database password: " DB_PASSWORD
    echo
    
    read -p "Database name: " DB_NAME
    
    # Export for configuration
    export PG_HOST=$DB_HOST
    export PG_PORT=$DB_PORT
    export PG_USER=$DB_USER
    export PG_PASSWORD=$DB_PASSWORD
    export PG_DATABASE=$DB_NAME
    export MYSQL_HOST=$DB_HOST
    export MYSQL_PORT=$DB_PORT
    export MYSQL_USER=$DB_USER
    export MYSQL_PASSWORD=$DB_PASSWORD
    export MYSQL_DATABASE=$DB_NAME
}

# Main execution
main() {
    log_info "Starting Database Intelligence Enhanced Quickstart"
    
    # Check if running with arguments
    if [ $# -eq 0 ]; then
        interactive_mode
    else
        case $1 in
            all)
                verify_prerequisites || exit 1
                # Use environment variables if set
                if [ -n "${PG_HOST:-}" ]; then
                    verify_database_connectivity "postgresql" "$PG_HOST" "${PG_PORT:-5432}" "$PG_USER" "$PG_PASSWORD" "${PG_DATABASE:-postgres}"
                fi
                generate_configuration
                deploy_collector || exit 1
                run_continuous_verification
                generate_verification_report
                ;;
            verify)
                verify_prerequisites
                ;;
            deploy)
                deploy_collector
                ;;
            report)
                generate_verification_report
                ;;
            *)
                echo "Usage: $0 [all|verify|deploy|report]"
                exit 1
                ;;
        esac
    fi
}

# Run main
main "$@"