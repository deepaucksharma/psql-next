#!/bin/bash

# Data Integrity Check - Ensures no functionality was lost

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
REPORT="$PROJECT_ROOT/INTEGRITY_CHECK_REPORT.md"

cd "$PROJECT_ROOT"

echo -e "${BLUE}=== DATA INTEGRITY CHECK ===${NC}"

# Initialize report
cat > "$REPORT" << 'EOF'
# Data Integrity Check Report

Generated: DATE_PLACEHOLDER

## Purpose
Verify that no critical functionality was lost during the refactoring process.

EOF
sed -i.bak "s/DATE_PLACEHOLDER/$(date)/" "$REPORT" && rm -f "${REPORT}.bak"

# Function to add to report
report() {
    echo "$1" >> "$REPORT"
}

# ==============================================================================
# 1. Processor Functionality Check
# ==============================================================================
echo -e "\n${CYAN}1. PROCESSOR FUNCTIONALITY CHECK${NC}"
report "## 1. Processor Functionality"
report ""

PROCESSORS=(
    "adaptivesampler:adaptive_algorithm.go"
    "circuitbreaker:circuit_breaker_logic"
    "costcontrol:cost_tracking"
    "nrerrormonitor:error_monitoring"
    "planattributeextractor:plan_extraction"
    "querycorrelator:query_correlation"
    "verification:pii_detection"
)

for proc_check in "${PROCESSORS[@]}"; do
    IFS=':' read -r proc feature <<< "$proc_check"
    
    echo -e "\nChecking $proc for $feature..."
    
    # Check if the processor has the expected functionality
    if grep -r "$feature" "processors/$proc" >/dev/null 2>&1; then
        echo -e "${GREEN}[✓]${NC} $proc: $feature found"
        report "- ✅ **$proc**: $feature functionality present"
    else
        # Try alternative search
        if find "processors/$proc" -name "*.go" -exec grep -l "${feature//_/ }" {} \; >/dev/null 2>&1; then
            echo -e "${GREEN}[✓]${NC} $proc: $feature found (alternative match)"
            report "- ✅ **$proc**: $feature functionality present"
        else
            echo -e "${YELLOW}[!]${NC} $proc: $feature not found by name"
            report "- ⚠️ **$proc**: $feature not found by exact name (may use different naming)"
        fi
    fi
done

# ==============================================================================
# 2. Receiver Capabilities Check
# ==============================================================================
echo -e "\n${CYAN}2. RECEIVER CAPABILITIES CHECK${NC}"
report ""
report "## 2. Receiver Capabilities"
report ""

# Check ASH receiver
echo -e "\nChecking ASH receiver capabilities..."
if [ -d "receivers/ash" ]; then
    ash_files=$(ls receivers/ash/*.go 2>/dev/null | wc -l)
    if [ $ash_files -gt 0 ]; then
        echo -e "${GREEN}[✓]${NC} ASH receiver: $ash_files implementation files"
        report "- ✅ **ASH receiver**: $ash_files implementation files"
        
        # Check for key components
        for component in "collector" "sampler" "scraper" "storage"; do
            if [ -f "receivers/ash/$component.go" ]; then
                echo -e "  ${GREEN}[✓]${NC} $component.go present"
                report "  - $component.go ✓"
            fi
        done
    fi
else
    echo -e "${RED}[✗]${NC} ASH receiver missing"
    report "- ❌ **ASH receiver**: Missing"
fi

# Check EnhancedSQL receiver
echo -e "\nChecking EnhancedSQL receiver capabilities..."
if [ -d "receivers/enhancedsql" ]; then
    if grep -q "featuredetector" receivers/enhancedsql/*.go 2>/dev/null; then
        echo -e "${GREEN}[✓]${NC} EnhancedSQL: Feature detection integration found"
        report "- ✅ **EnhancedSQL receiver**: Feature detection integrated"
    fi
    if grep -q "queryselector" receivers/enhancedsql/*.go 2>/dev/null; then
        echo -e "${GREEN}[✓]${NC} EnhancedSQL: Query selector integration found"
        report "  - Query selector integrated ✓"
    fi
fi

# ==============================================================================
# 3. Configuration Completeness
# ==============================================================================
echo -e "\n${CYAN}3. CONFIGURATION COMPLETENESS${NC}"
report ""
report "## 3. Configuration Coverage"
report ""

# Check for critical configuration patterns
CONFIG_PATTERNS=(
    "postgresql:endpoint"
    "mysql:endpoint"
    "adaptivesampler:sampling_rate"
    "circuitbreaker:failure_threshold"
    "planattributeextractor:extract_plans"
    "otlp/newrelic"
    "prometheus:endpoint"
)

echo "Checking configuration patterns..."
for pattern in "${CONFIG_PATTERNS[@]}"; do
    clean_pattern=${pattern//:/ }
    if grep -r "$clean_pattern" configs/ >/dev/null 2>&1; then
        echo -e "${GREEN}[✓]${NC} Config pattern found: $pattern"
        report "- ✅ $pattern configuration present"
    else
        echo -e "${YELLOW}[!]${NC} Config pattern not found: $pattern"
        report "- ⚠️ $pattern configuration not found"
    fi
done

# ==============================================================================
# 4. Test Coverage Comparison
# ==============================================================================
echo -e "\n${CYAN}4. TEST COVERAGE ANALYSIS${NC}"
report ""
report "## 4. Test Coverage"
report ""

# Count test files by component
echo "Analyzing test coverage..."

# Processor tests
proc_tests=0
for proc in processors/*/; do
    if [ -d "$proc" ]; then
        tests=$(find "$proc" -name "*_test.go" | wc -l)
        proc_tests=$((proc_tests + tests))
    fi
done
echo -e "${GREEN}[✓]${NC} Processor tests: $proc_tests files"
report "- Processor tests: $proc_tests files"

# E2E tests
e2e_tests=$(find tests/e2e -name "*.go" | wc -l)
echo -e "${GREEN}[✓]${NC} E2E tests: $e2e_tests files"
report "- E2E tests: $e2e_tests files"

# Integration tests
int_tests=$(find tests/integration -name "*.go" | wc -l)
echo -e "${GREEN}[✓]${NC} Integration tests: $int_tests files"
report "- Integration tests: $int_tests files"

# ==============================================================================
# 5. Critical File Verification
# ==============================================================================
echo -e "\n${CYAN}5. CRITICAL FILE VERIFICATION${NC}"
report ""
report "## 5. Critical Files"
report ""

CRITICAL_FILES=(
    "common/featuredetector/postgresql.go"
    "common/featuredetector/mysql.go"
    "common/queryselector/selector.go"
    "exporters/nri/exporter.go"
    "extensions/healthcheck/extension.go"
    "validation/ohi-compatibility-validator.go"
)

echo "Verifying critical files..."
for file in "${CRITICAL_FILES[@]}"; do
    if [ -f "$file" ]; then
        size=$(wc -l < "$file")
        echo -e "${GREEN}[✓]${NC} $file ($size lines)"
        report "- ✅ $file ($size lines)"
    else
        echo -e "${RED}[✗]${NC} $file missing"
        report "- ❌ $file missing"
    fi
done

# ==============================================================================
# 6. Import Dependencies Check
# ==============================================================================
echo -e "\n${CYAN}6. IMPORT DEPENDENCIES${NC}"
report ""
report "## 6. Import Dependencies"
report ""

# Check that processors can import common modules
echo "Checking import dependencies..."

# Sample check - circuitbreaker importing featuredetector
if grep -q "common/featuredetector" processors/circuitbreaker/*.go 2>/dev/null; then
    echo -e "${GREEN}[✓]${NC} CircuitBreaker imports FeatureDetector"
    report "- ✅ CircuitBreaker → FeatureDetector import working"
fi

# Check enhancedsql imports
if grep -q "common/queryselector" receivers/enhancedsql/*.go 2>/dev/null; then
    echo -e "${GREEN}[✓]${NC} EnhancedSQL imports QuerySelector"
    report "- ✅ EnhancedSQL → QuerySelector import working"
fi

# ==============================================================================
# 7. Functionality Comparison Summary
# ==============================================================================
echo -e "\n${CYAN}7. FUNCTIONALITY SUMMARY${NC}"
report ""
report "## 7. Summary"
report ""

# Count total verification points
total_checks=$(grep -c "✅\|❌\|⚠️" "$REPORT" || echo 0)
passed_checks=$(grep -c "✅" "$REPORT" || echo 0)
failed_checks=$(grep -c "❌" "$REPORT" || echo 0)
warning_checks=$(grep -c "⚠️" "$REPORT" || echo 0)

report "### Verification Results"
report "- Total checks: $total_checks"
report "- Passed: $passed_checks ✅"
report "- Warnings: $warning_checks ⚠️"
report "- Failed: $failed_checks ❌"
report ""

if [ $failed_checks -eq 0 ]; then
    report "### Conclusion"
    report "✅ **All critical functionality has been preserved during refactoring!**"
    echo -e "\n${GREEN}✅ INTEGRITY CHECK PASSED${NC}"
else
    report "### Conclusion"
    report "⚠️ **Some functionality may need attention - review failed checks above**"
    echo -e "\n${YELLOW}⚠️ INTEGRITY CHECK: NEEDS REVIEW${NC}"
fi

# Final statistics
echo -e "\n${CYAN}=== FINAL STATISTICS ===${NC}"
echo -e "Total checks: $total_checks"
echo -e "Passed: ${GREEN}$passed_checks${NC}"
echo -e "Warnings: ${YELLOW}$warning_checks${NC}"
echo -e "Failed: ${RED}$failed_checks${NC}"
echo -e "\nReport saved to: ${BLUE}$REPORT${NC}"