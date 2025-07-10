#!/bin/bash

echo "🔍 Comprehensive Code Review for Stale/Unreferenced Code"
echo "========================================================="

# Colors for output
RED='\033[0;31m'
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

# Create report file
REPORT="code_review_report.md"
echo "# Code Review Report - $(date)" > $REPORT
echo "" >> $REPORT

# 1. Check for unused imports using goimports
echo -e "\n${YELLOW}1. Checking for unused imports...${NC}"
echo "## Unused Imports" >> $REPORT
echo "" >> $REPORT

find . -name "*.go" -type f ! -path "./vendor/*" ! -path "./.git/*" ! -path "./distributions/*/bin/*" 2>/dev/null | while read -r file; do
    # Check if goimports would make changes
    if ! goimports -l "$file" > /dev/null 2>&1; then
        echo -e "${RED}Unused imports in: $file${NC}"
        echo "- $file" >> $REPORT
    fi
done

# 2. Find commented-out code blocks
echo -e "\n${YELLOW}2. Finding commented-out code blocks...${NC}"
echo -e "\n## Commented-Out Code Blocks" >> $REPORT
echo "" >> $REPORT

find . -name "*.go" -type f ! -path "./vendor/*" ! -path "./.git/*" 2>/dev/null | while read -r file; do
    # Look for blocks of commented code (3+ consecutive comment lines with code-like content)
    if grep -n "^\s*//.*[{};()]\|^\s*/\*" "$file" | grep -A2 -B2 "^\s*//" | grep -q "^\s*//.*[{};()]"; then
        echo -e "${YELLOW}Commented code in: $file${NC}"
        echo "### $file" >> $REPORT
        grep -n "^\s*//.*[{};()]\|^\s*/\*" "$file" | head -5 >> $REPORT
        echo "" >> $REPORT
    fi
done

# 3. Find TODO/FIXME/HACK comments
echo -e "\n${YELLOW}3. Finding TODO/FIXME/HACK comments...${NC}"
echo -e "\n## TODO/FIXME/HACK Comments" >> $REPORT
echo "" >> $REPORT

grep -rn "TODO\|FIXME\|HACK\|XXX" --include="*.go" --include="*.yaml" --include="*.yml" . 2>/dev/null | grep -v ".git" | while read -r line; do
    echo -e "${YELLOW}$line${NC}"
    echo "- $line" >> $REPORT
done

# 4. Find unused functions (functions that are defined but never called)
echo -e "\n${YELLOW}4. Checking for potentially unused functions...${NC}"
echo -e "\n## Potentially Unused Functions" >> $REPORT
echo "" >> $REPORT

# Find all function declarations
find . -name "*.go" -type f ! -path "./vendor/*" ! -path "./.git/*" ! -path "*_test.go" 2>/dev/null | while read -r file; do
    # Extract function names (excluding main, init, and test functions)
    grep -E "^func\s+[A-Z][a-zA-Z0-9_]*\s*\(" "$file" | sed -E 's/func\s+([A-Za-z0-9_]+).*/\1/' | while read -r func; do
        # Count occurrences across all files
        count=$(grep -r "\b$func\b" --include="*.go" . 2>/dev/null | grep -v "^.*:func\s\+$func" | wc -l)
        if [ "$count" -eq 0 ]; then
            echo -e "${RED}Potentially unused function: $func in $file${NC}"
            echo "- Function \`$func\` in \`$file\` (no references found)" >> $REPORT
        fi
    done
done

# 5. Find unused configuration files
echo -e "\n${YELLOW}5. Checking for unused configuration files...${NC}"
echo -e "\n## Potentially Unused Configuration Files" >> $REPORT
echo "" >> $REPORT

find . -name "*.yaml" -o -name "*.yml" | grep -E "(test-|example-|sample-|old-|backup-|temp-)" 2>/dev/null | while read -r file; do
    echo -e "${YELLOW}Potentially unused config: $file${NC}"
    echo "- $file" >> $REPORT
done

# 6. Find empty or nearly empty files
echo -e "\n${YELLOW}6. Finding empty or nearly empty files...${NC}"
echo -e "\n## Empty or Nearly Empty Files" >> $REPORT
echo "" >> $REPORT

find . -name "*.go" -type f ! -path "./vendor/*" ! -path "./.git/*" -size -100c 2>/dev/null | while read -r file; do
    lines=$(wc -l < "$file")
    if [ "$lines" -lt 10 ]; then
        echo -e "${RED}Nearly empty file: $file (${lines} lines)${NC}"
        echo "- $file (${lines} lines)" >> $REPORT
    fi
done

# 7. Check for duplicate code patterns
echo -e "\n${YELLOW}7. Checking for duplicate code patterns...${NC}"
echo -e "\n## Potential Code Duplication" >> $REPORT
echo "" >> $REPORT

# Look for similar function signatures
find . -name "*.go" -type f ! -path "./vendor/*" ! -path "./.git/*" 2>/dev/null | xargs grep -h "^func.*{$" | sort | uniq -c | sort -rn | head -10 | while read -r count pattern; do
    if [ "$count" -gt 2 ]; then
        echo -e "${YELLOW}Pattern appears $count times: $pattern${NC}"
        echo "- Pattern appears $count times: \`$pattern\`" >> $REPORT
    fi
done

# 8. Find unused test files
echo -e "\n${YELLOW}8. Checking for test files without corresponding source files...${NC}"
echo -e "\n## Orphaned Test Files" >> $REPORT
echo "" >> $REPORT

find . -name "*_test.go" -type f ! -path "./vendor/*" ! -path "./.git/*" 2>/dev/null | while read -r test_file; do
    # Get the corresponding source file name
    source_file="${test_file%_test.go}.go"
    if [ ! -f "$source_file" ]; then
        echo -e "${RED}Orphaned test file: $test_file${NC}"
        echo "- $test_file (no corresponding source file)" >> $REPORT
    fi
done

# 9. Check for unused struct fields
echo -e "\n${YELLOW}9. Checking for potentially unused struct fields...${NC}"
echo -e "\n## Potentially Unused Struct Fields" >> $REPORT
echo "" >> $REPORT

# This is a simplified check - looks for struct fields that are never referenced
find . -name "*.go" -type f ! -path "./vendor/*" ! -path "./.git/*" 2>/dev/null | while read -r file; do
    # Extract struct field names
    grep -E "^\s+[A-Z][a-zA-Z0-9_]*\s+.*\s+\`" "$file" | sed -E 's/\s+([A-Za-z0-9_]+)\s+.*/\1/' | while read -r field; do
        # Count references to this field
        count=$(grep -r "\.$field\b" --include="*.go" . 2>/dev/null | wc -l)
        if [ "$count" -eq 0 ]; then
            echo -e "${YELLOW}Potentially unused field: $field in $file${NC}"
            echo "- Field \`$field\` in \`$file\`" >> $REPORT
        fi
    done
done

# 10. Summary
echo -e "\n${GREEN}Review complete! Report saved to: $REPORT${NC}"
echo -e "\n## Summary" >> $REPORT
echo "" >> $REPORT
echo "Review completed at: $(date)" >> $REPORT

# Display key statistics
echo -e "\n${YELLOW}Key Statistics:${NC}"
TODO_COUNT=$(grep -rn "TODO\|FIXME" --include="*.go" . 2>/dev/null | wc -l)
COMMENT_BLOCKS=$(find . -name "*.go" -exec grep -l "^\s*//.*[{};()]" {} \; 2>/dev/null | wc -l)
echo "- TODO/FIXME comments: $TODO_COUNT"
echo "- Files with commented code: $COMMENT_BLOCKS"

echo "" >> $REPORT
echo "### Statistics" >> $REPORT
echo "- TODO/FIXME comments: $TODO_COUNT" >> $REPORT
echo "- Files with potential commented code: $COMMENT_BLOCKS" >> $REPORT