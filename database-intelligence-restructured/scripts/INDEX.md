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
