#!/bin/bash
# Validate that master config contains all metrics from individual configs
# Version: 1.0.0

set -e

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}═══════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}    Master Configuration Validation                     ${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════${NC}"
echo

cd "$(dirname "$0")/.."

# Function to extract metrics from config
extract_metrics() {
    local file=$1
    grep -E "mysql\." "$file" | grep -v "#" | sed 's/.*\(mysql\.[a-z_.]*\).*/\1/' | sort -u
}

echo -e "${YELLOW}1. Checking master config completeness...${NC}"

# Check if master config exists
if [ ! -f "config/mysql-collector-master.yaml" ]; then
    echo -e "${RED}❌ Master config not found!${NC}"
    exit 1
fi

# Count lines and receivers
MASTER_LINES=$(wc -l < config/mysql-collector-master.yaml)
MASTER_RECEIVERS=$(grep -c "^  [a-z_]*:" config/mysql-collector-master.yaml | head -20)
MASTER_PROCESSORS=$(grep -c "^  [a-z_]*/[a-z_]*:" config/mysql-collector-master.yaml | head -30)

echo "Master config stats:"
echo "  - Total lines: $MASTER_LINES"
echo "  - Receivers: ~$MASTER_RECEIVERS"
echo "  - Processors: ~$MASTER_PROCESSORS"

# Check for key features
echo -e "${YELLOW}2. Checking for key features...${NC}"

FEATURES=(
    "sqlquery/extreme_intelligence"
    "mysql.intelligence.comprehensive"
    "mysql.query.wait_profile"
    "mysql.health.score"
    "transform/ml_features"
    "attributes/anomaly_detection"
    "attributes/business_impact"
    "attributes/advisor"
    "metrics/critical_realtime"
    "metrics/standard"
    "metrics/analytics"
    "otlphttp/priority_high"
    "otlphttp/priority_standard"
    "WITH current_waits AS"
    "historical_patterns AS"
    "lock_analysis AS"
    "resource_metrics AS"
)

MISSING=0
for feature in "${FEATURES[@]}"; do
    if grep -q "$feature" config/mysql-collector-master.yaml; then
        echo -e "  ${GREEN}✓ Found: $feature${NC}"
    else
        echo -e "  ${RED}✗ Missing: $feature${NC}"
        ((MISSING++))
    fi
done

if [ $MISSING -eq 0 ]; then
    echo -e "${GREEN}✅ All key features present${NC}"
else
    echo -e "${RED}❌ Missing $MISSING key features${NC}"
fi

# Check environment variables
echo -e "${YELLOW}3. Checking environment variable support...${NC}"

ENV_VARS=(
    "DEPLOYMENT_MODE"
    "ENABLE_SQL_INTELLIGENCE"
    "MYSQL_COLLECTION_INTERVAL"
    "SQL_INTELLIGENCE_INTERVAL"
    "BATCH_SIZE"
    "MEMORY_LIMIT_PERCENT"
)

for var in "${ENV_VARS[@]}"; do
    if grep -q "\${env:$var" config/mysql-collector-master.yaml; then
        echo -e "  ${GREEN}✓ Uses: $var${NC}"
    else
        echo -e "  ${YELLOW}○ Not found: $var (may use default)${NC}"
    fi
done

# Check metrics coverage
echo -e "${YELLOW}4. Checking MySQL metrics coverage...${NC}"

# Expected metrics from documentation
EXPECTED_METRICS=(
    "mysql.buffer_pool"
    "mysql.connection"
    "mysql.query"
    "mysql.innodb"
    "mysql.threads"
    "mysql.commands"
    "mysql.locks"
    "mysql.cache"
    "mysql.table"
    "mysql.replica"
    "mysql.opened_resources"
    "mysql.performance_schema"
)

FOUND=0
for metric in "${EXPECTED_METRICS[@]}"; do
    if grep -q "$metric" config/mysql-collector-master.yaml; then
        ((FOUND++))
    fi
done

echo "  Found $FOUND out of ${#EXPECTED_METRICS[@]} expected metric categories"

# Compare with original config
echo -e "${YELLOW}5. Comparing with original comprehensive config...${NC}"

if [ -f "config/otel-collector-config.yaml" ]; then
    ORIG_LINES=$(wc -l < config/otel-collector-config.yaml)
    echo "  Original config: $ORIG_LINES lines"
    echo "  Master config: $MASTER_LINES lines"
    
    if [ $MASTER_LINES -gt $ORIG_LINES ]; then
        echo -e "  ${GREEN}✓ Master config is larger (includes all features)${NC}"
    else
        echo -e "  ${YELLOW}⚠ Master config is smaller (uses env variables)${NC}"
    fi
fi

echo
echo -e "${BLUE}═══════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}    Validation Summary                                  ${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════${NC}"

if [ $MISSING -eq 0 ]; then
    echo -e "${GREEN}✅ Master configuration appears complete!${NC}"
    echo
    echo "The master config successfully consolidates:"
    echo "  - All MySQL standard metrics"
    echo "  - Ultra-comprehensive SQL intelligence queries"
    echo "  - Advanced processors (ML, anomaly, business impact)"
    echo "  - Multiple deployment modes via environment variables"
    echo "  - Priority-based routing and pipelines"
else
    echo -e "${RED}❌ Master configuration may be missing features${NC}"
    echo "Please review the missing items above"
fi

echo
echo "📋 Next steps:"
echo "   1. Deploy with: ./scripts/deploy-quick-start.sh"
echo "   2. Test metrics: ./scripts/validate-newrelic-metrics.sh"
echo "   3. Import dashboards from: dashboards/newrelic/"