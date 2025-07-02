# Test Fix Action Plan

## Immediate Fixes Required

### 1. Configuration Issues
Fix the collector configuration files to use correct processor names:

```yaml
# In config/collector-simplified.yaml and others
processors:
  # Change from:
  circuit_breaker:
  # To:
  circuitbreaker:
  
  # Remove or comment out:
  # health_check extension (not implemented)
```

### 2. Test Environment Consolidation

Create a single source of truth for test types:

```go
// tests/e2e/common_test.go
package e2e

// Single definition of TestEnvironment
type TestEnvironment struct {
    PostgresDB *sql.DB
    MySQLDB    *sql.DB
    // ... other fields
}

// Single definition of helper functions
func getEnvOrDefault(key, defaultValue string) string {
    // implementation
}
```

### 3. Fix SQL Query Receiver Configuration

Update sqlquery receiver configuration:

```yaml
receivers:
  sqlquery:
    driver: postgres
    datasource: "..."
    queries:
      - query: "SELECT ..."
        metrics:
          - metric_name: "..."
    # Move collection_interval to receiver level
    collection_interval: 60s
```

### 4. Update Test Dependencies

```bash
# Update go.mod
go get -u github.com/testcontainers/testcontainers-go@latest
go mod tidy
```

## Testing Strategy

### Phase 1: Unit Tests (Already Passing ✅)
```bash
# Run all processor tests
for p in processors/*; do
    echo "Testing $p"
    (cd "$p" && go test -v -count=1 .)
done
```

### Phase 2: Integration Tests
```bash
# After fixing testcontainers
cd tests/integration
go test -v -count=1 .
```

### Phase 3: E2E Tests
```bash
# Run simplified tests first
cd tests/e2e
go test -v -run TestSimplified ./simplified_e2e_test.go ./package_test.go

# Then run comprehensive tests
go test -v -run TestComprehensiveE2EFlow .
```

### Phase 4: Performance Tests
```bash
# Run benchmarks
go test -bench=. -benchtime=30s ./processors/...
```

## Validation Checklist

- [ ] All processor unit tests pass
- [ ] Configuration files are valid
- [ ] No duplicate type definitions
- [ ] All imports are used
- [ ] E2E tests can connect to databases
- [ ] Collector starts without errors
- [ ] Metrics are collected and processed
- [ ] No memory leaks under load
- [ ] PII is properly redacted
- [ ] Circuit breaker activates on failures

## Expected Outcomes

After applying these fixes:

1. **Build**: Clean compilation with no errors
2. **Unit Tests**: 36/36 tests passing
3. **Integration Tests**: All tests passing with real containers
4. **E2E Tests**: Complete flow validation working
5. **Performance**: <5ms processing overhead, <512MB memory

## Next Steps

1. Apply configuration fixes
2. Consolidate test packages
3. Run full test suite
4. Document any new issues found
5. Create CI/CD pipeline for automated testing