#!/bin/bash

# Comprehensive Project Verification Script
# This script thoroughly verifies every aspect of the refactored project

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
CYAN='\033[0;36m'
NC='\033[0m'

PROJECT_ROOT="/Users/deepaksharma/syc/db-otel/database-intelligence-restructured"
MVP_ROOT="/Users/deepaksharma/syc/db-otel/database-intelligence-mvp"
VERIFICATION_LOG="$PROJECT_ROOT/COMPREHENSIVE_VERIFICATION_$(date +%Y%m%d-%H%M%S).log"

# Initialize counters
TOTAL_CHECKS=0
PASSED_CHECKS=0
FAILED_CHECKS=0
WARNING_CHECKS=0

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1" | tee -a "$VERIFICATION_LOG"
}

log_success() {
    echo -e "${GREEN}[✓ PASS]${NC} $1" | tee -a "$VERIFICATION_LOG"
    ((PASSED_CHECKS++))
    ((TOTAL_CHECKS++))
}

log_fail() {
    echo -e "${RED}[✗ FAIL]${NC} $1" | tee -a "$VERIFICATION_LOG"
    ((FAILED_CHECKS++))
    ((TOTAL_CHECKS++))
}

log_warning() {
    echo -e "${YELLOW}[⚠ WARN]${NC} $1" | tee -a "$VERIFICATION_LOG"
    ((WARNING_CHECKS++))
    ((TOTAL_CHECKS++))
}

log_section() {
    echo -e "\n${CYAN}════════════════════════════════════════════════════════${NC}" | tee -a "$VERIFICATION_LOG"
    echo -e "${CYAN}▶ $1${NC}" | tee -a "$VERIFICATION_LOG"
    echo -e "${CYAN}════════════════════════════════════════════════════════${NC}" | tee -a "$VERIFICATION_LOG"
}

# Start verification
echo -e "${BLUE}=== COMPREHENSIVE PROJECT VERIFICATION ===${NC}" | tee "$VERIFICATION_LOG"
echo "Started: $(date)" | tee -a "$VERIFICATION_LOG"
echo "Project: $PROJECT_ROOT" | tee -a "$VERIFICATION_LOG"
echo "" | tee -a "$VERIFICATION_LOG"

cd "$PROJECT_ROOT"

# ==============================================================================
# SECTION 1: Directory Structure Verification
# ==============================================================================
log_section "1. DIRECTORY STRUCTURE VERIFICATION"

REQUIRED_DIRS=(
    "configs/base"
    "configs/examples"
    "configs/overlays/environments"
    "configs/overlays/features"
    "configs/queries"
    "configs/templates"
    "configs/unified"
    "deployments/docker/compose"
    "deployments/docker/dockerfiles"
    "deployments/docker/init-scripts"
    "deployments/kubernetes/base"
    "deployments/kubernetes/overlays"
    "deployments/helm/database-intelligence"
    "docs/getting-started"
    "docs/architecture"
    "docs/operations"
    "docs/development"
    "docs/releases"
    "processors"
    "receivers"
    "exporters"
    "extensions"
    "common"
    "distributions"
    "tests"
    "tools/scripts"
)

log_info "Checking required directories..."
for dir in "${REQUIRED_DIRS[@]}"; do
    if [ -d "$dir" ]; then
        log_success "Directory exists: $dir"
    else
        log_fail "Directory missing: $dir"
    fi
done

# ==============================================================================
# SECTION 2: Core Components Verification
# ==============================================================================
log_section "2. CORE COMPONENTS VERIFICATION"

# Check processors
log_info "Verifying processors..."
PROCESSORS=(
    "adaptivesampler"
    "circuitbreaker"
    "costcontrol"
    "nrerrormonitor"
    "planattributeextractor"
    "querycorrelator"
    "verification"
)

for proc in "${PROCESSORS[@]}"; do
    if [ -d "processors/$proc" ]; then
        if [ -f "processors/$proc/go.mod" ]; then
            log_success "Processor $proc: directory ✓, go.mod ✓"
            
            # Check for key files
            for file in "factory.go" "processor.go" "config.go"; do
                if [ -f "processors/$proc/$file" ]; then
                    log_success "  └─ $file present"
                else
                    log_fail "  └─ $file missing"
                fi
            done
        else
            log_fail "Processor $proc: go.mod missing"
        fi
    else
        log_fail "Processor $proc: directory missing"
    fi
done

# Check receivers
log_info "Verifying receivers..."
RECEIVERS=(
    "ash"
    "enhancedsql"
    "kernelmetrics"
)

for recv in "${RECEIVERS[@]}"; do
    if [ -d "receivers/$recv" ]; then
        if [ -f "receivers/$recv/go.mod" ]; then
            log_success "Receiver $recv: directory ✓, go.mod ✓"
            
            # Check for factory.go
            if [ -f "receivers/$recv/factory.go" ]; then
                log_success "  └─ factory.go present"
            else
                log_fail "  └─ factory.go missing"
            fi
        else
            log_warning "Receiver $recv: go.mod missing (may use parent module)"
        fi
    else
        log_fail "Receiver $recv: directory missing"
    fi
done

# ==============================================================================
# SECTION 3: Configuration Files Verification
# ==============================================================================
log_section "3. CONFIGURATION FILES VERIFICATION"

log_info "Checking critical configuration files..."

CRITICAL_CONFIGS=(
    "configs/templates/collector-template.yaml"
    "configs/templates/environment-template.env"
    "configs/unified/database-intelligence-complete.yaml"
    "otelcol-builder-config.yaml"
)

for config in "${CRITICAL_CONFIGS[@]}"; do
    if [ -f "$config" ]; then
        # Check if file has content
        if [ -s "$config" ]; then
            lines=$(wc -l < "$config" | tr -d ' ')
            log_success "$config exists ($lines lines)"
        else
            log_fail "$config exists but is empty"
        fi
    else
        log_fail "$config missing"
    fi
done

# Verify no duplicate builder configs
log_info "Checking for duplicate builder configurations..."
DEPRECATED_BUILDERS=(
    "builder-config.yaml"
    "builder-config-v2.yaml"
    "builder-config-corrected.yaml"
    "builder-config-official.yaml"
)

duplicates_found=0
for builder in "${DEPRECATED_BUILDERS[@]}"; do
    if [ -f "$builder" ]; then
        log_fail "Duplicate builder config still exists: $builder"
        duplicates_found=1
    fi
done

if [ $duplicates_found -eq 0 ]; then
    log_success "No duplicate builder configurations found"
fi

# ==============================================================================
# SECTION 4: Documentation Verification
# ==============================================================================
log_section "4. DOCUMENTATION VERIFICATION"

log_info "Checking documentation completeness..."

REQUIRED_DOCS=(
    "README.md"
    "docs/README.md"
    "docs/getting-started/quickstart.md"
    "docs/getting-started/configuration.md"
    "docs/architecture/overview.md"
    "docs/operations/deployment.md"
    "docs/operations/troubleshooting.md"
    "docs/development/testing.md"
)

for doc in "${REQUIRED_DOCS[@]}"; do
    if [ -f "$doc" ]; then
        if [ -s "$doc" ]; then
            lines=$(wc -l < "$doc" | tr -d ' ')
            log_success "$doc present ($lines lines)"
        else
            log_fail "$doc exists but is empty"
        fi
    else
        log_fail "$doc missing"
    fi
done

# Check for old README variants
log_info "Checking for old README variants..."
OLD_READMES=(
    "README-UNIFIED.md"
    "README-NEW-RELIC-ONLY.md"
)

old_readme_found=0
for readme in "${OLD_READMES[@]}"; do
    if [ -f "$readme" ]; then
        log_fail "Old README variant still exists: $readme"
        old_readme_found=1
    fi
done

if [ $old_readme_found -eq 0 ]; then
    log_success "No old README variants found"
fi

# ==============================================================================
# SECTION 5: Build System Verification
# ==============================================================================
log_section "5. BUILD SYSTEM VERIFICATION"

# Check Go environment
log_info "Verifying Go environment..."
if command -v go &> /dev/null; then
    GO_VERSION=$(go version | awk '{print $3}')
    log_success "Go installed: $GO_VERSION"
else
    log_fail "Go not installed"
fi

# Check go.work
if [ -f "go.work" ]; then
    log_success "go.work exists"
    
    # Verify Go version in go.work
    WORK_GO_VERSION=$(grep "^go " go.work | awk '{print $2}')
    log_info "go.work Go version: $WORK_GO_VERSION"
    
    # Count modules in workspace
    MODULE_COUNT=$(grep -c "^\s*\./.*" go.work || true)
    log_success "Workspace contains $MODULE_COUNT modules"
else
    log_fail "go.work missing"
fi

# Test basic compilation
log_info "Testing Go compilation..."
cat > test_compile.go << 'EOF'
package main
import "fmt"
func main() { fmt.Println("Compilation test passed") }
EOF

if go build -o test_compile_binary test_compile.go 2>/dev/null; then
    log_success "Go compilation works"
    rm -f test_compile_binary test_compile.go
else
    log_fail "Go compilation failed"
    rm -f test_compile.go
fi

# Check build scripts
log_info "Checking build scripts..."
BUILD_SCRIPTS=(
    "build.sh"
    "fix-dependencies.sh"
)

for script in "${BUILD_SCRIPTS[@]}"; do
    if [ -f "$script" ]; then
        if [ -x "$script" ]; then
            log_success "$script exists and is executable"
        else
            log_warning "$script exists but is not executable"
        fi
    else
        log_fail "$script missing"
    fi
done

# ==============================================================================
# SECTION 6: Import Path Verification
# ==============================================================================
log_section "6. IMPORT PATH VERIFICATION"

log_info "Checking for old import paths..."
OLD_IMPORT="github.com/database-intelligence-mvp"
old_imports=$(grep -r "$OLD_IMPORT" . --include="*.go" --exclude-dir=backup* 2>/dev/null | wc -l || true)

if [ $old_imports -eq 0 ]; then
    log_success "No old import paths found"
else
    log_fail "Found $old_imports files with old import paths"
    grep -r "$OLD_IMPORT" . --include="*.go" --exclude-dir=backup* -l 2>/dev/null | head -5 | while read file; do
        log_fail "  └─ $file"
    done
fi

# ==============================================================================
# SECTION 7: Deployment Files Verification
# ==============================================================================
log_section "7. DEPLOYMENT FILES VERIFICATION"

# Check Docker files
log_info "Checking Docker deployment files..."
DOCKER_FILES=(
    "deployments/docker/compose/docker-compose.yaml"
    "deployments/docker/compose/docker-compose.prod.yaml"
    "deployments/docker/compose/docker-compose-databases.yaml"
    "deployments/docker/dockerfiles/Dockerfile"
)

for file in "${DOCKER_FILES[@]}"; do
    if [ -f "$file" ]; then
        log_success "$file present"
    else
        log_fail "$file missing"
    fi
done

# Check for duplicate docker-compose files
log_info "Checking for duplicate docker-compose files..."
compose_count=$(find . -name "docker-compose*.y*ml" -type f | grep -v backup | wc -l)
log_info "Total docker-compose files: $compose_count"

if [ $compose_count -gt 10 ]; then
    log_warning "Too many docker-compose files ($compose_count), consolidation may be incomplete"
else
    log_success "Docker-compose files appropriately consolidated ($compose_count files)"
fi

# ==============================================================================
# SECTION 8: Test Structure Verification
# ==============================================================================
log_section "8. TEST STRUCTURE VERIFICATION"

log_info "Checking test directories..."
TEST_DIRS=(
    "tests"
    "tests/e2e"
    "tests/integration"
    "tests/benchmarks"
    "tests/performance"
)

for dir in "${TEST_DIRS[@]}"; do
    if [ -d "$dir" ]; then
        # Count test files
        test_count=$(find "$dir" -name "*_test.go" -type f 2>/dev/null | wc -l || echo 0)
        log_success "$dir exists ($test_count test files)"
    else
        log_warning "$dir missing"
    fi
done

# ==============================================================================
# SECTION 9: Data Integrity Verification
# ==============================================================================
log_section "9. DATA INTEGRITY VERIFICATION"

log_info "Comparing with MVP to ensure no data loss..."

# Check if any unique Go files exist in MVP that aren't in restructured
if [ -d "$MVP_ROOT" ]; then
    unique_count=0
    log_info "Searching for unique files in MVP..."
    
    # Focus on implementation files, not test files
    for file in $(find "$MVP_ROOT" -name "*.go" -type f | grep -v "_test.go" | grep -v "/test"); do
        filename=$(basename "$file")
        relpath=${file#$MVP_ROOT/}
        
        # Check if this file exists anywhere in restructured
        if ! find "$PROJECT_ROOT" -name "$filename" -type f | grep -v backup >/dev/null 2>&1; then
            ((unique_count++))
            if [ $unique_count -le 5 ]; then
                log_warning "Unique file in MVP: $relpath"
            fi
        fi
    done
    
    if [ $unique_count -eq 0 ]; then
        log_success "No unique implementation files found in MVP"
    else
        log_warning "Found $unique_count potentially unique files in MVP"
    fi
else
    log_warning "MVP directory not found for comparison"
fi

# ==============================================================================
# SECTION 10: Final Summary
# ==============================================================================
log_section "VERIFICATION SUMMARY"

echo "" | tee -a "$VERIFICATION_LOG"
echo "═══════════════════════════════════════════" | tee -a "$VERIFICATION_LOG"
echo "TOTAL CHECKS: $TOTAL_CHECKS" | tee -a "$VERIFICATION_LOG"
echo -e "${GREEN}PASSED: $PASSED_CHECKS${NC}" | tee -a "$VERIFICATION_LOG"
echo -e "${YELLOW}WARNINGS: $WARNING_CHECKS${NC}" | tee -a "$VERIFICATION_LOG"
echo -e "${RED}FAILED: $FAILED_CHECKS${NC}" | tee -a "$VERIFICATION_LOG"
echo "═══════════════════════════════════════════" | tee -a "$VERIFICATION_LOG"

# Calculate success rate
if [ $TOTAL_CHECKS -gt 0 ]; then
    SUCCESS_RATE=$((PASSED_CHECKS * 100 / TOTAL_CHECKS))
    echo "SUCCESS RATE: ${SUCCESS_RATE}%" | tee -a "$VERIFICATION_LOG"
fi

echo "" | tee -a "$VERIFICATION_LOG"
echo "Verification completed: $(date)" | tee -a "$VERIFICATION_LOG"
echo "Full log saved to: $VERIFICATION_LOG" | tee -a "$VERIFICATION_LOG"

# Exit with appropriate code
if [ $FAILED_CHECKS -eq 0 ]; then
    echo -e "\n${GREEN}✅ VERIFICATION PASSED - Project is ready!${NC}"
    exit 0
else
    echo -e "\n${RED}❌ VERIFICATION FAILED - $FAILED_CHECKS critical issues found${NC}"
    exit 1
fi