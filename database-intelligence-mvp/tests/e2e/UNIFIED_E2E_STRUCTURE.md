# Unified E2E Test Structure

## Overview

The E2E testing framework has been consolidated into a clean, unified structure that supports comprehensive testing of all 7 custom processors in the Database Intelligence MVP.

## Directory Structure

```
tests/e2e/
├── run-e2e-tests.sh           # Main test runner script
├── docker-compose.yml         # Test environment setup
├── Makefile                   # Make targets for easy test execution
├── README.md                  # Quick start guide
│
├── config/                    # Test configurations
│   ├── collector-config.yaml  # Main collector config with all processors
│   ├── e2e-test-collector.yaml # Alternative test configuration
│   └── unified_test_config.yaml # Test orchestration configuration
│
├── framework/                 # Test framework
│   ├── interfaces.go         # Core test interfaces
│   └── types.go             # Test types and structures
│
├── orchestrator/             # Test orchestration
│   └── main.go              # Go-based test orchestrator
│
├── validators/               # Test validators
│   ├── metric_validator.go   # Metric validation
│   └── nrdb_validator.go    # New Relic DB validation
│
├── workloads/               # Database workload generators
│   ├── database_setup.go    # Database initialization
│   ├── query_templates.go   # Query templates
│   └── workload_generator.go # Workload generation
│
├── processor_tests/         # Processor-specific tests
│   └── adaptive_sampler_e2e_test.go
│
├── scripts/                 # Supporting scripts
│   └── lib/
│       └── common.sh       # Common shell functions
│
├── testdata/               # Test data and fixtures
│   ├── docker-compose.test.yml
│   ├── Dockerfile.test
│   ├── init-postgres-e2e.sql
│   ├── init-mysql-e2e.sql
│   ├── init-test-db.sql
│   └── nr-mock-expectations.json
│
└── archive/                # Archived old implementations
    ├── cleanup-20250703/   # Latest cleanup archive
    └── legacy-configs-20250702/
```

## Test Execution Methods

### 1. Using Make Targets (Recommended)

```bash
make test                    # Run all E2E tests
make test-unit              # Run unit tests only
make test-integration       # Run integration tests
make test-performance       # Run performance tests
make test-benchmark         # Run benchmarks
make test-coverage          # Run with coverage report
make docker-up              # Start test environment
make docker-down            # Stop test environment
make clean                  # Clean test artifacts
```

### 2. Direct Script Execution

```bash
# Quick validation
./run-e2e-tests.sh --quick

# Full test suite
./run-e2e-tests.sh --full

# Security tests
./run-e2e-tests.sh --security

# Performance tests
./run-e2e-tests.sh --performance
```

### 3. Docker Compose

```bash
# Start test environment
docker-compose -f docker-compose.yml up -d

# Run tests
./run-e2e-tests.sh

# Stop environment
docker-compose -f docker-compose.yml down
```

## Processors Tested

All 7 custom processors are tested in their appropriate pipelines:

### Logs Pipeline
1. **Verification** - Data quality and health monitoring
2. **Adaptive Sampler** - Intelligent sampling based on cost
3. **Circuit Breaker** - Failure protection and recovery
4. **Plan Attribute Extractor** - Query plan extraction

### Metrics Pipeline
5. **Query Correlator** - Query to database correlation
6. **NR Error Monitor** - New Relic error detection

### Both Pipelines
7. **Cost Control** - Cost monitoring and control

## Key Features

1. **Unified Test Runner**: Single script (`run-e2e-tests.sh`) handles all test scenarios
2. **Comprehensive Configuration**: All processors configured in `config/collector-config.yaml`
3. **Multiple Test Modes**: Quick, Full, Security, Performance
4. **Docker Environment**: Complete test environment with PostgreSQL and MySQL
5. **Go Test Framework**: Structured test framework with interfaces and validators
6. **Make Targets**: Easy-to-use make commands for common operations
7. **Clean Documentation**: Updated README and comprehensive test documentation

## Documentation

- **README.md**: Quick start guide
- **E2E_TESTS_DOCUMENTATION.md**: Comprehensive documentation
- **E2E_TEST_RESULTS.md**: Latest test execution results
- **COMPREHENSIVE_E2E_TEST_SUITE.md**: Detailed test suite documentation

## Cleanup Summary

### Removed/Archived
- Redundant configuration files (20+ YAML files consolidated)
- Old test scripts (run-unified-e2e.sh renamed to run-e2e-tests.sh)
- Duplicate docker-compose configurations
- Stale test implementations

### Retained
- Essential test configurations (3 main configs)
- Unified test runner script
- Test framework and validators
- Database initialization scripts
- Processor-specific tests

## Next Steps

The E2E testing framework is now unified and ready for:
1. Running comprehensive tests across all processors
2. Adding new test cases as needed
3. Performance optimization testing
4. CI/CD integration
5. Production readiness validation