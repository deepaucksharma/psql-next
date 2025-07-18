#!/bin/bash
# Script to identify obsolete configurations

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}=== Obsolete Configuration Identifier ===${NC}"

# Arrays to track configs
declare -a KEEP_CONFIGS
declare -a OBSOLETE_CONFIGS
declare -a MERGE_CONFIGS

# Function to analyze config file
analyze_config() {
    local config_file=$1
    local basename=$(basename "$config_file")
    
    # Check if it's a test config
    if [[ "$basename" == *"test"* ]] || [[ "$basename" == *"local"* ]]; then
        # Multiple test configs can be consolidated
        MERGE_CONFIGS+=("$config_file")
        return
    fi
    
    # Check for specific configs to keep
    case "$basename" in
        "collector.yaml" | \
        "collector-secure.yaml" | \
        "collector-gateway-enterprise.yaml" | \
        "collector-production.yaml")
            KEEP_CONFIGS+=("$config_file")
            ;;
        *"e2e"* | *"minimal"* | *"simple"* | *"alternate"*)
            OBSOLETE_CONFIGS+=("$config_file")
            ;;
        *)
            # Check if config has unique features
            if grep -q "processors:" "$config_file" && grep -q "exporters:" "$config_file"; then
                KEEP_CONFIGS+=("$config_file")
            else
                OBSOLETE_CONFIGS+=("$config_file")
            fi
            ;;
    esac
}

# Analyze MVP configs
echo -e "${YELLOW}Analyzing MVP configurations...${NC}"
for config in ../database-intelligence-mvp/config/*.yaml; do
    if [ -f "$config" ]; then
        analyze_config "$config"
    fi
done

# Analyze restructured configs
echo -e "\n${YELLOW}Analyzing restructured configurations...${NC}"
for config in configs/*.yaml; do
    if [ -f "$config" ]; then
        # All database-specific configs in restructured should be kept
        if [[ "$(basename "$config")" == *"maximum-extraction"* ]]; then
            KEEP_CONFIGS+=("$config")
        else
            analyze_config "$config"
        fi
    fi
done

# Display results
echo -e "\n${BLUE}=== Analysis Results ===${NC}"

echo -e "\n${GREEN}Configurations to KEEP (${#KEEP_CONFIGS[@]} files):${NC}"
for config in "${KEEP_CONFIGS[@]}"; do
    echo "  ✓ $config"
done

echo -e "\n${YELLOW}Configurations to MERGE (${#MERGE_CONFIGS[@]} files):${NC}"
echo "  These test configs can be consolidated into a single test configuration:"
for config in "${MERGE_CONFIGS[@]}"; do
    echo "  ⚡ $config"
done

echo -e "\n${RED}OBSOLETE configurations (${#OBSOLETE_CONFIGS[@]} files):${NC}"
echo "  These appear to be redundant or superseded:"
for config in "${OBSOLETE_CONFIGS[@]}"; do
    echo "  ✗ $config"
done

# Recommendations
echo -e "\n${BLUE}=== Recommendations ===${NC}"

echo -e "\n1. ${YELLOW}Create consolidated test configuration:${NC}"
echo "   Merge all test configs into: configs/collector-test.yaml"

echo -e "\n2. ${YELLOW}Essential configurations to maintain:${NC}"
echo "   - configs/*-maximum-extraction.yaml (database-specific)"
echo "   - configs/collector-production.yaml (production deployment)"
echo "   - configs/collector-secure.yaml (security-hardened)"
echo "   - configs/collector-test.yaml (consolidated testing)"

echo -e "\n3. ${YELLOW}Configuration naming convention:${NC}"
echo "   - Database configs: {database}-maximum-extraction.yaml"
echo "   - Deployment configs: collector-{environment}.yaml"
echo "   - Special configs: collector-{feature}.yaml"

# Create sample consolidated test config
echo -e "\n${YELLOW}Creating sample consolidated test configuration...${NC}"

cat > configs/collector-test-consolidated.yaml << 'EOF'
# Consolidated test configuration
# Combines features from multiple test configs

receivers:
  # Simple test receiver
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318
  
  # PostgreSQL for testing
  postgresql:
    endpoint: ${env:POSTGRES_HOST:localhost}:${env:POSTGRES_PORT:5432}
    username: ${env:POSTGRES_USER:postgres}
    password: ${env:POSTGRES_PASSWORD:password}
    databases:
      - ${env:POSTGRES_DB:postgres}
    collection_interval: 10s

processors:
  batch:
    timeout: 10s
    send_batch_size: 1000
  
  memory_limiter:
    check_interval: 1s
    limit_mib: 256
    spike_limit_mib: 50

exporters:
  debug:
    verbosity: detailed
    sampling_initial: 5
    sampling_thereafter: 20
  
  prometheus:
    endpoint: "0.0.0.0:8888"
    namespace: test
  
  # Optional: New Relic for integration testing
  otlp/newrelic:
    endpoint: ${env:NEW_RELIC_OTLP_ENDPOINT:otlp.nr-data.net:4317}
    headers:
      api-key: ${env:NEW_RELIC_LICENSE_KEY}

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [memory_limiter, batch]
      exporters: [debug, prometheus]
    
    metrics/otlp:
      receivers: [otlp]
      processors: [batch]
      exporters: [debug]

  telemetry:
    logs:
      level: debug
    metrics:
      level: detailed
      address: localhost:8889
EOF

echo -e "${GREEN}✓ Created configs/collector-test-consolidated.yaml${NC}"

# Summary
total_configs=$((${#KEEP_CONFIGS[@]} + ${#MERGE_CONFIGS[@]} + ${#OBSOLETE_CONFIGS[@]}))
echo -e "\n${BLUE}=== Summary ===${NC}"
echo "Total configurations analyzed: $total_configs"
echo "Recommended to keep: ${#KEEP_CONFIGS[@]}"
echo "Can be merged: ${#MERGE_CONFIGS[@]}"
echo "Obsolete: ${#OBSOLETE_CONFIGS[@]}"
echo ""
echo "Potential reduction: from $total_configs to ~10 configurations"