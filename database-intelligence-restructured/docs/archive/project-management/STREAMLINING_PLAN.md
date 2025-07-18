# Streamlining Plan for Database Intelligence

## Overview
This plan applies Phase 1 consolidation principles to streamline configs/, distributions/, and deployments/ directories.

## Phase 1 Principles Applied
1. **Module Consolidation**: Combine related modules under single go.mod
2. **Configuration Simplification**: Base + overlay pattern
3. **Clear Separation**: Archive legacy files, keep only essential ones

## Implementation Plan

### 1. Distributions Consolidation (Highest Priority)
**Current**: 3 separate distributions (minimal, production, enterprise)
**Target**: 1 unified distribution with build profiles

#### Changes:
- Create single `distributions/unified/` directory
- Use build tags for profile-specific code
- Single binary with `--profile` flag
- Archive existing 3 distributions

#### Structure:
```
distributions/
├── unified/
│   ├── main.go           # Single entry point
│   ├── components.go     # All components with build tags
│   ├── profiles/         # Profile configurations
│   │   ├── minimal.go    
│   │   ├── standard.go   
│   │   └── enterprise.go 
│   └── go.mod
└── archive/
    └── (current distributions)
```

### 2. Deployments Consolidation
**Current**: 4 Dockerfiles, multiple compose files, duplicate scripts
**Target**: 1 multi-stage Dockerfile, unified compose structure

#### Changes:
- Single Dockerfile with build arguments
- Base compose.yaml with environment overlays
- Consolidate 5 init scripts to 2 (postgres.sql, mysql.sql)

#### Structure:
```
deployments/
├── docker/
│   ├── Dockerfile         # Multi-stage with ARG BUILD_PROFILE
│   ├── compose.yaml       # Base configuration
│   └── compose.override/  # Environment-specific
└── archive/
```

### 3. Configs Consolidation
**Current**: 5 subdirectories with overlapping configs
**Target**: Simplified structure with clear hierarchy

#### Changes:
- Merge base/ components into single base.yaml
- Combine modes/ with distribution profiles
- Consolidate 5 example files into 1

#### Structure:
```
configs/
├── base.yaml            # Single base configuration
├── profiles/            # Distribution profiles
│   └── overlays/        # Environment overlays
├── examples.yaml        # All examples in one file
└── archive/
```

## Expected Results
- **Code Reduction**: ~60% less duplication
- **Build Simplification**: 3 builds → 1 parameterized build
- **Maintenance**: Single point of updates
- **Consistency**: Aligned with Phase 1 patterns

## Implementation Order
1. Week 1: Distributions (Create unified distribution)
2. Week 2: Deployments (Consolidate Docker/compose)
3. Week 3: Configs (Reorganize and archive)