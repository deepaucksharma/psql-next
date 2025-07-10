#!/bin/bash

# Script to clean up unused configuration files
# Run with --dry-run to see what would be removed without actually removing

set -e

DRY_RUN=false
if [[ "$1" == "--dry-run" ]]; then
    DRY_RUN=true
    echo "DRY RUN MODE - No files will be actually moved"
fi

# Create archive directory if it doesn't exist
ARCHIVE_DIR="config/archive/unused-$(date +%Y%m%d)"
if [[ "$DRY_RUN" == false ]]; then
    mkdir -p "$ARCHIVE_DIR"
fi

echo "=== Configuration Cleanup Script ==="
echo "Archive directory: $ARCHIVE_DIR"
echo ""

# List of unused config files to archive
UNUSED_CONFIGS=(
    "config/collector-minimal.yaml"
    "config/collector-telemetry.yaml"
    "config/demo-config.yaml"
    "config/demo-simple.yaml"
    "config/pii-detection-enhanced.yaml"
    "config/production-demo.yaml"
    "config/test-custom-processors.yaml"
    "config/test-immediate-output.yaml"
    "config/test-logs-pipeline.yaml"
)

# Archive unused configs
echo "Moving unused configuration files..."
for config in "${UNUSED_CONFIGS[@]}"; do
    if [[ -f "$config" ]]; then
        if [[ "$DRY_RUN" == true ]]; then
            echo "Would move: $config -> $ARCHIVE_DIR/$(basename $config)"
        else
            mv "$config" "$ARCHIVE_DIR/"
            echo "Moved: $config -> $ARCHIVE_DIR/$(basename $config)"
        fi
    else
        echo "Not found: $config (skipping)"
    fi
done

# Handle examples directory
if [[ -d "config/examples" ]]; then
    if [[ "$DRY_RUN" == true ]]; then
        echo "Would move: config/examples -> $ARCHIVE_DIR/examples"
    else
        mv "config/examples" "$ARCHIVE_DIR/"
        echo "Moved: config/examples -> $ARCHIVE_DIR/examples"
    fi
fi

echo ""
echo "=== Active Configuration Files ==="
echo "The following configuration files remain active:"
ls -la config/*.yaml 2>/dev/null | grep -v "config/archive" || echo "No yaml files in config/"

echo ""
echo "=== Summary ==="
if [[ "$DRY_RUN" == true ]]; then
    echo "Dry run complete. Run without --dry-run to actually move files."
else
    echo "Configuration cleanup complete!"
    echo "Archived files are in: $ARCHIVE_DIR"
    echo ""
    echo "To restore a file:"
    echo "  mv $ARCHIVE_DIR/<filename> config/"
fi