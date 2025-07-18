# Shell Scripts Reorganization Plan

## 🎯 Goal
Consolidate and organize ~150+ shell scripts into a clean, maintainable structure with clear purposes and no duplicates.

## 📊 Current State Analysis

### Problems Identified:
1. **Duplicate Scripts**: Multiple copies of build.sh, test.sh, and other scripts
2. **Scattered Organization**: Similar scripts in different directories
3. **Overlapping Directories**: `scripts/` vs `development/scripts/`
4. **Legacy Scripts**: Many archived scripts that are no longer needed
5. **Inconsistent Naming**: No clear naming conventions

### Script Count by Directory:
- Root: 8 scripts
- .ci/scripts/: 2 scripts
- development/scripts/: 15 scripts
- scripts/: 40+ scripts (multiple subdirectories)
- tests/e2e/: 20+ scripts
- archive directories: 50+ scripts

## 🏗️ Proposed New Structure

```
database-intelligence/
├── scripts/                    # All operational scripts
│   ├── build/                 # Build-related scripts
│   │   ├── build.sh          # Main build script (from .ci/scripts/)
│   │   └── README.md         # Build documentation
│   │
│   ├── test/                  # Testing scripts
│   │   ├── run-tests.sh      # Unified test runner
│   │   ├── unit.sh           # Unit tests
│   │   ├── integration.sh    # Integration tests
│   │   ├── e2e.sh            # E2E tests (calls tests/e2e/)
│   │   ├── performance.sh    # Performance tests
│   │   └── README.md         # Testing documentation
│   │
│   ├── deploy/                # Deployment scripts
│   │   ├── docker.sh         # Docker deployment
│   │   ├── kubernetes.sh     # K8s deployment
│   │   ├── start-services.sh # Start all services
│   │   ├── stop-services.sh  # Stop all services
│   │   └── README.md         # Deployment documentation
│   │
│   ├── dev/                   # Development utilities
│   │   ├── setup.sh          # Development environment setup
│   │   ├── fix-modules.sh    # Fix Go module issues
│   │   ├── lint.sh           # Code linting
│   │   ├── format.sh         # Code formatting
│   │   └── README.md         # Development documentation
│   │
│   ├── maintain/              # Maintenance scripts
│   │   ├── cleanup.sh        # General cleanup
│   │   ├── validate.sh       # Validation tasks
│   │   ├── update-deps.sh    # Update dependencies
│   │   └── README.md         # Maintenance documentation
│   │
│   └── utils/                 # Shared utilities
│       ├── common.sh         # Common functions
│       ├── colors.sh         # Color definitions
│       └── logging.sh        # Logging utilities
│
├── tests/                     # Test-specific scripts stay here
│   └── e2e/                  # E2E test implementation
│       ├── run_e2e_tests.sh  # Keep as-is
│       └── ...               # Other E2E scripts
│
└── .ci/                      # CI/CD specific only
    └── workflows/            # GitHub Actions workflows
```

## 📋 Consolidation Actions

### 1. Build Scripts
- **Keep**: `.ci/scripts/build.sh` (most comprehensive)
- **Remove**: 
  - `/build.sh` (duplicate)
  - `/development/scripts/build-collector.sh` (subset functionality)
  - `/scripts/building/build-collector.sh` (duplicate)
  - All archive build scripts

### 2. Test Scripts
- **Merge**: `scripts/testing/run-tests.sh` + `scripts/testing/test.sh` → `scripts/test/run-tests.sh`
- **Move**: Integration tests to `scripts/test/integration.sh`
- **Keep**: E2E scripts in `tests/e2e/` (they're well-organized)
- **Remove**: Duplicate test scripts in archives

### 3. Maintenance Scripts
- **Consolidate**: All fix-*.sh scripts into categorized maintenance scripts
- **Merge**: Similar cleanup scripts
- **Remove**: One-time migration scripts that have been completed

### 4. Development Scripts
- **Merge**: `development/scripts/` into appropriate categories
- **Remove**: Duplicate functionality
- **Keep**: Unique development utilities

## 🔄 Migration Steps

### Phase 1: Create New Structure
```bash
# Create new directory structure
mkdir -p scripts/{build,test,deploy,dev,maintain,utils}

# Create README files
for dir in build test deploy dev maintain utils; do
  touch scripts/$dir/README.md
done
```

### Phase 2: Move and Consolidate
1. Move build.sh from .ci/scripts/ to scripts/build/
2. Consolidate test scripts into scripts/test/
3. Move deployment scripts to scripts/deploy/
4. Organize development utilities in scripts/dev/
5. Move maintenance scripts to scripts/maintain/

### Phase 3: Update References
1. Update Makefile to use new paths
2. Update CI/CD workflows
3. Update documentation
4. Update CLAUDE.md with new structure

### Phase 4: Clean Up
1. Remove duplicate scripts
2. Archive old scripts with explanation
3. Remove empty directories
4. Update .gitignore if needed

## ✅ Benefits

1. **Clear Organization**: Each directory has a specific purpose
2. **No Duplicates**: Single source of truth for each script
3. **Easy Discovery**: Developers can find scripts quickly
4. **Better Maintenance**: Related scripts are together
5. **Consistent Naming**: Clear, descriptive names

## 📊 Success Metrics

- **Before**: ~150+ scripts across 10+ directories
- **After**: ~40-50 scripts in 6 organized directories
- **Reduction**: ~70% fewer scripts
- **Clarity**: 100% of scripts have clear purpose and location

## 🚨 Risks and Mitigation

1. **Breaking Changes**: 
   - Mitigation: Create symlinks temporarily
   - Update all references systematically

2. **Lost Functionality**:
   - Mitigation: Review each script before removal
   - Keep backups in archive with explanation

3. **CI/CD Disruption**:
   - Mitigation: Test in branch first
   - Update CI/CD configs carefully

## 📅 Timeline

- **Day 1**: Create new structure and consolidate build scripts
- **Day 2**: Consolidate test scripts
- **Day 3**: Organize deployment and development scripts
- **Day 4**: Clean up maintenance scripts and archives
- **Day 5**: Update all references and documentation

---

**Status**: Ready for implementation
**Estimated Effort**: 5 days
**Impact**: High - Significant improvement in maintainability