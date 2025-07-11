#!/bin/bash

# Build Working E2E Collector using OpenTelemetry Builder
# This creates a fully functional collector for E2E testing

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
CYAN='\033[0;36m'
NC='\033[0m'

PROJECT_ROOT="/Users/deepaksharma/syc/db-otel/database-intelligence-restructured"
BUILD_DIR="$PROJECT_ROOT/e2e-collector-build"

cd "$PROJECT_ROOT"

echo -e "${BLUE}=== BUILDING WORKING E2E COLLECTOR ===${NC}"

# ==============================================================================
# Step 1: Install OpenTelemetry Collector Builder
# ==============================================================================
echo -e "\n${CYAN}Step 1: Installing OpenTelemetry Collector Builder${NC}"

BUILDER_VERSION="0.110.0"
BUILDER_PATH="$PROJECT_ROOT/bin/ocb"

mkdir -p "$PROJECT_ROOT/bin"

if [ ! -f "$BUILDER_PATH" ]; then
    echo -e "${YELLOW}Downloading OpenTelemetry Collector Builder...${NC}"
    
    # Detect OS and architecture
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)
    
    if [ "$ARCH" = "x86_64" ]; then
        ARCH="amd64"
    elif [ "$ARCH" = "aarch64" ] || [ "$ARCH" = "arm64" ]; then
        ARCH="arm64"
    fi
    
    DOWNLOAD_URL="https://github.com/open-telemetry/opentelemetry-collector/releases/download/cmd%2Fbuilder%2Fv${BUILDER_VERSION}/ocb_${BUILDER_VERSION}_${OS}_${ARCH}"
    
    if curl -L "$DOWNLOAD_URL" -o "$BUILDER_PATH"; then
        chmod +x "$BUILDER_PATH"
        echo -e "${GREEN}[✓]${NC} OpenTelemetry Collector Builder installed"
    else
        echo -e "${RED}[✗]${NC} Failed to download builder"
        exit 1
    fi
else
    echo -e "${GREEN}[✓]${NC} OpenTelemetry Collector Builder already installed"
fi

# ==============================================================================
# Step 2: Create E2E Builder Configuration
# ==============================================================================
echo -e "\n${CYAN}Step 2: Creating E2E Builder Configuration${NC}"

mkdir -p "$BUILD_DIR"

cat > "$BUILD_DIR/e2e-builder-config.yaml" << 'EOF'
dist:
  name: database-intelligence-e2e
  description: Database Intelligence E2E Test Collector
  output_path: ./
  otelcol_version: 0.110.0

extensions:
  - gomod: go.opentelemetry.io/collector/extension/zpagesextension v0.110.0
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension v0.110.0

receivers:
  - gomod: go.opentelemetry.io/collector/receiver/otlpreceiver v0.110.0
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver v0.110.0
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver v0.110.0
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver v0.110.0
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlqueryreceiver v0.110.0

processors:
  - gomod: go.opentelemetry.io/collector/processor/batchprocessor v0.110.0
  - gomod: go.opentelemetry.io/collector/processor/memorylimiterprocessor v0.110.0
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/processor/attributesprocessor v0.110.0
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceprocessor v0.110.0
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor v0.110.0
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor v0.110.0

exporters:
  - gomod: go.opentelemetry.io/collector/exporter/debugexporter v0.110.0
  - gomod: go.opentelemetry.io/collector/exporter/otlpexporter v0.110.0
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter v0.110.0
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter v0.110.0

connectors:
  - gomod: go.opentelemetry.io/collector/connector/forwardconnector v0.110.0
EOF

echo -e "${GREEN}[✓]${NC} Created E2E builder configuration"

# ==============================================================================
# Step 3: Build the Collector
# ==============================================================================
echo -e "\n${CYAN}Step 3: Building E2E Collector${NC}"

cd "$BUILD_DIR"

if "$BUILDER_PATH" --config=e2e-builder-config.yaml; then
    echo -e "${GREEN}[✓]${NC} E2E collector built successfully"
    
    # Make it executable
    chmod +x database-intelligence-e2e
    
    # Copy to project root
    cp database-intelligence-e2e "$PROJECT_ROOT/e2e-collector"
    echo -e "${GREEN}[✓]${NC} E2E collector copied to project root"
else
    echo -e "${RED}[✗]${NC} Failed to build E2E collector"
    exit 1
fi

cd "$PROJECT_ROOT"

# ==============================================================================
# Step 4: Create E2E Test Configuration
# ==============================================================================
echo -e "\n${CYAN}Step 4: Creating E2E Test Configuration${NC}"

cat > "$PROJECT_ROOT/e2e-collector-config.yaml" << 'EOF'
extensions:
  health_check:
    endpoint: 0.0.0.0:13133
  zpages:
    endpoint: 0.0.0.0:55679

receivers:
  postgresql:
    endpoint: ${env:POSTGRES_HOST:-localhost}:${env:POSTGRES_PORT:-5432}
    username: ${env:POSTGRES_USER:-postgres}
    password: ${env:POSTGRES_PASSWORD:-password}
    databases:
      - ${env:POSTGRES_DB:-testdb}
    collection_interval: 10s
    tls:
      insecure: true

  mysql:
    endpoint: ${env:MYSQL_HOST:-localhost}:${env:MYSQL_PORT:-3306}
    username: ${env:MYSQL_USER:-root}
    password: ${env:MYSQL_PASSWORD:-password}
    database: ${env:MYSQL_DATABASE:-testdb}
    collection_interval: 10s

  hostmetrics:
    collection_interval: 10s
    scrapers:
      cpu:
      memory:
      disk:
      network:

processors:
  batch:
    timeout: 10s
    send_batch_size: 1000

  resource:
    attributes:
      - key: service.name
        value: database-intelligence-e2e
        action: insert
      - key: environment
        value: e2e-test
        action: insert

  attributes:
    actions:
      - key: test.run.id
        value: ${env:TEST_RUN_ID:-default}
        action: insert

exporters:
  debug:
    verbosity: detailed
    sampling_initial: 5
    sampling_thereafter: 200

  file:
    path: /tmp/e2e-metrics.json
    rotation:
      enabled: true
      max_megabytes: 10
      max_days: 3
      max_backups: 3

  prometheus:
    endpoint: "0.0.0.0:8889"
    namespace: database_intelligence
    const_labels:
      environment: e2e

service:
  telemetry:
    logs:
      level: debug
      development: true
      encoding: console
    metrics:
      level: detailed
      address: 0.0.0.0:8888

  extensions: [health_check, zpages]
  
  pipelines:
    metrics:
      receivers: [postgresql, mysql, hostmetrics]
      processors: [batch, resource, attributes]
      exporters: [debug, file, prometheus]
EOF

echo -e "${GREEN}[✓]${NC} Created E2E test configuration"

# ==============================================================================
# Step 5: Create E2E Test Runner Script
# ==============================================================================
echo -e "\n${CYAN}Step 5: Creating E2E Test Runner${NC}"

cat > "$PROJECT_ROOT/run-e2e-with-collector.sh" << 'EOF'
#!/bin/bash

# Run E2E Tests with Working Collector

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m'

PROJECT_ROOT="/Users/deepaksharma/syc/db-otel/database-intelligence-restructured"

cd "$PROJECT_ROOT"

echo -e "${BLUE}=== RUNNING E2E TESTS WITH COLLECTOR ===${NC}"

# Check if collector exists
if [ ! -f "./e2e-collector" ]; then
    echo -e "${RED}[✗]${NC} E2E collector not found. Run build-working-e2e-collector.sh first."
    exit 1
fi

# Start databases
echo -e "\n${YELLOW}Starting test databases...${NC}"
docker-compose -f deployments/docker/compose/docker-compose-databases.yaml up -d

# Wait for databases
echo -e "${YELLOW}Waiting for databases to be ready...${NC}"
sleep 15

# Check database health
docker exec db-intel-postgres pg_isready -U postgres
docker exec db-intel-mysql mysqladmin ping -h localhost -u root -ppassword

# Start collector
echo -e "\n${YELLOW}Starting E2E collector...${NC}"
export TEST_RUN_ID="e2e-$(date +%Y%m%d-%H%M%S)"

./e2e-collector --config=e2e-collector-config.yaml &
COLLECTOR_PID=$!

echo -e "${GREEN}[✓]${NC} Collector started with PID: $COLLECTOR_PID"

# Wait for collector to initialize
sleep 10

# Check collector health
if curl -s http://localhost:13133/health | grep -q "OK"; then
    echo -e "${GREEN}[✓]${NC} Collector is healthy"
else
    echo -e "${RED}[✗]${NC} Collector health check failed"
fi

# Run some test queries
echo -e "\n${YELLOW}Running test database queries...${NC}"

# PostgreSQL test queries
docker exec db-intel-postgres psql -U postgres -d testdb -c "SELECT * FROM test_users LIMIT 5;"
docker exec db-intel-postgres psql -U postgres -d testdb -c "SELECT COUNT(*) FROM test_orders;"

# MySQL test queries
docker exec db-intel-mysql mysql -u root -ppassword testdb -e "SELECT * FROM test_users LIMIT 5;"
docker exec db-intel-mysql mysql -u root -ppassword testdb -e "SELECT COUNT(*) FROM test_orders;"

# Wait for metrics to be collected
echo -e "\n${YELLOW}Waiting for metrics collection...${NC}"
sleep 30

# Check metrics
echo -e "\n${YELLOW}Checking collected metrics...${NC}"

# Check Prometheus metrics
if curl -s http://localhost:8889/metrics | grep -q "database_intelligence"; then
    echo -e "${GREEN}[✓]${NC} Prometheus metrics available"
    echo "Sample metrics:"
    curl -s http://localhost:8889/metrics | grep "database_intelligence" | head -5
fi

# Check file export
if [ -f "/tmp/e2e-metrics.json" ]; then
    echo -e "${GREEN}[✓]${NC} File export working"
    echo "Metrics file size: $(wc -c < /tmp/e2e-metrics.json) bytes"
fi

# Cleanup
echo -e "\n${YELLOW}Cleaning up...${NC}"

# Stop collector
kill $COLLECTOR_PID 2>/dev/null || true
wait $COLLECTOR_PID 2>/dev/null || true

# Stop databases
docker-compose -f deployments/docker/compose/docker-compose-databases.yaml down

echo -e "\n${GREEN}E2E test completed!${NC}"
echo -e "Check the following for results:"
echo -e "- Metrics file: /tmp/e2e-metrics.json"
echo -e "- Test run ID: $TEST_RUN_ID"
EOF

chmod +x "$PROJECT_ROOT/run-e2e-with-collector.sh"

echo -e "${GREEN}[✓]${NC} Created E2E test runner script"

# ==============================================================================
# Summary
# ==============================================================================
echo -e "\n${CYAN}=== BUILD COMPLETE ===${NC}"
echo -e "${GREEN}Successfully built working E2E collector!${NC}"
echo ""
echo "Files created:"
echo "- E2E Collector: ./e2e-collector"
echo "- Configuration: ./e2e-collector-config.yaml"
echo "- Test Runner: ./run-e2e-with-collector.sh"
echo ""
echo "To run E2E tests:"
echo "  ./run-e2e-with-collector.sh"
echo ""
echo "To run collector manually:"
echo "  ./e2e-collector --config=e2e-collector-config.yaml"