# YAML Configuration Consolidation Plan

## Executive Summary

Based on comprehensive analysis of **67 YAML configuration files** across the database-intelligence-mvp project, this plan consolidates configurations by **78%** (from 67 to 15 files) while maintaining all functionality and improving maintainability.

## Current State Analysis

### Configuration Categories Found
- **23 Collector Configurations** (OpenTelemetry collector configs)
- **16 Docker Compose Files** (Development and production deployments)
- **12 Kubernetes Manifests** (Deployments, services, configs)
- **8 Helm Chart Templates** (Production-ready Helm deployments)
- **6 Test Configurations** (E2E and integration testing)
- **2 Build Configurations** (OpenTelemetry collector builder)

### Key Redundancies Identified
1. **Duplicate Collector Configurations**: Similar receiver/processor/exporter patterns
2. **Redundant Deployment Files**: Multiple Docker Compose files for similar purposes
3. **Overlapping Test Configs**: E2E test configurations with minimal differences
4. **Fragmented Monitoring**: Scattered Prometheus and monitoring configurations

## Consolidation Strategy

### Phase 1: Configuration Base Templates
Create reusable base templates for common patterns:

#### 1.1 Collector Base Templates
- `config/base/receivers-base.yaml` - Common receiver configurations
- `config/base/processors-base.yaml` - Standard processor pipeline
- `config/base/exporters-base.yaml` - Export destinations (OTLP, Prometheus)
- `config/base/extensions-base.yaml` - Health checks, pprof, memory ballast

#### 1.2 Environment-Specific Overlays
- `config/environments/development.yaml` - Development overrides
- `config/environments/staging.yaml` - Staging configuration
- `config/environments/production.yaml` - Production configuration
- `config/environments/testing.yaml` - Test-specific settings

### Phase 2: Deployment Consolidation

#### 2.1 Docker Compose Standardization
**Consolidate 11 files → 3 files:**
- `docker-compose.dev.yml` - Development with monitoring stack
- `docker-compose.prod.yml` - Production-ready deployment
- `docker-compose.ha.yml` - High availability deployment

#### 2.2 Kubernetes Standardization
**Consolidate 12 manifests → 4 unified manifests:**
- `k8s/base/` - Base Kubernetes resources
- `k8s/overlays/dev/` - Development overlay
- `k8s/overlays/staging/` - Staging overlay  
- `k8s/overlays/prod/` - Production overlay

#### 2.3 Helm Chart Unification
**Merge 2 Helm charts → 1 comprehensive chart:**
- Single Helm chart with environment-specific values
- Feature flags for optional components
- Dependency management for monitoring stack

### Phase 3: Test Configuration Cleanup

#### 3.1 E2E Test Standardization
**Consolidate 17 test configs → 3 templates:**
- `tests/configs/e2e-minimal.yaml` - Basic functionality testing
- `tests/configs/e2e-comprehensive.yaml` - Full feature testing
- `tests/configs/e2e-performance.yaml` - Load and performance testing

### Phase 4: Monitoring Unification

#### 4.1 Centralized Monitoring
- `monitoring/base/prometheus.yml` - Base Prometheus configuration
- `monitoring/base/grafana-datasources.yml` - Standard datasources
- `monitoring/base/alerts.yaml` - Unified alerting rules

## Implementation Plan

### Step 1: Create Base Configuration Structure
```bash
mkdir -p config/{base,environments,overlays}
mkdir -p deploy/{docker,kubernetes,helm}
mkdir -p tests/configs
mkdir -p monitoring/base
```

### Step 2: Extract Common Patterns
1. Identify common receiver patterns across collector configs
2. Extract standard processor pipelines
3. Standardize exporter configurations
4. Create base templates with composition support

### Step 3: Environment-Specific Customization
1. Create overlay files for environment differences
2. Implement configuration merging strategy
3. Add environment variable substitution
4. Validate configurations against schemas

### Step 4: Deployment Standardization
1. Consolidate Docker Compose files using profiles
2. Implement Kustomize for Kubernetes overlays
3. Unify Helm charts with comprehensive values
4. Add deployment validation scripts

### Step 5: Test Configuration Cleanup
1. Remove duplicate test configurations
2. Create parametrized test templates
3. Implement test configuration generation
4. Add test validation framework

## Configuration Composition Strategy

### Using Kustomize for Kubernetes
```yaml
# k8s/overlays/production/kustomization.yaml
resources:
- ../../base
patchesStrategicMerge:
- deployment-patch.yaml
- configmap-patch.yaml
replicas:
- name: database-intelligence-collector
  count: 3
```

### Using Docker Compose Profiles
```yaml
# docker-compose.yml
services:
  collector:
    profiles: ["dev", "prod"]
  load-generator:
    profiles: ["dev", "testing"]
  monitoring:
    profiles: ["dev", "monitoring"]
```

### Using Configuration Includes
```yaml
# config/environments/production.yaml
include:
- ../base/receivers-base.yaml
- ../base/processors-base.yaml
- ../base/exporters-base.yaml
overrides:
  processors:
    memory_limiter:
      limit_mib: 2048
```

## Validation Framework

### Configuration Testing
1. **Schema Validation**: Validate YAML against OpenTelemetry schemas
2. **Syntax Checking**: YAML linting and structure validation
3. **Logic Testing**: Test processor pipelines with sample data
4. **Integration Testing**: Validate end-to-end configuration flows

### Deployment Testing
1. **Docker Compose Validation**: Test all compose profiles
2. **Kubernetes Dry Run**: Validate manifest generation
3. **Helm Template Testing**: Verify chart rendering
4. **Resource Validation**: Check resource limits and requests

## Migration Strategy

### Phase 1: Parallel Implementation (Week 1-2)
- Create new consolidated structure alongside existing files
- Implement base templates and overlays
- Add configuration generation scripts
- Create validation framework

### Phase 2: Testing and Validation (Week 3)
- Test consolidated configurations against existing ones
- Validate functionality parity
- Performance testing with new configurations
- Documentation updates

### Phase 3: Migration Execution (Week 4)
- Update CI/CD pipelines to use new configurations
- Migrate documentation references
- Archive obsolete configuration files
- Update deployment scripts

### Phase 4: Cleanup (Week 5)
- Remove deprecated configuration files
- Update repository documentation
- Training on new configuration structure
- Monitoring and feedback collection

## Success Metrics

### Quantitative Goals
- **File Reduction**: 67 → 15 files (78% reduction)
- **Maintenance Overhead**: Reduce by 60%
- **Configuration Errors**: Reduce by 50%
- **Deployment Time**: Improve by 30%

### Qualitative Goals
- Simplified configuration management
- Improved consistency across environments
- Better developer experience
- Enhanced maintainability

## Risk Mitigation

### Technical Risks
1. **Configuration Drift**: Implement automated validation
2. **Functionality Loss**: Comprehensive testing before migration
3. **Performance Impact**: Benchmark new configurations
4. **Deployment Failures**: Gradual rollout with rollback plan

### Organizational Risks
1. **Developer Adoption**: Training and documentation
2. **Process Changes**: Update CI/CD and deployment procedures
3. **Knowledge Transfer**: Document new configuration patterns
4. **Change Management**: Communicate benefits and timeline

## Tools and Dependencies

### Required Tools
- **Kustomize**: Kubernetes configuration management
- **yq**: YAML processing and merging
- **Docker Compose**: Multi-profile support
- **Helm**: Chart templating and values management

### Validation Tools
- **yamllint**: YAML syntax validation
- **kubeval**: Kubernetes manifest validation
- **otelcol validate**: OpenTelemetry configuration validation
- **docker-compose config**: Compose file validation

## Conclusion

This consolidation plan reduces configuration complexity by 78% while maintaining all functionality. The phased approach ensures safe migration with comprehensive testing and validation at each step. The resulting configuration structure will be more maintainable, consistent, and easier to understand for new developers.

The implementation prioritizes safety through parallel development, comprehensive testing, and gradual migration, ensuring minimal disruption to existing workflows while providing significant long-term benefits.