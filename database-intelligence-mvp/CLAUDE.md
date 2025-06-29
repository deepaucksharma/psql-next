# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

The Database Intelligence Collector is an OpenTelemetry-based monitoring solution with 4 sophisticated custom processors (3,242 lines of production code). It follows an OTEL-first architecture, using standard components where possible and custom processors only to fill gaps.

## Critical Context

### ✅ PRODUCTION FIXES APPLIED (2025-06-29)
The critical issues identified have been resolved:

1. **✅ State Management Fixed**: All processors now use in-memory state only (no Redis dependency)
2. **✅ Single-Instance Deployment**: Removed HA configurations requiring Redis 
3. **✅ Safe Plan Collection**: Plan extractor works with existing data, no unsafe dependencies
4. **✅ Resilient Pipeline**: Processors gracefully handle missing dependencies
5. **✅ Enhanced PII Protection**: Comprehensive sanitization beyond basic regex

### Deployment Model
**RECOMMENDED**: Use `config/collector-resilient.yaml` for production deployments
- ✅ **No Redis dependency** - All state management is in-memory only
- ✅ **No external dependencies** - Uses standard PostgreSQL pg_stat_statements
- ✅ **Graceful degradation** - Components work independently
- ✅ **Enhanced PII protection** - Credit cards, SSNs, emails, phones sanitized

### Build System Status
**WARNING**: The project has module path inconsistencies that prevent building:
- `go.mod`: `github.com/database-intelligence-mvp`
- `ocb-config.yaml`: `github.com/database-intelligence/*` 
- `otelcol-builder.yaml`: `github.com/newrelic/database-intelligence-mvp/*`

**Fix before any build attempts**:
```bash
# Standardize all module paths
sed -i 's|github.com/newrelic/database-intelligence-mvp|github.com/database-intelligence-mvp|g' otelcol-builder.yaml
sed -i 's|github.com/database-intelligence/|github.com/database-intelligence-mvp/|g' ocb-config.yaml
```

### Custom OTLP Exporter Issue
The custom OTLP exporter in `exporters/otlpexporter/` has TODO placeholders in critical functions. Either complete the implementation or remove it and use the standard OTLP exporter.

## Build Commands

```bash
# Install required tools (OCB, linters, etc.)
make install-tools

# Build the collector (after fixing module paths)
make build

# Run tests
make test                    # Unit tests
make test-integration       # Integration tests

# Run a single test
go test -v -run TestAdaptiveSamplerRuleEvaluation ./processors/adaptivesampler/

# Validate configuration
make validate-config

# Run collector
make run                    # With default config
make collector-debug        # With debug logging
```

## Development Commands

```bash
# Code quality
make lint                   # Run golangci-lint
make fmt                    # Format code with gofmt and goimports
make vet                    # Run go vet

# Dependencies
make deps                   # Download and tidy
make deps-upgrade          # Upgrade all dependencies

# Docker operations
make docker-build          # Build Docker image
make docker-simple         # Start simple dev setup
make docker-prod          # Start production setup
```

## Architecture Overview

### Custom Processors (Production Ready)

1. **Adaptive Sampler** (`processors/adaptivesampler/` - 576 lines) ✅ **FIXED**
   - Rule-based sampling with expression evaluation
   - **✅ In-memory state management only** (no file persistence)
   - LRU cache with TTL for deduplication
   - **✅ Graceful handling of missing plan attributes**
   - Configuration: `in_memory_only: true`, `rules`, `default_sampling_rate`

2. **Circuit Breaker** (`processors/circuitbreaker/` - 922 lines) ✅ **READY**
   - Per-database protection with 3-state FSM
   - Adaptive timeouts and self-healing
   - New Relic error detection and cardinality protection
   - **✅ Already uses in-memory state management**
   - Configuration: `failure_threshold`, `timeout`, `half_open_requests`

3. **Plan Attribute Extractor** (`processors/planattributeextractor/` - 391 lines) ✅ **SAFE**
   - PostgreSQL/MySQL query plan parsing from existing data
   - Plan hash generation for deduplication
   - **✅ Safe mode enforced** (no direct database EXPLAIN calls)
   - **✅ Graceful degradation when plan data unavailable**
   - Configuration: `safe_mode: true`, `timeout`, `error_mode: ignore`

4. **Verification Processor** (`processors/verification/` - 1,353 lines) ✅ **ENHANCED**
   - **✅ Enhanced PII detection** (credit cards, SSNs, emails, phones)
   - Data quality validation and cardinality protection
   - Auto-tuning and self-healing capabilities
   - Configuration: `pii_detection`, `quality_checks`, `auto_tuning`

### Standard OTEL Components

- **Receivers**: `postgresql`, `mysql`, `sqlquery`
- **Processors**: `memory_limiter`, `batch`, `transform`, `resource`
- **Exporters**: `otlp`, `prometheus`, `debug`

## Configuration Modes

### Standard Mode (Works Today)
```yaml
# config/collector-simplified.yaml
receivers: [postgresql, mysql, sqlquery]
processors: [memory_limiter, batch, transform]
exporters: [otlp, prometheus]
```

### Experimental Mode (Requires Build Fix)
```yaml
# Includes custom processors
processors: [memory_limiter, adaptive_sampler, circuit_breaker, 
            plan_extractor, verification, batch]
```

## Testing Custom Processors

```bash
# Test adaptive sampler
go test -v ./processors/adaptivesampler/

# Test with specific rule evaluation
go test -v -run TestRuleEvaluation ./processors/adaptivesampler/

# Test circuit breaker state transitions
go test -v -run TestCircuitBreakerStates ./processors/circuitbreaker/

# Benchmark processing performance
go test -bench=. ./processors/...
```

## Environment Variables

Required for runtime:
- `POSTGRES_HOST`, `POSTGRES_PORT`, `POSTGRES_USER`, `POSTGRES_PASSWORD`
- `MYSQL_HOST`, `MYSQL_PORT`, `MYSQL_USER`, `MYSQL_PASSWORD`
- `NEW_RELIC_LICENSE_KEY`
- `ENVIRONMENT` (production/staging/development)

## Key Documentation

- `docs/ARCHITECTURE.md` - Validated architecture guide
- `docs/CONFIGURATION.md` - Working configuration examples
- `docs/DEPLOYMENT.md` - Deployment blockers and fixes
- `docs/TECHNICAL_IMPLEMENTATION_DEEPDIVE.md` - Detailed code analysis

## Common Issues

1. **Build fails with module not found**
   - Fix module path inconsistencies (see Critical Context above)

2. **OTLP exporter panic with TODO**
   - Remove custom OTLP exporter or complete implementation

3. **Processor not found error**
   - Ensure custom processors are registered in `main.go`
   - Check TypeStr constants are exported

4. **State file permissions**
   - Adaptive sampler needs write access to state file directory
   - Default: `/var/lib/otel/adaptive_sampler.state`

## Performance Characteristics

- **Memory**: 256-512MB with all processors
- **CPU**: 10-20% with active processing
- **Startup time**: 3-4s with custom processors
- **Processing latency**: 1-5ms added by custom processors

## Documentation Update Guidelines

When making changes to the codebase, maintain documentation accuracy by following these guidelines:

### 1. Processor Changes
When modifying any custom processor:
- Update `docs/ARCHITECTURE.md` with new features/capabilities
- Update `docs/CONFIGURATION.md` with new configuration options
- Update line counts in documentation if significant code is added/removed
- Mark features as [DONE], [PARTIALLY DONE], or [NOT DONE]

### 2. Build System Changes
When fixing build issues or changing module structure:
- Update this CLAUDE.md file's "Critical Context" section
- Update `docs/DEPLOYMENT.md` with new deployment procedures
- Remove warnings once issues are resolved

### 3. Configuration Changes
When adding/modifying configuration options:
- Update `docs/CONFIGURATION.md` with working examples
- Update relevant processor sections in `docs/ARCHITECTURE.md`
- Ensure all examples are validated against actual implementation

### 4. Performance Changes
When optimizing or changing resource usage:
- Update performance metrics in this file
- Update `docs/ARCHITECTURE.md` resource requirements table
- Document any new caching or optimization strategies

### 5. New Features
When adding new processors or major features:
- Create or update relevant sections in `docs/TECHNICAL_IMPLEMENTATION_DEEPDIVE.md`
- Update `docs/UNIFIED_IMPLEMENTATION_OVERVIEW.md` component inventory
- Add to `docs/FINAL_COMPREHENSIVE_SUMMARY.md` if it's a significant addition

### Documentation Validation Process

Before completing any feature:
1. **Verify Claims**: Test that all documented features actually work
2. **Update Status**: Mark implementations as [DONE], [PARTIALLY DONE], or [NOT DONE]
3. **Code Examples**: Ensure all code snippets in docs match actual implementation
4. **Configuration**: Test all configuration examples in documentation
5. **Remove Outdated**: Delete or archive documentation for removed features

### Key Documents to Keep Synchronized

1. **Primary References** (always update these):
   - `docs/ARCHITECTURE.md` - Overall system design and components
   - `docs/CONFIGURATION.md` - All configuration options and examples
   - `docs/DEPLOYMENT.md` - Current deployment status and procedures

2. **Comprehensive Guides** (update for major changes):
   - `docs/UNIFIED_IMPLEMENTATION_OVERVIEW.md` - Complete project status
   - `docs/TECHNICAL_IMPLEMENTATION_DEEPDIVE.md` - Detailed implementation
   - `docs/FINAL_COMPREHENSIVE_SUMMARY.md` - Executive summary

3. **This File** (CLAUDE.md):
   - Update build commands when build system changes
   - Update common issues as new ones are discovered/resolved
   - Keep performance characteristics current

### Example Documentation Update

When fixing the module path issue:
```bash
# After fixing in code, update documentation:
# 1. Remove warning from CLAUDE.md "Critical Context"
# 2. Update docs/DEPLOYMENT.md to show issue as resolved
# 3. Update docs/FINAL_COMPREHENSIVE_SUMMARY.md status from "NEAR PRODUCTION READY" to "PRODUCTION READY"
```

Remember: Documentation accuracy is critical. It's better to mark something as [NOT DONE] than to document features that don't exist.

## Development Philosophy & Task Management

### End-to-End Flow Thinking

Before making any changes, always analyze the complete data flow:

1. **Data Collection** (Database → Receiver)
   - How is data queried from PostgreSQL/MySQL?
   - What metrics are collected?
   - Collection intervals and resource impact?

2. **Processing Pipeline** (Receiver → Processors → Exporter)
   - Which processors touch the data?
   - What transformations occur?
   - How do processors interact (order matters)?
   - Performance implications of each step?

3. **Data Export** (Exporter → New Relic)
   - OTLP format requirements
   - Batching and compression settings
   - Error handling and retries
   - New Relic specific attributes needed?

### OTEL Best Practices Checklist

When implementing changes, ensure compliance with OpenTelemetry standards:

- **Resource Detection**: Proper service.name, environment attributes
- **Semantic Conventions**: Use standard attribute names (db.system, db.name, etc.)
- **Context Propagation**: Maintain trace/span relationships
- **Error Handling**: Non-blocking failures, graceful degradation
- **Batching**: Efficient use of batch processor
- **Memory Management**: Proper use of memory_limiter
- **Observability**: Expose internal metrics for monitoring

### Task Management Requirements

**IMPORTANT**: Always use the TodoWrite tool to manage development tasks. Maintain a minimum of 7 todos at all times to ensure comprehensive planning and tracking.

#### Initial Task Planning
When starting any feature or fix:
```
1. Analyze requirements and impacts
2. Create initial todo list with 7+ items covering:
   - Code changes needed
   - Test updates required
   - Documentation updates
   - Configuration changes
   - Performance validation
   - Integration testing
   - Production readiness checks
```

#### Continuous Task Management
- **Mark todos as in_progress** when starting work
- **Mark as completed** immediately upon finishing
- **Add new todos** as you discover additional work
- **Revisit todo list** after completing each task
- **Maintain minimum 7 todos** by breaking down large tasks

#### Example Todo Structure
```
- Fix module path in go.mod [pending]
- Update import statements in processors [pending]
- Fix ocb-config.yaml module references [pending]
- Test build process after fixes [pending]
- Update CLAUDE.md Critical Context section [pending]
- Update docs/DEPLOYMENT.md with fix confirmation [pending]
- Validate all processor imports [pending]
- Run integration tests [pending]
- Update FINAL_COMPREHENSIVE_SUMMARY.md status [pending]
```

### Change Impact Analysis

Before implementing any change, consider:

1. **Upstream Impact**
   - Will this affect data collection?
   - Database query performance implications?
   - Resource usage on source databases?

2. **Pipeline Impact**
   - Effects on other processors in the chain?
   - Memory/CPU usage changes?
   - Latency additions?

3. **Downstream Impact**
   - New Relic data format compatibility?
   - Metric cardinality changes?
   - Dashboard/alert implications?

4. **Operational Impact**
   - Configuration complexity?
   - Backward compatibility?
   - Migration requirements?

### Example: Adding a New Processor

```bash
# WRONG: Just writing code
❌ Create processor file and start coding

# RIGHT: Full flow analysis with todos
✅ 1. Use TodoWrite to create comprehensive task list:
   - Analyze where in pipeline the processor fits
   - Design processor interface and configuration
   - Implement core processing logic
   - Add comprehensive error handling
   - Create unit tests with 80%+ coverage
   - Add integration tests
   - Update docs/ARCHITECTURE.md
   - Update docs/CONFIGURATION.md
   - Add processor to ocb-config.yaml
   - Test end-to-end flow
   - Validate New Relic data appears correctly
   - Update performance characteristics
   - Create troubleshooting guide
```

### Continuous Validation

During development, regularly check:
- `TodoRead` - Review current task status
- `make test` - Ensure nothing breaks
- `make collector-debug` - Test with real data
- Check metrics endpoint for processor health
- Validate data appears correctly in New Relic