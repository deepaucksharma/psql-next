# Database Intelligence Test Suite

Comprehensive testing framework for Database Intelligence collectors.

## Test Structure

```
tests/
├── unit/          # Unit tests for individual components
├── integration/   # Integration tests with real databases
├── e2e/           # End-to-end tests with full pipeline
├── performance/   # Performance benchmarks and load tests
├── fixtures/      # Test data and configurations
└── utils/         # Shared test utilities
```

## Running Tests

### All Tests
```bash
./scripts/testing/run-tests.sh all
```

### Specific Test Type
```bash
# Unit tests only
./scripts/testing/run-tests.sh unit

# Integration tests
./scripts/testing/run-tests.sh integration postgresql

# End-to-end tests
./scripts/testing/run-tests.sh e2e

# Performance tests
./scripts/testing/run-tests.sh performance mysql
```

### Individual Test Suites
```bash
# Configuration validation
./scripts/validation/validate-config.sh

# Database-specific test
./scripts/testing/test-database-config.sh postgresql

# Performance benchmark
./scripts/testing/benchmark-performance.sh postgresql 300
```

## Writing Tests

### Unit Tests
Place unit tests in `tests/unit/` and follow the naming convention `test_*.sh`.

Example:
```bash
#!/bin/bash
source "$(dirname "$0")/../utils/common.sh"

# Test assertions
assert_equals "expected" "actual" "Test description"
assert_file_exists "path/to/file"
assert_contains "file.yaml" "pattern" "Should contain pattern"
```

### Integration Tests
Integration tests should:
1. Set up test environment
2. Run collector with test config
3. Verify metrics are collected
4. Clean up resources

### Performance Tests
Performance tests measure:
- Metric collection rate
- Memory usage
- CPU utilization
- Cardinality impact

## CI/CD Integration

The test suite is designed to run in CI/CD pipelines:

```yaml
# GitHub Actions example
- name: Run tests
  run: ./scripts/testing/run-tests.sh all
```

## Test Coverage

Current test coverage includes:
- ✅ Configuration validation
- ✅ Metric naming conventions
- ✅ Database connectivity
- ✅ Collector startup/shutdown
- ✅ Metric export verification
- ✅ Performance benchmarks
- ✅ Cardinality analysis
