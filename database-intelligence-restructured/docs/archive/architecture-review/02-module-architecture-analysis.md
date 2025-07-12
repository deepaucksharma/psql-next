# Module Architecture Analysis

## Current State - Critical Problems

### Module Organization Chaos
```
database-intelligence-restructured/
├── common/                 # 2 separate go.mod files
├── core/                  # 1 go.mod
├── processors/            # 7 separate go.mod files
├── receivers/             # 3 separate go.mod files
├── exporters/             # 1 go.mod file
├── distributions/         # 3 separate go.mod files
├── internal/              # Inconsistent module usage
└── tests/                 # Separate go.mod
```

### Critical Issues

#### 1. Version Conflict Hell
```go
// processors/adaptivesampler/go.mod
require go.opentelemetry.io/collector v0.92.0

// processors/circuitbreaker/go.mod
require go.opentelemetry.io/collector v0.93.0  // Different version!
```
**Impact**: Runtime crashes, API incompatibilities, build failures

#### 2. Broken Workspace Configuration
```go
// go.work - Missing 10+ modules
use (
    .
    ./tests/e2e
    ./tools/minimal-db-check
    ./internal/database
    ./distributions/production
)
```
**Impact**: Developers must manually manage dependencies, builds fail randomly

#### 3. Circular Dependency Time Bombs
```
processors/* → common/* → internal/database
receivers/* → common/* → internal/database
distributions/* → ALL OF THE ABOVE
```
**Impact**: Cannot refactor, cannot update dependencies, maintenance nightmare

## Required Fixes

### Fix 1: Consolidate to 3-4 Modules Maximum
```
database-intelligence/
├── go.mod                   # Root module for shared code
├── components/
│   └── go.mod              # All processors, receivers, exporters
├── cmd/
│   └── go.mod              # Single binary, multiple profiles
└── tests/
    └── go.mod              # Test utilities only
```

### Fix 2: Single Version Management
```go
// Single go.mod for all components
module github.com/deepaksharma/db-otel/components

require (
    go.opentelemetry.io/collector v0.92.0
    // All components use same version
)
```

### Fix 3: Clear Dependency Direction
```
cmd → components → core libraries
                ↓
            external deps
```
No circular dependencies possible.

### Fix 4: Proper Workspace Usage
```go
// go.work
use (
    .
    ./components
    ./cmd
    ./tests
)
```

## Migration Steps

### Step 1: Create components/ module
```bash
mkdir components
cd components
go mod init github.com/deepaksharma/db-otel/components
```

### Step 2: Move all components
```bash
mv ../processors/* ./processors/
mv ../receivers/* ./receivers/
mv ../exporters/* ./exporters/
```

### Step 3: Fix imports
```go
// Before
import "github.com/deepaksharma/db-otel/processors/adaptivesampler"

// After
import "github.com/deepaksharma/db-otel/components/processors/adaptivesampler"
```

### Step 4: Single distribution
```go
// cmd/collector/main.go
func main() {
    profile := getProfile() // from env or flag
    components := loadComponents(profile)
    runCollector(components)
}
```

## Why This Is Critical

1. **Build Failures**: Current structure causes random build failures
2. **Version Conflicts**: Different OTel versions = runtime crashes
3. **Maintenance Burden**: 15+ go.mod files to update
4. **Development Speed**: Simple changes require multiple module updates
5. **Testing Complexity**: Can't test components together easily

## Success Metrics
- From 15+ modules to 3-4 modules
- Zero version conflicts
- Single `go mod tidy` updates everything
- All components in one module
- Clear, enforced dependency hierarchy