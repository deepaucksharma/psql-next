# YAML File Consolidation Summary

## Overview
This document summarizes the consolidation of YAML files across the Database Intelligence project.

## Consolidation Results

### Configuration Files (`configs/`)
- **Base Components**: `configs/base/` contains modular component definitions
  - `receivers.yaml` - All receiver configurations
  - `processors.yaml` - All processor configurations
  - `exporters.yaml` - All exporter configurations
  - `extensions.yaml` - All extension configurations

- **Operating Modes**: `configs/modes/`
  - `config-only.yaml` - Standard OTel components only
  - `enhanced.yaml` - Includes custom components

- **Environments**: `configs/environments/`
  - `development.yaml` - Development overrides
  - `staging.yaml` - Staging environment config
  - `production.yaml` - Production settings

- **Examples**: `configs/examples/`
  - Various working examples for different use cases
  - Docker compose configurations
  - Test configurations

### Build Configurations (`build-configs/`)
- `minimal.yaml` - Minimal distribution build
- `enhanced.yaml` - Full-featured build
- `legacy-builder.yaml` - Archived legacy config

### CI/CD Workflows (`ci-cd/`)
- `ci.yml` - Standard CI pipeline
- `ci-enhanced.yml` - Extended CI with more tests
- `e2e-tests.yml` - End-to-end test workflow
- `cd.yml` - Continuous deployment
- `release.yml` - Release automation

### Docker Compose (`deployments/docker/compose/`)
- `docker-compose.yaml` - Default development setup
- `docker-compose.prod.yaml` - Production deployment
- `docker-compose-databases.yaml` - Database-only setup
- `docker-compose-ha.yaml` - High availability configuration
- `docker-compose-parallel.yaml` - Parallel processing setup

### Test Configurations (`tests/e2e/configs/`)
- Organized test configurations for different scenarios
- Validation configurations in `validation/` subdirectory

## Removed Duplicates
1. Consolidated `dev.yaml` and `development.yaml` (kept `development.yaml`)
2. Consolidated `prod.yaml` and `production.yaml` (kept `production.yaml`)
3. Moved runtime configs to examples
4. Archived old CI workflows
5. Moved Taskfile.yml (using Makefile instead)

## Directory Cleanup
- Removed empty `runtime/` directory
- Removed empty `tools/ci/` tree
- Removed empty `tools/builder/` directory
- Consolidated docker compose configs

## Best Practices Applied
1. **Single Source of Truth**: Each configuration type has one canonical location
2. **Clear Hierarchy**: Base → Mode → Environment → Specific overrides
3. **Examples Separated**: Working examples in dedicated directory
4. **Archives Preserved**: Old configs moved to archive directories
5. **Consistent Naming**: All YAML files use `.yaml` extension (not `.yml`)

## Usage Guide

### For Developers
```bash
# Use enhanced mode locally
./database-intelligence-collector --config=configs/modes/enhanced.yaml

# Override with development settings
./database-intelligence-collector \
  --config=configs/modes/enhanced.yaml \
  --config=configs/environments/development.yaml
```

### For Production
```bash
# Config-only mode (recommended)
./database-intelligence-collector \
  --config=configs/modes/config-only.yaml \
  --config=configs/environments/production.yaml
```

### For Testing
```bash
# Run with test config
./database-intelligence-collector --config=tests/e2e/configs/collector-test.yaml
```

## Migration Notes
- All old configurations preserved in various `archive/` directories
- No breaking changes to existing deployments
- Environment variables still work as before