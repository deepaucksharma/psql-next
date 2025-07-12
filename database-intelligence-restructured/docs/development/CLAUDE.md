# AI Assistant Context for Database Intelligence

This file provides comprehensive guidance for AI assistants (Claude, GitHub Copilot, etc.) working with the Database Intelligence codebase.

## 🚨 Critical Context

### Current State (IMPORTANT - READ FIRST)
- **PostgreSQL Only**: MySQL support has been completely removed. All references to MySQL should be ignored/removed.
- **Two Modes**: Config-Only (standard OTel) and Custom (enhanced features)
- **Production Ready**: Both modes are stable and tested
- **Module Structure**: Consolidated from 15+ modules to single module (fixed circular dependencies)
- **Metrics**: 35+ PostgreSQL metrics in Config-Only, 50+ in Custom mode

### What Works vs What Doesn't
✅ **Working**:
- Config-Only mode with all PostgreSQL metrics
- Custom mode with ASH, query intelligence, adaptive sampling
- Parallel deployment for mode comparison
- New Relic OTLP export
- Docker/K8s deployments
- Comprehensive test suite

❌ **Not Working/Removed**:
- MySQL support (completely removed)
- Some custom processors may need updates
- OHI migration (partially implemented)

⚠️ **In Progress**:
- Performance optimization for high-volume environments
- Cost control refinements
- Additional query intelligence features

## 📁 Key Files Reference

### Configuration
- `configs/config-only-mode.yaml` - Standard OTel configuration
- `configs/custom-mode.yaml` - Enhanced mode configuration
- `deployments/docker/compose/docker-compose-parallel.yaml` - Parallel deployment

### Core Components
- `components/receivers/ash/` - Active Session History receiver
- `components/processors/adaptivesampler/` - Dynamic sampling
- `components/processors/circuitbreaker/` - Overload protection
- `components/processors/planattributeextractor/` - Query plan extraction

### Testing & Validation
- `tools/postgres-test-generator/` - Comprehensive metric generator
- `tools/load-generator/` - Load testing tool
- `scripts/verify-metrics.sh` - Metric validation
- `scripts/validate-metrics-e2e.sh` - E2E validation

### Documentation
- `README.md` - Main entry point
- `docs/guides/QUICK_START.md` - Getting started
- `docs/guides/TROUBLESHOOTING.md` - Problem solving
- `docs/reference/METRICS.md` - All metrics reference

## 🛠️ Common Tasks

### 1. Adding a New PostgreSQL Metric

```bash
# 1. Update receiver configuration
# Edit: configs/config-only-mode.yaml
postgresql:
  metrics:
    postgresql.new_metric:
      enabled: true

# 2. Update documentation
# Edit: docs/reference/METRICS.md

# 3. Test the metric
./scripts/verify-metrics.sh

# 4. Validate in New Relic
# Query: SELECT latest(postgresql.new_metric) FROM Metric
```

### 2. Debugging Metric Collection Issues

```bash
# Check collector logs
docker logs db-intel-collector-config-only

# Enable debug mode
# Add to config:
exporters:
  debug:
    verbosity: detailed

# Check PostgreSQL permissions
docker exec db-intel-postgres psql -U postgres -c "\du"

# Verify metric flow
curl -s http://localhost:13133/metrics | grep otelcol_receiver
```

### 3. Running Tests

```bash
# Quick validation
make test            # Unit tests only
make test-e2e        # End-to-end tests

# Full test suite
make test-coverage   # With coverage report

# Test specific component
go test -v ./components/receivers/ash/...

# Generate test load
cd tools/postgres-test-generator
go run main.go -workers=10
```

### 4. Building and Deploying

```bash
# Local development
make build          # Build binaries
make docker-build   # Build Docker images
make run-dev       # Run locally

# Deployment
./scripts/deploy-parallel-modes.sh  # Deploy both modes
./scripts/migrate-dashboard.sh deploy dashboards/newrelic/postgresql-parallel-dashboard.json
```

## 🏗️ Architecture Patterns

### Component Structure
```go
// All components follow this pattern
type Component struct {
    config *Config
    logger *zap.Logger
    // component-specific fields
}

func (c *Component) Start(ctx context.Context, host component.Host) error
func (c *Component) Shutdown(ctx context.Context) error
```

### Error Handling
```go
// Always wrap errors with context
if err != nil {
    return fmt.Errorf("failed to connect to PostgreSQL: %w", err)
}

// Use structured logging
logger.Error("Query failed", 
    zap.Error(err),
    zap.String("query", query),
    zap.Duration("elapsed", elapsed))
```

### Configuration Validation
```go
func (cfg *Config) Validate() error {
    if cfg.Datasource == "" {
        return errors.New("datasource is required")
    }
    // Additional validation
    return nil
}
```

## 🐛 Common Issues & Solutions

### Issue: No metrics appearing
1. Check NEW_RELIC_LICENSE_KEY is set
2. Verify PostgreSQL connectivity
3. Check collector logs for errors
4. Ensure metrics are enabled in config

### Issue: High memory usage
1. Reduce collection_interval
2. Enable adaptive sampling
3. Configure memory_limiter processor
4. Check for memory leaks with pprof

### Issue: Missing specific metrics
1. Check if metric is enabled in config
2. Verify PostgreSQL version compatibility
3. Check required PostgreSQL extensions (pg_stat_statements)
4. Review PostgreSQL user permissions

## 📊 Performance Guidelines

### Optimization Targets
- Metric collection: <5ms per cycle
- Memory usage: <512MB (Config-Only), <1GB (Custom)
- CPU usage: <5% (Config-Only), <20% (Custom)
- Network overhead: <1MB/s

### Profiling
```bash
# Enable pprof
# Add to config:
extensions:
  pprof:
    endpoint: 0.0.0.0:1777

# Access profiles
go tool pprof http://localhost:1777/debug/pprof/heap
go tool pprof http://localhost:1777/debug/pprof/profile
```

## 🔒 Security Considerations

### Credentials
- Never hardcode credentials
- Use environment variables
- Implement least-privilege PostgreSQL user
- Enable TLS for database connections

### Required PostgreSQL Permissions
```sql
-- Minimum permissions needed
GRANT CONNECT ON DATABASE postgres TO monitor_user;
GRANT USAGE ON SCHEMA pg_catalog TO monitor_user;
GRANT SELECT ON ALL TABLES IN SCHEMA pg_catalog TO monitor_user;
```

## 🚀 Development Workflow

### Making Changes
1. Create feature branch
2. Make changes following patterns
3. Add/update tests
4. Update documentation
5. Run `make pre-commit`
6. Submit PR with conventional commit message

### Testing Changes
```bash
# 1. Unit test your changes
go test -v ./path/to/changed/package

# 2. Run integration tests
make test-integration

# 3. Test with real PostgreSQL
docker-compose up -d postgres
make run-dev

# 4. Verify metrics in New Relic
# Check dashboard or run NRQL queries
```

## 📝 Code Style Guidelines

### Go Conventions
- Use standard Go formatting (`gofmt`)
- Follow effective Go guidelines
- Keep functions small and focused
- Add comments for exported functions
- Use meaningful variable names

### Error Messages
- Be specific about what failed
- Include relevant context
- Suggest solutions when possible
- Use consistent format

### Logging
- Use structured logging (zap)
- Include relevant fields
- Use appropriate log levels
- Avoid logging sensitive data

## 🎯 Priority Areas for Improvement

1. **Performance Optimization**
   - Reduce memory allocations
   - Optimize query execution
   - Improve batching efficiency

2. **Feature Enhancements**
   - Additional query intelligence
   - Better anomaly detection
   - Enhanced cost control

3. **Testing**
   - Increase test coverage
   - Add more E2E scenarios
   - Performance benchmarks

4. **Documentation**
   - More troubleshooting guides
   - Performance tuning guide
   - Production best practices

## 🤖 AI Assistant Tips

### When Asked to Fix Issues
1. First understand the current state (PostgreSQL-only)
2. Check logs and error messages
3. Verify configuration
4. Test the fix locally
5. Update relevant documentation

### When Adding Features
1. Follow existing patterns
2. Add comprehensive tests
3. Update configuration examples
4. Document new metrics/options
5. Consider performance impact

### When Reviewing Code
1. Check for PostgreSQL-only focus
2. Verify error handling
3. Look for performance issues
4. Ensure tests are included
5. Check documentation updates

## 📚 Additional Resources

### Internal Docs
- Architecture: `docs/reference/ARCHITECTURE.md`
- Metrics: `docs/reference/METRICS.md`
- API: `docs/reference/API.md`

### External Resources
- [OpenTelemetry Collector Docs](https://opentelemetry.io/docs/collector/)
- [PostgreSQL Statistics](https://www.postgresql.org/docs/current/monitoring-stats.html)
- [New Relic OTLP](https://docs.newrelic.com/docs/apis/otlp/)

## 🔄 Version History

- **v2.0**: PostgreSQL-only implementation
- **v1.5**: Module consolidation, memory fixes
- **v1.0**: Initial dual-database support

---

**Remember**: This is a PostgreSQL-only project. Any MySQL references in older code should be removed or ignored.