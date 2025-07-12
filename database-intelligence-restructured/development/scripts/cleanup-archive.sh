#!/bin/bash
# Script to clean up the archive directory

set -e

ARCHIVE_DIR="./archive"
BACKUP_DIR="./archive-backup-$(date +%Y%m%d-%H%M%S)"

echo "Archive Cleanup Script"
echo "====================="

# Check if archive directory exists
if [ ! -d "$ARCHIVE_DIR" ]; then
    echo "Archive directory not found. Nothing to clean."
    exit 0
fi

# Show current size
echo "Current archive size: $(du -sh $ARCHIVE_DIR | cut -f1)"

# Create backup
echo "Creating backup at: $BACKUP_DIR"
mkdir -p "$BACKUP_DIR"

# Identify what to keep
echo ""
echo "Preserving important files:"
# Keep any recent builder configs or critical scripts
find "$ARCHIVE_DIR" -name "otelcol-builder-config*.yaml" -type f -newer "$ARCHIVE_DIR" -mtime -7 | while read -r file; do
    echo "  - $file"
    cp --parents "$file" "$BACKUP_DIR/" 2>/dev/null || true
done

# Keep test configs backup (recent)
if [ -d "$ARCHIVE_DIR/test-configs-backup" ]; then
    echo "  - test-configs-backup/"
    cp -r "$ARCHIVE_DIR/test-configs-backup" "$BACKUP_DIR/" 2>/dev/null || true
fi

# Create a manifest of what's being removed
echo ""
echo "Creating manifest of archived content..."
find "$ARCHIVE_DIR" -type f | sort > "$BACKUP_DIR/archive-manifest.txt"
echo "Manifest saved to: $BACKUP_DIR/archive-manifest.txt"

# Compress the backup
echo ""
echo "Compressing backup..."
tar -czf "${BACKUP_DIR}.tar.gz" "$BACKUP_DIR"
rm -rf "$BACKUP_DIR"
echo "Compressed backup saved as: ${BACKUP_DIR}.tar.gz"

# Show what will be removed
echo ""
echo "Archive directory contents:"
echo "- phase1-module-consolidation: $(find $ARCHIVE_DIR/phase1-module-consolidation -type f 2>/dev/null | wc -l) files"
echo "- phase1-config-cleanup: $(find $ARCHIVE_DIR/phase1-config-cleanup -type f 2>/dev/null | wc -l) files"
echo "- scripts: $(find $ARCHIVE_DIR/scripts -type f 2>/dev/null | wc -l) files"
echo "- other: $(find $ARCHIVE_DIR -maxdepth 1 -type f 2>/dev/null | wc -l) files"

# Ask for confirmation
echo ""
read -p "Remove archive directory? This will free up ~1.9MB (y/N): " -n 1 -r
echo ""

if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "Removing archive directory..."
    rm -rf "$ARCHIVE_DIR"
    echo "Archive directory removed successfully."
    echo "Backup preserved at: ${BACKUP_DIR}.tar.gz"
else
    echo "Archive cleanup cancelled."
fi