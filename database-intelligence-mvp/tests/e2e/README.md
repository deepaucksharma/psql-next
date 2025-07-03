# E2E Tests - Database Intelligence MVP

End-to-end testing suite for the Database Intelligence MVP with all custom processors.

## Quick Start

```bash
# Run all E2E tests
./run-e2e-tests.sh

# Run with specific options
./run-e2e-tests.sh --quick      # Quick validation (10 min)
./run-e2e-tests.sh --full       # Full test suite (60 min)
./run-e2e-tests.sh --security   # Security and compliance tests
./run-e2e-tests.sh --performance # Performance tests

# Using Make
make e2e-test                    # Run standard E2E tests
make e2e-test-quick             # Quick validation
make e2e-test-full              # Full comprehensive test
```

## Test Structure

```
tests/e2e/
├── run-e2e-tests.sh           # Main test runner
├── docker-compose.yml         # Test environment setup
├── Makefile                   # Make targets for testing
│
├── config/
│   ├── collector-config.yaml  # Main collector configuration with all processors
│   ├── unified_test_config.yaml # Test orchestration configuration
│   └── e2e-test-collector.yaml  # Reference configuration
│
├── framework/                 # Test framework interfaces and types
├── orchestrator/             # Go-based test orchestrator
├── validators/               # Metric and data validators
├── workloads/               # Database workload generators
│
└── testdata/                # Test fixtures and data
    ├── docker-compose.test.yml
    └── sql/                 # Database initialization scripts
```

## Processors Tested

All 7 custom processors are tested:

1. **Verification** (Logs) - Data quality and health monitoring
2. **Adaptive Sampler** (Logs) - Intelligent sampling based on cost
3. **Circuit Breaker** (Logs) - Failure protection and recovery
4. **Plan Attribute Extractor** (Logs) - Query plan extraction
5. **Query Correlator** (Metrics) - Query to database correlation
6. **NR Error Monitor** (Metrics) - New Relic error detection
7. **Cost Control** (Metrics & Logs) - Cost monitoring and control

## Configuration

The main collector configuration (`config/collector-config.yaml`) includes:
- All receivers (PostgreSQL, MySQL, SQLQuery)
- All processors properly organized by pipeline type
- Debug and Prometheus exporters
- Health check extension

## Requirements

- Docker and Docker Compose
- Go 1.21+ (for building custom collector)
- PostgreSQL and MySQL containers (provided by docker-compose)
- 4GB+ RAM recommended
- Ports: 5432, 3306, 8889, 8890, 13133

## Test Results

See [E2E_TEST_RESULTS.md](E2E_TEST_RESULTS.md) for the latest test execution results.

## Documentation

See [E2E_TESTS_DOCUMENTATION.md](E2E_TESTS_DOCUMENTATION.md) for comprehensive documentation including:
- Architecture details
- Test suite descriptions
- Troubleshooting guide
- Best practices

## Troubleshooting

### Common Issues

1. **Port conflicts**: Stop conflicting services or adjust ports in docker-compose.yml
2. **Build failures**: Run `go mod tidy` in the project root
3. **Processor errors**: Check processor configurations match the implementation

### Debug Commands

```bash
# Check collector logs
docker logs e2e-otel-collector

# Check metrics endpoint
curl http://localhost:8890/metrics

# Check health endpoint
curl http://localhost:13133/health

# View detailed processor logs
docker logs e2e-otel-collector 2>&1 | grep -E "processor|pipeline"
```