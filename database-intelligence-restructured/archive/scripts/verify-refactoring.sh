#!/bin/bash

# Comprehensive Refactoring Verification Script
# This script verifies that no critical content was lost during refactoring

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m'

PROJECT_ROOT="/Users/deepaksharma/syc/db-otel/database-intelligence-restructured"
MVP_ROOT="/Users/deepaksharma/syc/db-otel/database-intelligence-mvp"
BACKUP_BASE="/Users/deepaksharma/syc/db-otel"
VERIFICATION_REPORT="$PROJECT_ROOT/VERIFICATION_REPORT_$(date +%Y%m%d-%H%M%S).md"

echo -e "${BLUE}=== Comprehensive Refactoring Verification ===${NC}"

# Initialize report
cat > "$VERIFICATION_REPORT" << EOF
# Refactoring Verification Report
Generated: $(date)

## Overview
This report verifies the integrity of the refactoring process.

EOF

# Function to check if content exists somewhere
check_content_exists() {
    local content_type=$1
    local pattern=$2
    local description=$3
    
    echo -e "\n${YELLOW}Checking: $description${NC}"
    
    # Search in current project
    if grep -r "$pattern" "$PROJECT_ROOT" --exclude-dir=backup* --exclude-dir=.git >/dev/null 2>&1; then
        echo -e "${GREEN}[✓]${NC} Found in restructured project"
        echo "✓ $description - Found in restructured project" >> "$VERIFICATION_REPORT"
        return 0
    fi
    
    # Search in backups
    for backup in $BACKUP_BASE/backup-*/; do
        if [ -d "$backup" ]; then
            if grep -r "$pattern" "$backup" >/dev/null 2>&1; then
                echo -e "${YELLOW}[!]${NC} Found only in backup: $backup"
                echo "⚠ $description - Found only in backup: $backup" >> "$VERIFICATION_REPORT"
                return 1
            fi
        fi
    done
    
    echo -e "${RED}[✗]${NC} Not found anywhere!"
    echo "✗ $description - NOT FOUND!" >> "$VERIFICATION_REPORT"
    return 2
}

# 1. Verify Critical Go Modules
echo -e "\n${BLUE}1. Verifying Critical Go Modules${NC}"
echo -e "\n## Critical Go Modules" >> "$VERIFICATION_REPORT"

CRITICAL_MODULES=(
    "processors/adaptivesampler"
    "processors/circuitbreaker"
    "processors/costcontrol"
    "processors/nrerrormonitor"
    "processors/planattributeextractor"
    "processors/querycorrelator"
    "processors/verification"
    "exporters/nri"
    "extensions/healthcheck"
    "common/featuredetector"
    "common/queryselector"
)

for module in "${CRITICAL_MODULES[@]}"; do
    if [ -d "$PROJECT_ROOT/$module" ] && [ -f "$PROJECT_ROOT/$module/go.mod" ]; then
        echo -e "${GREEN}[✓]${NC} $module exists with go.mod"
        echo "✓ $module - Present with go.mod" >> "$VERIFICATION_REPORT"
    else
        echo -e "${RED}[✗]${NC} $module missing or incomplete!"
        echo "✗ $module - MISSING!" >> "$VERIFICATION_REPORT"
    fi
done

# 2. Verify Critical Configurations
echo -e "\n${BLUE}2. Verifying Critical Configurations${NC}"
echo -e "\n## Critical Configurations" >> "$VERIFICATION_REPORT"

# Check for key configuration patterns
check_content_exists "config" "postgresql.*endpoint" "PostgreSQL receiver configuration"
check_content_exists "config" "mysql.*endpoint" "MySQL receiver configuration"
check_content_exists "config" "adaptivesampler" "Adaptive sampler configuration"
check_content_exists "config" "circuitbreaker" "Circuit breaker configuration"
check_content_exists "config" "planattributeextractor" "Plan extractor configuration"
check_content_exists "config" "otlp/newrelic" "New Relic exporter configuration"

# 3. Verify Docker/Deployment Files
echo -e "\n${BLUE}3. Verifying Deployment Files${NC}"
echo -e "\n## Deployment Files" >> "$VERIFICATION_REPORT"

DEPLOYMENT_FILES=(
    "deployments/docker/compose/docker-compose.yaml"
    "deployments/docker/dockerfiles/Dockerfile"
    "deployments/kubernetes/base/deployment.yaml"
    "deployments/helm/database-intelligence/Chart.yaml"
)

for file in "${DEPLOYMENT_FILES[@]}"; do
    if [ -f "$PROJECT_ROOT/$file" ]; then
        echo -e "${GREEN}[✓]${NC} $file exists"
        echo "✓ $file - Present" >> "$VERIFICATION_REPORT"
    else
        echo -e "${RED}[✗]${NC} $file missing!"
        echo "✗ $file - MISSING!" >> "$VERIFICATION_REPORT"
    fi
done

# 4. Check for Unique Content in MVP
echo -e "\n${BLUE}4. Checking for Unique Content in MVP${NC}"
echo -e "\n## Unique MVP Content Check" >> "$VERIFICATION_REPORT"

if [ -d "$MVP_ROOT" ]; then
    # Find unique Go files in MVP
    echo -e "${YELLOW}Searching for unique Go implementations...${NC}"
    
    for gofile in $(find "$MVP_ROOT" -name "*.go" -type f | grep -v test); do
        filename=$(basename "$gofile")
        # Check if this file exists in restructured
        if ! find "$PROJECT_ROOT" -name "$filename" -type f | grep -v backup >/dev/null 2>&1; then
            echo -e "${YELLOW}[!]${NC} Unique file in MVP: $gofile"
            echo "⚠ Unique Go file in MVP: $gofile" >> "$VERIFICATION_REPORT"
        fi
    done
fi

# 5. Verify Test Files
echo -e "\n${BLUE}5. Verifying Test Coverage${NC}"
echo -e "\n## Test Files" >> "$VERIFICATION_REPORT"

# Check for test patterns
check_content_exists "test" "TestPostgreSQLReceiver" "PostgreSQL receiver tests"
check_content_exists "test" "TestMySQLReceiver" "MySQL receiver tests"
check_content_exists "test" "TestAdaptiveSampler" "Adaptive sampler tests"
check_content_exists "test" "TestCircuitBreaker" "Circuit breaker tests"
check_content_exists "test" "TestE2E" "End-to-end tests"

# 6. Check Build Capability
echo -e "\n${BLUE}6. Checking Build Capability${NC}"
echo -e "\n## Build Verification" >> "$VERIFICATION_REPORT"

cd "$PROJECT_ROOT"

# Try to build a simple test program
echo -e "${YELLOW}Testing basic Go compilation...${NC}"
cat > test-compile.go << 'EOF'
package main
import "fmt"
func main() { fmt.Println("Build test successful") }
EOF

if go build -o test-compile-binary test-compile.go 2>/dev/null; then
    echo -e "${GREEN}[✓]${NC} Basic Go compilation works"
    echo "✓ Basic Go compilation - SUCCESS" >> "$VERIFICATION_REPORT"
    rm -f test-compile-binary test-compile.go
else
    echo -e "${RED}[✗]${NC} Basic Go compilation failed"
    echo "✗ Basic Go compilation - FAILED" >> "$VERIFICATION_REPORT"
fi

# 7. Check for Broken Imports
echo -e "\n${BLUE}7. Checking for Broken Imports${NC}"
echo -e "\n## Import Check" >> "$VERIFICATION_REPORT"

# Search for old import paths
if grep -r "github.com/database-intelligence-mvp" "$PROJECT_ROOT" --include="*.go" --exclude-dir=backup* >/dev/null 2>&1; then
    echo -e "${YELLOW}[!]${NC} Found old import paths that need updating"
    echo "⚠ Old import paths found - need updating" >> "$VERIFICATION_REPORT"
    
    # List files with old imports
    grep -r "github.com/database-intelligence-mvp" "$PROJECT_ROOT" --include="*.go" --exclude-dir=backup* -l | while read file; do
        echo "  - $file" >> "$VERIFICATION_REPORT"
    done
else
    echo -e "${GREEN}[✓]${NC} No old import paths found"
    echo "✓ Import paths - All updated" >> "$VERIFICATION_REPORT"
fi

# 8. Verify Documentation Completeness
echo -e "\n${BLUE}8. Verifying Documentation${NC}"
echo -e "\n## Documentation Check" >> "$VERIFICATION_REPORT"

REQUIRED_DOCS=(
    "README.md"
    "docs/getting-started/quickstart.md"
    "docs/architecture/overview.md"
    "docs/operations/deployment.md"
    "docs/development/testing.md"
)

for doc in "${REQUIRED_DOCS[@]}"; do
    if [ -f "$PROJECT_ROOT/$doc" ]; then
        echo -e "${GREEN}[✓]${NC} $doc exists"
        echo "✓ $doc - Present" >> "$VERIFICATION_REPORT"
    else
        echo -e "${YELLOW}[!]${NC} $doc missing"
        echo "⚠ $doc - Missing" >> "$VERIFICATION_REPORT"
    fi
done

# 9. Check Configuration Integrity
echo -e "\n${BLUE}9. Checking Configuration Integrity${NC}"
echo -e "\n## Configuration Integrity" >> "$VERIFICATION_REPORT"

# Verify YAML syntax for key configs
for config in configs/examples/*.yaml; do
    if [ -f "$config" ]; then
        if python3 -c "import yaml; yaml.safe_load(open('$config'))" 2>/dev/null; then
            echo -e "${GREEN}[✓]${NC} $(basename $config) - valid YAML"
        else
            echo -e "${RED}[✗]${NC} $(basename $config) - invalid YAML!"
            echo "✗ $(basename $config) - INVALID YAML" >> "$VERIFICATION_REPORT"
        fi
    fi
done

# 10. Summary and Recommendations
echo -e "\n${BLUE}=== Verification Summary ===${NC}"
echo -e "\n## Summary and Recommendations" >> "$VERIFICATION_REPORT"

# Count issues
CRITICAL_ISSUES=$(grep -c "✗" "$VERIFICATION_REPORT" || true)
WARNINGS=$(grep -c "⚠" "$VERIFICATION_REPORT" || true)
SUCCESSES=$(grep -c "✓" "$VERIFICATION_REPORT" || true)

echo -e "\nResults:"
echo -e "${GREEN}Successful checks: $SUCCESSES${NC}"
echo -e "${YELLOW}Warnings: $WARNINGS${NC}"
echo -e "${RED}Critical issues: $CRITICAL_ISSUES${NC}"

cat >> "$VERIFICATION_REPORT" << EOF

### Results Summary
- ✓ Successful checks: $SUCCESSES
- ⚠ Warnings: $WARNINGS  
- ✗ Critical issues: $CRITICAL_ISSUES

### Recommendations
EOF

if [ $CRITICAL_ISSUES -gt 0 ]; then
    echo -e "\n${RED}CRITICAL: Found $CRITICAL_ISSUES critical issues that need immediate attention!${NC}"
    cat >> "$VERIFICATION_REPORT" << EOF
1. **CRITICAL**: Address missing components before proceeding
2. Restore any missing critical files from backups
3. Verify build and test functionality after fixes
EOF
else
    echo -e "\n${GREEN}No critical issues found. Safe to proceed with testing.${NC}"
    cat >> "$VERIFICATION_REPORT" << EOF
1. Run comprehensive tests to verify functionality
2. Update any remaining old import paths
3. Complete documentation for any missing sections
4. Consider removing MVP directory after final validation
EOF
fi

echo -e "\n${GREEN}[✓]${NC} Verification report saved to: $VERIFICATION_REPORT"

# List all backup directories for reference
echo -e "\n${BLUE}Available Backup Directories:${NC}"
ls -la $BACKUP_BASE/backup-* 2>/dev/null | grep "^d" | awk '{print $9}' || echo "No backups found"