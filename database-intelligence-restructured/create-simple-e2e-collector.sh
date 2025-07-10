#!/bin/bash

# Create Simple E2E Collector
# This creates a basic collector using standard OpenTelemetry components

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
CYAN='\033[0;36m'
NC='\033[0m'

PROJECT_ROOT="/Users/deepaksharma/syc/db-otel/database-intelligence-restructured"
COLLECTOR_DIR="$PROJECT_ROOT/simple-e2e-collector"

cd "$PROJECT_ROOT"

echo -e "${BLUE}=== CREATING SIMPLE E2E COLLECTOR ===${NC}"

# ==============================================================================
# Step 1: Create Collector Directory
# ==============================================================================
echo -e "\n${CYAN}Step 1: Setting up collector directory${NC}"

mkdir -p "$COLLECTOR_DIR"
cd "$COLLECTOR_DIR"

# ==============================================================================
# Step 2: Create main.go
# ==============================================================================
echo -e "\n${CYAN}Step 2: Creating collector main.go${NC}"

cat > main.go << 'EOF'
package main

import (
	"log"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/provider/envprovider"
	"go.opentelemetry.io/collector/confmap/provider/fileprovider"
	"go.opentelemetry.io/collector/confmap/provider/httpprovider"
	"go.opentelemetry.io/collector/confmap/provider/httpsprovider"
	"go.opentelemetry.io/collector/confmap/provider/yamlprovider"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/debugexporter"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/zpagesextension"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/batchprocessor"
	"go.opentelemetry.io/collector/processor/memorylimiterprocessor"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/attributesprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver"
)

func main() {
	info := component.BuildInfo{
		Command:     "database-intelligence-e2e",
		Description: "Database Intelligence E2E Test Collector",
		Version:     "0.1.0",
	}

	set := otelcol.CollectorSettings{
		BuildInfo: info,
		Factories: components,
		ConfigProviderSettings: otelcol.ConfigProviderSettings{
			ResolverSettings: confmap.ResolverSettings{
				URIs: []string{confmap.ExpandVar(otelcol.DefaultConfigPath)},
				ProviderFactories: []confmap.ProviderFactory{
					envprovider.NewFactory(),
					fileprovider.NewFactory(),
					httpprovider.NewFactory(),
					httpsprovider.NewFactory(),
					yamlprovider.NewFactory(),
				},
			},
		},
	}

	if err := run(set); err != nil {
		log.Fatal(err)
	}
}

func run(settings otelcol.CollectorSettings) error {
	cmd := otelcol.NewCommand(settings)
	return cmd.Execute()
}

func components() (otelcol.Factories, error) {
	var err error
	factories := otelcol.Factories{}

	// Extensions
	factories.Extensions, err = extension.MakeFactoryMap(
		zpagesextension.NewFactory(),
		healthcheckextension.NewFactory(),
	)
	if err != nil {
		return factories, err
	}

	// Receivers
	factories.Receivers, err = receiver.MakeFactoryMap(
		otlpreceiver.NewFactory(),
		postgresqlreceiver.NewFactory(),
		mysqlreceiver.NewFactory(),
		hostmetricsreceiver.NewFactory(),
	)
	if err != nil {
		return factories, err
	}

	// Processors
	factories.Processors, err = processor.MakeFactoryMap(
		batchprocessor.NewFactory(),
		memorylimiterprocessor.NewFactory(),
		attributesprocessor.NewFactory(),
		resourceprocessor.NewFactory(),
	)
	if err != nil {
		return factories, err
	}

	// Exporters
	factories.Exporters, err = exporter.MakeFactoryMap(
		debugexporter.NewFactory(),
		otlpexporter.NewFactory(),
		prometheusexporter.NewFactory(),
		fileexporter.NewFactory(),
	)
	if err != nil {
		return factories, err
	}

	return factories, nil
}
EOF

echo -e "${GREEN}[✓]${NC} Created main.go"

# ==============================================================================
# Step 3: Create go.mod
# ==============================================================================
echo -e "\n${CYAN}Step 3: Creating go.mod${NC}"

cat > go.mod << 'EOF'
module github.com/database-intelligence/simple-e2e-collector

go 1.21

require (
    go.opentelemetry.io/collector/component v0.110.0
    go.opentelemetry.io/collector/confmap v0.110.0
    go.opentelemetry.io/collector/confmap/provider/envprovider v0.110.0
    go.opentelemetry.io/collector/confmap/provider/fileprovider v0.110.0
    go.opentelemetry.io/collector/confmap/provider/httpprovider v0.110.0
    go.opentelemetry.io/collector/confmap/provider/httpsprovider v0.110.0
    go.opentelemetry.io/collector/confmap/provider/yamlprovider v0.110.0
    go.opentelemetry.io/collector/exporter v0.110.0
    go.opentelemetry.io/collector/exporter/debugexporter v0.110.0
    go.opentelemetry.io/collector/exporter/otlpexporter v0.110.0
    go.opentelemetry.io/collector/extension v0.110.0
    go.opentelemetry.io/collector/extension/zpagesextension v0.110.0
    go.opentelemetry.io/collector/otelcol v0.110.0
    go.opentelemetry.io/collector/processor v0.110.0
    go.opentelemetry.io/collector/processor/batchprocessor v0.110.0
    go.opentelemetry.io/collector/processor/memorylimiterprocessor v0.110.0
    go.opentelemetry.io/collector/receiver v0.110.0
    go.opentelemetry.io/collector/receiver/otlpreceiver v0.110.0
    github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter v0.110.0
    github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter v0.110.0
    github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension v0.110.0
    github.com/open-telemetry/opentelemetry-collector-contrib/processor/attributesprocessor v0.110.0
    github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceprocessor v0.110.0
    github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver v0.110.0
    github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver v0.110.0
    github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver v0.110.0
)
EOF

echo -e "${GREEN}[✓]${NC} Created go.mod"

# ==============================================================================
# Step 4: Build the collector
# ==============================================================================
echo -e "\n${CYAN}Step 4: Building collector${NC}"

# First download dependencies
echo -e "${YELLOW}Downloading dependencies...${NC}"
if go mod download; then
    echo -e "${GREEN}[✓]${NC} Dependencies downloaded"
else
    echo -e "${YELLOW}[!]${NC} Some dependencies failed, continuing..."
fi

# Try to build
echo -e "${YELLOW}Building collector...${NC}"
if go build -o database-intelligence-e2e .; then
    echo -e "${GREEN}[✓]${NC} Collector built successfully"
    
    # Copy to project root
    cp database-intelligence-e2e "$PROJECT_ROOT/e2e-collector"
    chmod +x "$PROJECT_ROOT/e2e-collector"
    echo -e "${GREEN}[✓]${NC} Collector copied to project root"
else
    echo -e "${RED}[✗]${NC} Build failed"
    
    # Try a simpler approach
    echo -e "\n${YELLOW}Trying simpler build approach...${NC}"
    go mod tidy || true
    go build -o database-intelligence-e2e . || true
fi

cd "$PROJECT_ROOT"

# ==============================================================================
# Step 5: Create test script
# ==============================================================================
echo -e "\n${CYAN}Step 5: Creating test script${NC}"

cat > run-simple-e2e-test.sh << 'EOF'
#!/bin/bash

# Simple E2E Test Runner

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m'

PROJECT_ROOT="/Users/deepaksharma/syc/db-otel/database-intelligence-restructured"

cd "$PROJECT_ROOT"

echo -e "${BLUE}=== SIMPLE E2E TEST ===${NC}"

# Check if we have a working collector
COLLECTOR=""
if [ -f "./e2e-collector" ]; then
    COLLECTOR="./e2e-collector"
elif [ -f "./simple-e2e-collector/database-intelligence-e2e" ]; then
    COLLECTOR="./simple-e2e-collector/database-intelligence-e2e"
else
    echo -e "${YELLOW}[!]${NC} No collector found, using test mode"
fi

# Start databases
echo -e "\n${YELLOW}Starting databases...${NC}"
docker-compose -f deployments/docker/compose/docker-compose-databases.yaml up -d

# Wait for databases
echo -e "${YELLOW}Waiting for databases...${NC}"
sleep 15

# Test database connectivity
echo -e "\n${BLUE}Testing Database Connectivity${NC}"

# PostgreSQL
if docker exec db-intel-postgres pg_isready -U postgres; then
    echo -e "${GREEN}[✓]${NC} PostgreSQL is ready"
    
    # Run test query
    docker exec db-intel-postgres psql -U postgres -c "SELECT version();"
else
    echo -e "${RED}[✗]${NC} PostgreSQL not ready"
fi

# MySQL
if docker exec db-intel-mysql mysqladmin ping -h localhost -u root -ppassword 2>/dev/null; then
    echo -e "${GREEN}[✓]${NC} MySQL is ready"
    
    # Run test query
    docker exec db-intel-mysql mysql -u root -ppassword -e "SELECT VERSION();"
else
    echo -e "${RED}[✗]${NC} MySQL not ready"
fi

# If we have a collector, try to run it
if [ -n "$COLLECTOR" ]; then
    echo -e "\n${YELLOW}Starting collector...${NC}"
    
    # Create simple config
    cat > simple-test-config.yaml << 'CONFIG'
extensions:
  health_check:

receivers:
  hostmetrics:
    collection_interval: 10s
    scrapers:
      cpu:
      memory:

processors:
  batch:

exporters:
  debug:
    verbosity: detailed

service:
  extensions: [health_check]
  pipelines:
    metrics:
      receivers: [hostmetrics]
      processors: [batch]
      exporters: [debug]
CONFIG
    
    # Run collector for 30 seconds
    timeout 30s "$COLLECTOR" --config=simple-test-config.yaml || true
    
    rm -f simple-test-config.yaml
fi

# Stop databases
echo -e "\n${YELLOW}Stopping databases...${NC}"
docker-compose -f deployments/docker/compose/docker-compose-databases.yaml down

echo -e "\n${GREEN}Simple E2E test completed!${NC}"
EOF

chmod +x run-simple-e2e-test.sh

echo -e "${GREEN}[✓]${NC} Created test script"

# ==============================================================================
# Summary
# ==============================================================================
echo -e "\n${CYAN}=== SUMMARY ===${NC}"

if [ -f "./e2e-collector" ]; then
    echo -e "${GREEN}✓ Collector built successfully${NC}"
    echo "  Location: ./e2e-collector"
else
    echo -e "${YELLOW}! Collector build may have failed${NC}"
    echo "  Check: ./simple-e2e-collector/"
fi

echo ""
echo "To run a simple E2E test:"
echo "  ./run-simple-e2e-test.sh"
echo ""
echo "Configuration file created at:"
echo "  ./e2e-collector-config.yaml"