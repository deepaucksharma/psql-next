# Database Intelligence MVP - Project Restructuring Plan

## Executive Summary

The best approach is to adopt a **Monorepo with Well-Defined Module Boundaries** using Go workspaces. This maintains the benefits of code sharing while providing clear separation of concerns and independent versioning capabilities.

## Recommended Project Structure

```
database-intelligence/
├── go.work                          # Go workspace file
├── go.work.sum
├── Makefile                         # Root makefile
├── docker-compose.yml               # Development environment
├── README.md
│
├── core/                            # Core collector binary
│   ├── go.mod
│   ├── cmd/
│   │   └── collector/
│   │       └── main.go
│   ├── config/
│   │   └── default.yaml
│   └── internal/
│       └── builder/
│
├── processors/                      # Custom processors
│   ├── go.mod
│   ├── adaptivesampler/
│   ├── circuitbreaker/
│   ├── costcontrol/
│   ├── nrerrormonitor/
│   ├── planattributeextractor/
│   ├── querycorrelator/
│   ├── verification/
│   └── README.md
│
├── receivers/                       # Custom receivers
│   ├── go.mod
│   ├── ash/
│   ├── enhancedsql/
│   ├── kernelmetrics/
│   └── README.md
│
├── exporters/                       # Custom exporters
│   ├── go.mod
│   ├── nri/
│   └── README.md
│
├── extensions/                      # Extensions
│   ├── go.mod
│   ├── healthcheck/
│   ├── pg_querylens/               # C extension (separate build)
│   └── README.md
│
├── common/                          # Shared libraries
│   ├── go.mod
│   ├── featuredetector/
│   ├── queryselector/
│   ├── testutils/                   # Shared test utilities
│   └── types/                       # Common types/interfaces
│
├── distributions/                   # Pre-built distributions
│   ├── enterprise/                  # Full-featured build
│   │   ├── go.mod
│   │   ├── main.go
│   │   └── Dockerfile
│   ├── standard/                    # Standard build
│   │   ├── go.mod
│   │   ├── main.go
│   │   └── Dockerfile
│   └── minimal/                     # Minimal build
│       ├── go.mod
│       ├── main.go
│       └── Dockerfile
│
├── configs/                         # Configuration templates
│   ├── base/
│   ├── examples/
│   ├── overlays/
│   └── profiles/
│       ├── development.yaml
│       ├── production.yaml
│       └── enterprise.yaml
│
├── deployments/                     # Deployment artifacts
│   ├── docker/
│   ├── kubernetes/
│   │   ├── base/
│   │   └── overlays/
│   └── helm/
│       ├── database-intelligence/
│       └── database-intelligence-operator/
│
├── tests/                           # All test suites
│   ├── go.mod
│   ├── e2e/
│   ├── integration/
│   ├── performance/
│   ├── benchmarks/
│   └── testdata/
│
└── tools/                           # Build and dev tools
    ├── builder/                     # Custom OTEL builder config
    ├── scripts/
    └── ci/
```

## Implementation Steps

### Phase 1: Set Up Go Workspace (Week 1)

```bash
# 1. Create go.work file
go work init

# 2. Add modules to workspace
go work use ./core
go work use ./processors
go work use ./receivers
go work use ./exporters
go work use ./extensions
go work use ./common
go work use ./tests

# 3. Create module structure
mkdir -p {core,processors,receivers,exporters,extensions,common,distributions,tests}

# 4. Initialize go.mod for each module
cd core && go mod init github.com/database-intelligence/core
cd ../processors && go mod init github.com/database-intelligence/processors
# ... repeat for all modules
```

### Phase 2: Migrate Components (Week 2-3)

```bash
# Example migration script
#!/bin/bash
# migrate-processors.sh

SOURCE_DIR="database-intelligence-mvp/processors"
TARGET_DIR="database-intelligence/processors"

# Copy processor code
for processor in adaptivesampler circuitbreaker costcontrol nrerrormonitor planattributeextractor querycorrelator verification; do
    cp -r "$SOURCE_DIR/$processor" "$TARGET_DIR/"
    
    # Update import paths
    find "$TARGET_DIR/$processor" -name "*.go" -exec sed -i \
        's|github.com/database-intelligence-mvp|github.com/database-intelligence|g' {} \;
done
```

### Phase 3: Create Distribution Builds (Week 4)

```go
// distributions/enterprise/main.go
package main

import (
    "github.com/database-intelligence/core/builder"
    
    // Import all components
    _ "github.com/database-intelligence/processors/adaptivesampler"
    _ "github.com/database-intelligence/processors/circuitbreaker"
    _ "github.com/database-intelligence/processors/costcontrol"
    _ "github.com/database-intelligence/processors/nrerrormonitor"
    _ "github.com/database-intelligence/processors/planattributeextractor"
    _ "github.com/database-intelligence/processors/querycorrelator"
    _ "github.com/database-intelligence/processors/verification"
    
    _ "github.com/database-intelligence/receivers/ash"
    _ "github.com/database-intelligence/receivers/enhancedsql"
    _ "github.com/database-intelligence/receivers/kernelmetrics"
    
    _ "github.com/database-intelligence/exporters/nri"
    _ "github.com/database-intelligence/extensions/healthcheck"
)

func main() {
    builder.RunCollector("enterprise")
}
```

## Key Benefits of This Approach

### 1. **Modular Architecture**
- Clear separation of concerns
- Independent versioning per module
- Easy to add/remove components

### 2. **Flexible Distributions**
```yaml
# Different builds for different needs
distributions:
  minimal:
    - postgresql receiver
    - prometheus exporter
  
  standard:
    - postgresql + mysql receivers
    - adaptivesampler processor
    - prometheus + otlp exporters
  
  enterprise:
    - all receivers
    - all processors
    - all exporters
    - cost control enabled
```

### 3. **Simplified Testing**
```makefile
# Root Makefile
.PHONY: test-all
test-all:
	@echo "Running all tests..."
	cd common && go test ./...
	cd processors && go test ./...
	cd receivers && go test ./...
	cd exporters && go test ./...
	cd tests && go test ./...

.PHONY: test-e2e
test-e2e:
	cd tests/e2e && go test -tags=e2e ./...

.PHONY: build-all
build-all:
	@echo "Building all distributions..."
	cd distributions/minimal && go build -o ../../bin/collector-minimal
	cd distributions/standard && go build -o ../../bin/collector-standard
	cd distributions/enterprise && go build -o ../../bin/collector-enterprise
```

### 4. **Development Workflow**
```bash
# Local development with all components
go work sync
go run ./distributions/enterprise

# Test specific component
cd processors/adaptivesampler
go test ./...

# Build specific distribution
cd distributions/standard
go build
```

### 5. **CI/CD Integration**
```yaml
# .github/workflows/test.yml
name: Test All Modules
on: [push, pull_request]

jobs:
  test:
    strategy:
      matrix:
        module: [common, processors, receivers, exporters, core]
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      - name: Test ${{ matrix.module }}
        run: |
          cd ${{ matrix.module }}
          go test -v ./...
```

## Migration Checklist

- [ ] Create new repository structure
- [ ] Set up Go workspace
- [ ] Migrate common libraries first
- [ ] Migrate processors with updated imports
- [ ] Migrate receivers and exporters
- [ ] Create distribution builds
- [ ] Update configuration templates
- [ ] Migrate deployment artifacts
- [ ] Update CI/CD pipelines
- [ ] Update documentation
- [ ] Test all distributions
- [ ] Create migration guide for users

## Alternative: GitOps Multi-Repo Approach

If you prefer complete separation:

```yaml
# repo-structure.yaml
repositories:
  - name: database-intelligence-core
    url: github.com/org/database-intelligence-core
    
  - name: database-intelligence-processors
    url: github.com/org/database-intelligence-processors
    
  - name: database-intelligence-receivers
    url: github.com/org/database-intelligence-receivers
    
  - name: database-intelligence-deployments
    url: github.com/org/database-intelligence-deployments

# Managed by a meta-repository
  - name: database-intelligence
    url: github.com/org/database-intelligence
    submodules:
      - core
      - processors
      - receivers
      - deployments
```

## Conclusion

The monorepo with Go workspaces approach provides the best balance of:
- **Modularity**: Clear component boundaries
- **Flexibility**: Multiple distribution options
- **Maintainability**: Single source of truth
- **Testability**: Integrated testing across all components
- **Deployability**: Easy to create custom builds

This structure allows teams to work independently on components while maintaining overall system coherence.