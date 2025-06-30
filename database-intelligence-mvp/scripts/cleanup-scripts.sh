#!/bin/bash

# Script to clean up unused shell scripts
# Run with --dry-run to see what would be removed without actually removing

set -e

DRY_RUN=false
if [[ "$1" == "--dry-run" ]]; then
    DRY_RUN=true
    echo "DRY RUN MODE - No files will be actually moved"
fi

# Create archive directory if it doesn't exist
ARCHIVE_DIR="scripts/archive/unused-$(date +%Y%m%d)"
if [[ "$DRY_RUN" == false ]]; then
    mkdir -p "$ARCHIVE_DIR"
fi

echo "=== Script Cleanup Script ==="
echo "Archive directory: $ARCHIVE_DIR"
echo ""

# List of unused scripts to archive
UNUSED_SCRIPTS=(
    "scripts/check-newrelic-data.sh"
    "scripts/feedback-loop.sh"
    "scripts/generate-test-load.sh"
    "scripts/nerdgraph-verification.sh"
    "scripts/run-with-verification.sh"
    "scripts/send-test-logs.sh"
    "scripts/test-nr-connection.sh"
    "scripts/validate-entity-synthesis.sh"
    "scripts/validate-ohi-parity.sh"
    "scripts/validate-otel-metrics.sh"
    "scripts/validate-prerequisites.sh"
    "scripts/build_docker_image.sh"
    "tests/integration/test-experimental-components.sh"
    "tests/integration/validate-setup.sh"
    "./test-query-logs.sh"
)

# Archive unused scripts
echo "Moving unused scripts..."
moved_count=0
not_found_count=0

for script in "${UNUSED_SCRIPTS[@]}"; do
    if [[ -f "$script" ]]; then
        # Create subdirectory structure in archive
        script_dir=$(dirname "$script")
        archive_subdir="$ARCHIVE_DIR/${script_dir#./}"
        
        if [[ "$DRY_RUN" == true ]]; then
            echo "Would move: $script -> $archive_subdir/$(basename $script)"
        else
            mkdir -p "$archive_subdir"
            mv "$script" "$archive_subdir/"
            echo "Moved: $script -> $archive_subdir/$(basename $script)"
        fi
        ((moved_count++))
    else
        echo "Not found: $script (skipping)"
        ((not_found_count++))
    fi
done

echo ""
echo "=== Active Scripts ==="
echo "The following scripts remain active:"
echo ""
echo "In scripts/:"
ls scripts/*.sh 2>/dev/null | grep -v archive | head -20 || echo "No .sh files found"
echo ""
echo "In tests/:"
find tests -name "*.sh" -not -path "*/archive/*" 2>/dev/null | head -10 || echo "No .sh files found"

echo ""
echo "=== Summary ==="
echo "Scripts to be moved: $moved_count"
echo "Scripts not found: $not_found_count"

if [[ "$DRY_RUN" == true ]]; then
    echo ""
    echo "Dry run complete. Run without --dry-run to actually move files."
else
    echo ""
    echo "Script cleanup complete!"
    echo "Archived files are in: $ARCHIVE_DIR"
    echo ""
    echo "To restore a script:"
    echo "  mv $ARCHIVE_DIR/<path>/<filename> <original-path>/"
fi

# Check for the missing script referenced in Makefile
echo ""
echo "=== Missing Script Check ==="
if [[ ! -f "scripts/quickstart-enhanced.sh" ]]; then
    echo "WARNING: Makefile references 'quickstart-enhanced.sh' but it doesn't exist!"
    echo "Consider updating the Makefile or creating this script."
fi