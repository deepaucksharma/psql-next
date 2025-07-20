#!/bin/bash

# Test script for NerdGraph validation
# This demonstrates how to use the enhanced setup script for validation

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SETUP_SCRIPT="$SCRIPT_DIR/setup-newrelic.sh"

echo -e "${BLUE}🧪 Testing NerdGraph Dashboard Validation${NC}"
echo "==========================================="

# Check if setup script exists
if [[ ! -f "$SETUP_SCRIPT" ]]; then
    echo -e "${RED}❌ Setup script not found: $SETUP_SCRIPT${NC}"
    exit 1
fi

# Check if required environment variables are set for demo
if [[ -z "${NEW_RELIC_API_KEY:-}" ]] || [[ -z "${NEW_RELIC_ACCOUNT_ID:-}" ]]; then
    echo -e "${YELLOW}⚠️  Setting demo environment variables${NC}"
    echo ""
    echo "To run actual validation, set these environment variables:"
    echo "  export NEW_RELIC_API_KEY='your-user-api-key'"
    echo "  export NEW_RELIC_ACCOUNT_ID='your-account-id'"
    echo "  export NEW_RELIC_LICENSE_KEY='your-license-key'"
    echo ""
    echo "For this demo, we'll show the available commands:"
    echo ""
    
    # Show help
    "$SETUP_SCRIPT" help
    
    exit 0
fi

echo -e "${GREEN}✅ Environment variables found${NC}"
echo "Account ID: $NEW_RELIC_ACCOUNT_ID"
echo ""

# Test validation command
echo -e "${BLUE}🔍 Running dashboard validation...${NC}"
"$SETUP_SCRIPT" validate

echo ""
echo -e "${BLUE}📊 Testing dashboard deployment...${NC}"
"$SETUP_SCRIPT" deploy

echo ""
echo -e "${GREEN}✅ Validation test completed!${NC}"