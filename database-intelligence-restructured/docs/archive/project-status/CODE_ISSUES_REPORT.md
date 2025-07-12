# Code Issues Report - Database Intelligence Restructured

## Summary

This report documents common code issues found in the database-intelligence-restructured codebase. The scan focused on identifying TODO/FIXME comments, deprecated functions, error handling issues, hardcoded values, skipped tests, security issues, unused code, and naming convention inconsistencies.

## 1. TODO/FIXME Comments (5 files found)

### Critical TODOs requiring attention:
- `/common/featuredetector/types.go:210` - TODO: Check minimum version
- `/common/queryselector/selector.go:184` - TODO: Check version requirements  
- `/processors/querycorrelator/processor_test.go:99,101` - TODO: Add query categories to config
- `/processors/costcontrol/processor_test.go:101` - TODO: Add high cardinality dimensions to config

## 2. Deprecated Function Usage (2 files found)

### Files with deprecated markers:
- `/processors/adaptivesampler/config.go:96` - File storage is deprecated, forcing in-memory mode
- `/processors/planattributeextractor/processor.go:380` - Warning for deprecated algorithms, defaulting to SHA-256

## 3. Error Handling Issues (14 files with ignored errors)

### Critical ignored errors found:
- `/processors/adaptivesampler/adaptive_algorithm.go` - Multiple parsing operations ignore errors:
  - Lines 240, 243, 246, 249, 255: `strconv.Parse*` calls with ignored errors
- `/tests/e2e/docker_mysql_test.go:213` - Ignored error when getting container logs

### Files with `_ =` pattern (potential ignored errors):
- 14 files total including test files and processors

## 4. Hardcoded Values (43 files found)

### Security-sensitive hardcoded values:
- **Test files with hardcoded credentials:**
  - Multiple test files use hardcoded passwords like "mysql", "postgres", "testpassword"
  - Docker test files hardcode database passwords in container setup
  - Example: `/tests/e2e/docker_mysql_test.go:38` - `MYSQL_ROOT_PASSWORD=mysql`

### Hardcoded network addresses:
- Localhost addresses (127.0.0.1, localhost) found in 43+ files
- Common ports hardcoded: 3306 (MySQL), 5432 (PostgreSQL), 8080, 4317 (OTLP)
- Should be made configurable via environment variables or config files

## 5. Skipped Tests (21 test files)

### Test files with skip conditions:
- Many E2E tests skip when credentials are not available
- Integration tests skip in short mode
- Performance tests have skip conditions
- Example: `/tests/e2e/suites/adapter_integration_test.go:30` - Skips in short mode

## 6. Security Issues

### Credential Management:
- 47 files contain references to passwords, secrets, tokens, or keys
- Most are in test files, but some production code handles credentials
- `/core/internal/secrets/manager.go` - Proper secrets management implementation exists
- Test files should use environment variables instead of hardcoded credentials

### Recommendations:
1. Move all test credentials to environment variables
2. Use the existing secrets manager for production credentials
3. Add credential scanning to CI/CD pipeline

## 7. Unused Imports and Dead Code

Unable to run comprehensive analysis due to tooling limitations, but manual inspection shows:
- Code appears well-maintained with minimal obvious dead code
- Import statements appear to be properly managed

## 8. Naming Convention Issues

### Inconsistencies found:
- Mix of exported and unexported types in processor packages
- Example: `planAttributeExtractor` (unexported) vs other exported processor types
- Generally follows Go conventions but some inconsistencies in processor implementations

## Recommendations

### High Priority:
1. **Address ignored errors** in `/processors/adaptivesampler/adaptive_algorithm.go` - Add proper error handling for parsing operations
2. **Remove hardcoded credentials** from test files - Use environment variables
3. **Fix TODO items** related to version checking and configuration

### Medium Priority:
1. **Refactor hardcoded network addresses** - Make configurable via config files
2. **Update deprecated function usage** - Replace with recommended alternatives
3. **Standardize naming conventions** - Ensure consistency across processor implementations

### Low Priority:
1. **Document skipped tests** - Add clear skip reasons and conditions for re-enabling
2. **Add linting rules** - Enforce consistent code style and catch common issues
3. **Regular code cleanup** - Schedule periodic reviews to address accumulated technical debt

## Next Steps

1. Create GitHub issues for each high-priority item
2. Add pre-commit hooks to catch common issues
3. Set up automated code quality checks in CI/CD
4. Schedule regular code review sessions focused on technical debt