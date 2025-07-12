# Code Cleanup Guide

## Review Summary

A comprehensive code review identified several areas needing cleanup:

### Key Findings

1. **Import Issues**: 170+ files have unused imports
2. **Test Organization**: 40+ test files without corresponding implementations
3. **Code Duplication**: Multiple identical main() and components() functions
4. **Unused Code**: Some internal packages may be underutilized

### Cleanup Strategy

#### Phase 1: Import Cleanup (Immediate)
```bash
# Install goimports
go install golang.org/x/tools/cmd/goimports@latest

# Fix all imports
goimports -w .

# Alternative: use go fmt
go fmt ./...
```

#### Phase 2: Test Rationalization
- Keep E2E tests that are actively used
- Remove orphaned test suites without implementations
- Consolidate test utilities into shared packages

#### Phase 3: Code Consolidation
- Merge duplicate main.go implementations where possible
- Create shared component registries
- Extract common utilities

### Safe Cleanup List

These can be safely removed:
- `test-compile.go` (empty file)
- Test configuration files (test-*.yaml)
- Duplicate working collectors

### Requires Review

Before removing, verify these are truly unused:
- Internal packages (some are used by receivers)
- Test suites (may be planned for future)
- Example configurations (may be documentation)

### Best Practices Going Forward

1. **Regular Cleanup**: Run goimports before commits
2. **Test Hygiene**: Remove tests when removing features
3. **Documentation**: Mark experimental or planned code
4. **Code Reviews**: Check for unused imports and dead code

### Verification Steps

After any cleanup:
```bash
# Ensure everything builds
make build-all

# Run all tests
make test

# Check module dependencies
go mod tidy

# Verify no broken imports
go list ./...
```

This cleanup will improve code maintainability while preserving all functional code.