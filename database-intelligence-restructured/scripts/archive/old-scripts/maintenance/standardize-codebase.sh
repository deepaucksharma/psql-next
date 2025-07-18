#!/bin/bash
# Script to standardize the entire codebase for consistency

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== Standardizing Database Intelligence Codebase ===${NC}"

# 1. Fix deployment mode consistency in all configs
echo -e "${YELLOW}Fixing deployment mode consistency...${NC}"
for file in configs/*-maximum-extraction.yaml; do
    if [ -f "$file" ]; then
        # Standardize deployment.mode attribute
        sed -i.bak 's/db\.deployment\.mode/deployment.mode/g' "$file"
        sed -i.bak 's/config-only-[a-z]*-max/config-only-maximum/g' "$file"
        sed -i.bak 's/config_only_max/config_only_maximum/g' "$file"
        sed -i.bak 's/deployment_mode: config_only_max/deployment_mode: config_only_maximum/g' "$file"
        rm -f "$file.bak"
    fi
done

# 2. Standardize prometheus namespace values
echo -e "${YELLOW}Standardizing Prometheus namespaces...${NC}"
sed -i.bak 's/namespace: postgresql/namespace: db_postgresql/g' configs/postgresql-maximum-extraction.yaml
sed -i.bak 's/namespace: mysql/namespace: db_mysql/g' configs/mysql-maximum-extraction.yaml
sed -i.bak 's/namespace: mongodb/namespace: db_mongodb/g' configs/mongodb-maximum-extraction.yaml
sed -i.bak 's/namespace: mssql/namespace: db_mssql/g' configs/mssql-maximum-extraction.yaml
sed -i.bak 's/namespace: oracle/namespace: db_oracle/g' configs/oracle-maximum-extraction.yaml
rm -f configs/*.bak

# 3. Create standardized environment template files
echo -e "${YELLOW}Creating environment template files...${NC}"
mkdir -p configs/env-templates

# PostgreSQL env template
cat > configs/env-templates/postgresql.env << 'EOF'
# PostgreSQL Configuration
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=your_password
POSTGRES_DB=postgres

# New Relic Configuration
NEW_RELIC_LICENSE_KEY=your_license_key
NEW_RELIC_OTLP_ENDPOINT=https://otlp.nr-data.net:4317

# Optional Configuration
LOG_LEVEL=info
ENVIRONMENT=production
EOF

# MySQL env template
cat > configs/env-templates/mysql.env << 'EOF'
# MySQL Configuration
MYSQL_HOST=localhost
MYSQL_PORT=3306
MYSQL_USER=root
MYSQL_PASSWORD=your_password

# New Relic Configuration
NEW_RELIC_LICENSE_KEY=your_license_key
NEW_RELIC_OTLP_ENDPOINT=https://otlp.nr-data.net:4317

# Optional Configuration
LOG_LEVEL=info
ENVIRONMENT=production
EOF

# MongoDB env template
cat > configs/env-templates/mongodb.env << 'EOF'
# MongoDB Configuration
MONGODB_HOST=localhost
MONGODB_PORT=27017
MONGODB_USER=admin
MONGODB_PASSWORD=your_password

# MongoDB Atlas Configuration (Optional)
MONGODB_ATLAS_PUBLIC_KEY=
MONGODB_ATLAS_PRIVATE_KEY=
MONGODB_ATLAS_PROJECT=
MONGODB_ATLAS_CLUSTER=

# New Relic Configuration
NEW_RELIC_LICENSE_KEY=your_license_key
NEW_RELIC_OTLP_ENDPOINT=https://otlp.nr-data.net:4317

# Optional Configuration
LOG_LEVEL=info
ENVIRONMENT=production
EOF

# MSSQL env template
cat > configs/env-templates/mssql.env << 'EOF'
# MSSQL Configuration
MSSQL_HOST=localhost
MSSQL_PORT=1433
MSSQL_USER=sa
MSSQL_PASSWORD=your_password
MSSQL_INSTANCE_NAME=MSSQLSERVER
MSSQL_COMPUTER_NAME=

# New Relic Configuration
NEW_RELIC_LICENSE_KEY=your_license_key
NEW_RELIC_OTLP_ENDPOINT=https://otlp.nr-data.net:4317

# Optional Configuration
LOG_LEVEL=info
ENVIRONMENT=production
EOF

# Oracle env template
cat > configs/env-templates/oracle.env << 'EOF'
# Oracle Configuration
ORACLE_HOST=localhost
ORACLE_PORT=1521
ORACLE_SERVICE=ORCLPDB1
ORACLE_USER=system
ORACLE_PASSWORD=your_password

# New Relic Configuration
NEW_RELIC_LICENSE_KEY=your_license_key
NEW_RELIC_OTLP_ENDPOINT=https://otlp.nr-data.net:4317

# Optional Configuration
LOG_LEVEL=info
ENVIRONMENT=production
EOF

# 4. Create unified validation script
echo -e "${YELLOW}Creating unified validation script...${NC}"
cat > scripts/validate-metrics.sh << 'EOF'
#!/bin/bash
# Unified metrics validation script for all databases

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Check which database to validate
DATABASE=${1:-all}
NR_ACCOUNT_ID=${NEW_RELIC_ACCOUNT_ID:-$NR_ACCOUNT_ID}
NR_API_KEY=${NEW_RELIC_API_KEY:-$NR_API_KEY}

if [ -z "$NR_ACCOUNT_ID" ] || [ -z "$NR_API_KEY" ]; then
    echo -e "${RED}Error: NEW_RELIC_ACCOUNT_ID and NEW_RELIC_API_KEY must be set${NC}"
    exit 1
fi

# Function to check metrics for a specific database
check_database_metrics() {
    local db_type=$1
    local metric_prefix=$2
    
    echo -e "${YELLOW}Checking $db_type metrics...${NC}"
    
    # Query New Relic for metrics
    local query="SELECT count(*) FROM Metric WHERE metricName LIKE '${metric_prefix}%' AND deployment.mode = 'config-only-maximum' SINCE 5 minutes ago"
    
    local response=$(curl -s -X POST "https://api.newrelic.com/graphql" \
        -H "Content-Type: application/json" \
        -H "API-Key: $NR_API_KEY" \
        -d "{
            \"query\": \"{ actor { account(id: $NR_ACCOUNT_ID) { nrql(query: \\\"$query\\\") { results } } } }\"
        }")
    
    # Parse response and check if metrics exist
    if echo "$response" | grep -q '"count":0'; then
        echo -e "${RED}❌ No $db_type metrics found${NC}"
        return 1
    else
        local count=$(echo "$response" | grep -oE '"count":[0-9]+' | grep -oE '[0-9]+' | head -1)
        echo -e "${GREEN}✓ Found $count $db_type metrics${NC}"
        return 0
    fi
}

# Main validation logic
case $DATABASE in
    postgresql)
        check_database_metrics "PostgreSQL" "postgresql"
        ;;
    mysql)
        check_database_metrics "MySQL" "mysql"
        ;;
    mongodb)
        check_database_metrics "MongoDB" "mongodb"
        ;;
    mssql)
        check_database_metrics "MSSQL" "mssql"
        check_database_metrics "SQL Server" "sqlserver"
        ;;
    oracle)
        check_database_metrics "Oracle" "oracle"
        ;;
    all)
        echo -e "${BLUE}Validating all database metrics...${NC}"
        check_database_metrics "PostgreSQL" "postgresql"
        check_database_metrics "MySQL" "mysql"
        check_database_metrics "MongoDB" "mongodb"
        check_database_metrics "MSSQL" "mssql"
        check_database_metrics "SQL Server" "sqlserver"
        check_database_metrics "Oracle" "oracle"
        ;;
    *)
        echo -e "${RED}Unknown database: $DATABASE${NC}"
        echo "Usage: $0 [postgresql|mysql|mongodb|mssql|oracle|all]"
        exit 1
        ;;
esac
EOF

chmod +x scripts/validate-metrics.sh

# 5. Consolidate and clean up scripts
echo -e "${YELLOW}Consolidating scripts...${NC}"
# Move useful scripts from development/scripts to scripts/
if [ -d "development/scripts" ]; then
    # Copy only the useful, non-duplicate scripts
    for script in development/scripts/{deploy-parallel-modes.sh,verify-metrics.sh,migrate-dashboard.sh}; do
        if [ -f "$script" ] && [ ! -f "scripts/$(basename $script)" ]; then
            cp "$script" scripts/
        fi
    done
fi

# 6. Update cross-references in documentation
echo -e "${YELLOW}Updating documentation cross-references...${NC}"
# Fix references to old script locations
find docs -name "*.md" -type f -exec sed -i.bak 's|development/scripts/|scripts/|g' {} \;
find docs -name "*.md" -type f -exec sed -i.bak 's|docs/archive/scripts/|scripts/archive/|g' {} \;
find docs -name "*.bak" -type f -delete

# 7. Create a configuration validation script
cat > scripts/validate-config.sh << 'EOF'
#!/bin/bash
# Validate OpenTelemetry configurations

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

CONFIG_FILE=${1:-}

if [ -z "$CONFIG_FILE" ]; then
    echo "Usage: $0 <config-file>"
    echo "Example: $0 configs/postgresql-maximum-extraction.yaml"
    exit 1
fi

if [ ! -f "$CONFIG_FILE" ]; then
    echo -e "${RED}Error: Config file not found: $CONFIG_FILE${NC}"
    exit 1
fi

echo -e "${YELLOW}Validating configuration: $CONFIG_FILE${NC}"

# Basic YAML validation
if ! command -v yq &> /dev/null; then
    echo -e "${YELLOW}Warning: yq not found, skipping YAML validation${NC}"
else
    if yq eval '.' "$CONFIG_FILE" > /dev/null 2>&1; then
        echo -e "${GREEN}✓ Valid YAML syntax${NC}"
    else
        echo -e "${RED}✗ Invalid YAML syntax${NC}"
        exit 1
    fi
fi

# Check required sections
for section in receivers processors exporters service; do
    if grep -q "^$section:" "$CONFIG_FILE"; then
        echo -e "${GREEN}✓ Found $section section${NC}"
    else
        echo -e "${RED}✗ Missing $section section${NC}"
        exit 1
    fi
done

# Check for New Relic exporter
if grep -q "otlp/newrelic:" "$CONFIG_FILE"; then
    echo -e "${GREEN}✓ New Relic exporter configured${NC}"
else
    echo -e "${RED}✗ New Relic exporter not found${NC}"
fi

# Check for required environment variables
env_vars=$(grep -oE '\${env:[A-Z_]+' "$CONFIG_FILE" | sort | uniq | sed 's/${env://')
if [ -n "$env_vars" ]; then
    echo -e "${YELLOW}Required environment variables:${NC}"
    for var in $env_vars; do
        if [ -z "${!var}" ]; then
            echo -e "  ${RED}✗ $var (not set)${NC}"
        else
            echo -e "  ${GREEN}✓ $var${NC}"
        fi
    done
fi

echo -e "${GREEN}Configuration validation complete!${NC}"
EOF

chmod +x scripts/validate-config.sh

# 8. Create unified test script
cat > scripts/test-database-config.sh << 'EOF'
#!/bin/bash
# Test database configuration with OpenTelemetry collector

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

DATABASE=${1:-}
DURATION=${2:-60}

if [ -z "$DATABASE" ]; then
    echo "Usage: $0 <database> [duration-seconds]"
    echo "Example: $0 postgresql 60"
    echo "Databases: postgresql, mysql, mongodb, mssql, oracle"
    exit 1
fi

CONFIG_FILE="configs/${DATABASE}-maximum-extraction.yaml"
ENV_FILE="configs/env-templates/${DATABASE}.env"

if [ ! -f "$CONFIG_FILE" ]; then
    echo -e "${RED}Error: Config file not found: $CONFIG_FILE${NC}"
    exit 1
fi

echo -e "${BLUE}=== Testing $DATABASE Configuration ===${NC}"

# Load environment variables if env file exists
if [ -f "$ENV_FILE" ]; then
    echo -e "${YELLOW}Loading environment from $ENV_FILE${NC}"
    export $(grep -v '^#' "$ENV_FILE" | xargs)
fi

# Validate configuration
echo -e "${YELLOW}Validating configuration...${NC}"
./scripts/validate-config.sh "$CONFIG_FILE"

# Run collector in test mode
echo -e "${YELLOW}Starting collector for $DURATION seconds...${NC}"
timeout $DURATION otelcol-contrib --config="$CONFIG_FILE" 2>&1 | tee "/tmp/otel-${DATABASE}-test.log"

# Check for errors in log
if grep -i "error" "/tmp/otel-${DATABASE}-test.log"; then
    echo -e "${RED}Errors found in collector log${NC}"
    exit 1
else
    echo -e "${GREEN}No errors found in collector log${NC}"
fi

# Validate metrics were sent
echo -e "${YELLOW}Validating metrics...${NC}"
sleep 10  # Give metrics time to appear
./scripts/validate-metrics.sh "$DATABASE"

echo -e "${GREEN}Test completed successfully!${NC}"
EOF

chmod +x scripts/test-database-config.sh

echo -e "${GREEN}=== Codebase Standardization Complete ===${NC}"
echo ""
echo "Summary of changes:"
echo "1. ✓ Standardized deployment.mode values to 'config-only-maximum'"
echo "2. ✓ Standardized Prometheus namespace prefixes with 'db_'"
echo "3. ✓ Created environment template files in configs/env-templates/"
echo "4. ✓ Created unified validation scripts"
echo "5. ✓ Consolidated scripts directory"
echo "6. ✓ Updated documentation cross-references"
echo ""
echo "Next steps:"
echo "- Review the changes with: git diff"
echo "- Test configurations with: ./scripts/test-database-config.sh <database>"
echo "- Validate metrics with: ./scripts/validate-metrics.sh <database>"