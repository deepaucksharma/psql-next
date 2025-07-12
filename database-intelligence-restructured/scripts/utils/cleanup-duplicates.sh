#!/bin/bash
# Clean up duplicate scripts after consolidation

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Create archive directory for old scripts
ARCHIVE_DIR="$PROJECT_ROOT/docs/archive/scripts"
mkdir -p "$ARCHIVE_DIR"

echo "Archiving duplicate scripts..."

# List of scripts to archive (keeping them for reference)
OLD_SCRIPTS=(
    "build.sh"
    "build-complete-collector.sh"
    "build-with-all-components.sh"
    "build-working-components.sh"
    "test.sh"
    "start-collector.sh"
    "send-test-data.sh"
    "send-dashboard-data.sh"
    "send-user-session-data.sh"
    "continuous-data-generator.sh"
    "test-complete-setup.sh"
    "test-component-builds.sh"
    "fix-all-component-errors.sh"
    "fix-otel-versions.sh"
    "check-status.sh"
    "add-custom-components.sh"
)

# Move scripts to archive
for script in "${OLD_SCRIPTS[@]}"; do
    if [[ -f "$PROJECT_ROOT/$script" ]]; then
        echo "Archiving $script..."
        mv "$PROJECT_ROOT/$script" "$ARCHIVE_DIR/"
    fi
done

# Archive old scripts directories that are now consolidated
if [[ -d "$PROJECT_ROOT/tools/scripts/maintenance" ]]; then
    echo "Archiving tools/scripts/maintenance..."
    mv "$PROJECT_ROOT/tools/scripts/maintenance" "$ARCHIVE_DIR/maintenance"
fi

if [[ -d "$PROJECT_ROOT/tools/scripts/test" ]]; then
    echo "Archiving tools/scripts/test..."
    mv "$PROJECT_ROOT/tools/scripts/test" "$ARCHIVE_DIR/test"
fi

# Clean up empty directories
find "$PROJECT_ROOT/tools" -type d -empty -delete 2>/dev/null || true

# Archive duplicate config files
CONFIG_ARCHIVE="$PROJECT_ROOT/docs/archive/configs"
mkdir -p "$CONFIG_ARCHIVE"

# Move test configs that are duplicates
for config in test-*.yaml; do
    if [[ -f "$PROJECT_ROOT/configs/$config" ]]; then
        echo "Archiving config $config..."
        mv "$PROJECT_ROOT/configs/$config" "$CONFIG_ARCHIVE/"
    fi
done

# Create symlinks for backward compatibility (optional)
echo "Creating compatibility symlinks..."
ln -sf "$PROJECT_ROOT/scripts/build/build.sh" "$PROJECT_ROOT/build.sh" || true
ln -sf "$PROJECT_ROOT/scripts/test/test.sh" "$PROJECT_ROOT/test.sh" || true

echo "✅ Cleanup complete!"
echo ""
echo "Old scripts archived to: $ARCHIVE_DIR"
echo "Old configs archived to: $CONFIG_ARCHIVE"
echo ""
echo "Use 'make help' to see all available commands"
echo "See MIGRATION.md for detailed migration guide"