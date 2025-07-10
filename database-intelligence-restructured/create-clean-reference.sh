#!/bin/bash

# Create Clean Reference Distribution
# This creates a clean collector distribution from scratch for comparison

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
CYAN='\033[0;36m'
NC='\033[0m'

PROJECT_ROOT="/Users/deepaksharma/syc/db-otel/database-intelligence-restructured"
REFERENCE_DIR="$PROJECT_ROOT/reference-distribution"

cd "$PROJECT_ROOT"

echo -e "${BLUE}=== CREATING CLEAN REFERENCE DISTRIBUTION ===${NC}"

# ==============================================================================
# Step 1: Create reference directory
# ==============================================================================
echo -e "\n${CYAN}Step 1: Creating reference directory${NC}"

mkdir -p "$REFERENCE_DIR"
cd "$REFERENCE_DIR"

# ==============================================================================
# Step 2: Initialize go module with latest versions
# ==============================================================================
echo -e "\n${CYAN}Step 2: Initializing Go module${NC}"

go mod init github.com/database-intelligence/reference-distribution

# ==============================================================================
# Step 3: Create main.go with standard pattern
# ==============================================================================
echo -e "\n${CYAN}Step 3: Creating main.go with standard pattern${NC}"

cat > main.go << 'EOF'
package main

import (
	"log"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/otelcol"
)

func main() {
	info := component.BuildInfo{
		Command:     "database-intelligence-collector",
		Description: "Database Intelligence Collector",
		Version:     "0.1.0",
	}

	set := otelcol.CollectorSettings{
		BuildInfo: info,
		Factories: otelcol.Factories{},
	}

	cmd := otelcol.NewCommand(set)
	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
EOF

# ==============================================================================
# Step 4: Add dependencies one by one and check versions
# ==============================================================================
echo -e "\n${CYAN}Step 4: Adding OpenTelemetry dependencies${NC}"

# Get latest versions
echo -e "\n${YELLOW}Getting latest OpenTelemetry versions...${NC}"
go get go.opentelemetry.io/collector/component@latest
go get go.opentelemetry.io/collector/otelcol@latest

# Check what versions were pulled
echo -e "\n${YELLOW}Versions pulled:${NC}"
go list -m all | grep "go.opentelemetry.io/collector" | head -10

# ==============================================================================
# Step 5: Create a processor example
# ==============================================================================
echo -e "\n${CYAN}Step 5: Creating example processor pattern${NC}"

mkdir -p example-processor
cat > example-processor/factory.go << 'EOF'
package exampleprocessor

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
)

const (
	typeStr = "example"
)

func NewFactory() processor.Factory {
	return processor.NewFactory(
		component.MustNewType(typeStr),
		createDefaultConfig,
		processor.WithMetrics(createMetricsProcessor, component.StabilityLevelBeta),
	)
}

func createDefaultConfig() component.Config {
	return &Config{}
}

func createMetricsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (processor.Metrics, error) {
	return nil, nil
}
EOF

cat > example-processor/config.go << 'EOF'
package exampleprocessor

// Config represents the receiver config settings
type Config struct{}

// Validate checks if the processor configuration is valid
func (cfg *Config) Validate() error {
	return nil
}
EOF

# ==============================================================================
# Step 6: Compare with current implementations
# ==============================================================================
echo -e "\n${CYAN}Step 6: Analyzing current implementations${NC}"

cd "$PROJECT_ROOT"

# Create comparison report
cat > version-comparison-report.md << 'EOF'
# Version Comparison Report

## Clean Reference Distribution

### Latest OpenTelemetry Versions (as of analysis)
EOF

# Add versions from reference
echo -e "\n\`\`\`" >> version-comparison-report.md
cd "$REFERENCE_DIR" && go list -m all | grep "go.opentelemetry.io/collector" | sort >> "$PROJECT_ROOT/version-comparison-report.md"
echo -e "\`\`\`\n" >> "$PROJECT_ROOT/version-comparison-report.md"

cd "$PROJECT_ROOT"

# Add current implementation analysis
cat >> version-comparison-report.md << 'EOF'
## Current Implementation Issues

### 1. Core Module (core/go.mod)
- Uses v1.35.0 for component, receiver, extension
- Uses v0.129.0 for specific implementations (otlpreceiver, memorylimiterprocessor)
- **Issue**: Mixed versions between base packages and implementations

### 2. Production Distribution
- Uses v0.105.0 throughout
- **Issue**: Older version, not aligned with core module

### 3. Processors/Receivers
- Most use v0.110.0 with confmap v1.16.0
- **Issue**: Version mismatch with core module

## Key Differences from Clean Implementation

1. **Module Path Inconsistency**
   - Current: `github.com/database-intelligence-restructured/`
   - Should be: `github.com/database-intelligence/` (without -restructured)

2. **Version Alignment**
   - Clean reference uses consistent latest versions
   - Current implementation has 3 different version sets (v0.105.0, v0.110.0, v1.35.0)

3. **Import Structure**
   - Clean reference: Direct imports, no confmap needed in business logic
   - Current: Some modules import confmap directly (shouldn't be needed)

## Recommended Fixes

### Fix 1: Align Module Paths
All modules should use consistent module paths without "-restructured"

### Fix 2: Version Alignment Strategy
Choose one approach:
- **Option A**: Use v0.105.0 everywhere (stable, older)
- **Option B**: Use v0.110.0 + v1.16.0 for special modules
- **Option C**: Update to latest (v1.35.0 + v0.129.0 pattern)

### Fix 3: Remove Direct confmap Imports
Processors and receivers shouldn't import confmap directly. Use component.Config interface.

### Fix 4: Consistent Replace Directives
Use relative paths consistently in replace directives.
EOF

echo -e "${GREEN}[✓]${NC} Comparison report created"

# ==============================================================================
# Step 7: Create fix verification script
# ==============================================================================
echo -e "\n${CYAN}Step 7: Creating verification script${NC}"

cat > verify-fixes.sh << 'SCRIPT'
#!/bin/bash

# Verify fixes in current implementation

echo "=== MODULE PATH CHECK ==="
find . -name "go.mod" -exec grep -H "^module" {} \; | grep -v reference-distribution | head -10

echo -e "\n=== VERSION CONSISTENCY CHECK ==="
echo "Checking for mixed versions..."
for mod in processors/adaptivesampler core distributions/production; do
    if [ -f "$mod/go.mod" ]; then
        echo -e "\n$mod:"
        grep -E "go.opentelemetry.io/collector/[^ ]+ v" "$mod/go.mod" | head -5
    fi
done

echo -e "\n=== DIRECT CONFMAP IMPORTS CHECK ==="
echo "Checking for direct confmap imports in business logic..."
find processors receivers -name "*.go" -type f | xargs grep -l "confmap" | grep -v test | head -10
SCRIPT

chmod +x verify-fixes.sh

# ==============================================================================
# Summary
# ==============================================================================
echo -e "\n${CYAN}=== ANALYSIS COMPLETE ===${NC}"
echo -e "\nCreated:"
echo -e "- ${GREEN}reference-distribution/${NC} - Clean implementation example"
echo -e "- ${GREEN}version-comparison-report.md${NC} - Detailed comparison"
echo -e "- ${GREEN}verify-fixes.sh${NC} - Script to verify fixes"

echo -e "\n${YELLOW}Key Issues Found:${NC}"
echo "1. Module paths have '-restructured' suffix (should be removed)"
echo "2. Three different version sets in use (v0.105.0, v0.110.0, v1.35.0)"
echo "3. Version mismatch between core and distributions"
echo "4. Some modules may have direct confmap imports"

echo -e "\n${YELLOW}Next Steps:${NC}"
echo "1. Review version-comparison-report.md"
echo "2. Run ./verify-fixes.sh to see current state"
echo "3. Decide on version alignment strategy"
echo "4. Fix module paths and versions accordingly"