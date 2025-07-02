# Stale and Unused Code Analysis

Date: 2025-07-01

## Summary of Findings

### 1. Backup and Temporary Files (DELETE)
- `config-backup-20250630-221430/` - Old backup directory
- `config-backup-20250630-221800/` - Old backup directory  
- `config-backup-20250630-221817/` - Old backup directory
- `config/collector-local-test.yaml.fixing` - Temporary file
- `config/collector-local-test.yaml.fixing.bak` - Backup file

### 2. Duplicate or Similar Configuration Files (CONSOLIDATE)
- `config/collector-resilient.yaml` and `config/collector-resilient-fixed.yaml` - Very similar, keep fixed version
- `config/collector-local-test.yaml`, `config/simple-test.yaml`, `config/test-config.yaml` - Multiple test configs
- `config/collector-simple-alternate.yaml` - Appears to be an alternative to simplified config

### 3. Unused Processors/Components
- **waitanalysis processor** - Only referenced in `config/collector-ash.yaml`, not in main.go
- **ASH receiver** - Was removed from `receivers/` but config still exists (`config/collector-ash.yaml`)

### 4. Example Files (REVIEW)
- `.env.example` and `env.example` - Two different example env files (not duplicates)
- `sample-query.log` - Sample log file, check if needed for tests

### 5. TODO Comments (LOW PRIORITY)
- `common/featuredetector/types.go:210` - TODO: Check minimum version
- `common/queryselector/selector.go:184` - TODO: Check version requirements

### 6. Potentially Unused Imports (VERIFY)
- `processors/costcontrol/processor.go` - "fmt" imported and not used
- `processors/nrerrormonitor/processor.go` - "plog" and "processor" imported and not used
- `processors/querycorrelator/factory.go` - "zap" imported and not used
- `processors/querycorrelator/processor.go` - "pcommon" imported and not used

### 7. Compilation Issues (FIX REQUIRED)
- Several processors have undefined types/functions that need fixing
- `CircuitBreaker` type is undefined in feature_aware.go
- Various factory method signature mismatches

## Recommendations

### Immediate Actions
1. **Delete backup directories and files**:
   ```bash
   rm -rf config-backup-*
   rm config/*.bak config/*.fixing
   ```

2. **Consolidate test configurations**:
   - Keep one comprehensive test config
   - Remove redundant test configs

3. **Remove unused ASH-related files**:
   ```bash
   rm config/collector-ash.yaml
   rm -rf processors/waitanalysis  # if it exists
   ```

4. **Fix compilation errors** in processors before they accumulate

### Code Quality Improvements
1. Remove unused imports from processor files
2. Consolidate similar configuration files
3. Add version checking logic where TODOs exist
4. Document why two .env.example files exist or consolidate them

### Documentation
- Update README to reference only active configuration files
- Document the purpose of each configuration variant
- Remove references to deleted components

## File Size Analysis
No empty or suspiciously small Go files were found, indicating the codebase doesn't have placeholder files.

## Impact Assessment
- **High Impact**: Compilation errors in processors need immediate attention
- **Medium Impact**: Duplicate configs cause confusion
- **Low Impact**: TODO comments and backup files

Total estimated cleanup: ~20-30 files can be removed or consolidated.