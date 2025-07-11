# Build & Distribution Strategy Analysis

## Critical Build Problems

### 1. Three Confusing Distributions
```
distributions/
├── minimal/      # Not actually minimal (40MB)
├── production/   # Everything included (85MB)
└── enterprise/   # Same as production (85MB) - Why?
```
**Impact**: Confusion, maintenance burden, no clear purpose

### 2. Duplicate Main Files
```go
// minimal/main.go - 200 lines
// production/main.go - 200 lines (90% duplicate)
// enterprise/main.go - 200 lines (95% duplicate)
```
**Impact**: Triple maintenance, inconsistent updates, bugs

### 3. No Modularity
```go
// Everything compiled in, can't disable
factories.Processors = map[component.Type]processor.Factory{
    "adaptive": adaptiveFactory,      // Always included
    "circuit": circuitFactory,        // Always included
    "cost": costFactory,             // Always included
    // ... all processors always included
}
```
**Impact**: Large binaries, no flexibility, memory waste

## Required Fixes

### Fix 1: Single Binary with Profiles
```go
// One main.go, multiple profiles
type Profile string

const (
    ProfileDev        Profile = "development"
    ProfileProduction Profile = "production"
)

func main() {
    profile := Profile(os.Getenv("COLLECTOR_PROFILE"))
    if profile == "" {
        profile = ProfileProduction
    }
    
    components := loadComponentsForProfile(profile)
    runCollector(components)
}
```

### Fix 2: Profile-Based Component Loading
```go
func loadComponentsForProfile(profile Profile) ComponentSet {
    switch profile {
    case ProfileDev:
        return ComponentSet{
            Receivers: []string{"otlp", "postgresql"},
            Processors: []string{"batch"},
            Exporters: []string{"debug"},
        }
    case ProfileProduction:
        return ComponentSet{
            Receivers: []string{"otlp", "postgresql", "mysql"},
            Processors: []string{"batch", "memory_limiter", "adaptive"},
            Exporters: []string{"otlp", "prometheus"},
        }
    }
}
```

### Fix 3: Build Tags for Size Optimization
```go
// +build production

package components

func init() {
    RegisterProcessor("adaptive", adaptiveFactory)
    RegisterProcessor("circuit", circuitFactory)
}
```

```makefile
# Build for different profiles
build-dev:
    go build -tags="core" -o collector

build-prod:
    go build -tags="core,production" -o collector
```

## Simplified Structure
```
cmd/
└── collector/
    ├── main.go           # Single main file
    ├── profiles.go       # Profile definitions
    └── components.go     # Component registration
```

## Migration Steps

### Step 1: Create Unified Main
```go
// cmd/collector/main.go
package main

import (
    "github.com/deepaksharma/db-otel/internal/profiles"
)

func main() {
    profile := profiles.GetFromEnv()
    collector := profiles.BuildCollector(profile)
    collector.Run()
}
```

### Step 2: Remove Duplicate Distributions
```bash
# Delete these
rm -rf distributions/minimal
rm -rf distributions/enterprise

# Keep and rename
mv distributions/production cmd/collector
```

### Step 3: Profile Configuration
```yaml
# profiles/production.yaml
components:
  receivers:
    - postgresql
    - mysql
  processors:
    - batch
    - memory_limiter
  exporters:
    - otlp
    
limits:
  max_memory: 1GB
  max_connections: 100
```

## Binary Size Targets
- Development: < 30MB (basic components only)
- Production: < 50MB (standard components)
- Custom: Variable based on selected components

## Success Metrics
- Single binary to maintain
- Clear profile selection
- 50% reduction in binary size
- No code duplication
- Easy to add/remove components