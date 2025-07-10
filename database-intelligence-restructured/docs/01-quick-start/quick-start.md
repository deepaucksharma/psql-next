## Quick Start Guide for E2E Tests

# Quick Start Guide for E2E Tests

This guide will help you quickly set up and run the end-to-end tests for the Database Intelligence project.

## Prerequisites

- Docker and Docker Compose installed
- Go 1.21+ installed
- New Relic account with API access

## Step 1: Configure Credentials

Since you have already added the credentials to `.env`, the tests will automatically load them.

To verify your `.env` file has the required variables:

```bash
# Check if .env exists and has New Relic credentials
grep "NEW_RELIC" .env
```

You should see:
- `NEW_RELIC_LICENSE_KEY`
- `NEW_RELIC_USER_KEY`
- `NEW_RELIC_ACCOUNT_ID`

## Step 2: Initial Setup

```bash
# Run the setup (builds collector and prepares environment)
make setup

# Verify New Relic connection
make verify
```

## Step 3: Run Tests

### Quick Test (5 minutes)
```bash
# Start databases and run basic tests
make quick-test
```

### Comprehensive Test Suite (20 minutes)
```bash
# Run the comprehensive E2E test
make test-comprehensive
```

### New Relic Verification (15 minutes)
```bash
# Verify data accuracy in NRDB
make test-verification
```

### All Tests (45 minutes)
```bash
# Run all test suites
make test
```

## Step 4: View Results

```bash
# Generate coverage report
make coverage

# Open coverage report in browser
open coverage/coverage.html
```

## Common Commands

```bash
# Start test infrastructure
make docker-up

# View docker logs
make docker-logs

# Stop and clean up
make clean
```

## Test Specific Components

```bash
# Test specific processor
make test-processor-adaptivesampler
make test-processor-circuitbreaker

# Test specific database
make test-postgres
make test-mysql
```

## Troubleshooting

### 1. New Relic Connection Failed

```bash
# Verify credentials
make verify

# Check if API key is set correctly
echo $NEW_RELIC_API_KEY
```

### 2. Docker Issues

```bash
# Reset Docker environment
make docker-down
docker system prune -f
make docker-up
```

### 3. Test Timeouts

```bash
# Run with extended timeout
TEST_TIMEOUT=60m make test
```

### 4. View Collector Logs

```bash
# During test execution
docker logs e2e-otel-collector -f
```

## Quick Development Workflow

```bash
# 1. Set up development environment
make dev-setup

# 2. Make changes to code

# 3. Run quick tests
make quick-test

# 4. Run specific test suite
make test-adapters

# 5. Clean up when done
make dev-teardown
```

## CI/CD Integration

For CI/CD pipelines:

```bash
# Run all tests suitable for CI
make ci-test
```

## Next Steps

1. Review test results and coverage
2. Check New Relic dashboard for exported metrics
3. Read the full [README.md](README.md) for detailed documentation
4. Add your own test cases as needed

## Support

If you encounter issues:
1. Check the logs: `make docker-logs`
2. Verify environment: `make verify`
3. Review configuration in `.env`
4. See troubleshooting in [README.md](README.md)
