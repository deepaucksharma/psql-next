#!/bin/bash
# Script to consolidate environment templates

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}=== Environment Template Consolidation ===${NC}"

# Create consolidated env template
echo -e "${YELLOW}Creating master environment template...${NC}"

cat > configs/env-templates/database-intelligence.env << 'EOF'
# Database Intelligence Environment Configuration
# Copy this file to .env and fill in your values

# ==============================================================================
# Global Settings
# ==============================================================================

# New Relic Configuration
NEW_RELIC_LICENSE_KEY=your_license_key_here
NEW_RELIC_OTLP_ENDPOINT=otlp.nr-data.net:4317
# For EU region use: otlp.eu01.nr-data.net:4317

# Deployment Mode
DEPLOYMENT_MODE=config_only_maximum
ENVIRONMENT=development

# ==============================================================================
# PostgreSQL Configuration
# ==============================================================================

# Connection settings
POSTGRESQL_HOST=localhost
POSTGRESQL_PORT=5432
POSTGRESQL_USER=postgres
POSTGRESQL_PASSWORD=
POSTGRESQL_DB=postgres

# SSL/TLS (optional)
POSTGRESQL_SSL_MODE=disable
# Options: disable, require, verify-ca, verify-full

# Advanced options
POSTGRESQL_MAX_CONNECTIONS=10
POSTGRESQL_CONNECTION_TIMEOUT=30

# ==============================================================================
# MySQL Configuration
# ==============================================================================

# Connection settings
MYSQL_HOST=localhost
MYSQL_PORT=3306
MYSQL_USER=root
MYSQL_PASSWORD=
MYSQL_DATABASE=mysql

# SSL/TLS (optional)
MYSQL_SSL_MODE=false

# ==============================================================================
# MongoDB Configuration
# ==============================================================================

# Connection settings
MONGODB_HOST=localhost
MONGODB_PORT=27017
MONGODB_USER=
MONGODB_PASSWORD=
MONGODB_DATABASE=admin

# Connection string (alternative to individual settings)
# MONGODB_URI=mongodb://user:pass@localhost:27017/admin

# MongoDB Atlas (if using Atlas)
MONGODB_ATLAS_PUBLIC_KEY=
MONGODB_ATLAS_PRIVATE_KEY=
MONGODB_ATLAS_PROJECT_NAME=

# ==============================================================================
# MSSQL/SQL Server Configuration
# ==============================================================================

# Connection settings
MSSQL_HOST=localhost
MSSQL_PORT=1433
MSSQL_USER=sa
MSSQL_PASSWORD=
MSSQL_DATABASE=master

# Additional options
MSSQL_ENCRYPT=false
MSSQL_TRUST_SERVER_CERTIFICATE=true

# ==============================================================================
# Oracle Configuration
# ==============================================================================

# Connection settings
ORACLE_HOST=localhost
ORACLE_PORT=1521
ORACLE_USER=system
ORACLE_PASSWORD=
ORACLE_SERVICE=ORCLPDB1

# Alternative: Full connection string
# ORACLE_DSN=(DESCRIPTION=(ADDRESS=(PROTOCOL=TCP)(HOST=localhost)(PORT=1521))(CONNECT_DATA=(SERVICE_NAME=ORCLPDB1)))

# ==============================================================================
# Collector Configuration
# ==============================================================================

# Resource limits
OTEL_MEMORY_LIMIT_MIB=512
OTEL_CPU_LIMIT=2

# Logging
OTEL_LOG_LEVEL=info
# Options: debug, info, warn, error

# Metrics endpoint
OTEL_PROMETHEUS_PORT=8888
OTEL_HEALTH_CHECK_PORT=13133

# Batch processing
OTEL_BATCH_TIMEOUT=10s
OTEL_BATCH_SIZE=8192

# ==============================================================================
# Feature Flags
# ==============================================================================

# Enable/disable specific features
ENABLE_ASH_MONITORING=true
ENABLE_QUERY_PERFORMANCE=true
ENABLE_WAIT_ANALYSIS=true
ENABLE_BLOCKING_DETECTION=true

# Collection intervals (seconds)
HIGH_FREQUENCY_INTERVAL=5
STANDARD_INTERVAL=10
PERFORMANCE_INTERVAL=30
ANALYTICS_INTERVAL=60

# ==============================================================================
# Docker Configuration (if using Docker)
# ==============================================================================

# Container resource limits
DOCKER_MEMORY_LIMIT=2g
DOCKER_CPU_LIMIT=2

# Network
DOCKER_NETWORK=database-intelligence

# ==============================================================================
# Kubernetes Configuration (if using K8s)
# ==============================================================================

# Namespace
K8S_NAMESPACE=database-intelligence

# Resource requests/limits
K8S_MEMORY_REQUEST=512Mi
K8S_MEMORY_LIMIT=2Gi
K8S_CPU_REQUEST=500m
K8S_CPU_LIMIT=2000m
EOF

echo -e "${GREEN}✓ Created master environment template${NC}"

# Create database-specific minimal templates
echo -e "\n${YELLOW}Creating minimal database-specific templates...${NC}"

# PostgreSQL minimal
cat > configs/env-templates/postgresql-minimal.env << 'EOF'
# PostgreSQL Minimal Configuration
POSTGRESQL_HOST=localhost
POSTGRESQL_PORT=5432
POSTGRESQL_USER=postgres
POSTGRESQL_PASSWORD=
NEW_RELIC_LICENSE_KEY=your_license_key_here
EOF

# MySQL minimal
cat > configs/env-templates/mysql-minimal.env << 'EOF'
# MySQL Minimal Configuration
MYSQL_HOST=localhost
MYSQL_PORT=3306
MYSQL_USER=root
MYSQL_PASSWORD=
NEW_RELIC_LICENSE_KEY=your_license_key_here
EOF

# MongoDB minimal
cat > configs/env-templates/mongodb-minimal.env << 'EOF'
# MongoDB Minimal Configuration
MONGODB_HOST=localhost
MONGODB_PORT=27017
MONGODB_USER=
MONGODB_PASSWORD=
NEW_RELIC_LICENSE_KEY=your_license_key_here
EOF

# MSSQL minimal
cat > configs/env-templates/mssql-minimal.env << 'EOF'
# MSSQL Minimal Configuration
MSSQL_HOST=localhost
MSSQL_PORT=1433
MSSQL_USER=sa
MSSQL_PASSWORD=
NEW_RELIC_LICENSE_KEY=your_license_key_here
EOF

# Oracle minimal
cat > configs/env-templates/oracle-minimal.env << 'EOF'
# Oracle Minimal Configuration
ORACLE_HOST=localhost
ORACLE_PORT=1521
ORACLE_USER=system
ORACLE_PASSWORD=
ORACLE_SERVICE=ORCLPDB1
NEW_RELIC_LICENSE_KEY=your_license_key_here
EOF

echo -e "${GREEN}✓ Created minimal database templates${NC}"

# Create .env.example from master template
echo -e "\n${YELLOW}Creating .env.example...${NC}"
cp configs/env-templates/database-intelligence.env .env.example
echo -e "${GREEN}✓ Created .env.example${NC}"

# Clean up duplicate env files
echo -e "\n${YELLOW}Cleaning up duplicate env files...${NC}"

# Remove old database-specific templates (keeping for backward compatibility)
# rm -f configs/env-templates/postgresql.env
# rm -f configs/env-templates/mysql.env
# rm -f configs/env-templates/mongodb.env
# rm -f configs/env-templates/mssql.env
# rm -f configs/env-templates/oracle.env

# Update references
echo -e "\n${YELLOW}Note: Original database-specific templates preserved for backward compatibility${NC}"

# Summary
echo -e "\n${BLUE}=== Consolidation Complete ===${NC}"
echo "Created:"
echo "  - Master template: configs/env-templates/database-intelligence.env"
echo "  - Minimal templates: configs/env-templates/*-minimal.env"
echo "  - Example file: .env.example"
echo ""
echo "Benefits:"
echo "  - Single source of truth for all environment variables"
echo "  - Comprehensive documentation of all options"
echo "  - Minimal templates for quick setup"
echo "  - Consistent naming conventions"
echo ""
echo -e "${YELLOW}Usage:${NC}"
echo "  Quick start: cp configs/env-templates/postgresql-minimal.env .env"
echo "  Full config: cp configs/env-templates/database-intelligence.env .env"