#!/bin/bash

# Production deployment script for MySQL Wait-Based Monitoring
# This script handles phased rollout with validation at each step

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# Deployment configuration
DEPLOYMENT_PHASE="${1:-validate}"
ROLLOUT_PERCENTAGE="${2:-25}"
ENVIRONMENT="${ENVIRONMENT:-production}"

echo -e "${BLUE}=== MySQL Wait-Based Monitoring Production Deployment ===${NC}"
echo -e "${CYAN}Phase: $DEPLOYMENT_PHASE | Environment: $ENVIRONMENT${NC}"
echo ""

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

# Function to validate prerequisites
validate_prerequisites() {
    print_status "step" "Validating prerequisites..."
    
    # Check required environment variables
    local required_vars=("NEW_RELIC_LICENSE_KEY" "MYSQL_MONITOR_USER" "MYSQL_MONITOR_PASS")
    for var in "${required_vars[@]}"; do
        if [ -z "${!var}" ]; then
            print_status "error" "$var is not set"
            return 1
        fi
    done
    print_status "success" "All required environment variables are set"
    
    # Check for required binaries
    local required_bins=("otelcol-contrib" "mysql" "curl" "jq")
    for bin in "${required_bins[@]}"; do
        if ! command -v $bin &> /dev/null; then
            print_status "error" "$bin is not installed"
            return 1
        fi
    done
    print_status "success" "All required binaries are available"
    
    # Test MySQL connectivity
    if mysql -h localhost -u "$MYSQL_MONITOR_USER" -p"$MYSQL_MONITOR_PASS" -e "SELECT 1" &> /dev/null; then
        print_status "success" "MySQL connectivity verified"
    else
        print_status "error" "Cannot connect to MySQL"
        return 1
    fi
    
    # Check Performance Schema
    local ps_enabled=$(mysql -h localhost -u "$MYSQL_MONITOR_USER" -p"$MYSQL_MONITOR_PASS" -sN -e "SELECT @@performance_schema")
    if [ "$ps_enabled" = "1" ]; then
        print_status "success" "Performance Schema is enabled"
    else
        print_status "error" "Performance Schema is not enabled"
        return 1
    fi
    
    return 0
}

# Function to deploy edge collector
deploy_edge_collector() {
    local config_file="/etc/otel/mysql-edge-collector.yaml"
    local service_name="otel-collector-edge"
    
    print_status "step" "Deploying edge collector..."
    
    # Create configuration directory
    sudo mkdir -p /etc/otel/certs
    sudo mkdir -p /var/log/otel
    
    # Deploy configuration
    envsubst < "$PROJECT_ROOT/config/edge-collector-wait.yaml" | sudo tee "$config_file" > /dev/null
    print_status "success" "Configuration deployed to $config_file"
    
    # Create systemd service
    cat << EOF | sudo tee /etc/systemd/system/${service_name}.service > /dev/null
[Unit]
Description=OpenTelemetry Collector Edge for MySQL
After=network.target

[Service]
Type=simple
User=otel
Group=otel
ExecStart=/usr/local/bin/otelcol-contrib --config=${config_file}
Restart=on-failure
RestartSec=5
EnvironmentFile=/etc/otel/otel.env
LimitNOFILE=65536
MemoryLimit=400M
CPUQuota=50%

[Install]
WantedBy=multi-user.target
EOF

    # Create otel user if not exists
    if ! id -u otel &> /dev/null; then
        sudo useradd -r -s /bin/false otel
        print_status "success" "Created otel user"
    fi
    
    # Create environment file
    cat << EOF | sudo tee /etc/otel/otel.env > /dev/null
MYSQL_MONITOR_USER=${MYSQL_MONITOR_USER}
MYSQL_MONITOR_PASS=${MYSQL_MONITOR_PASS}
MYSQL_PRIMARY_HOST=localhost
GATEWAY_ENDPOINT=${GATEWAY_ENDPOINT:-gateway.monitoring.internal:4317}
HOSTNAME=$(hostname)
ENVIRONMENT=${ENVIRONMENT}
EOF

    # Set permissions
    sudo chown -R otel:otel /etc/otel
    sudo chown -R otel:otel /var/log/otel
    
    # Enable and start service
    sudo systemctl daemon-reload
    sudo systemctl enable ${service_name}
    sudo systemctl start ${service_name}
    
    # Wait for service to be ready
    local retries=30
    while [ $retries -gt 0 ]; do
        if curl -s http://localhost:8888/metrics > /dev/null; then
            print_status "success" "Edge collector is running"
            return 0
        fi
        retries=$((retries - 1))
        sleep 2
    done
    
    print_status "error" "Edge collector failed to start"
    sudo journalctl -u ${service_name} -n 50
    return 1
}

# Function to validate edge collector metrics
validate_edge_metrics() {
    print_status "step" "Validating edge collector metrics..."
    
    # Check collector health
    if ! curl -s http://localhost:13133/ > /dev/null; then
        print_status "error" "Health check endpoint not responding"
        return 1
    fi
    print_status "success" "Health check passed"
    
    # Check metrics endpoint
    local metrics=$(curl -s http://localhost:8888/metrics)
    
    # Verify key metrics are present
    local required_metrics=(
        "otelcol_receiver_accepted_metric_points"
        "otelcol_exporter_sent_metric_points"
        "otelcol_processor_batch_batch_size_trigger_send"
    )
    
    for metric in "${required_metrics[@]}"; do
        if echo "$metrics" | grep -q "$metric"; then
            print_status "success" "Found metric: $metric"
        else
            print_status "error" "Missing metric: $metric"
            return 1
        fi
    done
    
    # Check for MySQL metrics
    if curl -s http://localhost:9091/metrics | grep -q "mysql_query_wait_profile"; then
        print_status "success" "MySQL wait metrics are being collected"
    else
        print_status "warning" "MySQL wait metrics not yet visible"
    fi
    
    return 0
}

# Function to perform phased rollout
phased_rollout() {
    local phase=$1
    local percentage=$2
    
    print_status "step" "Performing phased rollout: ${percentage}% in ${phase}"
    
    case $phase in
        "canary")
            # Deploy to single host first
            deploy_edge_collector
            sleep 30
            validate_edge_metrics
            ;;
        
        "pilot")
            # Deploy to pilot group (25%)
            print_status "info" "Deploying to pilot group (${percentage}% of hosts)"
            # In production, this would use your deployment tool
            # ansible-playbook -i inventory deploy-collector.yml --limit "mysql_hosts[0:${percentage}%]"
            ;;
        
        "rollout")
            # Progressive rollout
            print_status "info" "Rolling out to ${percentage}% of hosts"
            # ansible-playbook -i inventory deploy-collector.yml --limit "mysql_hosts[0:${percentage}%]"
            ;;
        
        "complete")
            # Full deployment
            print_status "info" "Completing deployment to all hosts"
            # ansible-playbook -i inventory deploy-collector.yml
            ;;
    esac
}

# Function to validate deployment health
validate_deployment_health() {
    print_status "step" "Validating deployment health..."
    
    # Check New Relic for data
    if [ -n "$NEW_RELIC_API_KEY" ]; then
        local nrql_query="SELECT count(*) FROM Metric WHERE collector.name = 'mysql-wait-gateway' SINCE 5 minutes ago"
        local result=$(curl -s -X POST "https://api.newrelic.com/graphql" \
            -H "Content-Type: application/json" \
            -H "API-Key: $NEW_RELIC_API_KEY" \
            -d "{\"query\": \"{ nrql(accounts: [$NEW_RELIC_ACCOUNT_ID], query: \\\"$nrql_query\\\") { results } }\"}" | \
            jq -r '.data.nrql.results[0].count')
        
        if [ "$result" -gt "0" ]; then
            print_status "success" "Metrics are flowing to New Relic"
        else
            print_status "warning" "No metrics in New Relic yet"
        fi
    fi
    
    # Check for critical advisories
    local advisories=$(curl -s http://localhost:9091/metrics | grep 'advisor_priority="P0"' | wc -l)
    if [ "$advisories" -gt "0" ]; then
        print_status "warning" "Found $advisories P0 advisories - review before continuing"
    else
        print_status "success" "No critical advisories detected"
    fi
    
    return 0
}

# Function to generate deployment report
generate_report() {
    local report_file="/tmp/mysql-monitoring-deployment-$(date +%Y%m%d-%H%M%S).txt"
    
    print_status "step" "Generating deployment report..."
    
    cat << EOF > "$report_file"
MySQL Wait-Based Monitoring Deployment Report
Generated: $(date)
Environment: $ENVIRONMENT
Hostname: $(hostname)

Deployment Status:
EOF

    # Add collector status
    if systemctl is-active otel-collector-edge &> /dev/null; then
        echo "- Edge Collector: RUNNING" >> "$report_file"
    else
        echo "- Edge Collector: STOPPED" >> "$report_file"
    fi
    
    # Add metrics summary
    echo -e "\nMetrics Summary:" >> "$report_file"
    curl -s http://localhost:8888/metrics | grep -E "^otelcol_" | grep -v "#" | head -10 >> "$report_file"
    
    # Add configuration
    echo -e "\nConfiguration:" >> "$report_file"
    echo "- Collection Interval: 10s" >> "$report_file"
    echo "- Memory Limit: 384MB" >> "$report_file"
    echo "- Gateway Endpoint: ${GATEWAY_ENDPOINT:-gateway.monitoring.internal:4317}" >> "$report_file"
    
    print_status "success" "Report generated: $report_file"
    cat "$report_file"
}

# Main execution
main() {
    case $DEPLOYMENT_PHASE in
        "validate")
            validate_prerequisites
            ;;
        
        "deploy-edge")
            validate_prerequisites || exit 1
            deploy_edge_collector || exit 1
            validate_edge_metrics || exit 1
            generate_report
            ;;
        
        "canary")
            validate_prerequisites || exit 1
            phased_rollout "canary" 1 || exit 1
            validate_deployment_health
            ;;
        
        "pilot")
            phased_rollout "pilot" "$ROLLOUT_PERCENTAGE" || exit 1
            validate_deployment_health
            ;;
        
        "rollout")
            phased_rollout "rollout" "$ROLLOUT_PERCENTAGE" || exit 1
            validate_deployment_health
            ;;
        
        "complete")
            phased_rollout "complete" 100 || exit 1
            validate_deployment_health
            generate_report
            ;;
        
        "rollback")
            print_status "warning" "Rolling back deployment..."
            sudo systemctl stop otel-collector-edge
            sudo systemctl disable otel-collector-edge
            print_status "success" "Rollback completed"
            ;;
        
        *)
            echo "Usage: $0 [validate|deploy-edge|canary|pilot|rollout|complete|rollback] [percentage]"
            echo ""
            echo "Phases:"
            echo "  validate    - Validate prerequisites only"
            echo "  deploy-edge - Deploy edge collector on current host"
            echo "  canary      - Deploy to single canary host"
            echo "  pilot       - Deploy to pilot group (default 25%)"
            echo "  rollout     - Progressive rollout (specify percentage)"
            echo "  complete    - Complete deployment to all hosts"
            echo "  rollback    - Rollback deployment"
            echo ""
            echo "Examples:"
            echo "  $0 validate"
            echo "  $0 deploy-edge"
            echo "  $0 pilot 25"
            echo "  $0 rollout 50"
            echo "  $0 complete"
            exit 1
            ;;
    esac
}

# Run main function
main