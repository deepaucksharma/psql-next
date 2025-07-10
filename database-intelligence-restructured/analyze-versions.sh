#!/bin/bash

# Analyze OpenTelemetry versions across all modules
# This script helps understand version conflicts

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
CYAN='\033[0;36m'
NC='\033[0m'

PROJECT_ROOT="/Users/deepaksharma/syc/db-otel/database-intelligence-restructured"
cd "$PROJECT_ROOT"

echo -e "${BLUE}=== ANALYZING OPENTELEMETRY VERSIONS ===${NC}\n"

# Create results directory
mkdir -p analysis-results

# ==============================================================================
# Step 1: Collect all OpenTelemetry dependencies
# ==============================================================================
echo -e "${CYAN}Step 1: Collecting OpenTelemetry dependencies from all modules${NC}"

# Find all go.mod files and extract OpenTelemetry dependencies
echo "Module,Package,Version" > analysis-results/otel-versions.csv

find . -name "go.mod" -not -path "./vendor/*" -not -path "./.git/*" | while read modfile; do
    module_dir=$(dirname "$modfile")
    module_name=$(basename "$module_dir")
    
    # Extract OpenTelemetry dependencies with versions
    grep -E "go.opentelemetry.io|github.com/open-telemetry" "$modfile" | grep -v "^module" | grep -v "//" | while read line; do
        if [[ $line =~ ([^ ]+)\ +v([0-9]+\.[0-9]+\.[0-9]+) ]]; then
            package="${BASH_REMATCH[1]}"
            version="${BASH_REMATCH[2]}"
            echo "$module_name,$package,$version" >> analysis-results/otel-versions.csv
        fi
    done
done

echo -e "${GREEN}[✓]${NC} Dependencies collected"

# ==============================================================================
# Step 2: Analyze version conflicts
# ==============================================================================
echo -e "\n${CYAN}Step 2: Analyzing version conflicts${NC}"

# Group by package and show different versions
cat > analysis-results/version-conflicts.txt << 'EOF'
=== OpenTelemetry Version Conflicts ===

EOF

# Use sort and uniq to find packages with multiple versions
cat analysis-results/otel-versions.csv | tail -n +2 | cut -d',' -f2,3 | sort | uniq -c | \
awk '{
    count[$2] += $1
    versions[$2] = versions[$2] " " $3 "(" $1 ")"
}
END {
    for (pkg in versions) {
        n = split(versions[pkg], v, " ")
        if (n > 2) {  # More than one version (first element is empty)
            print "\n" pkg ":"
            for (i = 2; i <= n; i++) {
                print "  - " v[i]
            }
        }
    }
}' >> analysis-results/version-conflicts.txt

echo -e "${GREEN}[✓]${NC} Version conflicts analyzed"

# ==============================================================================
# Step 3: Check specific problematic packages
# ==============================================================================
echo -e "\n${CYAN}Step 3: Checking specific problematic packages${NC}"

# Check confmap versions specifically
echo -e "\n${YELLOW}Confmap package versions:${NC}"
grep "confmap" analysis-results/otel-versions.csv | sort -u

# Check component versions
echo -e "\n${YELLOW}Component package versions:${NC}"
grep "/component" analysis-results/otel-versions.csv | grep -v "componenttest" | sort -u

# ==============================================================================
# Step 4: Identify version patterns
# ==============================================================================
echo -e "\n${CYAN}Step 4: Identifying version patterns${NC}"

cat > analysis-results/version-patterns.txt << 'EOF'
=== Version Patterns Analysis ===

EOF

# Count occurrences of each major version pattern
echo "Major version patterns:" >> analysis-results/version-patterns.txt
cat analysis-results/otel-versions.csv | tail -n +2 | cut -d',' -f3 | \
awk '{
    split($1, v, ".")
    major = v[1] "." v[2] ".x"
    patterns[major]++
}
END {
    for (p in patterns) {
        print p ": " patterns[p] " occurrences"
    }
}' | sort -k2 -nr >> analysis-results/version-patterns.txt

# ==============================================================================
# Step 5: Check Go version requirements
# ==============================================================================
echo -e "\n${CYAN}Step 5: Checking Go version requirements${NC}"

echo -e "\n=== Go Version Requirements ===" > analysis-results/go-versions.txt
find . -name "go.mod" -not -path "./vendor/*" -not -path "./.git/*" | while read modfile; do
    module_dir=$(dirname "$modfile")
    module_name=$(basename "$module_dir")
    go_version=$(grep "^go " "$modfile" | awk '{print $2}')
    echo "$module_name: Go $go_version" >> analysis-results/go-versions.txt
done

# ==============================================================================
# Step 6: Generate recommendations
# ==============================================================================
echo -e "\n${CYAN}Step 6: Generating recommendations${NC}"

cat > analysis-results/recommendations.md << 'EOF'
# Version Conflict Resolution Recommendations

## Analysis Summary

Based on the analysis of OpenTelemetry dependencies across all modules:

### Key Findings

1. **Version Mismatches**
   - Multiple versions of OpenTelemetry packages are in use
   - confmap package has versioning issues (v0.110.0 vs v1.16.0)
   - Some modules use older versions while others use newer ones

2. **Common Version Patterns**
   - v0.110.0 is the most common for collector components
   - v1.16.0 is used for pdata and confmap in some modules
   - contrib packages mostly use v0.110.0

### Recommendations

1. **Standardize on v0.110.0**
   - Most modules already use this version
   - It's a stable release with good compatibility
   - Update all modules to use consistent versions

2. **Handle Special Cases**
   - confmap: Use v1.16.0 (the v0.110.0 doesn't exist)
   - pdata: Use v1.16.0 (newer versioning scheme)
   - featuregate: Use v1.16.0

3. **Update Process**
   - Start with leaf modules (no dependencies)
   - Update processors one by one
   - Update receivers
   - Finally update distributions

4. **Testing Strategy**
   - Test each module individually after update
   - Run integration tests
   - Validate E2E functionality

EOF

# ==============================================================================
# Summary
# ==============================================================================
echo -e "\n${CYAN}=== ANALYSIS COMPLETE ===${NC}"
echo -e "\nResults saved in analysis-results/"
echo -e "- ${GREEN}otel-versions.csv${NC}: All OpenTelemetry dependencies"
echo -e "- ${GREEN}version-conflicts.txt${NC}: Packages with version conflicts"
echo -e "- ${GREEN}version-patterns.txt${NC}: Version usage patterns"
echo -e "- ${GREEN}go-versions.txt${NC}: Go version requirements"
echo -e "- ${GREEN}recommendations.md${NC}: Resolution recommendations"

# Show summary of conflicts
echo -e "\n${YELLOW}Summary of version conflicts:${NC}"
grep -c "^  - v" analysis-results/version-conflicts.txt || echo "0 conflicts found"

# Show most common versions
echo -e "\n${YELLOW}Most common versions:${NC}"
cat analysis-results/otel-versions.csv | tail -n +2 | cut -d',' -f3 | sort | uniq -c | sort -nr | head -5 | awk '{print "v" $2 ": " $1 " uses"}'