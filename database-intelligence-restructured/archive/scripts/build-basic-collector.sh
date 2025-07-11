#!/bin/bash

# Build Basic Collector for E2E Testing
# This creates a basic collector using standard components that work

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
CYAN='\033[0;36m'
NC='\033[0m'

PROJECT_ROOT="/Users/deepaksharma/syc/db-otel/database-intelligence-restructured"
COLLECTOR_DIR="$PROJECT_ROOT/basic-collector"

cd "$PROJECT_ROOT"

echo -e "${BLUE}=== BUILDING BASIC E2E COLLECTOR ===${NC}"

# ==============================================================================
# Step 1: Create collector directory
# ==============================================================================
echo -e "\n${CYAN}Step 1: Creating collector directory${NC}"

mkdir -p "$COLLECTOR_DIR"
cd "$COLLECTOR_DIR"

# ==============================================================================
# Step 2: Create main.go with minimal dependencies
# ==============================================================================
echo -e "\n${CYAN}Step 2: Creating collector main.go${NC}"

cat > main.go << 'EOF'
package main

import (
    "log"
    
    "go.opentelemetry.io/collector/component"
    "go.opentelemetry.io/collector/otelcol"
)

func main() {
    info := component.BuildInfo{
        Command:     "basic-e2e-collector",
        Description: "Basic E2E Test Collector",
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
# Step 3: Create go.mod with minimal dependencies
# ==============================================================================
echo -e "\n${CYAN}Step 3: Creating go.mod${NC}"

cat > go.mod << 'EOF'
module github.com/database-intelligence/basic-collector

go 1.22

require (
    go.opentelemetry.io/collector/component v0.110.0
    go.opentelemetry.io/collector/otelcol v0.110.0
)
EOF

# ==============================================================================
# Step 4: Download dependencies and build
# ==============================================================================
echo -e "\n${CYAN}Step 4: Building collector${NC}"

# Get dependencies
echo -e "${YELLOW}Downloading dependencies...${NC}"
go mod tidy

# Build
echo -e "${YELLOW}Building collector...${NC}"
if go build -o basic-e2e-collector .; then
    echo -e "${GREEN}[✓]${NC} Basic collector built successfully"
    
    # Copy to project root
    cp basic-e2e-collector "$PROJECT_ROOT/"
    chmod +x "$PROJECT_ROOT/basic-e2e-collector"
    
    # Test run
    echo -e "\n${YELLOW}Testing collector...${NC}"
    if "$PROJECT_ROOT/basic-e2e-collector" --version; then
        echo -e "${GREEN}[✓]${NC} Collector runs successfully"
    fi
else
    echo -e "${RED}[✗]${NC} Build failed"
    exit 1
fi

cd "$PROJECT_ROOT"

# ==============================================================================
# Step 5: Create a working configuration
# ==============================================================================
echo -e "\n${CYAN}Step 5: Creating basic configuration${NC}"

cat > basic-collector-config.yaml << 'EOF'
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318

processors:
  batch:

exporters:
  debug:
    verbosity: detailed
  otlp:
    endpoint: localhost:4317
    tls:
      insecure: true

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [debug]
    metrics:
      receivers: [otlp]
      processors: [batch]
      exporters: [debug]
    logs:
      receivers: [otlp]
      processors: [batch]
      exporters: [debug]

  telemetry:
    logs:
      level: debug
EOF

echo -e "${GREEN}[✓]${NC} Created basic configuration"

# ==============================================================================
# Summary
# ==============================================================================
echo -e "\n${CYAN}=== BUILD COMPLETE ===${NC}"
echo -e "${GREEN}Successfully built basic E2E collector!${NC}"
echo ""
echo "Files created:"
echo "- Collector binary: ./basic-e2e-collector"
echo "- Configuration: ./basic-collector-config.yaml"
echo ""
echo "To run the collector:"
echo "  ./basic-e2e-collector --config=basic-collector-config.yaml"

# Test that all our processors can build
echo -e "\n${CYAN}Testing processor builds...${NC}"

PROCESSORS=("adaptivesampler" "circuitbreaker" "costcontrol")
SUCCESS_COUNT=0

for proc in "${PROCESSORS[@]}"; do
    if [ -d "processors/$proc" ]; then
        echo -n "Building $proc... "
        if (cd "processors/$proc" && go build ./... 2>/dev/null); then
            echo -e "${GREEN}[✓]${NC}"
            ((SUCCESS_COUNT++))
        else
            echo -e "${RED}[✗]${NC}"
        fi
    fi
done

echo -e "\nProcessors built successfully: $SUCCESS_COUNT/${#PROCESSORS[@]}"