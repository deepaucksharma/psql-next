# E2E Test Cleanup Plan

## Current State Analysis

### Redundant Files to Remove

#### 1. **Redundant Configuration Files**
- `collector-simple.yaml` - Superseded by final-comprehensive-config.yaml
- `comprehensive-test-config.yaml` - Early version with errors
- `correct-processor-config.yaml` - Intermediate fix attempt
- `processor-test-config.yaml` - Old test config
- `test-processor-config.yaml` - Duplicate test config
- `working-processor-config.yaml` - Intermediate working version
- `working-final-config.yaml` - Another intermediate version
- `simple-working-config.yaml` - Partial config for testing
- `final-processor-config.yaml` - Superseded by final-comprehensive-config.yaml

#### 2. **Config Directory Redundancy**
- `config/e2e-test-collector-basic.yaml` - Duplicate functionality
- `config/e2e-test-collector-local.yaml` - Duplicate functionality
- `config/e2e-test-collector-minimal.yaml` - Duplicate functionality
- `config/e2e-test-collector-simple.yaml` - Duplicate functionality
- `config/working-test-config.yaml` - Old working config

#### 3. **Redundant Scripts**
- `run-processor-tests.sh` - Functionality included in run-unified-e2e.sh
- `run-simple-e2e.sh` - Simplified version, not needed with unified runner

#### 4. **Testdata Redundancy**
- `testdata/collector-e2e-config.yaml` - Old config
- `testdata/config-monitoring.yaml` - Partial config
- `testdata/config-newrelic.yaml` - Partial config
- `testdata/custom-processors-e2e.yaml` - Old processor config
- `testdata/e2e-collector.yaml` - Old collector config
- `testdata/full-e2e-collector.yaml` - Old full config
- `testdata/simple-e2e-collector.yaml` - Old simple config
- `testdata/simple-real-e2e-collector.yaml` - Old real config

#### 5. **Log Files**
- `e2e-test-output.log` - Old test output
- All other `.log` files from testing

## Final Unified Structure

### Keep and Enhance:
```
tests/e2e/
├── README.md                           # Main E2E documentation
├── run-e2e-tests.sh                   # Unified test runner (renamed from run-unified-e2e.sh)
├── docker-compose.yml                 # Main docker compose (renamed from docker-compose.e2e.yml)
├── Makefile                           # E2E test targets
│
├── config/
│   ├── collector-config.yaml          # Main collector config (from final-comprehensive-config.yaml)
│   ├── unified_test_config.yaml       # Test orchestration config
│   └── e2e-test-collector.yaml        # Keep as reference config
│
├── framework/                         # Test framework (keep as is)
│   ├── interfaces.go
│   └── types.go
│
├── orchestrator/                      # Test orchestrator (keep as is)
│   └── main.go
│
├── testdata/                         # Test data and fixtures
│   ├── docker-compose.test.yml       # Alternative test setup
│   └── sql/                         # SQL fixtures
│
├── validators/                       # Validators (keep as is)
│   ├── metric_validator.go
│   └── nrdb_validator.go
│
├── workloads/                       # Workload generators (keep as is)
│   ├── database_setup.go
│   ├── query_templates.go
│   └── workload_generator.go
│
├── scripts/
│   └── lib/
│       └── common.sh               # Common functions
│
└── docs/
    ├── E2E_TESTS_DOCUMENTATION.md  # Comprehensive documentation
    └── E2E_TEST_RESULTS.md         # Latest test results
```

## Actions to Take

1. Archive redundant files to `archive/cleanup-20250703/`
2. Rename key files for clarity
3. Consolidate configurations into single source of truth
4. Update documentation to reflect new structure
5. Create simple Makefile targets for common operations