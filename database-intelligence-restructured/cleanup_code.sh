#!/bin/bash

echo "🧹 Code Cleanup Script"
echo "====================="
echo "This script will help clean up the identified issues."
echo ""

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

# Check for required tools
check_tool() {
    if ! command -v $1 &> /dev/null; then
        echo -e "${RED}Error: $1 is not installed. Please install it first.${NC}"
        exit 1
    fi
}

echo "Checking required tools..."
check_tool "goimports"
check_tool "go"

# Backup before cleanup
echo -e "\n${YELLOW}Creating backup...${NC}"
BACKUP_DIR="backup_$(date +%Y%m%d_%H%M%S)"
mkdir -p "$BACKUP_DIR"
echo "Backup directory: $BACKUP_DIR"

# 1. Fix all import issues
echo -e "\n${YELLOW}Step 1: Fixing unused imports...${NC}"
echo "Running goimports on all Go files..."

# Create a list of files to process
find . -name "*.go" -type f ! -path "./vendor/*" ! -path "./.git/*" ! -path "./$BACKUP_DIR/*" > /tmp/go_files.txt

# Process files and track changes
FIXED_COUNT=0
while IFS= read -r file; do
    # Backup the file
    cp "$file" "$BACKUP_DIR/$(basename "$file").bak" 2>/dev/null
    
    # Run goimports
    if goimports -w "$file" 2>/dev/null; then
        if ! diff -q "$file" "$BACKUP_DIR/$(basename "$file").bak" >/dev/null 2>&1; then
            ((FIXED_COUNT++))
            echo -e "${GREEN}Fixed imports in: $file${NC}"
        fi
    fi
done < /tmp/go_files.txt

echo -e "${GREEN}Fixed imports in $FIXED_COUNT files${NC}"

# 2. Remove clearly unused files
echo -e "\n${YELLOW}Step 2: Identifying files to remove...${NC}"

# Create removal list
cat > /tmp/files_to_remove.txt << EOF
# Empty or nearly empty files
test-compile.go

# Test configuration files
tests/e2e/e2e-test-config.yaml
configs/examples/test-pipeline.yaml
configs/examples/test-config.yaml
distributions/enterprise/test-config.yaml
distributions/test-simple/config.yaml
distributions/production/test-receivers-config.yaml
distributions/production/test-receivers.yaml
distributions/production/test-basic.yaml

# Duplicate/unused main files
simple-working-collector.go
minimal-working-collector.go
basic-collector.go
working-collector/database-collector.go
working-collector/main.go
EOF

echo "Files marked for removal:"
while IFS= read -r file; do
    if [[ ! "$file" =~ ^# ]] && [ -n "$file" ] && [ -f "$file" ]; then
        echo "  - $file"
    fi
done < /tmp/files_to_remove.txt

# 3. Find and report duplicate code
echo -e "\n${YELLOW}Step 3: Identifying duplicate code patterns...${NC}"

# Find duplicate main.go files
echo "Duplicate main.go files found in:"
find . -name "main.go" -type f ! -path "./vendor/*" ! -path "./.git/*" -exec dirname {} \; | sort | uniq -c | sort -rn | head -10

# 4. Generate cleanup recommendations
echo -e "\n${YELLOW}Step 4: Generating cleanup recommendations...${NC}"

cat > cleanup_recommendations.md << 'EOF'
# Cleanup Recommendations

## Automated Cleanup Complete
- Fixed imports in all Go files
- Identified files for removal

## Manual Cleanup Required

### 1. Remove Orphaned Test Files
Review and remove test files without corresponding source:
```bash
# List all orphaned test files
find tests -name "*_test.go" -type f | while read f; do
    src="${f%_test.go}.go"
    [ ! -f "$src" ] && echo "Orphaned: $f"
done
```

### 2. Consolidate Duplicate Code
- Merge multiple main.go implementations
- Create shared utility packages
- Consolidate test helpers

### 3. Remove Unused Packages
Review and remove if unused:
- core/internal/database/
- core/internal/secrets/
- core/internal/health/
- core/internal/ratelimit/
- core/internal/conventions/
- core/internal/performance/

### 4. Clean Up Configurations
Remove test and example configs:
```bash
find . -name "test-*.yaml" -o -name "example-*.yaml" | grep -v docs
```

## Next Steps
1. Review and commit import fixes
2. Delete identified unused files
3. Consolidate duplicate code
4. Run tests to ensure nothing broke
EOF

echo -e "${GREEN}Cleanup recommendations written to: cleanup_recommendations.md${NC}"

# 5. Summary
echo -e "\n${GREEN}========== Cleanup Summary ==========${NC}"
echo "1. Fixed imports in $FIXED_COUNT files"
echo "2. Identified $(grep -v '^#' /tmp/files_to_remove.txt | grep -v '^$' | wc -l) files for removal"
echo "3. Created backup in: $BACKUP_DIR"
echo "4. Generated cleanup_recommendations.md"
echo ""
echo -e "${YELLOW}Next steps:${NC}"
echo "1. Review the changes (git diff)"
echo "2. Remove files listed in /tmp/files_to_remove.txt"
echo "3. Follow cleanup_recommendations.md"
echo "4. Run 'go mod tidy' in each module"
echo "5. Run tests to ensure everything works"

# Cleanup temp files
rm -f /tmp/go_files.txt

echo -e "\n${GREEN}Cleanup script complete!${NC}"