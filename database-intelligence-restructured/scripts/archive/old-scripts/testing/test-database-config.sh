#!/bin/bash
# Test database configuration with OpenTelemetry collector

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

DATABASE=${1:-}
DURATION=${2:-60}

if [ -z "$DATABASE" ]; then
    echo "Usage: $0 <database> [duration-seconds]"
    echo "Example: $0 postgresql 60"
    echo "Databases: postgresql, mysql, mongodb, mssql, oracle"
    exit 1
fi

CONFIG_FILE="configs/${DATABASE}-maximum-extraction.yaml"
ENV_FILE="configs/env-templates/${DATABASE}.env"

if [ ! -f "$CONFIG_FILE" ]; then
    echo -e "${RED}Error: Config file not found: $CONFIG_FILE${NC}"
    exit 1
fi

echo -e "${BLUE}=== Testing $DATABASE Configuration ===${NC}"

# Load environment variables if env file exists
if [ -f "$ENV_FILE" ]; then
    echo -e "${YELLOW}Loading environment from $ENV_FILE${NC}"
    export $(grep -v '^#' "$ENV_FILE" | xargs)
fi

# Validate configuration
echo -e "${YELLOW}Validating configuration...${NC}"
./scripts/validate-config.sh "$CONFIG_FILE"

# Run collector in test mode
echo -e "${YELLOW}Starting collector for $DURATION seconds...${NC}"
timeout $DURATION otelcol-contrib --config="$CONFIG_FILE" 2>&1 | tee "/tmp/otel-${DATABASE}-test.log"

# Check for errors in log
if grep -i "error" "/tmp/otel-${DATABASE}-test.log"; then
    echo -e "${RED}Errors found in collector log${NC}"
    exit 1
else
    echo -e "${GREEN}No errors found in collector log${NC}"
fi

# Validate metrics were sent
echo -e "${YELLOW}Validating metrics...${NC}"
sleep 10  # Give metrics time to appear
./scripts/validate-metrics.sh "$DATABASE"

echo -e "${GREEN}Test completed successfully!${NC}"
