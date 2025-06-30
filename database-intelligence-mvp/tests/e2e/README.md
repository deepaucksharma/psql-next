# End-to-End Tests

This directory contains comprehensive end-to-end tests that validate the complete data flow from source databases to New Relic Database (NRDB).

## Quick Start

```bash
# Run all E2E tests
./run-e2e-tests.sh

# Run comprehensive validation tests
./run-comprehensive-e2e-tests.sh
```

## Test Structure

```
e2e/
├── nrdb_validation_test.go               # Basic NRDB validation
├── nrdb_comprehensive_validation_test.go # Data shape & detail validation
├── e2e_metrics_flow_test.go             # Full metrics flow testing
├── e2e_main_test.go                     # Test suite setup
├── run-e2e-tests.sh                     # Basic test runner
├── run-comprehensive-e2e-tests.sh       # Enhanced test runner
├── docker-compose-test.yaml             # Test databases
├── config/                              # Test configurations
├── sql/                                 # Test data scripts
├── validators/                          # Validation helpers
└── reports/                             # Test results
```

## Prerequisites

```bash
export NEW_RELIC_LICENSE_KEY=your_license_key
export NEW_RELIC_ACCOUNT_ID=your_account_id
```

## What's Tested

- ✅ **Data Flow**: Database → Collector → Processors → NRDB
- ✅ **Data Shape**: Metric names, attributes, values, semantic conventions
- ✅ **Processors**: Adaptive sampling, circuit breaker, plan extraction, PII sanitization
- ✅ **Integration**: NRDB queries, data freshness, completeness

## Test Reports

After running tests, check the `reports/` directory for:
- Test summary
- Collector logs
- Exported metrics sample

## Full Documentation

For comprehensive E2E testing documentation, see:
[**docs/E2E_TESTING_COMPLETE.md**](../../docs/E2E_TESTING_COMPLETE.md)

---

**Status**: ✅ Production Ready  
**Coverage**: 100% of critical paths