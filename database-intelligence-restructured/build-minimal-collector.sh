#!/bin/bash

# Build Minimal Working Collector
# This creates a minimal collector that can be used for E2E testing

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
CYAN='\033[0;36m'
NC='\033[0m'

PROJECT_ROOT="/Users/deepaksharma/syc/db-otel/database-intelligence-restructured"
DIST_DIR="$PROJECT_ROOT/distributions/minimal-e2e"

cd "$PROJECT_ROOT"

echo -e "${BLUE}=== BUILDING MINIMAL E2E COLLECTOR ===${NC}"

# ==============================================================================
# Step 1: Create Distribution Directory
# ==============================================================================
echo -e "\n${CYAN}Step 1: Creating minimal distribution${NC}"

mkdir -p "$DIST_DIR"

# Create main.go
cat > "$DIST_DIR/main.go" << 'EOF'
package main

import (
    "log"
    "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter"
    "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver"
    "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver"
    "go.opentelemetry.io/collector/component"
    "go.opentelemetry.io/collector/exporter"
    "go.opentelemetry.io/collector/exporter/debugexporter"
    "go.opentelemetry.io/collector/extension"
    "go.opentelemetry.io/collector/otelcol"
    "go.opentelemetry.io/collector/processor"
    "go.opentelemetry.io/collector/processor/batchprocessor"
    "go.opentelemetry.io/collector/receiver"
)

func main() {
    factories, err := components()
    if err != nil {
        log.Fatalf("failed to build components: %v", err)
    }

    info := component.BuildInfo{
        Command:     "database-intelligence-minimal",
        Description: "Database Intelligence Minimal Collector",
        Version:     "0.1.0",
    }

    cfg := otelcol.CollectorSettings{
        BuildInfo: info,
        Factories: factories,
    }

    if err := run(otelcol.NewCommand(cfg)); err != nil {
        log.Fatal(err)
    }
}

func run(cmd *otelcol.Command) error {
    return cmd.Execute()
}

func components() (otelcol.Factories, error) {
    var err error
    factories := otelcol.Factories{}

    factories.Extensions, err = extension.MakeFactoryMap()
    if err != nil {
        return factories, err
    }

    factories.Receivers, err = receiver.MakeFactoryMap(
        postgresqlreceiver.NewFactory(),
        mysqlreceiver.NewFactory(),
    )
    if err != nil {
        return factories, err
    }

    factories.Processors, err = processor.MakeFactoryMap(
        batchprocessor.NewFactory(),
    )
    if err != nil {
        return factories, err
    }

    factories.Exporters, err = exporter.MakeFactoryMap(
        debugexporter.NewFactory(),
        fileexporter.NewFactory(),
    )
    if err != nil {
        return factories, err
    }

    return factories, nil
}
EOF

# Create go.mod
cat > "$DIST_DIR/go.mod" << 'EOF'
module github.com/database-intelligence/distributions/minimal-e2e

go 1.22

require (
    github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter v0.110.0
    github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver v0.110.0
    github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver v0.110.0
    go.opentelemetry.io/collector/component v0.110.0
    go.opentelemetry.io/collector/exporter v0.110.0
    go.opentelemetry.io/collector/exporter/debugexporter v0.110.0
    go.opentelemetry.io/collector/extension v0.110.0
    go.opentelemetry.io/collector/otelcol v0.110.0
    go.opentelemetry.io/collector/processor v0.110.0
    go.opentelemetry.io/collector/processor/batchprocessor v0.110.0
    go.opentelemetry.io/collector/receiver v0.110.0
)
EOF

echo -e "${GREEN}[✓]${NC} Created minimal distribution"

# ==============================================================================
# Step 2: Add to workspace
# ==============================================================================
echo -e "\n${CYAN}Step 2: Adding to workspace${NC}"

go work use "$DIST_DIR"
echo -e "${GREEN}[✓]${NC} Added to go.work"

# ==============================================================================
# Step 3: Build the collector
# ==============================================================================
echo -e "\n${CYAN}Step 3: Building collector${NC}"

cd "$DIST_DIR"

# Download dependencies
echo -e "${YELLOW}Downloading dependencies...${NC}"
go mod tidy

# Build
echo -e "${YELLOW}Building collector...${NC}"
if go build -o database-intelligence-minimal .; then
    echo -e "${GREEN}[✓]${NC} Collector built successfully"
    
    # Copy to project root
    cp database-intelligence-minimal "$PROJECT_ROOT/minimal-collector"
    chmod +x "$PROJECT_ROOT/minimal-collector"
    echo -e "${GREEN}[✓]${NC} Collector copied to project root"
else
    echo -e "${RED}[✗]${NC} Build failed"
    exit 1
fi

cd "$PROJECT_ROOT"

# ==============================================================================
# Step 4: Create test configuration
# ==============================================================================
echo -e "\n${CYAN}Step 4: Creating test configuration${NC}"

cat > "$PROJECT_ROOT/minimal-collector-config.yaml" << 'EOF'
receivers:
  postgresql:
    endpoint: localhost:5432
    username: postgres
    password: password
    databases:
      - testdb
    collection_interval: 10s
    tls:
      insecure: true

  mysql:
    endpoint: localhost:3306
    username: root
    password: password
    database: testdb
    collection_interval: 10s

processors:
  batch:
    timeout: 10s

exporters:
  debug:
    verbosity: detailed
    sampling_initial: 5
    sampling_thereafter: 100

  file:
    path: ./metrics-output.json

service:
  pipelines:
    metrics:
      receivers: [postgresql, mysql]
      processors: [batch]
      exporters: [debug, file]

  telemetry:
    logs:
      level: debug
EOF

echo -e "${GREEN}[✓]${NC} Created test configuration"

# ==============================================================================
# Summary
# ==============================================================================
echo -e "\n${CYAN}=== BUILD COMPLETE ===${NC}"
echo -e "${GREEN}Successfully built minimal E2E collector!${NC}"
echo ""
echo "Files created:"
echo "- Collector binary: ./minimal-collector"
echo "- Configuration: ./minimal-collector-config.yaml"
echo ""
echo "To test the collector:"
echo "  ./minimal-collector --config=minimal-collector-config.yaml"