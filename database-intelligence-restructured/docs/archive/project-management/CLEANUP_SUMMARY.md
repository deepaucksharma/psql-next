# Cleanup Summary

## Files and Directories Archived

### 1. Distributions
Moved to `archive/distributions-legacy/`:
- `distributions/minimal/`
- `distributions/production/`
- `distributions/enterprise/`

### 2. Deployments
Moved to `archive/deployments-legacy/`:
- `Dockerfile.enterprise`
- `Dockerfile.loadgen`
- `Dockerfile.production`
- `Dockerfile.standard`
- `compose/` directory (old compose files)

### 3. Configs
Moved to `archive/`:
- `configs/modes/`
- `configs/environments/`
- `configs/base/` (already archived)
- `configs/examples/` (already archived)

### 4. Removed
- `deployments/docker/archive/` (redundant)
- `deployments/docker/dockerfiles/` (test files)

## Current Clean Structure

```
distributions/
├── unified/          # New unified distribution
└── README.md

deployments/
└── docker/
    ├── Dockerfile    # Single multi-stage Dockerfile
    ├── compose.yaml  # Base compose file
    ├── compose.override/
    │   ├── dev.yaml
    │   └── prod.yaml
    └── init-scripts/
        ├── postgres-init.sql
        └── mysql-init.sql

configs/
├── base.yaml         # Consolidated base config
├── profiles/         # Distribution profiles
│   ├── minimal.yaml
│   ├── standard.yaml
│   ├── enterprise.yaml
│   └── overlays/
├── examples.yaml     # All examples
└── STRUCTURE.md
```

## Archive Location

All legacy files are preserved in:
```
archive/
├── distributions-legacy/
│   ├── minimal/
│   ├── production/
│   └── enterprise/
├── deployments-legacy/
│   ├── Dockerfile.*
│   └── compose/
├── modes/
├── environments/
└── [other archived items]
```

## Updated Files
- `go.work` - Removed references to archived distributions
- All legacy files moved to archive for reference

The cleanup is complete. The project now has a streamlined structure with all legacy files safely archived.