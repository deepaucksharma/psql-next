# Architecture Impact Analysis

This document analyzes the implications of the configuration changes made during E2E test fixes on the overall Database Intelligence MVP architecture.

## Executive Summary

The configuration fixes have revealed significant architectural inconsistencies:
1. **Critical**: Dashboard queries depend on `collector.name = 'otelcol'` attribute that isn't added in production configs
2. **High**: Custom processors are documented but not built or deployed
3. **Medium**: 70+ configuration files with inconsistent syntax and settings
4. **Low**: Documentation describes features that don't exist

## Detailed Impact Analysis

### 1. Custom Processors Impact

**Current State:**
- Only `planattributeextractor` is included in build config
- `adaptivesampler`, `circuitbreaker`, `verification` are commented out
- Production configs reference these processors but they won't exist

**Impact:**
```yaml
# In deployments/helm/db-intelligence/values.yaml
adaptiveSampler:
  enabled: true  # This processor doesn't exist!
  
circuitBreaker:
  enabled: false  # This processor doesn't exist!
```

**Recommendation:** Either enable these processors in the build or remove all references.

### 2. Dashboard Creation Impact

**Critical Issue:** Dashboard queries require `collector.name = 'otelcol'` attribute:
```javascript
// From scripts/create-database-dashboard.js
query: `FROM Log SELECT average(numeric(avg_duration_ms)) 
        WHERE query_id IS NOT NULL 
        AND collector.name = 'otelcol'`  // Will return no data without resource processor!
```

**Affected Queries:**
- 5 dashboard widgets will show no data
- Query performance tracking completely broken
- All log-based metrics affected

**Fix Required:**
```yaml
processors:
  resource:
    attributes:
      - key: collector.name
        value: otelcol
        action: upsert
```

### 3. Deployment Configurations Impact

#### Kubernetes (`deployments/kubernetes/configmap.yaml`)
**Issues Found:**
1. Uses deprecated `memory_ballast` extension
2. Missing `resource` processor for collector.name
3. Old environment variable syntax
4. References non-existent `leader_election` extension
5. SQL query receiver missing required `logs:` section

#### Docker Compose
**Status:** Uses simplified configs, minimal impact

#### Helm Charts
**Issues Found:**
1. Values reference non-existent processors
2. ConfigMap templates use old syntax
3. Missing critical resource processor

### 4. Environment Variable Syntax Impact

**Breaking Change:** All configs must update from `${VAR:default}` to `${env:VAR:-default}`

**Affected Files:**
- All deployment configs
- All example configs
- Documentation examples

**Production Impact:** Connections will fail with old syntax

### 5. Memory Limiter Configuration Impact

**Breaking Change:** Must update from percentage to MiB values
```yaml
# Old (will fail)
memory_limiter:
  limit_percentage: 75
  
# New (required)
memory_limiter:
  limit_mib: 1024
```

### 6. SQL Query Receiver Impact

**Breaking Change:** Must add `logs:` configuration
```yaml
# Will fail without logs section
queries:
  - sql: "SELECT ..."
    logs:  # Required!
      - body_column: query_text
        attributes:
          query_id: query_id
```

## Risk Assessment

### High Risk Items
1. **Dashboard Data Loss** (P0)
   - Dashboards will show no query performance data
   - Root cause: Missing `collector.name` attribute
   - Fix: Add resource processor to all configs

2. **Production Connection Failures** (P0)
   - Database connections will fail
   - Root cause: Old environment variable syntax
   - Fix: Update all configs to `${env:VAR:-default}`

3. **Collector Startup Failures** (P1)
   - Collector won't start with current configs
   - Root cause: Invalid memory_limiter, missing logs section
   - Fix: Update all configuration syntax

### Medium Risk Items
1. **Feature Confusion** (P2)
   - Documentation describes non-existent features
   - Custom processors aren't available
   - Fix: Either build processors or update docs

2. **Maintenance Burden** (P2)
   - 70+ config files with different settings
   - No clear production vs development configs
   - Fix: Consolidate configurations

## Recommendations

### Immediate Actions (P0)
1. **Fix Production Configs**
   ```yaml
   processors:
     resource:
       attributes:
         - key: collector.name
           value: otelcol
           action: upsert
   ```

2. **Update Environment Variables**
   - Search and replace all `${VAR:default}` with `${env:VAR:-default}`

3. **Fix Memory Limiter**
   - Update all configs to use MiB values

### Short-term Actions (P1)
1. **Processor Decision**
   - Option A: Enable custom processors in build
   - Option B: Remove all references and code
   - Current state is confusing

2. **Configuration Consolidation**
   - Create clear config hierarchy
   - Remove duplicate configs
   - Document which to use when

### Long-term Actions (P2)
1. **Documentation Accuracy**
   - Audit all docs for accuracy
   - Remove planned features
   - Add "current vs roadmap" sections

2. **Testing Strategy**
   - Add config validation tests
   - Test dashboard queries
   - Validate production configs

## Configuration File Inventory

### Files Needing Updates
**Critical (Production):**
- `deployments/kubernetes/configmap.yaml` - Add resource processor
- `deployments/helm/db-intelligence/templates/configmap.yaml` - Fix syntax
- All Helm values files - Update processor references

**High Priority:**
- All example configs in `config/examples/`
- All test configs
- Documentation examples

**Medium Priority:**
- Archive old configs
- Remove duplicate configs
- Consolidate similar configs

## Summary

The configuration changes exposed fundamental architectural issues:
1. Production configs are broken and will cause data loss
2. Custom processors exist in code but not in builds
3. Documentation describes a different system than what exists
4. No clear configuration management strategy

These issues must be addressed before production deployment.