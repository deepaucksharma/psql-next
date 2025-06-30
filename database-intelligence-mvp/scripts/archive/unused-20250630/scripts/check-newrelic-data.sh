#!/bin/bash
# Quick script to check if data is arriving in New Relic

set -euo pipefail

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${YELLOW}=== New Relic Data Verification ===${NC}\n"

# Check environment
if [[ -z "${NEW_RELIC_LICENSE_KEY:-}" ]]; then
    echo -e "${RED}ERROR: NEW_RELIC_LICENSE_KEY not set${NC}"
    echo "Please set your New Relic license key in the .env file"
    exit 1
fi

# Set a default account ID if not provided
if [[ -z "${NEW_RELIC_ACCOUNT_ID:-}" ]]; then
    echo -e "${YELLOW}WARNING: NEW_RELIC_ACCOUNT_ID not set, some queries may fail${NC}"
    NEW_RELIC_ACCOUNT_ID="YOUR_ACCOUNT_ID"
fi

echo "Using New Relic API..."
echo ""

# Function to run NRQL query
run_nrql() {
    local query="$1"
    local description="$2"
    
    echo -e "${YELLOW}Query:${NC} $description"
    echo -e "${YELLOW}NRQL:${NC} $query"
    echo ""
    
    # For demonstration, show the curl command
    echo "To run this query:"
    echo "curl -X POST https://api.newrelic.com/graphql \\"
    echo "  -H 'Content-Type: application/json' \\"
    echo "  -H 'API-Key: \$NEW_RELIC_LICENSE_KEY' \\"
    echo "  -d '{\"query\": \"{ actor { account(id: \$NEW_RELIC_ACCOUNT_ID) { nrql(query: \\\"$query\\\") { results } } } }\"}'"
    echo ""
    echo "---"
    echo ""
}

# Key queries to verify integration
echo -e "${GREEN}=== Key Verification Queries ===${NC}\n"

# 1. Check for any OpenTelemetry data
run_nrql \
    "SELECT count(*) FROM Log WHERE instrumentation.provider = 'opentelemetry' SINCE 30 minutes ago" \
    "Check for any OpenTelemetry data"

# 2. Check for database intelligence collector data
run_nrql \
    "SELECT count(*) FROM Log WHERE collector.name = 'database-intelligence' SINCE 30 minutes ago" \
    "Check for database intelligence collector data"

# 3. Check for integration errors
run_nrql \
    "SELECT count(*) FROM NrIntegrationError WHERE newRelicFeature = 'Metrics' AND message LIKE '%database%' SINCE 1 hour ago" \
    "Check for integration errors"

# 4. Check for database entities
run_nrql \
    "SELECT uniques(entity.guid) FROM Log WHERE entity.type = 'DATABASE' SINCE 1 hour ago" \
    "Check for database entity creation"

# 5. Check collector metrics
run_nrql \
    "SELECT latest(otelcol_receiver_accepted_log_records) FROM Metric WHERE instrumentation.provider = 'opentelemetry' SINCE 10 minutes ago" \
    "Check collector metrics"

echo -e "${GREEN}=== Manual Verification Steps ===${NC}\n"
echo "1. Log into New Relic: https://one.newrelic.com"
echo "2. Go to Query Builder"
echo "3. Run the NRQL queries above"
echo "4. Check the Logs UI for any data from 'database-intelligence'"
echo "5. Check for NrIntegrationError events"
echo ""
echo -e "${YELLOW}Note:${NC} If no data appears:"
echo "  - Verify your license key is correct"
echo "  - Check collector logs: docker logs db-intel-primary"
echo "  - Ensure database DSNs are valid in .env"
echo "  - The collector might not have database access (using example DSNs)"