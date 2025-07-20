#!/bin/bash

# Database Intelligence Health Check Script
# Consolidated health checks from all module Makefiles
# This script checks the health of all database intelligence modules

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    local status=$1
    local message=$2
    if [ "$status" = "OK" ]; then
        echo -e "${GREEN}✓${NC} $message"
    elif [ "$status" = "WARN" ]; then
        echo -e "${YELLOW}⚠${NC} $message"
    else
        echo -e "${RED}✗${NC} $message"
    fi
}

# Function to check HTTP endpoint
check_endpoint() {
    local url=$1
    local name=$2
    local timeout=${3:-5}
    
    if curl -s -f --max-time $timeout "$url" > /dev/null 2>&1; then
        print_status "OK" "$name endpoint is healthy"
        return 0
    else
        print_status "FAIL" "$name endpoint is not responding"
        return 1
    fi
}

# Function to check metrics endpoint
check_metrics() {
    local url=$1
    local name=$2
    local pattern=${3:-""}
    local timeout=${4:-5}
    
    local response=$(curl -s --max-time $timeout "$url" 2>/dev/null)
    if [ $? -eq 0 ] && [ -n "$response" ]; then
        if [ -n "$pattern" ]; then
            if echo "$response" | grep -q "$pattern"; then
                print_status "OK" "$name metrics available with expected data"
            else
                print_status "WARN" "$name metrics available but no expected data pattern found"
            fi
        else
            print_status "OK" "$name metrics endpoint is healthy"
        fi
        return 0
    else
        print_status "FAIL" "$name metrics endpoint is not responding"
        return 1
    fi
}

echo "========================================"
echo "Database Intelligence Health Check"
echo "========================================"
echo ""

# Track overall health
overall_health=0

# Core Metrics Module (Port 8081)
echo "=== Core Metrics Module ==="
if check_metrics "http://localhost:8081/metrics" "Core Metrics"; then
    # Check for specific core metrics patterns
    check_metrics "http://localhost:8081/metrics" "Core Metrics" "mysql_"
else
    overall_health=1
fi
echo ""

# SQL Intelligence Module (Port 8082)
echo "=== SQL Intelligence Module ==="
if check_metrics "http://localhost:8082/metrics" "SQL Intelligence"; then
    # Check for SQL-specific metrics
    check_metrics "http://localhost:8082/metrics" "SQL Intelligence" "mysql_query\|mysql_table"
else
    overall_health=1
fi
echo ""

# Wait Profiler Module (Port 8083)
echo "=== Wait Profiler Module ==="
if check_metrics "http://localhost:8083/metrics" "Wait Profiler"; then
    # Check for wait-specific metrics
    check_metrics "http://localhost:8083/metrics" "Wait Profiler" "mysql_wait\|mysql_mutex\|mysql_io\|mysql_lock"
else
    overall_health=1
fi
echo ""

# Anomaly Detector Module (Port 8084)
echo "=== Anomaly Detector Module ==="
if check_metrics "http://localhost:8084/metrics" "Anomaly Detector"; then
    # Check for anomaly-specific metrics
    check_metrics "http://localhost:8084/metrics" "Anomaly Detector" "anomaly_score\|anomaly_alert"
    
    # Check dependency services
    echo "Checking dependencies..."
    check_endpoint "http://localhost:8081/metrics" "Core Metrics (dependency)" 3 || print_status "WARN" "Core Metrics dependency not accessible"
    check_endpoint "http://localhost:8082/metrics" "SQL Intelligence (dependency)" 3 || print_status "WARN" "SQL Intelligence dependency not accessible"
    check_endpoint "http://localhost:8083/metrics" "Wait Profiler (dependency)" 3 || print_status "WARN" "Wait Profiler dependency not accessible"
else
    overall_health=1
fi
echo ""

# Business Impact Module (Port 8085)
echo "=== Business Impact Module ==="
if check_endpoint "http://localhost:13133/health" "Business Impact Collector Health"; then
    check_metrics "http://localhost:8888/metrics" "Business Impact" "business_impact\|business_category"
else
    overall_health=1
fi
echo ""

# Replication Monitor Module (Port 8086)
echo "=== Replication Monitor Module ==="
if check_endpoint "http://localhost:13133/health" "Replication Monitor Collector Health"; then
    check_metrics "http://localhost:8889/metrics" "Replication Monitor" "mysql_replication_"
    
    # Check container status if docker-compose is available
    if command -v docker-compose >/dev/null 2>&1; then
        echo "Checking container status..."
        if docker-compose -f modules/replication-monitor/docker-compose.yaml ps 2>/dev/null | grep -q "Up"; then
            print_status "OK" "Replication Monitor containers are running"
        else
            print_status "WARN" "Cannot verify container status or containers not running"
        fi
    fi
else
    overall_health=1
fi
echo ""

# Performance Advisor Module (Port 8087)
echo "=== Performance Advisor Module ==="
if check_endpoint "http://localhost:13133/" "Performance Advisor Collector Health"; then
    if check_metrics "http://localhost:8888/metrics" "Performance Advisor Prometheus"; then
        check_metrics "http://localhost:8888/metrics" "Performance Advisor Recommendations" "db_performance_recommendation"
    fi
else
    overall_health=1
fi
echo ""

# Resource Monitor Module (Port 8088-8091)
echo "=== Resource Monitor Module ==="
if check_endpoint "http://localhost:8091/" "Resource Monitor Health"; then
    check_metrics "http://localhost:8090/metrics" "Resource Monitor"
    
    # Check additional Resource Monitor endpoints
    check_endpoint "http://localhost:8088" "Resource Monitor OTLP gRPC" 3 || print_status "WARN" "OTLP gRPC endpoint not accessible"
    check_endpoint "http://localhost:8089" "Resource Monitor OTLP HTTP" 3 || print_status "WARN" "OTLP HTTP endpoint not accessible"
else
    overall_health=1
fi
echo ""

# Alert Manager Module (Port 9091, Health 13134)
echo "=== Alert Manager Module ==="
if check_endpoint "http://localhost:13134/health" "Alert Manager Health"; then
    check_metrics "http://localhost:9091/metrics" "Alert Manager" "alert_manager_"
    
    # Check OTLP endpoints
    echo "Checking OTLP endpoints..."
    if curl -s -f -X POST "http://localhost:4318/v1/metrics" -H "Content-Type: application/json" -d '{}' >/dev/null 2>&1; then
        print_status "OK" "OTLP HTTP endpoint responding (expected 400)"
    else
        print_status "WARN" "OTLP HTTP endpoint check inconclusive"
    fi
else
    overall_health=1
fi
echo ""

# Canary Tester Module (Port 8090)
echo "=== Canary Tester Module ==="
if check_endpoint "http://localhost:13133/health" "Canary Tester Collector Health"; then
    check_metrics "http://localhost:8090/metrics" "Canary Tester" "canary_"
    check_metrics "http://localhost:8888/metrics" "Canary Tester Prometheus"
    
    # Check if docker-compose services are available
    if command -v docker-compose >/dev/null 2>&1; then
        echo "Checking MySQL status for canary tests..."
        if docker-compose -f modules/canary-tester/docker-compose.yaml exec mysql mysqladmin -uroot -prootpassword status >/dev/null 2>&1; then
            print_status "OK" "Canary MySQL is ready"
        else
            print_status "WARN" "Canary MySQL not ready or not accessible"
        fi
    fi
else
    overall_health=1
fi
echo ""

# Cross-Signal Correlator Module (Port 8892, Health 13137)
echo "=== Cross-Signal Correlator Module ==="
if check_endpoint "http://localhost:13137/health" "Cross-Signal Correlator Health"; then
    check_metrics "http://localhost:8892/metrics" "Cross-Signal Correlator"
else
    overall_health=1
fi
echo ""

# Summary
echo "========================================"
echo "Health Check Summary"
echo "========================================"

if [ $overall_health -eq 0 ]; then
    print_status "OK" "All modules are healthy"
    echo ""
    echo "All database intelligence modules are running correctly."
    exit 0
else
    print_status "FAIL" "Some modules have health issues"
    echo ""
    echo "Some database intelligence modules are not healthy."
    echo "Please check the individual module logs and configurations."
    exit 1
fi