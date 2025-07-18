#!/bin/bash

# macOS-compatible script to fix all module path references

echo "Module Path Migration Script (macOS)"
echo "===================================="
echo ""

# Define the old and new module paths
OLD_MODULE="github.com/deepaksharma/db-otel"
NEW_MODULE="github.com/database-intelligence/db-intel"

# Create backup directory
BACKUP_DIR=".module-path-backup-$(date +%Y%m%d-%H%M%S)"
mkdir -p "$BACKUP_DIR"
echo "Creating backups in: $BACKUP_DIR"
echo ""

# Function to backup and update a file
update_file() {
    local file=$1
    local backup_path="$BACKUP_DIR/$(dirname "$file")"
    
    # Create backup directory structure
    mkdir -p "$backup_path"
    
    # Copy file to backup
    cp "$file" "$backup_path/"
    
    # Update the file (macOS compatible)
    sed -i '' "s|$OLD_MODULE|$NEW_MODULE|g" "$file"
}

# 1. Fix go.mod files
echo "Step 1: Updating go.mod files..."
echo "--------------------------------"
find . -name "go.mod" -type f ! -path "./$BACKUP_DIR/*" ! -path "./.module-path-backup*/*" | while read -r file; do
    if grep -q "$OLD_MODULE" "$file"; then
        echo "  Updating: $file"
        update_file "$file"
    fi
done

# 2. Fix go.work file
echo ""
echo "Step 2: Updating go.work file..."
echo "--------------------------------"
if [ -f "go.work" ]; then
    if grep -q "$OLD_MODULE" "go.work"; then
        echo "  Updating: go.work"
        update_file "go.work"
    fi
fi

# 3. Fix all .go source files
echo ""
echo "Step 3: Updating .go source files..."
echo "-----------------------------------"
find . -name "*.go" -type f ! -path "./$BACKUP_DIR/*" ! -path "./.module-path-backup*/*" | while read -r file; do
    if grep -q "$OLD_MODULE" "$file"; then
        echo "  Updating: $file"
        update_file "$file"
    fi
done

# 4. Fix go.sum files (if any)
echo ""
echo "Step 4: Cleaning go.sum files..."
echo "--------------------------------"
find . -name "go.sum" -type f ! -path "./$BACKUP_DIR/*" ! -path "./.module-path-backup*/*" | while read -r file; do
    if [ -f "$file" ]; then
        backup_path="$BACKUP_DIR/$(dirname "$file")"
        mkdir -p "$backup_path"
        echo "  Removing (will be regenerated): $file"
        cp "$file" "$backup_path/"
        rm "$file"
    fi
done

# 5. Summary
echo ""
echo "Migration Complete!"
echo "=================="
echo ""
echo "Module path changed from:"
echo "  $OLD_MODULE"
echo "to:"
echo "  $NEW_MODULE"
echo ""
echo "Backups saved in: $BACKUP_DIR"
echo ""
echo "Next steps:"
echo "1. Run 'go work sync' to update workspace"
echo "2. Run 'go mod tidy' in each module directory"
echo "3. Test the build with 'go build ./...'"
echo "4. If everything works, remove the backup directory:"
echo "   rm -rf $BACKUP_DIR"
echo ""
echo "To restore from backup:"
echo "   cp -r $BACKUP_DIR/* ."