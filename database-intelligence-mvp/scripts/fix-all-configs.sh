#!/bin/bash
# Comprehensive script to fix all configuration issues across the codebase

set -e

echo "=== Comprehensive Configuration Fix Script ==="

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

# Counters
FIXED_COUNT=0
SKIPPED_COUNT=0
ERROR_COUNT=0

# Backup directory
BACKUP_DIR="config-backup-$(date +%Y%m%d-%H%M%S)"
mkdir -p "$BACKUP_DIR"

echo -e "${YELLOW}Backing up configs to: $BACKUP_DIR${NC}"

# Function to fix a YAML file
fix_yaml_file() {
    local file=$1
    local filename=$(basename "$file")
    
    echo -e "\n${YELLOW}Processing: $file${NC}"
    
    # Skip already fixed files
    if [[ "$filename" == *"-fixed.yaml" ]]; then
        echo -e "${GREEN}✓ Already fixed, skipping${NC}"
        ((SKIPPED_COUNT++))
        return
    fi
    
    # Skip backup files
    if [[ "$filename" == *".backup-"* ]]; then
        echo -e "${GREEN}✓ Backup file, skipping${NC}"
        ((SKIPPED_COUNT++))
        return
    fi
    
    # Create backup
    cp "$file" "$BACKUP_DIR/$filename.backup" 2>/dev/null || true
    
    # Create temporary file
    local tmpfile="${file}.fixing"
    cp "$file" "$tmpfile"
    
    # Fix environment variables
    echo "  - Fixing environment variable syntax..."
    sed -i.bak 's/\${POSTGRES_HOST:/\${env:POSTGRES_HOST:-/g' "$tmpfile"
    sed -i.bak 's/\${POSTGRES_PORT:/\${env:POSTGRES_PORT:-/g' "$tmpfile"
    sed -i.bak 's/\${POSTGRES_USER:/\${env:POSTGRES_USER:-/g' "$tmpfile"
    sed -i.bak 's/\${POSTGRES_PASSWORD:/\${env:POSTGRES_PASSWORD:-/g' "$tmpfile"
    sed -i.bak 's/\${POSTGRES_DB:/\${env:POSTGRES_DB:-/g' "$tmpfile"
    sed -i.bak 's/\${POSTGRES_DATABASE:/\${env:POSTGRES_DATABASE:-/g' "$tmpfile"
    
    sed -i.bak 's/\${MYSQL_HOST:/\${env:MYSQL_HOST:-/g' "$tmpfile"
    sed -i.bak 's/\${MYSQL_PORT:/\${env:MYSQL_PORT:-/g' "$tmpfile"
    sed -i.bak 's/\${MYSQL_USER:/\${env:MYSQL_USER:-/g' "$tmpfile"
    sed -i.bak 's/\${MYSQL_PASSWORD:/\${env:MYSQL_PASSWORD:-/g' "$tmpfile"
    sed -i.bak 's/\${MYSQL_DB:/\${env:MYSQL_DB:-/g' "$tmpfile"
    sed -i.bak 's/\${MYSQL_DATABASE:/\${env:MYSQL_DATABASE:-/g' "$tmpfile"
    
    sed -i.bak 's/\${NEW_RELIC_LICENSE_KEY}/\${env:NEW_RELIC_LICENSE_KEY}/g' "$tmpfile"
    sed -i.bak 's/\${NEW_RELIC_OTLP_ENDPOINT:/\${env:NEW_RELIC_OTLP_ENDPOINT:-/g' "$tmpfile"
    sed -i.bak 's/\${OTLP_ENDPOINT:/\${env:OTLP_ENDPOINT:-/g' "$tmpfile"
    sed -i.bak 's/\${ENVIRONMENT:/\${env:ENVIRONMENT:-/g' "$tmpfile"
    sed -i.bak 's/\${HOSTNAME}/\${env:HOSTNAME}/g' "$tmpfile"
    sed -i.bak 's/\${TEST_RUN_ID:/\${env:TEST_RUN_ID:-/g' "$tmpfile"
    sed -i.bak 's/\${PG_REPLICA_DSN}/\${env:PG_REPLICA_DSN}/g' "$tmpfile"
    sed -i.bak 's/\${MYSQL_READONLY_DSN}/\${env:MYSQL_READONLY_DSN}/g' "$tmpfile"
    
    # Fix memory limiter
    echo "  - Fixing memory limiter configuration..."
    sed -i.bak 's/limit_percentage: *[0-9]*/limit_mib: 1024/g' "$tmpfile"
    sed -i.bak 's/spike_limit_percentage: *[0-9]*/spike_limit_mib: 256/g' "$tmpfile"
    
    # Remove memory_ballast extension
    if grep -q "memory_ballast:" "$tmpfile"; then
        echo "  - Removing deprecated memory_ballast extension..."
        sed -i.bak '/memory_ballast:/,/^[[:space:]]*$/d' "$tmpfile"
        sed -i.bak 's/, *memory_ballast//g' "$tmpfile"
        sed -i.bak 's/memory_ballast, *//g' "$tmpfile"
    fi
    
    # Check if resource processor with collector.name exists
    if ! grep -q "collector\.name" "$tmpfile"; then
        echo -e "  ${RED}⚠ Missing collector.name in resource processor${NC}"
        echo -e "  ${YELLOW}Manual fix required: Add resource processor with collector.name = otelcol${NC}"
    fi
    
    # Check for sqlquery receivers without logs section
    if grep -q "sqlquery" "$tmpfile"; then
        if ! grep -A10 "sqlquery" "$tmpfile" | grep -q "logs:"; then
            echo -e "  ${RED}⚠ SQL query receiver missing logs section${NC}"
            echo -e "  ${YELLOW}Manual fix required: Add logs configuration to sqlquery receivers${NC}"
        fi
    fi
    
    # Clean up backup files
    rm -f "${tmpfile}.bak"
    
    # Move fixed file back
    mv "$tmpfile" "$file"
    
    echo -e "${GREEN}✓ Fixed: $file${NC}"
    ((FIXED_COUNT++))
}

# Function to create a migration guide entry
add_to_migration_guide() {
    local file=$1
    local issue=$2
    echo "- File: $file" >> migration-guide.md
    echo "  Issue: $issue" >> migration-guide.md
    echo "" >> migration-guide.md
}

# Start migration guide
cat > migration-guide.md << 'EOF'
# Configuration Migration Guide

This guide helps you migrate from old configuration syntax to the new format.

## Automated Fixes Applied

1. **Environment Variables**: Changed from `${VAR:default}` to `${env:VAR:-default}`
2. **Memory Limiter**: Changed from percentage to MiB values
3. **Deprecated Extensions**: Removed memory_ballast references

## Manual Fixes Required

### 1. Add Resource Processor

All configurations must include a resource processor with collector.name:

```yaml
processors:
  resource:
    attributes:
      - key: collector.name
        value: otelcol
        action: upsert
```

### 2. Fix SQL Query Receivers

All sqlquery receivers must have logs or metrics configuration:

```yaml
sqlquery/postgresql:
  queries:
    - sql: "SELECT ..."
      logs:
        - body_column: query_text
          attributes:
            query_id: query_id
            avg_duration_ms: avg_duration_ms
```

### 3. Update Service Pipelines

Ensure processors are in correct order:

```yaml
service:
  pipelines:
    metrics:
      processors: [memory_limiter, resource, transform/metrics, batch]
```

## Files Requiring Manual Review

EOF

# Find and fix all YAML files
echo -e "\n${YELLOW}Finding all YAML configuration files...${NC}"

# Process main config files
for file in config/*.yaml; do
    if [ -f "$file" ]; then
        fix_yaml_file "$file"
    fi
done

# Process deployment configs
for file in deployments/**/*.yaml; do
    if [ -f "$file" ] && [[ ! "$file" == *"/templates/"* ]]; then
        fix_yaml_file "$file"
    fi
done

# Process test configs
for file in tests/**/*.yaml; do
    if [ -f "$file" ]; then
        fix_yaml_file "$file"
    fi
done

# Find files that need manual review
echo -e "\n${YELLOW}Files needing manual review:${NC}"
find . -name "*.yaml" -type f | while read -r file; do
    # Skip node_modules and vendor directories
    if [[ "$file" == *"node_modules"* ]] || [[ "$file" == *"vendor"* ]]; then
        continue
    fi
    
    # Check for missing collector.name
    if ! grep -q "collector\.name" "$file" 2>/dev/null; then
        if grep -q "processors:" "$file" 2>/dev/null; then
            echo "  - $file (missing collector.name)"
            add_to_migration_guide "$file" "Missing collector.name in resource processor"
        fi
    fi
    
    # Check for sqlquery without logs
    if grep -q "sqlquery" "$file" 2>/dev/null; then
        if ! grep -A20 "sqlquery" "$file" | grep -q "logs:" 2>/dev/null; then
            echo "  - $file (sqlquery missing logs section)"
            add_to_migration_guide "$file" "SQL query receiver missing logs configuration"
        fi
    fi
done

# Create example configurations
echo -e "\n${YELLOW}Creating example configurations...${NC}"

cat > config/example-minimal.yaml << 'EOF'
# Minimal working configuration
extensions:
  zpages:
    endpoint: 0.0.0.0:55679

receivers:
  postgresql:
    endpoint: ${env:POSTGRES_HOST:-localhost}:${env:POSTGRES_PORT:-5432}
    username: ${env:POSTGRES_USER:-postgres}
    password: ${env:POSTGRES_PASSWORD:-postgres}
    databases:
      - ${env:POSTGRES_DB:-postgres}
    collection_interval: 60s
    tls:
      insecure: true

processors:
  memory_limiter:
    limit_mib: 512
    spike_limit_mib: 128
    
  resource:
    attributes:
      - key: collector.name
        value: otelcol
        action: upsert
        
  batch:
    timeout: 10s

exporters:
  otlp/newrelic:
    endpoint: ${env:OTLP_ENDPOINT:-https://otlp.nr-data.net:4317}
    headers:
      api-key: ${env:NEW_RELIC_LICENSE_KEY}

service:
  extensions: [zpages]
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [memory_limiter, resource, batch]
      exporters: [otlp/newrelic]
EOF

echo -e "${GREEN}✓ Created config/example-minimal.yaml${NC}"

# Summary
echo -e "\n${YELLOW}=== Summary ===${NC}"
echo -e "Fixed: ${GREEN}$FIXED_COUNT${NC} files"
echo -e "Skipped: ${YELLOW}$SKIPPED_COUNT${NC} files"
echo -e "Errors: ${RED}$ERROR_COUNT${NC} files"
echo -e "\nBackup directory: ${YELLOW}$BACKUP_DIR${NC}"
echo -e "Migration guide: ${YELLOW}migration-guide.md${NC}"
echo -e "\n${GREEN}✓ Configuration fixes complete!${NC}"

# Validate a sample config
if [ -f "./dist/database-intelligence-collector" ]; then
    echo -e "\n${YELLOW}Validating example configuration...${NC}"
    if ./dist/database-intelligence-collector validate --config=config/example-minimal.yaml; then
        echo -e "${GREEN}✓ Example configuration is valid${NC}"
    else
        echo -e "${RED}✗ Example configuration validation failed${NC}"
    fi
fi