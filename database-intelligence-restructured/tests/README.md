# Testing Infrastructure

This directory contains all testing tools, frameworks, and test suites for the Database Intelligence project.

## Directory Structure

```
tests/
├── tools/              # Testing utilities and tools
│   ├── load-generator/     # Database load generation tool
│   ├── postgres-test-generator/  # PostgreSQL test data generator
│   ├── validation/         # OHI compatibility validator
│   └── minimal-db-check/   # Minimal database connectivity checker
├── e2e/               # End-to-end testing framework
│   ├── framework/         # Test framework utilities
│   ├── configs/          # Test-specific configurations
│   └── cmd/              # Test execution commands
├── fixtures/          # Test data and fixtures
├── dashboard-validation/  # Dashboard validation tools
└── archive/           # Historical test configurations
```

## Testing Tools

### Load Generator (`tools/load-generator/`)
Generates realistic database load for testing:
```bash
cd tests/tools/load-generator
go run main.go --database=postgres --connections=10 --duration=5m
```

### PostgreSQL Test Generator (`tools/postgres-test-generator/`)
Creates test data for PostgreSQL databases:
```bash
cd tests/tools/postgres-test-generator  
go run main.go --host=localhost --database=testdb
```

### Validation Tools (`tools/validation/`)
Validates OHI compatibility and metrics accuracy:
```bash
cd tests/tools/validation
go run ohi-compatibility-validator.go
```

### Minimal DB Check (`tools/minimal-db-check/`)
Basic database connectivity verification:
```bash
cd tests/tools/minimal-db-check
go run minimal_db_check.go --host=localhost --port=5432
```

## E2E Testing Framework

The `e2e/` directory contains comprehensive end-to-end testing:
- **Framework**: Reusable testing utilities and helpers
- **Configs**: Test-specific collector configurations
- **Commands**: Test execution and validation tools

## Usage Patterns

### Development Testing
```bash
# Run unit tests
make test

# Run with test database
make test-integration

# Run E2E tests
make test-e2e
```

### Load Testing
```bash
# Generate database load
cd tests/tools/load-generator
go run main.go --config=../../configs/load-test.yaml
```

### Validation Testing
```bash
# Validate metrics accuracy
cd tests/tools/validation
go run . --config=../../e2e/configs/validation-test.yaml
```

## Test Data Management

- **Fixtures**: Static test data in `fixtures/`
- **Generators**: Dynamic test data creation tools
- **Cleanup**: Automated cleanup after test runs

This consolidation provides a unified testing infrastructure with clear organization and easy discovery of testing tools.
EOF < /dev/null
This consolidation provides a unified testing infrastructure with clear organization and easy discovery of testing tools.
