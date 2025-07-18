#!/bin/bash
# Unified metrics validation script for all databases

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Check which database to validate
DATABASE=${1:-all}
NR_ACCOUNT_ID=${NEW_RELIC_ACCOUNT_ID:-$NR_ACCOUNT_ID}
NR_API_KEY=${NEW_RELIC_API_KEY:-$NR_API_KEY}

if [ -z "$NR_ACCOUNT_ID" ] || [ -z "$NR_API_KEY" ]; then
    echo -e "${RED}Error: NEW_RELIC_ACCOUNT_ID and NEW_RELIC_API_KEY must be set${NC}"
    exit 1
fi

# Function to check metrics for a specific database
check_database_metrics() {
    local db_type=$1
    local metric_prefix=$2
    
    echo -e "${YELLOW}Checking $db_type metrics...${NC}"
    
    # Query New Relic for metrics
    local query="SELECT count(*) FROM Metric WHERE metricName LIKE '${metric_prefix}%' AND deployment.mode = 'config-only-maximum' SINCE 5 minutes ago"
    
    local response=$(curl -s -X POST "https://api.newrelic.com/graphql" \
        -H "Content-Type: application/json" \
        -H "API-Key: $NR_API_KEY" \
        -d "{
            \"query\": \"{ actor { account(id: $NR_ACCOUNT_ID) { nrql(query: \\\"$query\\\") { results } } } }\"
        }")
    
    # Parse response and check if metrics exist
    if echo "$response" | grep -q '"count":0'; then
        echo -e "${RED}❌ No $db_type metrics found${NC}"
        return 1
    else
        local count=$(echo "$response" | grep -oE '"count":[0-9]+' | grep -oE '[0-9]+' | head -1)
        echo -e "${GREEN}✓ Found $count $db_type metrics${NC}"
        return 0
    fi
}

# Main validation logic
case $DATABASE in
    postgresql)
        check_database_metrics "PostgreSQL" "postgresql"
        ;;
    mysql)
        check_database_metrics "MySQL" "mysql"
        ;;
    mongodb)
        check_database_metrics "MongoDB" "mongodb"
        ;;
    mssql)
        check_database_metrics "MSSQL" "mssql"
        check_database_metrics "SQL Server" "sqlserver"
        ;;
    oracle)
        check_database_metrics "Oracle" "oracle"
        ;;
    all)
        echo -e "${BLUE}Validating all database metrics...${NC}"
        check_database_metrics "PostgreSQL" "postgresql"
        check_database_metrics "MySQL" "mysql"
        check_database_metrics "MongoDB" "mongodb"
        check_database_metrics "MSSQL" "mssql"
        check_database_metrics "SQL Server" "sqlserver"
        check_database_metrics "Oracle" "oracle"
        ;;
    *)
        echo -e "${RED}Unknown database: $DATABASE${NC}"
        echo "Usage: $0 [postgresql|mysql|mongodb|mssql|oracle|all]"
        exit 1
        ;;
esac
