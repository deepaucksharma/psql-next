#!/bin/bash

# Script to fix federation endpoints across all modules
# Changes localhost endpoints to Docker service names

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}================================${NC}"
echo -e "${BLUE}Fixing Federation Endpoints${NC}"
echo -e "${BLUE}================================${NC}"

# Fix function for .env files
fix_env_file() {
    local file=$1
    local module=$(basename $(dirname "$file"))
    
    if [ -f "$file" ]; then
        echo -e "\n${YELLOW}Fixing $module/.env${NC}"
        
        # Replace localhost with service names
        sed -i.bak \
            -e 's/CORE_METRICS_ENDPOINT=localhost:8081/CORE_METRICS_ENDPOINT=core-metrics:8081/g' \
            -e 's/SQL_INTELLIGENCE_ENDPOINT=localhost:8082/SQL_INTELLIGENCE_ENDPOINT=sql-intelligence:8082/g' \
            -e 's/WAIT_PROFILER_ENDPOINT=localhost:8083/WAIT_PROFILER_ENDPOINT=wait-profiler:8083/g' \
            -e 's/ANOMALY_DETECTOR_ENDPOINT=localhost:8084/ANOMALY_DETECTOR_ENDPOINT=anomaly-detector:8084/g' \
            -e 's/BUSINESS_IMPACT_ENDPOINT=localhost:8085/BUSINESS_IMPACT_ENDPOINT=business-impact:8085/g' \
            -e 's/REPLICATION_MONITOR_ENDPOINT=localhost:8086/REPLICATION_MONITOR_ENDPOINT=replication-monitor:8086/g' \
            -e 's/PERFORMANCE_ADVISOR_ENDPOINT=localhost:8087/PERFORMANCE_ADVISOR_ENDPOINT=performance-advisor:8087/g' \
            -e 's/RESOURCE_MONITOR_ENDPOINT=localhost:8088/RESOURCE_MONITOR_ENDPOINT=resource-monitor:8088/g' \
            -e 's/ALERT_MANAGER_ENDPOINT=localhost:8089/ALERT_MANAGER_ENDPOINT=alert-manager:8089/g' \
            -e 's/CANARY_TESTER_ENDPOINT=localhost:8090/CANARY_TESTER_ENDPOINT=canary-tester:8090/g' \
            -e 's/CROSS_SIGNAL_CORRELATOR_ENDPOINT=localhost:8099/CROSS_SIGNAL_CORRELATOR_ENDPOINT=cross-signal-correlator:8099/g' \
            "$file"
        
        echo -e "  ${GREEN}✓${NC} Fixed federation endpoints"
    fi
}

# Fix all module .env files
for module_dir in "$PROJECT_ROOT/modules"/*; do
    if [ -d "$module_dir" ]; then
        fix_env_file "$module_dir/.env"
    fi
done

# Fix root .env file
fix_env_file "$PROJECT_ROOT/.env"

# Clean up backup files
echo -e "\n${BLUE}Cleaning up backup files...${NC}"
find "$PROJECT_ROOT" -name "*.env.bak" -delete

echo -e "\n${GREEN}✓ Federation endpoints fixed!${NC}"
echo -e "\nNext steps:"
echo -e "1. Restart affected containers:"
echo -e "   ${BLUE}cd modules/performance-advisor && docker-compose restart${NC}"
echo -e "2. Verify federation is working:"
echo -e "   ${BLUE}docker logs <container> 2>&1 | grep -v 'Failed to scrape'${NC}"
echo -e "3. Check metrics endpoints:"
echo -e "   ${BLUE}curl http://localhost:8087/metrics${NC}"