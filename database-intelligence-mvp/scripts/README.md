# Scripts Directory - Database Intelligence MVP

Organized collection of utility scripts for building, deploying, monitoring, and maintaining the Database Intelligence MVP.

## Directory Structure

```
scripts/
├── README.md                    # This file
├── init-env.sh                  # Environment setup (core)
├── preflight-check.sh          # Pre-deployment checks (core)
├── compare-modes.sh             # Interactive mode comparison (core)
├── lib/
│   └── common.sh               # Shared utilities library
├── build/
│   ├── build-custom-collector.sh  # Custom collector builder
│   └── generate-config.sh         # Dynamic config generator
├── deployment/
│   ├── docker-start.sh            # Docker environment startup
│   ├── deploy-database-dashboard.sh # Dashboard deployment
│   └── generate-mtls-certs.sh     # Certificate generation
├── monitoring/
│   ├── check-metrics.sh           # New Relic metrics checker
│   ├── verify-metrics.sh          # Metrics verification
│   ├── verify-newrelic-integration.sh # NR integration check
│   ├── verify-collected-metrics.js    # JS metrics verification
│   ├── create-database-dashboard.js   # Dashboard creation
│   └── generate-architecture-diagram.py # Visual diagrams
├── maintenance/
│   ├── fix-configs.sh             # Unified config fixer (NEW)
│   ├── cleanup-configs.sh         # Configuration cleanup
│   ├── cleanup-scripts.sh         # Script cleanup
│   └── organize-documentation.sh  # Documentation organizer
├── testing/
│   ├── validate-all.sh            # Comprehensive validation
│   ├── validate-e2e.sh           # End-to-end testing
│   ├── validate-e2e-setup.sh     # E2E setup validation
│   ├── validate-env.sh           # Environment validation
│   ├── validate-project-consistency.sh # Project validation
│   └── generate-db-load.sh       # Test data generator
├── database/
│   ├── mysql-init.sql            # MySQL initialization
│   └── postgres-init.sql         # PostgreSQL initialization
└── archive/
    ├── sql-duplicates-20250703/   # Archived duplicate SQL files
    └── config-fix-scripts-20250703/ # Archived old fix scripts
```

## Core Scripts (Root Level)

### `init-env.sh`
Interactive environment setup with validation.
```bash
./init-env.sh
```

### `preflight-check.sh`
Pre-deployment system validation.
```bash
./preflight-check.sh
```

### `compare-modes.sh`
Interactive mode comparison tool.
```bash
./compare-modes.sh
```

## Build Scripts

### `build/build-custom-collector.sh`
Builds OpenTelemetry collector with custom processors.
```bash
./build/build-custom-collector.sh
```

### `build/generate-config.sh`
Generates dynamic configurations based on environment.
```bash
./build/generate-config.sh [template] [output]
```

## Deployment Scripts

### `deployment/docker-start.sh`
Starts complete Docker Compose environment.
```bash
./deployment/docker-start.sh
```

### `deployment/deploy-database-dashboard.sh`
Deploys New Relic dashboard using NerdGraph API.
```bash
./deployment/deploy-database-dashboard.sh
```

### `deployment/generate-mtls-certs.sh`
Generates mTLS certificates for secure communication.
```bash
./deployment/generate-mtls-certs.sh
```

## Monitoring Scripts

### `monitoring/check-metrics.sh`
Queries New Relic for collected metrics.
```bash
./monitoring/check-metrics.sh
```

### `monitoring/verify-metrics.sh`
Verifies metrics collection and forwarding.
```bash
./monitoring/verify-metrics.sh
```

### `monitoring/verify-newrelic-integration.sh`
Comprehensive New Relic integration verification.
```bash
./monitoring/verify-newrelic-integration.sh
```

### `monitoring/create-database-dashboard.js`
Creates New Relic dashboard via NerdGraph API.
```bash
node monitoring/create-database-dashboard.js
```

### `monitoring/generate-architecture-diagram.py`
Generates visual architecture diagrams.
```bash
python monitoring/generate-architecture-diagram.py
```

## Maintenance Scripts

### `maintenance/fix-configs.sh` (NEW - Unified)
Unified configuration fix script with multiple modes.
```bash
# Quick fixes (env vars, memory)
./maintenance/fix-configs.sh quick

# Critical fixes (default)
./maintenance/fix-configs.sh critical

# Comprehensive fixes  
./maintenance/fix-configs.sh all
```

### `maintenance/cleanup-configs.sh`
Archives unused configuration files.
```bash
./maintenance/cleanup-configs.sh [--dry-run]
```

### `maintenance/cleanup-scripts.sh`
Archives unused scripts.
```bash
./maintenance/cleanup-scripts.sh [--dry-run]
```

## Testing Scripts

### `testing/validate-all.sh`
Comprehensive validation framework.
```bash
./testing/validate-all.sh [mode]
```

### `testing/validate-e2e.sh`
End-to-end pipeline testing.
```bash
./testing/validate-e2e.sh
```

### `testing/validate-env.sh`
Environment and .env file validation.
```bash
./testing/validate-env.sh
```

### `testing/generate-db-load.sh`
Generates test data and queries.
```bash
./testing/generate-db-load.sh
```

## Database Scripts

### `database/mysql-init.sql`
Comprehensive MySQL initialization with monitoring user, sample data, and procedures.

### `database/postgres-init.sql`
Comprehensive PostgreSQL initialization with extensions, monitoring user, and sample data.

## Common Library

### `lib/common.sh`
Shared utility functions used by all scripts:
- Logging functions (`log_info`, `log_error`, `log_success`)
- Environment loading
- Database testing utilities
- Validation functions

## Usage Patterns

### Quick Start
```bash
# 1. Setup environment
./init-env.sh

# 2. Run preflight checks
./preflight-check.sh

# 3. Start environment
./deployment/docker-start.sh

# 4. Validate setup
./testing/validate-all.sh
```

### Maintenance Workflow
```bash
# Fix configurations
./maintenance/fix-configs.sh critical

# Clean up unused files
./maintenance/cleanup-configs.sh --dry-run
./maintenance/cleanup-scripts.sh --dry-run

# Validate changes
./testing/validate-project-consistency.sh
```

### Monitoring Workflow
```bash
# Check metrics collection
./monitoring/verify-metrics.sh

# Verify New Relic integration
./monitoring/verify-newrelic-integration.sh

# Create dashboard
./monitoring/deploy-database-dashboard.sh
```

## Dependencies

### Required Tools
- **bash** (4.0+)
- **Docker** and **docker-compose**
- **Go** (1.21+)
- **curl** and **jq**
- **yq** (recommended)

### Optional Tools
- **Node.js** (for JavaScript scripts)
- **Python** (for diagram generation)
- **OpenSSL** (for certificate generation)

## Recent Changes

### Consolidated (2025-07-03)
- **Unified fix-configs.sh**: Combined 3 separate fix scripts into one with modes
- **Organized structure**: Moved scripts into logical subdirectories
- **Archived duplicates**: Removed redundant SQL initialization scripts
- **Enhanced documentation**: Comprehensive usage examples

### Quality Improvements
- All scripts use shared `lib/common.sh` utilities
- Consistent error handling and logging
- Comprehensive validation and testing
- Backup creation for destructive operations

## Contributing

When adding new scripts:
1. Place in appropriate subdirectory
2. Use `lib/common.sh` for shared functionality
3. Follow existing naming conventions
4. Include usage documentation
5. Add validation and error handling

## Support

For script issues:
1. Check script documentation (this file)
2. Run with debug: `bash -x ./script-name.sh`
3. Verify dependencies and permissions
4. Check logs in script output