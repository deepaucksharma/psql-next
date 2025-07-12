# CLAUDE.md - Database Intelligence Project Context

This file provides comprehensive guidance for Claude Code and other AI assistants working with the Database Intelligence codebase. Based on extensive consolidation and restructuring work, this document contains deep insights into the project's architecture, patterns, and best practices.

## 🚨 Critical Project State (READ FIRST)

### Consolidated Architecture (2025)
- **Directory Structure**: Recently consolidated from 14 to 11 top-level directories
- **Shared Utilities**: All utilities consolidated into single `internal/` directory  
- **Build System**: Unified under `.ci/` following industry standards
- **Two Operating Modes**: Config-Only (standard OTel) vs Enhanced (custom components)
- **Go Workspace**: Single workspace managing all modules with proper dependency resolution

### What's Production Ready ✅
- **Config-Only Mode**: Uses only standard OpenTelemetry components (PostgreSQL + MySQL)
- **Enhanced Mode**: Includes 7 custom processors for advanced database intelligence
- **Build System**: Unified build scripts and OpenTelemetry Builder configurations
- **Testing Framework**: Comprehensive E2E testing with real databases
- **Deployment**: Docker, Kubernetes, and binary deployment options

### Known Issues ⚠️
- **Module Dependencies**: Some OpenTelemetry version conflicts (use fix scripts in `development/scripts/`)
- **Custom Components**: Exist in source but require proper registration in distributions
- **Go Versions**: Standardized to 1.22 (script available to fix inconsistencies)

## 📁 Project Structure (Post-Consolidation)

### Core Directories
```
Database Intelligence/
├── .ci/                    # Build & CI/CD automation (NEW)
│   ├── build/             # OpenTelemetry Builder configurations
│   ├── workflows/         # CI/CD workflow definitions  
│   └── scripts/           # Build and deployment scripts
├── components/            # Custom OpenTelemetry components
│   ├── receivers/         # ASH, Enhanced SQL, Kernel Metrics
│   ├── processors/        # Adaptive sampling, circuit breaker, etc.
│   ├── exporters/         # New Relic integration
│   └── extensions/        # Health checks and utilities
├── configs/               # Runtime configurations (CLEANED)
│   ├── base/             # Modular component definitions
│   ├── modes/            # config-only.yaml vs enhanced.yaml
│   ├── environments/     # dev, staging, production overlays
│   └── examples/         # Working configuration examples
├── deployments/          # Deployment configurations
│   ├── docker/           # Docker and compose files
│   ├── kubernetes/       # K8s manifests and Helm charts
│   └── helm/             # Helm chart definitions
├── development/          # Development tools (NEW)
│   └── scripts/          # Utility scripts for maintenance
├── distributions/        # Binary distributions
│   ├── minimal/          # Standard components only
│   ├── production/       # Full-featured build
│   └── enterprise/       # All components + enterprise features
├── docs/                 # Documentation (CONSOLIDATED)
│   ├── guides/           # User guides and tutorials
│   ├── reference/        # Technical reference
│   └── development/      # Developer documentation
├── internal/             # Shared utilities (CONSOLIDATED)
│   ├── featuredetector/  # Database feature detection
│   ├── queryselector/    # Query selection utilities
│   ├── database/         # Database connection management
│   ├── health/           # Health checking utilities
│   ├── performance/      # Performance optimization
│   ├── secrets/          # Secret management
│   └── config/           # Configuration loading with secrets
├── scripts/              # Operational scripts (STREAMLINED)
│   └── test/             # Testing scripts
└── tests/                # Testing infrastructure (CONSOLIDATED)
    ├── tools/            # Testing utilities and generators
    ├── e2e/              # End-to-end testing framework
    └── fixtures/         # Test data and configurations
```

### Key Architectural Insights

#### 1. **Two-Mode Architecture**
The project supports two distinct operational modes:

**Config-Only Mode** (`configs/modes/config-only.yaml`):
- Uses only standard OpenTelemetry Collector components
- Works with official OTel Collector Contrib without custom builds
- Minimal resource usage: <5% CPU, <512MB memory
- Production-ready for standard database monitoring

**Enhanced Mode** (`configs/modes/enhanced.yaml`):
- Includes 7 custom processors for advanced database intelligence
- Requires custom build with component registration
- Higher resource usage: <20% CPU, <2GB memory  
- Provides query intelligence, adaptive sampling, cost control

#### 2. **Modular Configuration System**
```yaml
# Base components (configs/base/)
receivers.yaml    # All receiver definitions
processors.yaml   # All processor definitions
exporters.yaml    # All exporter definitions
extensions.yaml   # All extension definitions

# Mode selection (configs/modes/)
config-only.yaml  # Standard mode configuration
enhanced.yaml     # Enhanced mode configuration

# Environment overlays (configs/environments/)
development.yaml  # Development overrides
staging.yaml      # Staging configuration
production.yaml   # Production optimizations
```

#### 3. **Consolidated Utility Libraries**
All shared code consolidated into `internal/` directory:
- **featuredetector/**: Database capability detection (PostgreSQL, MySQL)
- **queryselector/**: Query selection and filtering logic
- **database/**: Connection pooling and management
- **health/**: Health checking and monitoring utilities
- **performance/**: Performance optimization utilities
- **secrets/**: Secret management and resolution
- **config/**: Advanced configuration loading with secret resolution

## 🛠️ Development Patterns & Best Practices

### Component Development Pattern
```go
// Standard component structure used throughout the project
type Component struct {
    config *Config           // Component-specific configuration
    logger *zap.Logger      // Structured logging
    telemetry component.TelemetrySettings  // OTel telemetry
    // component-specific fields
}

// Required lifecycle methods
func (c *Component) Start(ctx context.Context, host component.Host) error {
    // Initialize component
    return nil
}

func (c *Component) Shutdown(ctx context.Context) error {
    // Clean shutdown logic
    return nil
}

// Configuration validation pattern
func (cfg *Config) Validate() error {
    if cfg.RequiredField == "" {
        return fmt.Errorf("required_field is mandatory")
    }
    return nil
}
```

### Error Handling Conventions
```go
// Always wrap errors with meaningful context
if err := database.Connect(); err != nil {
    return fmt.Errorf("failed to connect to PostgreSQL database %s: %w", 
        cfg.Database, err)
}

// Use structured logging for errors
logger.Error("Query execution failed",
    zap.Error(err),
    zap.String("query", query),
    zap.String("database", dbName),
    zap.Duration("elapsed", elapsed))
```

### Configuration Loading Pattern
```go
// Use the consolidated config loader with secret resolution
loader := config.NewSecureConfigLoader(logger)
conf, err := loader.LoadConfig(ctx, configPath)
if err != nil {
    return fmt.Errorf("failed to load configuration: %w", err)
}
```

## 🔧 Common Development Tasks

### 1. Adding a New Custom Component

```bash
# 1. Create component directory
mkdir -p components/processors/mynewprocessor

# 2. Implement component following patterns
# See existing components for reference

# 3. Add to distribution registration
# Edit: distributions/production/components_enhanced.go

# 4. Update builder configuration
# Edit: .ci/build/enhanced.yaml

# 5. Add tests
mkdir -p components/processors/mynewprocessor/tests

# 6. Test the build
.ci/scripts/build.sh production
```

### 2. Fixing Module Dependencies

```bash
# Use the consolidated fix scripts
development/scripts/fix-otel-dependencies.sh  # Fix OpenTelemetry versions
development/scripts/fix-go-versions.sh        # Standardize Go versions

# Manual go.work management
go work sync
go work edit -replace github.com/example/module=./local/path
```

### 3. Building and Testing

```bash
# Use unified build system
.ci/scripts/build.sh minimal     # Build minimal distribution
.ci/scripts/build.sh production  # Build production distribution

# Use make targets (defined in Makefile)
make build                # Build production distribution
make build-minimal        # Build minimal distribution
make test                 # Run unit tests
make test-e2e            # Run end-to-end tests
make dev                 # Run development checks (fmt, lint, security)
```

### 4. Working with Configurations

```bash
# Test config-only mode
./database-intelligence-collector \
  --config=configs/modes/config-only.yaml \
  --config=configs/environments/development.yaml

# Test enhanced mode
./database-intelligence-collector \
  --config=configs/modes/enhanced.yaml \
  --config=configs/environments/development.yaml

# Validate configuration
make validate-config
```

## 🏗️ Architecture Principles

### 1. **Defense in Depth**
Multiple protection layers prevent system overload:
- Memory limiter processor (first line of defense)
- Circuit breaker processor (prevents cascade failures) 
- Adaptive sampler (reduces load under pressure)
- Cost control processor (enforces budget limits)

### 2. **Zero Persistence Design**
- All state maintained in memory for performance
- No local storage dependencies
- Graceful degradation when components fail
- Clean restart capabilities

### 3. **OpenTelemetry-First Approach**
- Leverages standard OpenTelemetry components wherever possible
- Custom components only for database-specific intelligence
- Standard OTel configuration patterns and conventions
- Compatible with OTel ecosystem tools

### 4. **Environment-Specific Configuration**
- Base configuration defines all available components
- Mode selection (config-only vs enhanced) determines component set
- Environment overlays provide deployment-specific overrides
- Secret resolution integrated into configuration loading

## 📊 Performance Characteristics

### Resource Usage Targets
```yaml
Config-Only Mode:
  cpu: "<5%"
  memory: "<512MB" 
  processing_latency: "<5ms per metric"
  
Enhanced Mode:  
  cpu: "<20%"
  memory: "<2GB"
  processing_latency: "<10ms per metric"
  additional_features: "query intelligence, adaptive sampling, cost control"
```

### Scalability Patterns
- Memory usage scales with number of unique queries
- Circuit breaker activates at 80% error rate
- Adaptive sampler adjusts based on CPU/memory utilization
- Batch processor optimizes network efficiency

## 🐛 Common Issues & Resolution

### Module Dependency Conflicts
```bash
# Symptoms: "unknown revision" errors, version conflicts
# Resolution: Use fix scripts
development/scripts/fix-otel-dependencies.sh
development/scripts/fix-go-versions.sh

# Manual resolution for specific conflicts
go work edit -replace problem-module=./local/path
```

### Custom Component Registration Issues
```bash
# Symptoms: Component not found at runtime
# Resolution: Check component registration
# 1. Verify component exists in components/ directory
# 2. Check registration in distributions/*/components*.go
# 3. Verify builder configuration in .ci/build/*.yaml
# 4. Rebuild with: .ci/scripts/build.sh production
```

### Configuration Loading Problems
```bash
# Symptoms: Config validation errors, secret resolution failures
# Resolution: Use debugging tools
# 1. Enable debug logging in exporter
# 2. Check environment variables for secrets
# 3. Validate YAML syntax
# 4. Test with: make validate-config
```

## 🔒 Security Best Practices

### Secret Management
```yaml
# Use environment variable placeholders in configs
exporters:
  otlp/newrelic:
    headers:
      api-key: "${env:NEW_RELIC_LICENSE_KEY}"
```

```bash
# Store secrets in environment or external systems
export NEW_RELIC_LICENSE_KEY="your-actual-key"
export DB_PASSWORD="secure-password"
```

### Database Permissions
```sql
-- Minimal PostgreSQL permissions for monitoring
CREATE USER db_monitor WITH PASSWORD 'secure_password';
GRANT CONNECT ON DATABASE postgres TO db_monitor;
GRANT USAGE ON SCHEMA public TO db_monitor;
GRANT SELECT ON ALL TABLES IN SCHEMA pg_catalog TO db_monitor;
GRANT SELECT ON ALL TABLES IN SCHEMA information_schema TO db_monitor;
```

## 🚀 Deployment Patterns

### Local Development
```bash
# Quick start for development
make dev-run    # Build and run with development config
make docker-up  # Start with Docker Compose
```

### Production Deployment
```bash
# Use config-only mode for stability
./database-intelligence-collector \
  --config=configs/modes/config-only.yaml \
  --config=configs/environments/production.yaml

# Or use Docker
docker run -d \
  -v $(pwd)/configs:/etc/otelcol \
  -e NEW_RELIC_LICENSE_KEY=${NEW_RELIC_LICENSE_KEY} \
  database-intelligence:latest \
  --config=/etc/otelcol/modes/config-only.yaml
```

### Kubernetes Deployment
```bash
# Use Helm chart
helm install database-intelligence deployments/helm/database-intelligence/ \
  --set mode=config-only \
  --set environment=production \
  --set newrelic.licenseKey=${NEW_RELIC_LICENSE_KEY}
```

## 📚 Development Workflow

### Making Changes
1. **Understand the architecture**: Review this document and relevant code
2. **Follow patterns**: Use existing components as templates
3. **Test thoroughly**: Unit tests + integration tests + E2E tests
4. **Update documentation**: Keep docs in sync with changes
5. **Use quality tools**: `make dev` runs formatting, linting, security checks

### Testing Strategy
```bash
# 1. Unit tests for individual components
go test -v ./components/processors/mynewprocessor/...

# 2. Integration tests with real databases  
make test-integration

# 3. End-to-end tests with full pipeline
make test-e2e

# 4. Performance/load testing
cd tests/tools/load-generator
go run main.go --database=postgres --duration=5m
```

### Code Quality Standards
- **Formatting**: Use `gofmt` and `goimports`
- **Linting**: Pass `golangci-lint` checks
- **Testing**: Maintain >80% test coverage
- **Documentation**: Comment exported functions and types
- **Error Handling**: Always wrap errors with context
- **Logging**: Use structured logging with appropriate levels

## 🎯 Current Development Priorities

### High Priority
1. **Complete Custom Component Registration**: Ensure all custom components are properly registered
2. **Resolve Module Dependencies**: Fix remaining OpenTelemetry version conflicts
3. **Performance Optimization**: Reduce memory allocations and improve efficiency

### Medium Priority  
1. **Enhanced Monitoring**: Add more comprehensive health checks and metrics
2. **Configuration Validation**: Improve configuration validation and error messages
3. **Documentation**: Expand troubleshooting guides and best practices

### Low Priority
1. **Additional Integrations**: Support for more monitoring backends
2. **Query Intelligence**: Enhanced query analysis and recommendations
3. **Cost Optimization**: More sophisticated cost control mechanisms

## 🤖 AI Assistant Guidelines

### When Fixing Issues
1. **Check consolidation status**: Use new directory structure (internal/, .ci/, etc.)
2. **Use fix scripts**: Apply `development/scripts/fix-*.sh` for common problems
3. **Follow patterns**: Maintain consistency with existing code
4. **Test changes**: Always verify fixes work in both config-only and enhanced modes
5. **Update documentation**: Keep guides and references current

### When Adding Features
1. **Understand two-mode architecture**: Ensure feature works appropriately in both modes
2. **Use shared utilities**: Leverage consolidated `internal/` libraries
3. **Follow component patterns**: Use established interfaces and lifecycle methods
4. **Add comprehensive tests**: Unit, integration, and E2E tests
5. **Document thoroughly**: Update relevant guides and references

### When Reviewing Code
1. **Verify consolidation compliance**: Ensure code uses new structure
2. **Check dependency management**: Look for version conflicts
3. **Validate error handling**: Ensure proper error wrapping and logging
4. **Review test coverage**: Confirm adequate testing
5. **Check documentation**: Ensure docs are updated

---

**Last Updated**: Based on comprehensive consolidation completed 2025-01-12
**Key Changes**: Directory consolidation, unified build system, streamlined configurations, enhanced documentation

This document reflects the current state after major architectural improvements and should be the authoritative source for understanding the Database Intelligence project structure and patterns.