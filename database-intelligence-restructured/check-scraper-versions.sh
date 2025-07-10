#!/bin/bash

# Check available versions for scraper packages

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
CYAN='\033[0;36m'
NC='\033[0m'

PROJECT_ROOT="/Users/deepaksharma/syc/db-otel/database-intelligence-restructured"
cd "$PROJECT_ROOT"

echo -e "${BLUE}=== CHECKING SCRAPER PACKAGE VERSIONS ===${NC}"

# Create a temporary directory for testing
TEMP_DIR=$(mktemp -d)
cd "$TEMP_DIR"

echo -e "\n${CYAN}Creating test module to check versions...${NC}"

# Initialize a test module
go mod init test-scraper-versions

# Try different version patterns for scraper
echo -e "\n${YELLOW}Testing scraper package versions...${NC}"

# Try v0.129.0 pattern (implementation version)
echo -e "\nTrying v0.129.0..."
go get go.opentelemetry.io/collector/scraper@v0.129.0 2>&1 || echo "Failed"

# Try v1.35.0 pattern 
echo -e "\nTrying v1.35.0..."
go get go.opentelemetry.io/collector/scraper@v1.35.0 2>&1 || echo "Failed"

# Try latest
echo -e "\nTrying latest..."
go get go.opentelemetry.io/collector/scraper@latest 2>&1 || echo "Failed"

# List what we got
echo -e "\n${CYAN}Checking go.mod contents:${NC}"
cat go.mod

# Also check scraperhelper specifically
echo -e "\n${YELLOW}Testing scraperhelper import...${NC}"
cat > test.go << 'EOF'
package main

import (
    _ "go.opentelemetry.io/collector/scraper/scraperhelper"
)

func main() {}
EOF

go mod tidy 2>&1 || echo "Failed to resolve scraperhelper"

echo -e "\n${CYAN}Final go.mod:${NC}"
cat go.mod

# Clean up
cd "$PROJECT_ROOT"
rm -rf "$TEMP_DIR"

echo -e "\n${GREEN}Version check complete!${NC}"