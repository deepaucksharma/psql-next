#!/bin/bash

# Comprehensive validation script for MySQL Wait-Based Monitoring
# This script validates all configurations and dependencies

set -e

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

echo -e "${BLUE}=== MySQL Wait-Based Monitoring - Comprehensive Validation ===${NC}"
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
        "warning")
            echo -e "${YELLOW}⚠${NC} $message"
            ;;
        "info")
            echo -e "${CYAN}ℹ${NC} $message"
            ;;
    esac
}

# Function to check file exists
check_file() {
    local file=$1
    local description=$2
    if [ -f "$file" ]; then
        print_status "success" "$description exists: $file"
        return 0
    else
        print_status "error" "$description missing: $file"
        return 1
    fi
}

# Function to validate YAML
validate_yaml() {
    local file=$1
    if command -v yq &> /dev/null; then
        if yq eval '.' "$file" > /dev/null 2>&1; then
            print_status "success" "Valid YAML: $file"
            return 0
        else
            print_status "error" "Invalid YAML: $file"
            yq eval '.' "$file" 2>&1 | head -5
            return 1
        fi
    else
        print_status "warning" "yq not installed, skipping YAML validation for $file"
        return 0
    fi
}

# Function to check environment variables
check_env_var() {
    local var_name=$1
    local required=$2
    local value="${!var_name}"
    
    if [ -n "$value" ] && [[ ! "$value" =~ ^your.*here$ ]]; then
        print_status "success" "$var_name is set"
        return 0
    elif [ "$required" = "required" ]; then
        print_status "error" "$var_name is not set or has default value"
        return 1
    else
        print_status "warning" "$var_name is not set (optional)"
        return 0
    fi
}

# Function to check Docker
check_docker() {
    echo -e "\n${CYAN}Checking Docker environment...${NC}"
    
    if command -v docker &> /dev/null; then
        print_status "success" "Docker is installed"
        docker version --format 'Client: {{.Client.Version}} Server: {{.Server.Version}}' 2>/dev/null || {
            print_status "error" "Docker daemon is not running or requires sign-in"
            print_status "info" "Solution: Open Docker Desktop and sign in with your organization account"
            return 1
        }
    else
        print_status "error" "Docker is not installed"
        return 1
    fi
    
    if command -v docker-compose &> /dev/null; then
        print_status "success" "Docker Compose is installed: $(docker-compose version --short)"
    else
        print_status "error" "Docker Compose is not installed"
        return 1
    fi
}

# Function to validate configurations
validate_configs() {
    echo -e "\n${CYAN}Validating configuration files...${NC}"
    
    local configs=(
        "config/edge-collector-wait.yaml"
        "config/gateway-advisory.yaml"
        "config/gateway-ha.yaml"
        "docker-compose.yml"
        "docker-compose-ha.yml"
    )
    
    local all_valid=true
    for config in "${configs[@]}"; do
        if check_file "$config" "Configuration"; then
            validate_yaml "$config" || all_valid=false
        else
            all_valid=false
        fi
    done
    
    return $([ "$all_valid" = true ] && echo 0 || echo 1)
}

# Function to check MySQL init scripts
check_mysql_scripts() {
    echo -e "\n${CYAN}Checking MySQL initialization scripts...${NC}"
    
    local scripts=(
        "mysql/init/01-create-monitoring-user.sql"
        "mysql/init/02-enable-performance-schema.sql"
        "mysql/init/03-create-sample-database.sql"
        "mysql/init/04-enable-wait-analysis.sql"
        "mysql/init/05-create-test-workload.sql"
    )
    
    local all_exist=true
    for script in "${scripts[@]}"; do
        check_file "$script" "MySQL script" || all_exist=false
    done
    
    # Check for SQL syntax issues
    if [ "$all_exist" = true ]; then
        print_status "info" "Checking SQL syntax..."
        for script in "${scripts[@]}"; do
            if grep -E '(CREATE USER|GRANT|CREATE DATABASE|CREATE TABLE)' "$script" > /dev/null; then
                print_status "success" "Found SQL statements in $script"
            else
                print_status "warning" "No SQL statements found in $script"
            fi
        done
    fi
    
    return $([ "$all_exist" = true ] && echo 0 || echo 1)
}

# Function to check environment setup
check_environment() {
    echo -e "\n${CYAN}Checking environment configuration...${NC}"
    
    if [ -f ".env" ]; then
        print_status "success" ".env file exists"
        source .env
        
        # Check required variables
        check_env_var "NEW_RELIC_LICENSE_KEY" "required"
        check_env_var "NEW_RELIC_ACCOUNT_ID" "optional"
        check_env_var "MYSQL_ROOT_PASSWORD" "required"
        check_env_var "MYSQL_DATABASE" "required"
        
        # Check optional variables
        check_env_var "ENVIRONMENT" "optional"
        check_env_var "HOSTNAME" "optional"
    else
        print_status "error" ".env file not found"
        print_status "info" "Run: cp .env.example .env"
        return 1
    fi
}

# Function to validate collector configurations
validate_collector_configs() {
    echo -e "\n${CYAN}Validating collector configurations...${NC}"
    
    # Check edge collector config
    if [ -f "config/edge-collector-wait.yaml" ]; then
        print_status "info" "Analyzing edge collector configuration..."
        
        # Check for required receivers
        if grep -q "mysql/waits:" "config/edge-collector-wait.yaml"; then
            print_status "success" "MySQL wait receiver configured"
        else
            print_status "error" "MySQL wait receiver not found"
        fi
        
        if grep -q "sqlquery/waits:" "config/edge-collector-wait.yaml"; then
            print_status "success" "SQL query receiver configured"
        else
            print_status "error" "SQL query receiver not found"
        fi
        
        # Check for processors
        if grep -q "memory_limiter:" "config/edge-collector-wait.yaml"; then
            print_status "success" "Memory limiter configured"
        else
            print_status "warning" "Memory limiter not configured"
        fi
    fi
    
    # Check gateway config
    if [ -f "config/gateway-advisory.yaml" ]; then
        print_status "info" "Analyzing gateway configuration..."
        
        # Check for advisory processing
        if grep -q "transform/composite_advisors:" "config/gateway-advisory.yaml"; then
            print_status "success" "Advisory processor configured"
        else
            print_status "error" "Advisory processor not found"
        fi
        
        # Check for New Relic exporter
        if grep -q "otlphttp/newrelic:" "config/gateway-advisory.yaml"; then
            print_status "success" "New Relic exporter configured"
        else
            print_status "error" "New Relic exporter not found"
        fi
    fi
}

# Function to check port availability
check_ports() {
    echo -e "\n${CYAN}Checking port availability...${NC}"
    
    local ports=(
        "3306:MySQL Primary"
        "3307:MySQL Replica"
        "4317:OTLP gRPC"
        "4318:OTLP HTTP"
        "8888:Collector Metrics"
        "9091:Prometheus Metrics"
        "9104:MySQL Exporter"
    )
    
    for port_desc in "${ports[@]}"; do
        IFS=':' read -r port description <<< "$port_desc"
        if lsof -Pi :$port -sTCP:LISTEN -t >/dev/null 2>&1; then
            print_status "warning" "Port $port ($description) is already in use"
        else
            print_status "success" "Port $port ($description) is available"
        fi
    done
}

# Function to generate test commands
generate_test_commands() {
    echo -e "\n${CYAN}Generating test commands...${NC}"
    
    cat << EOF > test-commands.sh
#!/bin/bash
# Test commands for MySQL Wait-Based Monitoring

echo "=== Starting services ==="
docker-compose up -d

echo "=== Waiting for services to be ready ==="
sleep 30

echo "=== Testing MySQL connection ==="
docker exec mysql-primary mysql -u root -p\${MYSQL_ROOT_PASSWORD} -e "SHOW DATABASES;"

echo "=== Checking Performance Schema ==="
docker exec mysql-primary mysql -u root -p\${MYSQL_ROOT_PASSWORD} -e "
SELECT * FROM performance_schema.setup_instruments 
WHERE NAME LIKE 'wait/%' AND ENABLED = 'NO' LIMIT 10;"

echo "=== Testing collector health ==="
curl -s http://localhost:13133/health | jq .

echo "=== Checking metrics ==="
curl -s http://localhost:8888/metrics | grep mysql_query

echo "=== Running load test ==="
docker exec mysql-primary mysql -u root -p\${MYSQL_ROOT_PASSWORD} production -e "
CALL generate_workload(100, 'mixed');"

echo "=== Monitoring waits ==="
./scripts/monitor-waits.sh waits

echo "=== Checking advisories ==="
curl -s http://localhost:9091/metrics | grep advisor_type
EOF
    
    chmod +x test-commands.sh
    print_status "success" "Generated test-commands.sh"
}

# Function to create troubleshooting guide
create_troubleshooting_guide() {
    echo -e "\n${CYAN}Creating troubleshooting guide...${NC}"
    
    cat << 'EOF' > TROUBLESHOOTING.md
# MySQL Wait-Based Monitoring - Troubleshooting Guide

## Common Issues and Solutions

### 1. Docker Sign-in Required

**Error**: "Sign-in enforcement is enabled. Open Docker Desktop..."

**Solution**:
1. Open Docker Desktop application
2. Sign in with your organization account
3. Verify sign-in: `docker pull hello-world`

### 2. New Relic License Key Missing

**Error**: Metrics not appearing in New Relic

**Solution**:
1. Get your New Relic Ingest License Key from: https://one.newrelic.com/api-keys
2. Update .env file: `NEW_RELIC_LICENSE_KEY=your_actual_key`
3. Restart gateway: `docker-compose restart otel-gateway`

### 3. MySQL Connection Failed

**Error**: "Access denied for user 'otel_monitor'"

**Possible causes**:
- Init scripts didn't run
- MySQL not fully started

**Solution**:
```bash
# Check MySQL logs
docker logs mysql-primary

# Manually create monitoring user
docker exec mysql-primary mysql -u root -p${MYSQL_ROOT_PASSWORD} -e "
CREATE USER IF NOT EXISTS 'otel_monitor'@'%' IDENTIFIED BY 'otelmonitorpass';
GRANT SELECT, PROCESS, REPLICATION CLIENT ON *.* TO 'otel_monitor'@'%';
FLUSH PRIVILEGES;"
```

### 4. No Metrics Appearing

**Diagnostic steps**:
```bash
# 1. Check collector status
docker logs otel-collector-edge | tail -20

# 2. Verify Performance Schema
docker exec mysql-primary mysql -u otel_monitor -potelmonitorpass -e "
SELECT COUNT(*) FROM performance_schema.events_statements_summary_by_digest;"

# 3. Check metrics endpoint
curl -s http://localhost:8888/metrics | grep receiver_accepted_metric_points

# 4. Verify gateway connection
docker logs otel-gateway | grep "connection refused"
```

### 5. High Memory Usage

**Symptoms**: Collector using >500MB memory

**Solutions**:
1. Adjust memory limits in configs
2. Increase collection intervals
3. Enable sampling for non-critical queries
4. Use light-load configuration

### 6. Missing Advisories

**Check advisory processing**:
```bash
# View gateway logs
docker logs otel-gateway | grep -i "advisor"

# Check metrics
curl -s http://localhost:9091/metrics | grep advisor_type | wc -l
```

## Quick Diagnostic Commands

```bash
# Full system check
./scripts/validate-everything.sh

# Monitor real-time waits
./scripts/monitor-waits.sh waits

# Check all services
docker-compose ps

# View all logs
docker-compose logs -f

# Restart everything
docker-compose down && docker-compose up -d
```

## Performance Tuning

For different environments, use appropriate configurations:
- Development: `config/tuning/light-load.yaml`
- Production: `config/tuning/heavy-load.yaml`
- Critical: `config/tuning/critical-system.yaml`

## Getting Help

1. Check operational runbooks: `docs/OPERATIONAL_RUNBOOKS.md`
2. Review architecture: `docs/WAIT_BASED_MONITORING_GUIDE.md`
3. Run validation: `./scripts/validate-everything.sh`
EOF
    
    print_status "success" "Created TROUBLESHOOTING.md"
}

# Main validation flow
main() {
    local errors=0
    
    # Check Docker environment
    check_docker || ((errors++))
    
    # Check environment configuration
    check_environment || ((errors++))
    
    # Validate configuration files
    validate_configs || ((errors++))
    
    # Check MySQL scripts
    check_mysql_scripts || ((errors++))
    
    # Validate collector configurations
    validate_collector_configs || ((errors++))
    
    # Check port availability
    check_ports || ((errors++))
    
    # Generate test commands
    generate_test_commands
    
    # Create troubleshooting guide
    create_troubleshooting_guide
    
    # Summary
    echo -e "\n${CYAN}=== Validation Summary ===${NC}"
    if [ $errors -eq 0 ]; then
        print_status "success" "All validations passed!"
        print_status "info" "Next steps:"
        echo "  1. Sign in to Docker Desktop"
        echo "  2. Set NEW_RELIC_LICENSE_KEY in .env"
        echo "  3. Run: docker-compose up -d"
        echo "  4. Monitor: ./scripts/monitor-waits.sh"
    else
        print_status "error" "Found $errors validation errors"
        print_status "info" "Review errors above and check TROUBLESHOOTING.md"
    fi
    
    return $errors
}

# Run main validation
main
exit $?