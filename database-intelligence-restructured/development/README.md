# Development Tools and Utilities

This directory contains development tools, utility scripts, and helpers for working with the Database Intelligence project.

## Directory Structure

```
development/
├── scripts/            # Development and utility scripts
│   ├── common.sh           # Common shell functions
│   ├── fix-go-versions.sh  # Go version standardization
│   ├── fix-otel-dependencies.sh  # OpenTelemetry version fixes
│   ├── cleanup-duplicates.sh     # Remove duplicate files
│   └── [other utility scripts]
└── tools/              # Development tools and helpers
```

## Available Scripts

### Version Management
- `fix-go-versions.sh` - Standardize Go versions across all modules
- `fix-otel-dependencies.sh` - Fix OpenTelemetry version conflicts

### Code Quality
- `cleanup-duplicates.sh` - Remove duplicate files and consolidate
- `common.sh` - Shared functions for other scripts

### Project Maintenance
- Various utility scripts for project maintenance and automation

## Usage

All scripts are designed to be run from the project root:

```bash
# Fix version conflicts
development/scripts/fix-otel-dependencies.sh

# Clean up duplicates
development/scripts/cleanup-duplicates.sh

# Standardize Go versions
development/scripts/fix-go-versions.sh
```

## Contributing

When adding new development tools:
1. Place scripts in `development/scripts/`
2. Place compiled tools in `development/tools/` 
3. Use `common.sh` for shared functionality
4. Document usage in this README

This directory is for development-time tools only. Runtime scripts belong in `scripts/`.