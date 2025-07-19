#!/bin/bash
# Validate metrics are flowing to New Relic
# Version: 1.0.0

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}═══════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}         New Relic Metrics Validation                   ${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════${NC}"
echo

# Check prerequisites
if [ -z "$NEW_RELIC_LICENSE_KEY" ] || [ -z "$NEW_RELIC_ACCOUNT_ID" ]; then
    echo -e "${RED}ERROR: Required environment variables not set${NC}"
    echo "Please set: NEW_RELIC_LICENSE_KEY and NEW_RELIC_ACCOUNT_ID"
    exit 1
fi

# Function to execute NRQL query
execute_nrql() {
    local query="$1"
    local response=$(curl -s -X POST https://api.newrelic.com/graphql \
        -H "Content-Type: application/json" \
        -H "API-Key: $NEW_RELIC_LICENSE_KEY" \
        -d "{
            \"query\": \"query { actor { account(id: $NEW_RELIC_ACCOUNT_ID) { nrql(query: \\\"$query\\\") { results } } } }\"
        }")
    echo "$response"
}

# Test 1: Basic connectivity
echo -e "${YELLOW}1. Testing basic OTEL metrics...${NC}"
BASIC_QUERY="SELECT count(*) FROM Metric WHERE instrumentation.provider = 'opentelemetry' SINCE 5 minutes ago"
RESULT=$(execute_nrql "$BASIC_QUERY")

if echo "$RESULT" | grep -q "results"; then
    COUNT=$(echo "$RESULT" | grep -o '"count":[0-9]*' | grep -o '[0-9]*' | head -1)
    if [ -n "$COUNT" ] && [ "$COUNT" -gt 0 ]; then
        echo -e "${GREEN}✅ Found $COUNT OpenTelemetry metrics${NC}"
    else
        echo -e "${RED}❌ No OpenTelemetry metrics found${NC}"
    fi
else
    echo -e "${RED}❌ Failed to query New Relic${NC}"
    echo "Response: $RESULT"
fi

# Test 2: MySQL standard metrics
echo -e "${YELLOW}2. Testing MySQL standard metrics...${NC}"
MYSQL_QUERY="SELECT uniqueCount(metricName) FROM Metric WHERE metricName LIKE 'mysql.%' AND instrumentation.provider = 'opentelemetry' SINCE 5 minutes ago"
RESULT=$(execute_nrql "$MYSQL_QUERY")

if echo "$RESULT" | grep -q "results"; then
    COUNT=$(echo "$RESULT" | grep -o '"uniqueCount":[0-9]*' | grep -o '[0-9]*' | head -1)
    if [ -n "$COUNT" ] && [ "$COUNT" -gt 0 ]; then
        echo -e "${GREEN}✅ Found $COUNT unique MySQL metric types${NC}"
    else
        echo -e "${RED}❌ No MySQL metrics found${NC}"
    fi
fi

# Test 3: Intelligence metrics
echo -e "${YELLOW}3. Testing intelligence metrics...${NC}"
INTEL_QUERY="SELECT count(*) FROM Metric WHERE metricName = 'mysql.intelligence.comprehensive' SINCE 5 minutes ago"
RESULT=$(execute_nrql "$INTEL_QUERY")

if echo "$RESULT" | grep -q "results"; then
    COUNT=$(echo "$RESULT" | grep -o '"count":[0-9]*' | grep -o '[0-9]*' | head -1)
    if [ -n "$COUNT" ] && [ "$COUNT" -gt 0 ]; then
        echo -e "${GREEN}✅ Found $COUNT intelligence metrics${NC}"
    else
        echo -e "${YELLOW}⚠️  No intelligence metrics yet (may take time to generate)${NC}"
    fi
fi

# Test 4: Wait profile metrics
echo -e "${YELLOW}4. Testing wait profile metrics...${NC}"
WAIT_QUERY="SELECT count(*) FROM Metric WHERE metricName = 'mysql.query.wait_profile' SINCE 5 minutes ago"
RESULT=$(execute_nrql "$WAIT_QUERY")

if echo "$RESULT" | grep -q "results"; then
    COUNT=$(echo "$RESULT" | grep -o '"count":[0-9]*' | grep -o '[0-9]*' | head -1)
    if [ -n "$COUNT" ] && [ "$COUNT" -gt 0 ]; then
        echo -e "${GREEN}✅ Found $COUNT wait profile metrics${NC}"
    else
        echo -e "${YELLOW}⚠️  No wait profile metrics yet${NC}"
    fi
fi

# Test 5: Check attributes
echo -e "${YELLOW}5. Checking metric attributes...${NC}"
ATTR_QUERY="SELECT keyset(attributes) FROM Metric WHERE instrumentation.provider = 'opentelemetry' LIMIT 1 SINCE 5 minutes ago"
RESULT=$(execute_nrql "$ATTR_QUERY")

if echo "$RESULT" | grep -q "results"; then
    echo -e "${GREEN}✅ Metrics have attributes${NC}"
    
    # Check for key attributes
    KEY_ATTRS=("advisor.type" "anomaly.detected" "business.revenue_impact" "ml.is_anomaly")
    for attr in "${KEY_ATTRS[@]}"; do
        CHECK_QUERY="SELECT count(*) FROM Metric WHERE attributes['$attr'] IS NOT NULL SINCE 5 minutes ago"
        RESULT=$(execute_nrql "$CHECK_QUERY")
        COUNT=$(echo "$RESULT" | grep -o '"count":[0-9]*' | grep -o '[0-9]*' | head -1)
        if [ -n "$COUNT" ] && [ "$COUNT" -gt 0 ]; then
            echo -e "   ${GREEN}✓ Found attribute: $attr${NC}"
        else
            echo -e "   ${YELLOW}○ Missing attribute: $attr${NC}"
        fi
    done
fi

# Test 6: Sample intelligence data
echo -e "${YELLOW}6. Sample intelligence data...${NC}"
SAMPLE_QUERY="SELECT max(value) as 'Max Score', average(value) as 'Avg Score', latest(attributes['recommendations']) as 'Latest Recommendation' FROM Metric WHERE metricName = 'mysql.intelligence.comprehensive' SINCE 10 minutes ago"
RESULT=$(execute_nrql "$SAMPLE_QUERY")

if echo "$RESULT" | grep -q "results"; then
    echo -e "${GREEN}✅ Intelligence data available${NC}"
    echo "$RESULT" | python3 -m json.tool 2>/dev/null | grep -E "(Max Score|Avg Score|Latest Recommendation)" || true
fi

echo
echo -e "${BLUE}═══════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}         Validation Complete                            ${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════${NC}"
echo
echo "📊 Try these queries in New Relic:"
echo "   FROM Metric SELECT * WHERE instrumentation.provider = 'opentelemetry' SINCE 5 minutes ago LIMIT 10"
echo "   FROM Metric SELECT * WHERE metricName = 'mysql.intelligence.comprehensive' SINCE 10 minutes ago"
echo "   FROM Metric SELECT keyset(attributes) WHERE metricName LIKE 'mysql.%' LIMIT 1"
echo
echo "🔍 Dashboard links:"
echo "   https://one.newrelic.com/dashboards"
echo "   Import the JSON files from dashboards/newrelic/"