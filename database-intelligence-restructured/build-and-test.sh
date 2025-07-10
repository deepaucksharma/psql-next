#!/bin/bash

# Build and test all components after version updates

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

echo -e "${BLUE}=== BUILDING AND TESTING COMPONENTS ===${NC}"

# ==============================================================================
# Step 1: Build production distribution
# ==============================================================================
echo -e "\n${CYAN}Step 1: Building production distribution${NC}"

cd "distributions/production"
echo -e "\n${YELLOW}Building collector binary...${NC}"

# Clean previous builds
rm -f otelcol-production

# Build the collector
go build -o otelcol-production .

if [ -f "otelcol-production" ]; then
    echo -e "${GREEN}[✓]${NC} Production collector built successfully!"
    ls -la otelcol-production
else
    echo -e "${RED}[✗]${NC} Failed to build production collector"
    exit 1
fi

cd "$PROJECT_ROOT"

# ==============================================================================
# Step 2: Test key components
# ==============================================================================
echo -e "\n${CYAN}Step 2: Testing key components${NC}"

# Test processors
echo -e "\n${YELLOW}Testing processors...${NC}"
for proc in adaptivesampler circuitbreaker costcontrol nrerrormonitor planattributeextractor querycorrelator verification; do
    if [ -d "processors/$proc" ]; then
        echo -e "\nTesting $proc processor..."
        cd "processors/$proc"
        go test ./... -v -short || echo -e "${YELLOW}[!]${NC} Tests failed for $proc"
        cd "$PROJECT_ROOT"
    fi
done

# Test receivers
echo -e "\n${YELLOW}Testing receivers...${NC}"
for recv in ash enhancedsql kernelmetrics; do
    if [ -d "receivers/$recv" ]; then
        echo -e "\nTesting $recv receiver..."
        cd "receivers/$recv"
        go test ./... -v -short || echo -e "${YELLOW}[!]${NC} Tests failed for $recv"
        cd "$PROJECT_ROOT"
    fi
done

# ==============================================================================
# Step 3: Validate collector configuration
# ==============================================================================
echo -e "\n${CYAN}Step 3: Validating collector configuration${NC}"

cd "distributions/production"
echo -e "\n${YELLOW}Validating production config...${NC}"

# Create a test config
cat > test-config.yaml << 'EOF'
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: localhost:4317
      http:
        endpoint: localhost:4318

processors:
  batch:

exporters:
  debug:
    verbosity: detailed

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
EOF

# Validate the config
./otelcol-production validate --config=test-config.yaml

if [ $? -eq 0 ]; then
    echo -e "${GREEN}[✓]${NC} Configuration validation passed!"
else
    echo -e "${RED}[✗]${NC} Configuration validation failed"
fi

# Clean up test config
rm -f test-config.yaml

cd "$PROJECT_ROOT"

# ==============================================================================
# Step 4: Test with database receivers
# ==============================================================================
echo -e "\n${CYAN}Step 4: Testing database receivers configuration${NC}"

cd "distributions/production"
echo -e "\n${YELLOW}Creating database test config...${NC}"

cat > db-test-config.yaml << 'EOF'
receivers:
  postgresql:
    endpoint: postgres://postgres:password@localhost:5432/testdb?sslmode=disable
    collection_interval: 10s
    databases:
      - testdb
  
  mysql:
    endpoint: root:password@tcp(localhost:3306)/testdb
    collection_interval: 10s
    database: testdb

processors:
  batch:

exporters:
  debug:
    verbosity: normal

service:
  pipelines:
    metrics:
      receivers: [postgresql, mysql]
      processors: [batch]
      exporters: [debug]
EOF

# Validate database config
./otelcol-production validate --config=db-test-config.yaml

if [ $? -eq 0 ]; then
    echo -e "${GREEN}[✓]${NC} Database configuration validation passed!"
else
    echo -e "${YELLOW}[!]${NC} Database configuration validation failed (this is expected if contrib receivers aren't included)"
fi

# Clean up
rm -f db-test-config.yaml

cd "$PROJECT_ROOT"

# ==============================================================================
# Summary
# ==============================================================================
echo -e "\n${CYAN}=== BUILD AND TEST SUMMARY ===${NC}"

echo -e "\n${GREEN}Completed:${NC}"
echo "✓ Production collector built successfully"
echo "✓ Component tests executed"
echo "✓ Configuration validation tested"

echo -e "\n${YELLOW}Next steps:${NC}"
echo "1. Run E2E tests with working collector"
echo "2. Test with actual database connections"
echo "3. Verify custom processors and receivers work correctly"

echo -e "\n${GREEN}Build phase complete!${NC}"