# Critical Cleanup Tasks

## Immediate Actions Required

### 1. Fix Import Issues (170+ files affected)
Every Go file needs import cleanup. Run:
```bash
goimports -w .
```
Or manually with go fmt:
```bash
go fmt ./...
```

### 2. Remove Orphaned Test Files
These test files have no corresponding source files:
```bash
# All integration tests (no implementation)
rm -rf tests/integration/

# All benchmark tests (no implementation)
rm -rf tests/benchmarks/

# All performance tests (no implementation)  
rm -rf tests/performance/

# Orphaned E2E test suites
rm tests/e2e/suites/adapter_integration_test.go
rm tests/e2e/suites/newrelic_verification_test.go
# ... (and others listed in report)
```

### 3. Remove Unused Files
```bash
# Empty files
rm test-compile.go

# Duplicate collectors
rm simple-working-collector.go
rm minimal-working-collector.go
rm basic-collector.go
rm -rf working-collector/

# Test configs
rm distributions/enterprise/test-config.yaml
rm distributions/production/test-*.yaml
```

### 4. Remove Unused Internal Packages
These packages have no references:
```bash
# If confirmed unused:
rm -rf core/internal/database/
rm -rf core/internal/secrets/
rm -rf core/internal/health/
rm -rf core/internal/ratelimit/
rm -rf core/internal/conventions/
rm -rf core/internal/performance/
```

### 5. Consolidate Duplicate Code

#### Main Functions (37 duplicates)
- Keep only necessary distribution main.go files
- Remove duplicate implementations

#### Component Functions (13 duplicates)
- Create single shared components package
- Reference from all distributions

### 6. Address TODO Comments
Review and resolve:
- `processors/verification/processor.go:630` - Context handling
- `processors/nrerrormonitor/processor.go:296` - Context handling
- `tests/e2e/suites/adapter_integration_test.go:545` - Prometheus metrics

## Verification After Cleanup

1. **Build all distributions**:
```bash
make build-all
```

2. **Run remaining tests**:
```bash
go test ./...
```

3. **Check for broken imports**:
```bash
go mod tidy
```

## Expected Impact

- **Code reduction**: ~30-40% fewer files
- **Clarity**: Clear separation of used vs unused code
- **Maintainability**: Easier to understand project structure
- **Build time**: Faster builds with fewer files

## Priority Order

1. **HIGH**: Fix imports (blocking issue)
2. **HIGH**: Remove orphaned tests 
3. **MEDIUM**: Remove unused internal packages
4. **MEDIUM**: Consolidate duplicate code
5. **LOW**: Clean up test configurations