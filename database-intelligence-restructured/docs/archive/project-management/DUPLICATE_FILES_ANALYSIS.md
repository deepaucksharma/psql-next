# Duplicate Files Analysis Report

## Summary
After searching the entire codebase, I've identified numerous duplicate and redundant files that can be cleaned up to simplify the project structure.

## 1. Duplicate Shell Scripts

### Test Scripts (Highly Redundant)
The following test scripts appear to serve similar purposes and should be consolidated:

**E2E Test Runners (Multiple versions of the same functionality):**
- `/database-intelligence-mvp/tests/e2e/run-e2e-tests.sh` (current)
- `/database-intelligence-mvp/tests/e2e/archive/legacy-scripts-20250703/run-e2e-tests.sh` (archived)
- `/database-intelligence-mvp/tests/e2e/archive/legacy-scripts-20250703/run_e2e_tests.sh` (archived)
- `/database-intelligence-mvp/tests/e2e/archive/legacy-scripts-20250703/run-comprehensive-e2e-tests.sh` (archived)
- `/database-intelligence-mvp/tests/e2e/archive/legacy-scripts-20250703/run_comprehensive_e2e_tests.sh` (archived)
- `/database-intelligence-mvp/tests/e2e/archive/legacy-runners-20250702/run_comprehensive_tests.sh` (archived)
- `/database-intelligence-mvp/tests/e2e/archive/legacy-runners-20250702/run_working_e2e_tests.sh` (archived)

**Recommendation:** Keep only `/database-intelligence-mvp/tests/e2e/run-e2e-tests.sh` and remove all archived versions.

### Validation Scripts
Multiple validation scripts exist in different locations:

**Database Intelligence MVP:**
- `/database-intelligence-mvp/scripts/testing/validate-e2e.sh`
- `/database-intelligence-mvp/scripts/testing/validate-all.sh`
- `/database-intelligence-mvp/scripts/testing/validate-env.sh`
- `/database-intelligence-mvp/scripts/testing/validate-project-consistency.sh`

**Database Intelligence Restructured:**
- `/database-intelligence-restructured/scripts/validate-e2e.sh`
- `/database-intelligence-restructured/scripts/validate-config.sh`
- `/database-intelligence-restructured/scripts/validate-metrics.sh`
- `/database-intelligence-restructured/development/scripts/validate-metrics-e2e.sh`

**Archived Duplicates:**
- `/database-intelligence-restructured/docs/archive/scripts/maintenance/validate-*.sh` (5 files)
- `/database-intelligence-mvp/tests/e2e/archive/legacy-miscellaneous-20250702/validate*.sh` (2 files)

**Recommendation:** Consolidate validation functionality into a single directory under `/database-intelligence-restructured/scripts/validation/`

### Build Scripts
Multiple build scripts with similar functionality:

- `/database-intelligence-mvp/scripts/build/build-custom-collector.sh`
- `/database-intelligence-restructured/development/scripts/build-collector.sh`
- `/database-intelligence-restructured/docs/archive/scripts/build*.sh` (4 files)
- `/database-intelligence-restructured/tests/e2e/archive/scripts/build*.sh` (3 files)

**Recommendation:** Keep only `/database-intelligence-restructured/development/scripts/build-collector.sh`

## 2. Configuration Files

### Collector Configurations
There are 33+ collector YAML files, many serving similar purposes:

**Main configs to keep:**
- `/database-intelligence-mvp/config/collector.yaml` (main config)
- `/database-intelligence-mvp/config/collector-secure.yaml`
- `/database-intelligence-mvp/config/collector-gateway-enterprise.yaml`

**Redundant test configs:**
- `collector-e2e-test.yaml` and `collector-end-to-end-test.yaml` (same purpose)
- `collector-local-test.yaml` and `collector-minimal-test.yaml` (similar purpose)
- `collector-simple-alternate.yaml` and `collector-simplified.yaml` (similar purpose)

**Recommendation:** Reduce to 5-6 core configurations with clear naming.

### Docker Compose Files
26 docker-compose files exist, many are duplicates:

**Keep:**
- `/database-intelligence-restructured/docker-compose.databases.yml`
- `/database-intelligence-mvp/docker-compose.yml` (main)
- `/database-intelligence-mvp/docker-compose.production.yml`
- `/database-intelligence-mvp/docker-compose.secure.yml`

**Remove:**
- All archived docker-compose files in `/archive/` directories
- Test-specific compose files that duplicate main functionality

## 3. Documentation Files

### Architecture Documentation (Heavy Duplication)
- `/database-intelligence-mvp/ARCHITECTURE_REVIEW.md`
- `/database-intelligence-mvp/ARCHITECTURE_SUMMARY.md`
- `/database-intelligence-mvp/COMPREHENSIVE_ARCHITECTURE_REVIEW.md`
- `/database-intelligence-mvp/docs/ARCHITECTURE.md`
- `/database-intelligence-mvp/docs/ENTERPRISE_ARCHITECTURE.md`
- `/database-intelligence-restructured/docs/reference/ARCHITECTURE.md`

**Recommendation:** Consolidate into a single `/docs/ARCHITECTURE.md`

### E2E Testing Documentation (Multiple versions)
- `/database-intelligence-mvp/E2E_TESTS_DOCUMENTATION.md`
- `/database-intelligence-mvp/tests/e2e/E2E_TESTS_DOCUMENTATION.md`
- `/database-intelligence-mvp/tests/e2e/archive/cleanup-20250703/docs/E2E_TESTS_COMPREHENSIVE_DOCUMENTATION.md`

**Recommendation:** Keep only one comprehensive E2E documentation file

### Migration Guides
- `/database-intelligence-mvp/docs/MIGRATION_GUIDE.md`
- `/database-intelligence-mvp/docs/OHI_MIGRATION_GUIDE.md`
- Multiple migration-related files in strategy directories

**Recommendation:** Consolidate into `/docs/guides/MIGRATION.md`

## 4. README Files
39 README files exist throughout the project, many in archive directories:

**Essential READMEs to keep:**
- Root README in each main directory
- `/database-intelligence-restructured/README.md`
- `/database-intelligence-mvp/README.md`

**Remove:**
- All README files in archive directories
- Redundant README files in subdirectories that don't add value

## 5. Archive Directories

The following archive directories contain outdated code and should be cleaned up:

- `/database-intelligence-mvp/tests/e2e/archive/` (contains 7+ subdirectories of old test files)
- `/database-intelligence-restructured/archive/` (old deployment and distribution files)
- `/database-intelligence-restructured/docs/archive/` (outdated documentation)
- `/database-intelligence-restructured/tests/archive/` (old test files)

**Recommendation:** Review archive directories and remove files older than 3 months or create a single archive at the project root.

## 6. Duplicate Functionality Across Projects

The `database-intelligence-mvp` and `database-intelligence-restructured` directories contain significant overlap:

- Both have their own test suites
- Both have validation scripts
- Both have deployment configurations
- Both have documentation

**Recommendation:** Since `database-intelligence-restructured` appears to be the newer version, consider:
1. Migrating any unique functionality from `mvp` to `restructured`
2. Archiving the entire `database-intelligence-mvp` directory
3. Maintaining only `database-intelligence-restructured` going forward

## Summary Statistics

- **Total Shell Scripts:** 168 (at least 50% appear redundant)
- **Docker Compose Files:** 26 (could be reduced to 4-5)
- **Collector Config Files:** 33+ (could be reduced to 5-6)
- **README Files:** 39 (could be reduced to 10-12)
- **Archive Directories:** 12+ (should be consolidated or removed)

## Recommended Actions

1. **Immediate cleanup:** Remove all files in archive directories older than 2 months
2. **Consolidate test scripts:** Merge all E2E test runners into a single, well-documented script
3. **Standardize configurations:** Create a clear hierarchy of base, development, and production configs
4. **Unify documentation:** Merge similar documentation files and remove outdated versions
5. **Choose one project structure:** Decide between `mvp` and `restructured` and migrate accordingly

This cleanup would significantly simplify the codebase and make it easier to maintain and understand.