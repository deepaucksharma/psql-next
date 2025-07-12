# Script and Configuration Migration Guide

This guide explains the consolidation of scripts and configurations in the Database Intelligence project.

## What Changed

### Scripts Consolidation
All scripts have been organized into a clear directory structure:

```
scripts/
├── build/      # All build-related scripts
├── test/       # All test scripts  
├── deploy/     # Deployment scripts
└── utils/      # Shared utilities
```

### Old → New Script Mappings

| Old Script | New Location | Makefile Target |
|------------|--------------|-----------------|
| `build.sh` | `scripts/build/build.sh` | `make build` |
| `build-complete-collector.sh` | `scripts/build/build.sh production` | `make build` |
| `build-with-all-components.sh` | `scripts/build/build.sh all` | `make build-all` |
| `test.sh` | `scripts/test/test.sh` | `make test` |
| `start-collector.sh` | Use Makefile | `make run` |
| `send-test-data.sh` | Moved to archive | Use test suite |

### Configuration Structure

```
configs/
├── base.yaml           # Base configuration template
├── modes/              # Different operational modes
│   ├── config-only.yaml
│   └── enhanced.yaml
├── environments/       # Environment-specific overlays
│   ├── dev.yaml
│   ├── staging.yaml
│   └── production.yaml
└── tests/              # Test configurations
```

## Quick Command Reference

### Building
```bash
# Old way
./build.sh

# New way
make build                    # Build production
make build-all               # Build all distributions
make build-minimal           # Build minimal only
```

### Testing
```bash
# Old way
./test.sh

# New way
make test                    # Run all tests
make test-unit              # Unit tests only
make test-e2e               # E2E tests only
make test-coverage          # With coverage report
```

### Running
```bash
# Old way
./start-collector.sh

# New way
make run                    # Build and run
make run-debug             # Run with debug logging
make run-config-only       # Run config-only mode
```

### Docker Operations
```bash
# Old way
cd deployments/docker && docker-compose up

# New way
make docker-up             # Start environment
make docker-down           # Stop environment
make docker-logs           # View logs
make docker-build          # Build images
```

### Deployment
```bash
# Old way
Various scripts in different locations

# New way
make deploy                # Deploy to production
make deploy-staging        # Deploy to staging
make deploy-k8s           # Deploy to Kubernetes
make deploy-binary        # Deploy as service
```

## Environment Variables

All environment variables remain the same. Use `.env` files:
- `.env` - Default/development
- `.env.production` - Production
- `.env.staging` - Staging
- `.env.test` - Testing

## Getting Help

View all available commands:
```bash
make help
```

## Benefits of Consolidation

1. **Single Entry Point**: Use `make` for all operations
2. **Consistent Interface**: Same patterns across all operations
3. **Better Documentation**: Built-in help for all commands
4. **Reduced Duplication**: One script per function
5. **Clear Organization**: Scripts organized by lifecycle phase

## Migration Steps

1. Update your CI/CD pipelines to use `make` commands
2. Update documentation to reference new locations
3. Update any custom scripts that call old scripts
4. Remove old scripts from your local environment

## Archived Scripts

Old scripts have been moved to `docs/archive/scripts/` for reference.