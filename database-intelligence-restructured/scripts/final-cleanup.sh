#!/bin/bash
# Final cleanup and organization script

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}=== Final Cleanup and Organization ===${NC}"

# Function to move scripts to proper locations
organize_scripts() {
    echo -e "${YELLOW}Organizing remaining scripts...${NC}"
    
    # Move maintenance scripts
    if [ -f "scripts/cleanup-archives.sh" ]; then
        mkdir -p scripts/maintenance
        mv scripts/cleanup-archives.sh scripts/maintenance/ 2>/dev/null || true
        mv scripts/consolidate-*.sh scripts/maintenance/ 2>/dev/null || true
        mv scripts/identify-*.sh scripts/maintenance/ 2>/dev/null || true
        mv scripts/standardize-*.sh scripts/maintenance/ 2>/dev/null || true
        mv scripts/fix-*.sh scripts/maintenance/ 2>/dev/null || true
        mv scripts/create-*.sh scripts/maintenance/ 2>/dev/null || true
    fi
    
    # Move deployment scripts
    mkdir -p scripts/deployment
    mv scripts/start-*.sh scripts/deployment/ 2>/dev/null || true
    mv scripts/stop-*.sh scripts/deployment/ 2>/dev/null || true
    
    echo -e "${GREEN}✓ Scripts organized${NC}"
}

# Function to clean empty directories
clean_empty_dirs() {
    echo -e "${YELLOW}Removing empty directories...${NC}"
    find . -type d -empty -not -path "./.git/*" -delete 2>/dev/null || true
    echo -e "${GREEN}✓ Empty directories removed${NC}"
}

# Function to update script permissions
fix_permissions() {
    echo -e "${YELLOW}Fixing script permissions...${NC}"
    find scripts -name "*.sh" -type f -exec chmod +x {} \;
    echo -e "${GREEN}✓ Permissions fixed${NC}"
}

# Function to create index files
create_indexes() {
    echo -e "${YELLOW}Creating index files...${NC}"
    
    # Scripts index
    cat > scripts/INDEX.md << 'EOF'
# Scripts Directory Index

## Directory Structure

```
scripts/
├── validation/          # Configuration and system validation
├── testing/            # Test execution and benchmarking
├── building/           # Build and compilation scripts
├── deployment/         # Start/stop and deployment tools
└── maintenance/        # Cleanup, fixes, and reorganization
```

## Key Scripts

### Validation
- `validate-all.sh` - Run all validation checks
- `validate-config.sh` - Validate YAML configurations
- `validate-metrics.sh` - Check metric collection
- `validate-metric-naming.sh` - Verify naming conventions
- `validate-e2e.sh` - End-to-end validation

### Testing
- `run-tests.sh` - Unified test runner
- `test-database-config.sh` - Test specific database
- `test-integration.sh` - Integration tests
- `benchmark-performance.sh` - Performance testing
- `check-metric-cardinality.sh` - Cardinality analysis

### Building
- `build-collector.sh` - Build custom collector
- `build-ci.sh` - CI/CD build script

### Deployment
- `start-all-databases.sh` - Start all database containers
- `stop-all-databases.sh` - Stop all containers

### Maintenance
- `cleanup-archives.sh` - Remove archive directories
- `consolidate-docs.sh` - Merge documentation
- `standardize-readmes.sh` - Update README files
- `reorganize-project.sh` - Project structure tool
EOF

    # Configs index
    cat > configs/INDEX.md << 'EOF'
# Configurations Directory Index

## Database-Specific Configurations

Maximum metric extraction configurations for each supported database:

- `postgresql-maximum-extraction.yaml` - PostgreSQL with ASH simulation
- `mysql-maximum-extraction.yaml` - MySQL with Performance Schema
- `mongodb-maximum-extraction.yaml` - MongoDB with Atlas support
- `mssql-maximum-extraction.yaml` - SQL Server with wait stats
- `oracle-maximum-extraction.yaml` - Oracle with V$ views

## Other Configurations

- `collector-test-consolidated.yaml` - Unified test configuration
- `base.yaml` - Base configuration template
- `examples.yaml` - Example configurations

## Environment Templates

Located in `env-templates/`:

- `database-intelligence.env` - Master template with all options
- `*-minimal.env` - Minimal templates for quick setup
- Individual database templates for backward compatibility
EOF

    echo -e "${GREEN}✓ Index files created${NC}"
}

# Function to create final project structure visualization
create_structure_viz() {
    echo -e "${YELLOW}Creating project structure visualization...${NC}"
    
    cat > PROJECT_STRUCTURE.md << 'EOF'
# Database Intelligence - Project Structure

## Directory Layout

```
database-intelligence-restructured/
│
├── configs/                    # Configuration files
│   ├── *-maximum-extraction.yaml    # Database-specific configs
│   ├── env-templates/              # Environment variable templates
│   └── INDEX.md                    # Configuration guide
│
├── scripts/                    # All executable scripts
│   ├── validation/            # Validation tools
│   │   ├── validate-all.sh
│   │   ├── validate-config.sh
│   │   ├── validate-metrics.sh
│   │   └── validate-e2e.sh
│   │
│   ├── testing/              # Test execution
│   │   ├── run-tests.sh
│   │   ├── test-database-config.sh
│   │   ├── benchmark-performance.sh
│   │   └── check-metric-cardinality.sh
│   │
│   ├── building/             # Build scripts
│   │   ├── build-collector.sh
│   │   └── build-ci.sh
│   │
│   ├── deployment/           # Deployment tools
│   │   ├── start-all-databases.sh
│   │   └── stop-all-databases.sh
│   │
│   ├── maintenance/          # Maintenance utilities
│   │   ├── cleanup-archives.sh
│   │   ├── consolidate-docs.sh
│   │   └── reorganize-project.sh
│   │
│   └── INDEX.md              # Script documentation
│
├── docs/                      # Documentation
│   ├── guides/               # User guides
│   │   ├── QUICK_START.md
│   │   ├── CONFIGURATION.md
│   │   ├── DEPLOYMENT.md
│   │   ├── TROUBLESHOOTING.md
│   │   └── *_MAXIMUM_GUIDE.md
│   │
│   ├── reference/            # Technical reference
│   │   ├── ARCHITECTURE.md
│   │   ├── METRICS.md
│   │   └── API.md
│   │
│   ├── development/          # Developer docs
│   │   ├── SETUP.md
│   │   ├── TESTING.md
│   │   └── TEST_REPORT.md
│   │
│   └── consolidated/         # Merged documentation
│       └── *.md
│
├── tests/                     # Test suites
│   ├── unit/                 # Unit tests
│   ├── integration/          # Integration tests
│   ├── e2e/                  # End-to-end tests
│   ├── performance/          # Performance tests
│   ├── fixtures/             # Test data
│   ├── utils/               # Test utilities
│   └── README.md
│
├── deployments/              # Deployment configurations
│   ├── docker/              # Docker files
│   ├── kubernetes/          # K8s manifests
│   └── examples/            # Example deployments
│
├── development/              # Development resources
│   └── scripts/             # Development scripts
│
├── docker-compose.databases.yml  # Multi-database setup
├── .env.example                  # Environment template
├── README.md                     # Main documentation
└── PROJECT_STRUCTURE.md          # This file
```

## Key Files

### Configuration
- Database configs: `configs/*-maximum-extraction.yaml`
- Environment: `.env.example` → `.env`
- Docker: `docker-compose.databases.yml`

### Scripts
- Validation: `scripts/validate-all.sh`
- Testing: `scripts/testing/run-tests.sh`
- Deployment: `scripts/deployment/start-all-databases.sh`

### Documentation
- Quick Start: `docs/guides/QUICK_START.md`
- Troubleshooting: `docs/guides/TROUBLESHOOTING.md`
- Architecture: `docs/reference/ARCHITECTURE.md`

## Workflows

### 1. Initial Setup
```bash
cp .env.example .env
./scripts/validate-all.sh
```

### 2. Development
```bash
./scripts/testing/test-database-config.sh postgresql
./scripts/testing/run-tests.sh unit
```

### 3. Deployment
```bash
./scripts/deployment/start-all-databases.sh
./scripts/testing/benchmark-performance.sh postgresql
```

### 4. Maintenance
```bash
./scripts/maintenance/cleanup-archives.sh
./scripts/validate-all.sh
```
EOF

    echo -e "${GREEN}✓ Project structure documented${NC}"
}

# Function to create quick reference card
create_quick_ref() {
    echo -e "${YELLOW}Creating quick reference card...${NC}"
    
    cat > QUICK_REFERENCE.md << 'EOF'
# Database Intelligence - Quick Reference

## Essential Commands

### Setup
```bash
# Initial setup
cp .env.example .env
vim .env  # Add your credentials

# Validate everything
./scripts/validate-all.sh
```

### Testing
```bash
# Run all tests
./scripts/testing/run-tests.sh all

# Test specific database
./scripts/testing/test-database-config.sh postgresql

# Performance test
./scripts/testing/benchmark-performance.sh mysql 300
```

### Deployment
```bash
# Start all databases
./scripts/deployment/start-all-databases.sh

# Start single database
docker-compose -f docker-compose.databases.yml up -d postgres-collector

# Check metrics
curl http://localhost:8888/metrics
```

### Validation
```bash
# Validate configuration
./scripts/validation/validate-config.sh postgresql

# Check metrics
./scripts/validation/validate-metrics.sh

# Full E2E validation
./scripts/validation/validate-e2e.sh
```

## Environment Variables

### Required
- `NEW_RELIC_LICENSE_KEY` - Your New Relic license key
- `{DATABASE}_HOST` - Database hostname
- `{DATABASE}_PASSWORD` - Database password

### Optional
- `DEPLOYMENT_MODE` - Default: `config_only_maximum`
- `OTEL_LOG_LEVEL` - Default: `info`

## File Locations

- **Configs**: `configs/*-maximum-extraction.yaml`
- **Env Templates**: `configs/env-templates/`
- **Scripts**: `scripts/{category}/*.sh`
- **Docs**: `docs/guides/`, `docs/reference/`
- **Tests**: `tests/{unit,integration,e2e}/`

## Common Issues

### No metrics appearing
```bash
# Check collector logs
docker logs otel-collector-postgresql

# Validate config
./scripts/validation/validate-config.sh postgresql
```

### High memory usage
```bash
# Check cardinality
./scripts/testing/check-metric-cardinality.sh postgresql

# Adjust memory limits in config
```

### Connection errors
```bash
# Test database connection
docker exec otel-collector-postgresql nc -zv $POSTGRESQL_HOST $POSTGRESQL_PORT

# Check credentials in .env
```

## Support

- Troubleshooting: `docs/guides/TROUBLESHOOTING.md`
- Architecture: `docs/reference/ARCHITECTURE.md`
- Database Guides: `docs/guides/*_MAXIMUM_GUIDE.md`
EOF

    echo -e "${GREEN}✓ Quick reference created${NC}"
}

# Main execution
echo -e "${YELLOW}Starting final cleanup...${NC}\n"

# Run all cleanup functions
organize_scripts
clean_empty_dirs
fix_permissions
create_indexes
create_structure_viz
create_quick_ref

# Final summary
echo -e "\n${BLUE}=== Final Cleanup Complete ===${NC}"
echo -e "${GREEN}✓ Scripts organized into categories${NC}"
echo -e "${GREEN}✓ Empty directories removed${NC}"
echo -e "${GREEN}✓ Permissions fixed${NC}"
echo -e "${GREEN}✓ Index files created${NC}"
echo -e "${GREEN}✓ Project structure documented${NC}"
echo -e "${GREEN}✓ Quick reference created${NC}"

echo -e "\n${YELLOW}New documentation created:${NC}"
echo "  - PROJECT_STRUCTURE.md - Visual directory layout"
echo "  - QUICK_REFERENCE.md - Essential commands"
echo "  - scripts/INDEX.md - Script documentation"
echo "  - configs/INDEX.md - Configuration guide"

echo -e "\n${BLUE}Project is now fully organized and documented!${NC}"