#!/bin/bash
# Comprehensive cleanup script to streamline file organization

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

# Dry run by default
DRY_RUN=${1:-true}

if [ "$1" = "--execute" ]; then
    DRY_RUN=false
    echo -e "${RED}WARNING: Running in EXECUTE mode - files will be deleted!${NC}"
    echo -e "${RED}This will remove hundreds of files permanently.${NC}"
    read -p "Type 'yes' to confirm: " -r
    if [[ ! $REPLY == "yes" ]]; then
        echo "Aborted."
        exit 1
    fi
else
    echo -e "${YELLOW}Running in DRY RUN mode - no files will be deleted${NC}"
    echo -e "Use '$0 --execute' to actually delete files\n"
fi

echo -e "${BLUE}=== Streamlined Cleanup Tool ===${NC}"

# Track statistics
TOTAL_FILES=0
TOTAL_DIRS=0
ARCHIVE_FILES=0
BACKUP_FILES=0
LOG_FILES=0
STATUS_FILES=0
BUILD_ARTIFACTS=0

# Function to remove files/directories
remove_item() {
    local item=$1
    local type=$2
    
    if [ -e "$item" ]; then
        if [ "$DRY_RUN" = false ]; then
            rm -rf "$item"
            echo -e "${GREEN}✓ Removed${NC} $item"
        else
            echo -e "${YELLOW}Would remove${NC} $item"
        fi
        
        if [ "$type" = "file" ]; then
            ((TOTAL_FILES++))
        else
            ((TOTAL_DIRS++))
        fi
        return 0
    fi
    return 1
}

# 1. Clean up archive directories
echo -e "\n${YELLOW}[1/7] Removing archive directories...${NC}"
for dir in archive docs/archive tests/e2e/archive tests/archive configs/archive configs/examples-archived; do
    if [ -d "$dir" ]; then
        count=$(find "$dir" -type f 2>/dev/null | wc -l | tr -d ' ')
        ARCHIVE_FILES=$((ARCHIVE_FILES + count))
        remove_item "$dir" "dir"
    fi
done

# 2. Remove status/summary files from root
echo -e "\n${YELLOW}[2/7] Removing project status files...${NC}"
STATUS_DOCS=(
    "BIG_PICTURE_SUMMARY.md"
    "CLEANUP_SUMMARY.md"
    "CODEBASE_REALITY_CHECK.md"
    "CODEBASE_REVIEW_ACTIONS.md"
    "DIVERGENCE_FIX_CHECKLIST.md"
    "DUPLICATE_FILES_ANALYSIS.md"
    "E2E_MONGODB_EXAMPLE.md"
    "E2E_READINESS_REPORT.md"
    "IMPLEMENTATION_FOCUS.md"
    "IMPLEMENTATION_STATUS.md"
    "MULTI_DATABASE_EXTENSION_PLAN.md"
    "MULTI_DATABASE_TODOS.md"
    "STREAMLINING_PLAN.md"
    "STREAMLINING_SUMMARY.md"
)

for file in "${STATUS_DOCS[@]}"; do
    if remove_item "$file" "file"; then
        ((STATUS_FILES++))
    fi
done

# 3. Remove backup files
echo -e "\n${YELLOW}[3/7] Removing backup files (.bak)...${NC}"
while IFS= read -r -d '' file; do
    if remove_item "$file" "file"; then
        ((BACKUP_FILES++))
    fi
done < <(find . -name "*.bak" -type f -print0 2>/dev/null)

# 4. Remove log files
echo -e "\n${YELLOW}[4/7] Removing log files...${NC}"
LOG_PATTERNS=(
    "*.log"
    "*.out"
    "*.tmp"
)

for pattern in "${LOG_PATTERNS[@]}"; do
    while IFS= read -r -d '' file; do
        if remove_item "$file" "file"; then
            ((LOG_FILES++))
        fi
    done < <(find . -name "$pattern" -type f -not -path "./.git/*" -print0 2>/dev/null)
done

# 5. Remove build artifacts
echo -e "\n${YELLOW}[5/7] Removing build artifacts...${NC}"
for dir in bin tests/e2e/dist; do
    if [ -d "$dir" ]; then
        count=$(find "$dir" -type f 2>/dev/null | wc -l | tr -d ' ')
        BUILD_ARTIFACTS=$((BUILD_ARTIFACTS + count))
        remove_item "$dir" "dir"
    fi
done

# 6. Clean up duplicate scripts and root files
echo -e "\n${YELLOW}[6/7] Cleaning duplicate scripts and root files...${NC}"

# Remove duplicate fix-module scripts (keep only fix-module-paths.sh)
DUPLICATE_SCRIPTS=(
    "fix-module-paths-comprehensive.sh"
    "fix-module-paths-macos.sh"
    "fix-featuredetector-imports.sh"
    "update-all-modules.sh"
    "verify-module-paths.sh"
)

for script in "${DUPLICATE_SCRIPTS[@]}"; do
    remove_item "$script" "file"
done

# Remove old build/test scripts from root
OLD_SCRIPTS=(
    "build.sh"
    "test.sh"
)

for script in "${OLD_SCRIPTS[@]}"; do
    if [ -f "$script" ] && [ -f "scripts/building/build-collector.sh" ]; then
        remove_item "$script" "file"
    fi
done

# Remove CLAUDE.md if it exists
if [ -f "CLAUDE.md" ]; then
    remove_item "CLAUDE.md" "file"
fi

# 7. Clean empty directories
echo -e "\n${YELLOW}[7/7] Removing empty directories...${NC}"
if [ "$DRY_RUN" = false ]; then
    find . -type d -empty -not -path "./.git/*" -delete 2>/dev/null || true
    echo -e "${GREEN}✓ Empty directories removed${NC}"
else
    empty_count=$(find . -type d -empty -not -path "./.git/*" 2>/dev/null | wc -l | tr -d ' ')
    echo -e "${YELLOW}Would remove $empty_count empty directories${NC}"
fi

# Create .gitignore if needed
if [ "$DRY_RUN" = false ]; then
    echo -e "\n${YELLOW}Updating .gitignore...${NC}"
    cat >> .gitignore << 'EOF'

# Build artifacts
bin/
dist/
*.exe

# Logs
*.log
*.out

# Backups
*.bak
*.backup
*.old
*~

# Test artifacts
coverage/
.coverage
*.cover

# IDE
.idea/
.vscode/
*.swp
*.swo

# OS
.DS_Store
Thumbs.db

# Environment
.env
.env.local
.env.*.local

# Archives
archive/
*.tar.gz
*.zip
EOF
    echo -e "${GREEN}✓ Updated .gitignore${NC}"
fi

# Summary
echo -e "\n${BLUE}=== Cleanup Summary ===${NC}"
echo -e "Archive files: ${ARCHIVE_FILES}"
echo -e "Status documents: ${STATUS_FILES}"
echo -e "Backup files: ${BACKUP_FILES}"
echo -e "Log files: ${LOG_FILES}"
echo -e "Build artifacts: ${BUILD_ARTIFACTS}"
echo -e "Total files: ${TOTAL_FILES}"
echo -e "Total directories: ${TOTAL_DIRS}"

TOTAL_REMOVED=$((ARCHIVE_FILES + STATUS_FILES + BACKUP_FILES + LOG_FILES + BUILD_ARTIFACTS))
echo -e "\n${YELLOW}Total items to be removed: ${TOTAL_REMOVED}${NC}"

if [ "$DRY_RUN" = true ]; then
    echo -e "\n${YELLOW}This was a dry run. To execute cleanup:${NC}"
    echo -e "${BLUE}$0 --execute${NC}"
else
    echo -e "\n${GREEN}✓ Cleanup complete!${NC}"
    echo -e "The codebase is now streamlined and organized."
    
    # Suggest next steps
    echo -e "\n${YELLOW}Next steps:${NC}"
    echo "1. Review changes: git status"
    echo "2. Stage changes: git add -A"
    echo "3. Commit: git commit -m 'chore: Major cleanup - remove stale files and archives'"
fi
# 8. Remove module backup directories
echo -e "\n${YELLOW}[8/8] Removing module backup directories...${NC}"
for dir in .module-path-backup-*; do
    if [ -d "$dir" ]; then
        count=$(find "$dir" -type f 2>/dev/null | wc -l | tr -d ' ')
        echo -e "${YELLOW}Found backup: $dir ($count files)${NC}"
        remove_item "$dir" "dir"
    fi
done
