# Database Intelligence Scripts

This directory contains streamlined scripts for building, testing, and managing the database-intelligence project.

## Consolidated Scripts

After refactoring, we've consolidated all build and test functionality into two main scripts:

### 1. build.sh
Unified build script that handles all distribution types and build scenarios.

**Usage:**
```bash
./build.sh [MODE] [OPTIONS]

# Modes:
#   all         - Build all distributions (default)
#   minimal     - Build minimal distribution only
#   production  - Build production distribution only
#   enterprise  - Build enterprise distribution only
#   e2e         - Build E2E test collector with OCB
#   test        - Build and run component tests

# Examples:
./build.sh                    # Build all distributions
./build.sh production         # Build production only
./build.sh test              # Build and test all components
VERBOSE=true ./build.sh all  # Verbose build
```

### 2. test.sh
Unified test runner that handles all testing scenarios.

**Usage:**
```bash
./test.sh [MODE] [OPTIONS]

# Modes:
#   e2e           - Full E2E test with databases (default)
#   smoke         - Quick smoke test with minimal config
#   validate      - Validate project structure only
#   production    - Production-style test with monitoring
#   comprehensive - Run all test types

# Examples:
./test.sh                         # Run E2E test
./test.sh smoke                   # Quick smoke test
./test.sh validate               # Structure validation
KEEP_RUNNING=true ./test.sh e2e  # Keep services running after test
```

## Common Library

The `lib/common.sh` file contains shared functions used by both scripts:
- Color-coded logging functions
- Database management (start/stop/health checks)
- Prerequisites checking
- Environment file management
- Go workspace synchronization
- Report generation helpers

## Legacy Scripts

All previous individual scripts have been archived in `/archive/scripts/` for reference:
- Multiple build-*.sh scripts → consolidated into build.sh
- Multiple run-*.sh scripts → consolidated into test.sh
- Various fix-*.sh scripts → one-time fixes, no longer needed
- Utility scripts → functionality moved to common library

## Benefits of Consolidation

1. **Reduced Redundancy**: Common functions are now in one place
2. **Consistent Interface**: Both scripts use similar command patterns
3. **Better Maintenance**: Fewer scripts to maintain and update
4. **Preserved Functionality**: All features from original scripts are retained
5. **Improved Documentation**: Clear usage patterns and examples

## Migration Guide

If you were using the old scripts, here's how to migrate:

| Old Script | New Command |
|------------|-------------|
| build-and-test.sh | `./build.sh test` |
| build-minimal-collector.sh | `./build.sh minimal` |
| build-working-e2e-collector.sh | `./build.sh e2e` |
| run-complete-e2e-tests.sh | `./test.sh comprehensive` |
| run-simple-e2e-test.sh | `./test.sh smoke` |
| run-production.sh | `./test.sh production` |
| validate-e2e-structure.sh | `./test.sh validate` |