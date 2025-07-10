# Version Comparison Report

## Clean Reference Distribution

### Latest OpenTelemetry Versions (as of analysis)

```
```

## Current Implementation Issues

### 1. Core Module (core/go.mod)
- Uses v1.35.0 for component, receiver, extension
- Uses v0.129.0 for specific implementations (otlpreceiver, memorylimiterprocessor)
- **Issue**: Mixed versions between base packages and implementations

### 2. Production Distribution
- Uses v0.105.0 throughout
- **Issue**: Older version, not aligned with core module

### 3. Processors/Receivers
- Most use v0.110.0 with confmap v1.16.0
- **Issue**: Version mismatch with core module

## Key Differences from Clean Implementation

1. **Module Path Inconsistency**
   - Current: `github.com/database-intelligence-restructured/`
   - Should be: `github.com/database-intelligence/` (without -restructured)

2. **Version Alignment**
   - Clean reference uses consistent latest versions
   - Current implementation has 3 different version sets (v0.105.0, v0.110.0, v1.35.0)

3. **Import Structure**
   - Clean reference: Direct imports, no confmap needed in business logic
   - Current: Some modules import confmap directly (shouldn't be needed)

## Recommended Fixes

### Fix 1: Align Module Paths
All modules should use consistent module paths without "-restructured"

### Fix 2: Version Alignment Strategy
Choose one approach:
- **Option A**: Use v0.105.0 everywhere (stable, older)
- **Option B**: Use v0.110.0 + v1.16.0 for special modules
- **Option C**: Update to latest (v1.35.0 + v0.129.0 pattern)

### Fix 3: Remove Direct confmap Imports
Processors and receivers shouldn't import confmap directly. Use component.Config interface.

### Fix 4: Consistent Replace Directives
Use relative paths consistently in replace directives.
