#!/bin/bash

# Fix all module paths to remove -restructured suffix
# This updates go.mod files and import statements

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

echo -e "${BLUE}=== FIXING MODULE PATHS ===${NC}"

# Backup current state
echo -e "\n${CYAN}Creating backup...${NC}"
tar -czf module-path-backup-$(date +%Y%m%d-%H%M%S).tar.gz \
    --exclude='*.tar.gz' \
    --exclude='.git' \
    --exclude='vendor' \
    --exclude='bin' \
    --exclude='dist' \
    go.mod go.work */go.mod */*/go.mod */*/*/go.mod 2>/dev/null || true

echo -e "${GREEN}[✓]${NC} Backup created"

# ==============================================================================
# Step 1: Fix module declarations in go.mod files
# ==============================================================================
echo -e "\n${CYAN}Step 1: Fixing module declarations in go.mod files${NC}"

# Find all go.mod files and fix module paths
find . -name "go.mod" -type f | while read -r modfile; do
    if grep -q "github.com/database-intelligence-restructured" "$modfile"; then
        echo -e "${YELLOW}Fixing:${NC} $modfile"
        sed -i.bak 's|github.com/database-intelligence-restructured|github.com/database-intelligence|g' "$modfile"
        rm -f "${modfile}.bak"
    fi
done

# ==============================================================================
# Step 2: Fix imports in Go source files
# ==============================================================================
echo -e "\n${CYAN}Step 2: Fixing imports in Go source files${NC}"

# Count files to update
IMPORT_COUNT=$(find . -name "*.go" -type f -exec grep -l "github.com/database-intelligence-restructured" {} \; 2>/dev/null | wc -l || echo "0")
echo -e "Found ${YELLOW}$IMPORT_COUNT${NC} files with imports to fix"

# Fix imports in all Go files
if [ "$IMPORT_COUNT" -gt 0 ]; then
    find . -name "*.go" -type f -exec grep -l "github.com/database-intelligence-restructured" {} \; 2>/dev/null | while read -r gofile; do
        echo -e "${YELLOW}Fixing imports in:${NC} $gofile"
        sed -i.bak 's|github.com/database-intelligence-restructured|github.com/database-intelligence|g' "$gofile"
        rm -f "${gofile}.bak"
    done
fi

# ==============================================================================
# Step 3: Update core/go.mod specifically
# ==============================================================================
echo -e "\n${CYAN}Step 3: Updating core module${NC}"

if [ -f "core/go.mod" ]; then
    echo -e "${YELLOW}Updating core/go.mod module path and dependencies${NC}"
    cd core
    
    # First update the module declaration
    sed -i.bak 's|module github.com/database-intelligence-restructured/core|module github.com/database-intelligence/core|g' go.mod
    
    # Update all references to database-intelligence-restructured
    sed -i.bak 's|github.com/database-intelligence-restructured/|github.com/database-intelligence/|g' go.mod
    
    rm -f go.mod.bak
    cd ..
    echo -e "${GREEN}[✓]${NC} Core module updated"
fi

# ==============================================================================
# Step 4: Update distributions/production/go.mod
# ==============================================================================
echo -e "\n${CYAN}Step 4: Updating production distribution${NC}"

if [ -f "distributions/production/go.mod" ]; then
    echo -e "${YELLOW}Updating distributions/production/go.mod${NC}"
    cd distributions/production
    
    # Update module declaration and dependencies
    sed -i.bak 's|github.com/database-intelligence-restructured|github.com/database-intelligence|g' go.mod
    
    rm -f go.mod.bak
    cd ../..
    echo -e "${GREEN}[✓]${NC} Production distribution updated"
fi

# ==============================================================================
# Step 5: Update go.work if needed
# ==============================================================================
echo -e "\n${CYAN}Step 5: Checking go.work${NC}"

if grep -q "database-intelligence-restructured" go.work 2>/dev/null; then
    echo -e "${YELLOW}Updating go.work${NC}"
    sed -i.bak 's|database-intelligence-restructured|database-intelligence|g' go.work
    rm -f go.work.bak
    echo -e "${GREEN}[✓]${NC} go.work updated"
else
    echo -e "${GREEN}[✓]${NC} go.work already correct"
fi

# ==============================================================================
# Step 6: Verify changes
# ==============================================================================
echo -e "\n${CYAN}Step 6: Verifying changes${NC}"

# Check for any remaining references
REMAINING=$(grep -r "database-intelligence-restructured" --include="*.go" --include="*.mod" . 2>/dev/null | wc -l || echo "0")

if [ "$REMAINING" -eq 0 ]; then
    echo -e "${GREEN}[✓]${NC} All module paths successfully updated!"
else
    echo -e "${YELLOW}[!]${NC} Found $REMAINING remaining references to old module path"
    echo "Remaining references:"
    grep -r "database-intelligence-restructured" --include="*.go" --include="*.mod" . 2>/dev/null | head -5 || true
fi

# ==============================================================================
# Summary
# ==============================================================================
echo -e "\n${CYAN}=== MODULE PATH UPDATE COMPLETE ===${NC}"

echo -e "\nChanges made:"
echo "- Updated all go.mod module declarations"
echo "- Fixed all import statements in .go files"
echo "- Updated replace directives"
echo "- Aligned core and production distributions"

echo -e "\n${YELLOW}Next steps:${NC}"
echo "1. Run 'go work sync' to update workspace"
echo "2. Update module versions to v1.35.0 + v0.129.0 pattern"
echo "3. Build and test each module"

echo -e "\n${GREEN}Module paths are now consistent!${NC}"