#!/bin/bash

# Fix old import paths
set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

PROJECT_ROOT="/Users/deepaksharma/syc/db-otel/database-intelligence-restructured"

echo -e "${BLUE}=== Fixing Old Import Paths ===${NC}"

cd "$PROJECT_ROOT"

# Find all Go files with old imports
echo -e "${YELLOW}Finding files with old import paths...${NC}"
FILES_WITH_OLD_IMPORTS=$(grep -r "github.com/database-intelligence-mvp" . --include="*.go" -l | grep -v backup || true)

if [ -z "$FILES_WITH_OLD_IMPORTS" ]; then
    echo -e "${GREEN}[✓]${NC} No old import paths found!"
    exit 0
fi

echo -e "${YELLOW}Files to fix:${NC}"
echo "$FILES_WITH_OLD_IMPORTS"

# Fix each file
for file in $FILES_WITH_OLD_IMPORTS; do
    echo -e "\n${YELLOW}Fixing: $file${NC}"
    
    # Create backup
    cp "$file" "${file}.bak"
    
    # Replace old imports with new ones
    sed -i.tmp 's|github.com/database-intelligence-mvp|github.com/database-intelligence|g' "$file"
    
    # Clean up temp file
    rm -f "${file}.tmp"
    
    echo -e "${GREEN}[✓]${NC} Fixed imports in $file"
done

echo -e "\n${GREEN}[✓]${NC} All imports updated!"
echo -e "${YELLOW}Note: Backup files created with .bak extension${NC}"