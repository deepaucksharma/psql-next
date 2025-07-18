#!/bin/bash
# Script to clean up archive directories

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Dry run mode by default
DRY_RUN=${1:-true}

if [ "$1" = "--execute" ]; then
    DRY_RUN=false
    echo -e "${RED}WARNING: Running in EXECUTE mode - files will be deleted!${NC}"
    read -p "Are you sure? (yes/no): " -r
    if [[ ! $REPLY =~ ^[Yy][Ee][Ss]$ ]]; then
        echo "Aborted."
        exit 1
    fi
else
    echo -e "${YELLOW}Running in DRY RUN mode - no files will be deleted${NC}"
    echo -e "Use '$0 --execute' to actually delete files\n"
fi

echo -e "${BLUE}=== Archive Cleanup Tool ===${NC}"

# Track statistics
TOTAL_FILES=0
TOTAL_SIZE=0
DIRS_TO_REMOVE=""

# Function to get directory size
get_dir_size() {
    local dir=$1
    if [ -d "$dir" ]; then
        du -sh "$dir" 2>/dev/null | awk '{print $1}'
    else
        echo "0"
    fi
}

# Function to count files
count_files() {
    local dir=$1
    if [ -d "$dir" ]; then
        find "$dir" -type f | wc -l | tr -d ' '
    else
        echo "0"
    fi
}

# Find all archive directories
echo -e "${YELLOW}Searching for archive directories...${NC}"

# Archive directories to check
ARCHIVE_DIRS=(
    "./archive"
    "./docs/archive"
    "./tests/archive"
    "./tests/e2e/archive"
    "./development/archive"
    "../database-intelligence-mvp/tests/e2e/archive"
    "../database-intelligence-mvp/docs/archive"
)

# Check each archive directory
for dir in "${ARCHIVE_DIRS[@]}"; do
    if [ -d "$dir" ]; then
        size=$(get_dir_size "$dir")
        files=$(count_files "$dir")
        TOTAL_FILES=$((TOTAL_FILES + files))
        
        echo -e "\n${YELLOW}Found: $dir${NC}"
        echo -e "  Size: $size"
        echo -e "  Files: $files"
        
        # Show subdirectories
        if [ -d "$dir" ]; then
            echo -e "  Subdirectories:"
            find "$dir" -type d -maxdepth 1 -mindepth 1 | while read subdir; do
                subsize=$(get_dir_size "$subdir")
                subfiles=$(count_files "$subdir")
                echo -e "    - $(basename "$subdir") ($subfiles files, $subsize)"
            done
        fi
        
        DIRS_TO_REMOVE="$DIRS_TO_REMOVE $dir"
    fi
done

# Old test files pattern
echo -e "\n${YELLOW}Searching for old test files...${NC}"
OLD_TEST_FILES=$(find . -name "*test*.sh" -path "*/archive/*" -type f 2>/dev/null | wc -l | tr -d ' ')
echo -e "Found $OLD_TEST_FILES old test scripts in archive directories"

# Check for backup files
echo -e "\n${YELLOW}Searching for backup files...${NC}"
BACKUP_FILES=$(find . -name "*.bak" -o -name "*.backup" -o -name "*.old" -o -name "*~" 2>/dev/null | wc -l | tr -d ' ')
echo -e "Found $BACKUP_FILES backup files"

# Summary
echo -e "\n${BLUE}=== Cleanup Summary ===${NC}"
echo -e "Archive directories to remove: $(echo $DIRS_TO_REMOVE | wc -w)"
echo -e "Total files in archives: $TOTAL_FILES"
echo -e "Backup files to remove: $BACKUP_FILES"
echo -e "Old test scripts: $OLD_TEST_FILES"

if [ "$DRY_RUN" = false ]; then
    echo -e "\n${RED}Executing cleanup...${NC}"
    
    # Remove archive directories
    for dir in $DIRS_TO_REMOVE; do
        if [ -d "$dir" ]; then
            echo -e "Removing $dir..."
            rm -rf "$dir"
        fi
    done
    
    # Remove backup files
    echo -e "\nRemoving backup files..."
    find . -name "*.bak" -o -name "*.backup" -o -name "*.old" -o -name "*~" -exec rm -f {} \;
    
    # Clean up empty directories
    echo -e "\nCleaning up empty directories..."
    find . -type d -empty -delete 2>/dev/null || true
    
    echo -e "\n${GREEN}✓ Cleanup complete!${NC}"
else
    echo -e "\n${YELLOW}No files were deleted (dry run mode)${NC}"
    
    # Estimate space savings
    TOTAL_SPACE=0
    for dir in $DIRS_TO_REMOVE; do
        if [ -d "$dir" ]; then
            space=$(du -sb "$dir" 2>/dev/null | awk '{print $1}')
            TOTAL_SPACE=$((TOTAL_SPACE + space))
        fi
    done
    
    # Convert to human readable
    if [ $TOTAL_SPACE -gt 1073741824 ]; then
        SPACE_SAVED=$(echo "scale=2; $TOTAL_SPACE / 1073741824" | bc)
        UNIT="GB"
    elif [ $TOTAL_SPACE -gt 1048576 ]; then
        SPACE_SAVED=$(echo "scale=2; $TOTAL_SPACE / 1048576" | bc)
        UNIT="MB"
    else
        SPACE_SAVED=$(echo "scale=2; $TOTAL_SPACE / 1024" | bc)
        UNIT="KB"
    fi
    
    echo -e "\nEstimated space to be freed: ${GREEN}${SPACE_SAVED} ${UNIT}${NC}"
fi