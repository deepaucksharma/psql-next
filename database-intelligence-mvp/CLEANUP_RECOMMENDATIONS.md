# Database Intelligence MVP - Cleanup Recommendations

## Executive Summary

This document provides comprehensive cleanup recommendations after analyzing the entire codebase for stale, unreferenced, and unused code, configurations, and documentation.

## Critical Issues to Fix First

### 1. Build Breaking Issues

**main.go Function Name Mismatch**
- **Issue**: Line 20 calls `components()` but function is defined as `Components()` (capital C)
- **Fix**: Change line 20 to use `Components()` with capital C
- **Impact**: Build will fail without this fix

**Version Inconsistencies**
- **Issue**: ocb-config.yaml uses OTEL v0.127.0 while go.mod uses v0.128.0/v1.33.0
- **Fix**: Align all OTEL dependencies to use consistent versions
- **Impact**: Potential compatibility issues

### 2. Processor Registration Mismatch

**Issue**: main.go registers 4 processors but ocb-config.yaml only includes 1
- main.go imports: adaptivesampler, circuitbreaker, planattributeextractor, verification
- ocb-config.yaml includes: only planattributeextractor (others commented out)

**Fix Options**:
1. Remove unused processor imports from main.go, OR
2. Uncomment processors in ocb-config.yaml if they're needed

## Code Cleanup Recommendations

### 1. Unused Custom Components

**Processors** (Implemented but not used in builds):
- `processors/adaptivesampler/` - Commented out in ocb-config.yaml
- `processors/circuitbreaker/` - Commented out in ocb-config.yaml  
- `processors/verification/` - Commented out in ocb-config.yaml

**Recommendation**: Either:
- Enable these processors in ocb-config.yaml and use them in configurations, OR
- Move to archive/experimental directory if keeping for future use, OR
- Remove entirely if not needed

**Extensions**:
- `extensions/healthcheck/` - Custom extension not used in any configuration
- `extensions/pg_querylens/` - PostgreSQL C extension, separate from OTEL collector

**Recommendation**: Archive or remove if not actively used

### 2. Dependencies

**go-redis/redis/v8**
- Currently listed as indirect dependency but used directly in code
- **Fix**: Move to direct dependencies in go.mod

## Configuration Cleanup

### 1. Unused Configuration Files

**In config/ directory** (11 unused files):
```
- collector-minimal.yaml
- collector-telemetry.yaml  
- demo-config.yaml
- demo-simple.yaml
- pii-detection-enhanced.yaml
- production-demo.yaml
- test-custom-processors.yaml
- test-immediate-output.yaml
- test-logs-pipeline.yaml
- Files in config/examples/
```

**Recommendation**: Move to `config/archive/` or delete

### 2. Missing Referenced Configurations

**Files referenced but don't exist**:
```
- config/attribute-mapping.yaml
- config/collector-experimental.yaml
- config/collector-otel-first.yaml
- config/collector-unified.yaml
- config/collector-working.yaml
- config/collector-with-verification.yaml
```

**Recommendation**: Update docker-compose files to remove these references

### 3. Directory Structure

**Issue**: Two config directories (`config/` and `configs/`)
**Recommendation**: Consolidate into single `config/` directory with clear subdirectories:
```
config/
├── production/
├── development/
├── examples/
└── archive/
```

## Documentation Cleanup

### 1. Missing Critical Files

**Root README.md**
- **Issue**: No README.md in project root (deleted per git status)
- **Fix**: Create new README.md with project overview and link to docs/

### 2. Broken Links

**In docs/README.md**:
- Line 27: `./development/API.md` - DOES NOT EXIST
- Line 147: `./operations/MIGRATION.md` - DOES NOT EXIST

**Fix**: Create missing files or update links

### 3. Stale Documentation

**Files to remove/update**:
- `docs/README-OLD.md` - Conflicts with current docs/README.md
- `docs/archive/pre-validation-20250629/` - Contains 37+ outdated files
- `docs/archive/redundant-20250629/` - Contains 25+ redundant files

**Recommendation**: Move archive to separate branch or delete

### 4. References to Deleted Files

Update documentation that references these deleted files:
```
- Cargo.toml, Cargo.lock (Rust files)
- comprehensive-improvements-summary.md
- postgres-collector-deployment-patterns.md
- postgres-unified-collector-architecture.md
- enhanced-deployment-patterns.md
- implementation-roadmap.md
```

## Script Cleanup

### 1. Unused Scripts (15 files)

**Remove or archive these unreferenced scripts**:
```
scripts/
├── check-newrelic-data.sh
├── feedback-loop.sh
├── generate-test-load.sh
├── nerdgraph-verification.sh
├── run-with-verification.sh
├── send-test-logs.sh
├── test-nr-connection.sh
├── validate-entity-synthesis.sh
├── validate-ohi-parity.sh
├── validate-otel-metrics.sh
├── validate-prerequisites.sh
├── build_docker_image.sh
tests/
├── integration/test-experimental-components.sh
└── integration/validate-setup.sh
./test-query-logs.sh (in root)
```

### 2. Missing Referenced Script

**Makefile references**: `quickstart-enhanced.sh`
**Issue**: File doesn't exist
**Fix**: Remove reference from Makefile or create the script

## Archive Directory

**Issue**: Large archive directory with experimental/old content
**Size**: Contains numerous subdirectories and files
**Recommendation**: 
1. Move entire `archive/` to separate git branch: `git checkout -b archive-content`
2. Remove from main branch to reduce clutter

## Git Status Cleanup

**Files marked as deleted in git**:
- All Rust-related files (Cargo.toml, Cargo.lock, crates/)
- Various markdown documentation files
- Target directory

**Recommendation**: Complete the deletion with `git add -A && git commit`

## Implementation Priority

### High Priority (Build Breaking)
1. Fix main.go function name
2. Align OTEL versions
3. Fix processor registration mismatch
4. Create root README.md

### Medium Priority (Functionality)
1. Clean unused configurations
2. Fix missing script references
3. Update docker-compose files
4. Move go-redis to direct dependency

### Low Priority (Maintenance)
1. Archive unused processors/extensions
2. Clean documentation archives
3. Remove unused scripts
4. Consolidate config directories

## Verification After Cleanup

Run these commands to verify cleanup:
```bash
# Check build works
make build

# Verify no broken imports
go mod tidy
go mod verify

# Check for broken links in configs
grep -r "config/" docker-compose*.yml | grep -v "#"

# Verify documentation links
find docs -name "*.md" -exec grep -l "](.*md)" {} \;
```

## Estimated Impact

- **Code reduction**: ~30% fewer files
- **Documentation**: ~50% reduction in archived content
- **Configuration**: ~40% fewer config files
- **Scripts**: ~40% fewer shell scripts
- **Clarity**: Significantly improved project structure

This cleanup will make the project more maintainable and easier for new contributors to understand.