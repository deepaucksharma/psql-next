# Codebase Divergence Fix Checklist

## 🔴 Critical Fixes (Block Deployment)

### 1. Module Version Standardization
- [ ] Update all `go.mod` files to use `go 1.22.0` consistently
  ```bash
  find . -name "go.mod" -exec sed -i 's/^go 1\.[0-9]\+\(\.[0-9]\+\)\?$/go 1.22.0/' {} +
  ```

### 2. Remove Hardcoded Database Connections
- [ ] **tests/e2e/cmd/e2e_verification/main.go:64**
  ```go
  // Replace:
  dsn := "host=localhost port=5432 user=postgres password=postgres dbname=postgres sslmode=disable"
  // With:
  dsn := os.Getenv("E2E_POSTGRES_DSN")
  ```

### 3. Fix Context Propagation TODOs
- [ ] **components/processors/verification/processor.go:629**
- [ ] **components/processors/nrerrormonitor/processor.go:296**
  ```go
  // Add to processor struct:
  type verificationProcessor struct {
      // ... existing fields ...
      ctx    context.Context
      cancel context.CancelFunc
  }
  ```

## 🟡 High Priority Fixes (This Week)

### 4. Create Database Abstraction Interface
- [ ] Create `internal/database/interfaces.go`:
  ```go
  package database
  
  type Driver interface {
      Name() string
      Connect(ctx context.Context, dsn string) (Client, error)
      ParseDSN(dsn string) (*Config, error)
      SupportedFeatures() []Feature
  }
  
  type Client interface {
      Query(ctx context.Context, query string, args ...interface{}) (*Rows, error)
      Exec(ctx context.Context, query string, args ...interface{}) (Result, error)
      Close() error
      Ping(ctx context.Context) error
      Stats() Statistics
  }
  ```

### 5. Refactor Database-Specific Switch Statements
- [ ] **components/processors/planattributeextractor/processor.go:149**
- [ ] **components/receivers/ash/ash_sampler.go:47-49**
- [ ] **components/receivers/enhancedsql/receiver.go:108-114**
- [ ] **components/receivers/enhancedsql/receiver.go:657-677**

Replace with driver registry pattern:
```go
// internal/database/registry.go
var drivers = make(map[string]Driver)

func RegisterDriver(name string, driver Driver) {
    drivers[name] = driver
}

func GetDriver(name string) (Driver, error) {
    driver, ok := drivers[name]
    if !ok {
        return nil, fmt.Errorf("unknown database driver: %s", name)
    }
    return driver, nil
}
```

### 6. Add Missing Test Files
Create test files for components with 0% coverage:

- [ ] **components/exporters/nri/**
  - [ ] Create `exporter_test.go`
  - [ ] Create `writer_test.go`
  - [ ] Create `factory_test.go`

- [ ] **components/receivers/ash/**
  - [ ] Create `receiver_test.go`
  - [ ] Create `scraper_test.go`
  - [ ] Create `sampler_test.go`
  - [ ] Create `storage_test.go`

- [ ] **components/receivers/enhancedsql/**
  - [ ] Create `receiver_test.go`
  - [ ] Create `collect_test.go`

- [ ] **components/receivers/kernelmetrics/**
  - [ ] Create `receiver_test.go`
  - [ ] Create `scraper_test.go`

- [ ] **components/internal/boundedmap/**
  - [ ] Create `boundedmap_test.go`

## 🟢 Medium Priority Fixes (Next Sprint)

### 7. Update Legacy References
- [ ] **configs/base.yaml:419** - Remove "Legacy compatibility" comment
- [ ] **configs/base.yaml:525** - Remove "Legacy compatibility" comment
- [ ] **configs/base.yaml:685** - Update deprecated memory_ballast documentation
- [ ] **distributions/unified/README.md:72-74** - Update migration instructions

### 8. MongoDB & Redis E2E Implementation
- [ ] Create MongoDB E2E test structure:
  ```
  tests/e2e/databases/mongodb/
  ├── mongodb_test.go
  ├── workload.go
  ├── verifier.go
  └── docker-compose.yaml
  ```

- [ ] Create Redis E2E test structure:
  ```
  tests/e2e/databases/redis/
  ├── redis_test.go
  ├── workload.go
  ├── verifier.go
  └── docker-compose.yaml
  ```

### 9. Processor Database Support
Update processors to support MongoDB and Redis:

- [ ] **components/processors/adaptivesampler/processor.go**
  - [ ] Add MongoDB metric handling
  - [ ] Add Redis metric handling

- [ ] **components/processors/circuitbreaker/processor.go**
  - [ ] Add MongoDB error patterns
  - [ ] Add Redis error patterns

- [ ] **components/processors/costcontrol/processor.go**
  - [ ] Add MongoDB cost metrics
  - [ ] Add Redis cost metrics

- [ ] **components/processors/planattributeextractor/processor.go**
  - [ ] Add MongoDB query plan extraction
  - [ ] Add Redis command analysis

- [ ] **components/processors/querycorrelator/processor.go**
  - [ ] Add MongoDB correlation logic
  - [ ] Add Redis correlation logic

## 📋 Testing Verification Commands

### Check Go Version Consistency
```bash
find . -name "go.mod" -exec grep "^go " {} + | sort | uniq -c
```

### Find Remaining Hardcoded Values
```bash
grep -r "localhost\|127.0.0.1\|hardcoded" --include="*.go" . | grep -v "_test.go"
```

### Check Test Coverage
```bash
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | grep -E "(0.0%|[0-9]\.0%)" | head -20
```

### Verify Database Abstractions
```bash
grep -r "switch.*driver\|switch.*database" --include="*.go" components/
```

### Find Missing Test Files
```bash
for dir in $(find components -type d); do
  if ls $dir/*.go 2>/dev/null | grep -v _test.go | head -1 > /dev/null; then
    if ! ls $dir/*_test.go 2>/dev/null > /dev/null; then
      echo "Missing tests in: $dir"
    fi
  fi
done
```

## 🎯 Definition of Done

### For Each Component
- [ ] All hardcoded values removed
- [ ] Test coverage > 80%
- [ ] Database-specific logic abstracted
- [ ] Documentation updated
- [ ] E2E tests passing

### For Overall Codebase
- [ ] All Go modules use same version
- [ ] No TODOs in critical paths
- [ ] All processors support MongoDB/Redis
- [ ] E2E tests exist for all configured databases
- [ ] Zero hardcoded connection strings

## 📊 Progress Tracking

| Component | Coverage Before | Coverage After | Status |
|-----------|----------------|----------------|---------|
| exporters/nri | 0% | ___ | ⏳ |
| receivers/ash | 0% | ___ | ⏳ |
| receivers/enhancedsql | 0% | ___ | ⏳ |
| receivers/kernelmetrics | 0% | ___ | ⏳ |
| processors/* | 60% | ___ | ⏳ |
| MongoDB E2E | 0% | ___ | ⏳ |
| Redis E2E | 0% | ___ | ⏳ |

## 🚀 Quick Start for Developers

1. **Clone and setup**:
   ```bash
   git checkout -b fix/codebase-divergences
   go work sync
   ```

2. **Run verification**:
   ```bash
   make verify-all
   ```

3. **Fix issues**:
   - Start with critical fixes
   - Run tests after each fix
   - Update this checklist

4. **Submit PR**:
   - Reference this checklist
   - Show before/after metrics
   - Include test results