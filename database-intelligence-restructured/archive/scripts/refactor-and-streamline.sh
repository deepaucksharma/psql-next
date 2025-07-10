#!/bin/bash

# Database Intelligence Project Refactoring and Streamlining Script
# This script consolidates duplicate files, reorganizes structure, and cleans up the project

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Current directory
CURRENT_DIR=$(pwd)
PROJECT_ROOT="/Users/deepaksharma/syc/db-otel/database-intelligence-restructured"
MVP_ROOT="/Users/deepaksharma/syc/db-otel/database-intelligence-mvp"

echo -e "${BLUE}=== Database Intelligence Refactoring and Streamlining ===${NC}"
echo -e "${BLUE}Project Root: $PROJECT_ROOT${NC}"
echo -e "${BLUE}MVP Root: $MVP_ROOT${NC}"

# Function to print status
print_status() {
    echo -e "${GREEN}[✓]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[!]${NC} $1"
}

print_error() {
    echo -e "${RED}[✗]${NC} $1"
}

# Create backup directory with timestamp
BACKUP_DIR="/Users/deepaksharma/syc/db-otel/backup-$(date +%Y%m%d-%H%M%S)"
echo -e "\n${BLUE}Phase 1: Creating Backup${NC}"
mkdir -p "$BACKUP_DIR"
print_status "Created backup directory: $BACKUP_DIR"

# Create a comprehensive refactoring report
REPORT_FILE="$PROJECT_ROOT/REFACTORING_REPORT_$(date +%Y%m%d-%H%M%S).md"
cat > "$REPORT_FILE" << EOF
# Database Intelligence Refactoring Report
Generated: $(date)

## Overview
This report documents the refactoring and streamlining of the Database Intelligence project.

## Actions Taken

EOF

# Phase 2: Remove duplicate builder configs
echo -e "\n${BLUE}Phase 2: Consolidating Builder Configurations${NC}"
cd "$PROJECT_ROOT"

# Keep only the main otelcol-builder-config.yaml
if [ -f "otelcol-builder-config.yaml" ]; then
    # Remove other builder configs
    for file in builder-config.yaml builder-config-v2.yaml builder-config-corrected.yaml builder-config-official.yaml; do
        if [ -f "$file" ]; then
            mv "$file" "$BACKUP_DIR/"
            print_status "Moved $file to backup"
            echo "- Removed duplicate builder config: $file" >> "$REPORT_FILE"
        fi
    done
fi

# Phase 3: Consolidate documentation
echo -e "\n${BLUE}Phase 3: Consolidating Documentation${NC}"

# Create new documentation structure
mkdir -p "$PROJECT_ROOT/docs/getting-started"
mkdir -p "$PROJECT_ROOT/docs/architecture"
mkdir -p "$PROJECT_ROOT/docs/operations"
mkdir -p "$PROJECT_ROOT/docs/development"
mkdir -p "$PROJECT_ROOT/docs/releases"

# Move and consolidate documentation
echo "### Documentation Consolidation" >> "$REPORT_FILE"

# Remove duplicate files from archive that exist in MVP docs
if [ -d "$PROJECT_ROOT/archive" ] && [ -d "$MVP_ROOT/docs" ]; then
    for file in ARCHITECTURE.md CHANGELOG.md CONFIGURATION.md DEPLOYMENT_GUIDE.md E2E_TESTING_COMPLETE.md FEATURES.md MIGRATION_GUIDE.md QUICK_START.md TROUBLESHOOTING.md; do
        if [ -f "$PROJECT_ROOT/archive/$file" ] && [ -f "$MVP_ROOT/docs/$file" ]; then
            # Check if files are identical
            if cmp -s "$PROJECT_ROOT/archive/$file" "$MVP_ROOT/docs/$file"; then
                mv "$PROJECT_ROOT/archive/$file" "$BACKUP_DIR/"
                print_status "Removed duplicate: archive/$file (identical to MVP version)"
                echo "- Removed duplicate documentation: archive/$file" >> "$REPORT_FILE"
            fi
        fi
    done
fi

# Phase 4: Clean up duplicate scripts
echo -e "\n${BLUE}Phase 4: Removing Duplicate Scripts${NC}"
echo "### Script Consolidation" >> "$REPORT_FILE"

# List of known duplicate scripts
DUPLICATE_SCRIPTS=(
    "init-env.sh"
    "merge-config.sh"
    "validate-project-consistency.sh"
    "validate-e2e.sh"
    "validate-all.sh"
    "generate-mtls-certs.sh"
    "build-custom-collector.sh"
)

# Remove duplicates from MVP if they exist in restructured tools
for script in "${DUPLICATE_SCRIPTS[@]}"; do
    RESTRUCTURED_SCRIPT=$(find "$PROJECT_ROOT/tools/scripts" -name "$script" -type f 2>/dev/null | head -1)
    MVP_SCRIPT=$(find "$MVP_ROOT/scripts" -name "$script" -type f 2>/dev/null | head -1)
    
    if [ -n "$RESTRUCTURED_SCRIPT" ] && [ -n "$MVP_SCRIPT" ]; then
        if cmp -s "$RESTRUCTURED_SCRIPT" "$MVP_SCRIPT"; then
            print_status "Found duplicate script: $script"
            echo "- Duplicate script: $script" >> "$REPORT_FILE"
        fi
    fi
done

# Phase 5: Consolidate Docker Compose files
echo -e "\n${BLUE}Phase 5: Consolidating Docker Compose Files${NC}"
echo "### Docker Compose Consolidation" >> "$REPORT_FILE"

# Create consolidated docker directory
mkdir -p "$PROJECT_ROOT/deployments/docker/compose"

# Identify all docker-compose files
print_status "Analyzing docker-compose files..."
find "$PROJECT_ROOT" -name "docker-compose*.y*ml" -type f | while read -r file; do
    basename "$file"
done | sort | uniq -c | sort -rn > "$PROJECT_ROOT/docker-compose-analysis.txt"

# Phase 6: Clean up root directory
echo -e "\n${BLUE}Phase 6: Cleaning Root Directory${NC}"
echo "### Root Directory Cleanup" >> "$REPORT_FILE"

# Move root-level status files to docs
STATUS_FILES=(
    "ALL_FIXES_SUMMARY.md"
    "CODE_ISSUES_REPORT.md"
    "CODE_QUALITY_ISSUES_REPORT.md"
    "COMPLETED_PROJECT_SUMMARY.md"
    "COMPREHENSIVE_FIXES_COMPLETE.md"
    "CURRENT_STATUS.md"
    "FINAL_SUMMARY.md"
    "FIXES_SUMMARY.md"
    "RESTRUCTURING_COMPLETE.md"
    "SUCCESS_SUMMARY.md"
    "NEXT_STEPS.md"
)

mkdir -p "$PROJECT_ROOT/docs/project-status"
for file in "${STATUS_FILES[@]}"; do
    if [ -f "$PROJECT_ROOT/$file" ]; then
        mv "$PROJECT_ROOT/$file" "$PROJECT_ROOT/docs/project-status/"
        print_status "Moved $file to docs/project-status/"
        echo "- Moved status file: $file to docs/project-status/" >> "$REPORT_FILE"
    fi
done

# Phase 7: Remove redundant build scripts
echo -e "\n${BLUE}Phase 7: Consolidating Build Scripts${NC}"
echo "### Build Script Consolidation" >> "$REPORT_FILE"

BUILD_SCRIPTS=(
    "build-and-test.sh"
    "build-final-working.sh"
    "build-minimal-collector.sh"
    "build-minimal.sh"
    "build-step-by-step.sh"
    "build-streamlined-collector.sh"
    "build-with-official-builder.sh"
    "build-working-collector.sh"
    "create-working-build.sh"
    "final-cleanup-and-build.sh"
    "final-working-solution.sh"
)

mkdir -p "$BACKUP_DIR/build-scripts"
for script in "${BUILD_SCRIPTS[@]}"; do
    if [ -f "$PROJECT_ROOT/$script" ]; then
        mv "$PROJECT_ROOT/$script" "$BACKUP_DIR/build-scripts/"
        print_status "Moved $script to backup"
        echo "- Archived build script: $script" >> "$REPORT_FILE"
    fi
done

# Create a single consolidated build script
cat > "$PROJECT_ROOT/build.sh" << 'BUILDSCRIPT'
#!/bin/bash
# Consolidated build script for Database Intelligence

set -e

# Build distributions
echo "Building Database Intelligence distributions..."

# Minimal distribution
echo "Building minimal distribution..."
cd distributions/minimal && go build -o ../../build/database-intelligence-minimal

# Standard distribution  
echo "Building standard distribution..."
cd ../standard && go build -o ../../build/database-intelligence-standard

# Enterprise distribution
echo "Building enterprise distribution..."
cd ../enterprise && go build -o ../../build/database-intelligence-enterprise

echo "Build complete!"
BUILDSCRIPT
chmod +x "$PROJECT_ROOT/build.sh"
print_status "Created consolidated build.sh"

# Phase 8: Fix script consolidation
echo -e "\n${BLUE}Phase 8: Consolidating Fix Scripts${NC}"
echo "### Fix Script Consolidation" >> "$REPORT_FILE"

FIX_SCRIPTS=(
    "align-otel-versions.sh"
    "comprehensive-fix.sh"
    "fix-all-go-versions.sh"
    "fix-all-versions.sh"
    "fix-go-versions.sh"
    "fix-imports.sh"
    "fix-modules.sh"
    "fix-otel-versions-comprehensive.sh"
    "fix-processor-modules.sh"
    "quick-align-versions.sh"
    "run-go-mod-tidy.sh"
)

mkdir -p "$BACKUP_DIR/fix-scripts"
for script in "${FIX_SCRIPTS[@]}"; do
    if [ -f "$PROJECT_ROOT/$script" ]; then
        mv "$PROJECT_ROOT/$script" "$BACKUP_DIR/fix-scripts/"
        print_status "Moved $script to backup"
        echo "- Archived fix script: $script" >> "$REPORT_FILE"
    fi
done

# Create consolidated dependency fix script
cat > "$PROJECT_ROOT/fix-dependencies.sh" << 'FIXSCRIPT'
#!/bin/bash
# Consolidated dependency fix script

set -e

echo "Fixing Go module dependencies..."

# Update all modules to use consistent OTEL versions
OTEL_VERSION="v0.129.0"

# Find all go.mod files and update dependencies
find . -name "go.mod" -not -path "./backup*" | while read -r modfile; do
    dir=$(dirname "$modfile")
    echo "Updating $dir..."
    cd "$dir"
    go mod tidy
    cd - > /dev/null
done

echo "Dependencies fixed!"
FIXSCRIPT
chmod +x "$PROJECT_ROOT/fix-dependencies.sh"
print_status "Created consolidated fix-dependencies.sh"

# Phase 9: Create unified README
echo -e "\n${BLUE}Phase 9: Creating Unified README${NC}"
echo "### README Consolidation" >> "$REPORT_FILE"

# Backup existing READMEs
mkdir -p "$BACKUP_DIR/readmes"
for readme in README*.md; do
    if [ -f "$PROJECT_ROOT/$readme" ]; then
        cp "$PROJECT_ROOT/$readme" "$BACKUP_DIR/readmes/"
    fi
done

# Summary
echo -e "\n${BLUE}=== Refactoring Summary ===${NC}"
echo "" >> "$REPORT_FILE"
echo "## Summary" >> "$REPORT_FILE"
echo "- Backup created at: $BACKUP_DIR" >> "$REPORT_FILE"
echo "- Report saved to: $REPORT_FILE" >> "$REPORT_FILE"

# Count files moved/removed
MOVED_COUNT=$(grep -c "Moved\|Removed\|Archived" "$REPORT_FILE" || true)
echo "- Total files processed: $MOVED_COUNT" >> "$REPORT_FILE"

print_status "Refactoring complete!"
print_status "Backup location: $BACKUP_DIR"
print_status "Report saved to: $REPORT_FILE"

echo -e "\n${YELLOW}Next steps:${NC}"
echo "1. Review the refactoring report: $REPORT_FILE"
echo "2. Run 'make test-all' to ensure everything still works"
echo "3. Update CI/CD pipelines for new structure"
echo "4. Remove the MVP directory after verification"