#!/bin/bash
# Validate OpenTelemetry configurations

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

CONFIG_FILE=${1:-}

if [ -z "$CONFIG_FILE" ]; then
    echo "Usage: $0 <config-file>"
    echo "Example: $0 configs/postgresql-maximum-extraction.yaml"
    exit 1
fi

if [ ! -f "$CONFIG_FILE" ]; then
    echo -e "${RED}Error: Config file not found: $CONFIG_FILE${NC}"
    exit 1
fi

echo -e "${YELLOW}Validating configuration: $CONFIG_FILE${NC}"

# Basic YAML validation
if ! command -v yq &> /dev/null; then
    echo -e "${YELLOW}Warning: yq not found, skipping YAML validation${NC}"
else
    if yq eval '.' "$CONFIG_FILE" > /dev/null 2>&1; then
        echo -e "${GREEN}✓ Valid YAML syntax${NC}"
    else
        echo -e "${RED}✗ Invalid YAML syntax${NC}"
        exit 1
    fi
fi

# Check required sections
for section in receivers processors exporters service; do
    if grep -q "^$section:" "$CONFIG_FILE"; then
        echo -e "${GREEN}✓ Found $section section${NC}"
    else
        echo -e "${RED}✗ Missing $section section${NC}"
        exit 1
    fi
done

# Check for New Relic exporter
if grep -q "otlp/newrelic:" "$CONFIG_FILE"; then
    echo -e "${GREEN}✓ New Relic exporter configured${NC}"
else
    echo -e "${RED}✗ New Relic exporter not found${NC}"
fi

# Check for required environment variables
env_vars=$(grep -oE '\${env:[A-Z_]+' "$CONFIG_FILE" | sort | uniq | sed 's/${env://')
if [ -n "$env_vars" ]; then
    echo -e "${YELLOW}Required environment variables:${NC}"
    for var in $env_vars; do
        if [ -z "${!var}" ]; then
            echo -e "  ${RED}✗ $var (not set)${NC}"
        else
            echo -e "  ${GREEN}✓ $var${NC}"
        fi
    done
fi

echo -e "${GREEN}Configuration validation complete!${NC}"
