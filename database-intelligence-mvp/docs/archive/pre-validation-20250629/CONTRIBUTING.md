# Contributing to Database Intelligence

## Contribution Philosophy

We follow the OpenTelemetry principle: **"Contribute back, don't fork"**. Every enhancement should have a path to upstream contribution.

## Types of Contributions

### 1. Configuration Improvements

**What We Need**:
- Database-specific configurations
- Safety improvements
- Performance optimizations
- New query patterns

**Example Contribution**:
```yaml
# Oracle support configuration
receivers:
  sqlquery/oracle_plans:
    driver: oracle
    dsn: "${ORACLE_DSN}"
    queries:
      - sql: |
          SELECT 
            sql_id,
            sql_text,
            DBMS_SQLTUNE.REPORT_SQL_PLAN_BASELINE(
              sql_handle => sql_handle,
              format => 'JSON'
            ) as plan
          FROM dba_sql_plan_baselines
          WHERE last_executed > SYSTIMESTAMP - INTERVAL '5' MINUTE
```

### 2. Processor Enhancements

**Priority Areas**:
- Additional plan format parsers
- New attribute extractors
- Sampling strategies
- State storage backends

**Contribution Process**:
1. Discuss in issue first
2. Implement with tests
3. Document thoroughly
4. Submit PR with benchmarks

### 3. Documentation

**Always Needed**:
- Real-world examples
- Troubleshooting scenarios
- Performance tuning guides
- Database-specific guides

**Good Documentation PRs**:
- Include actual command outputs
- Explain the "why" not just "how"
- Add diagrams where helpful
- Test on fresh installs

### 4. Testing & Validation

**Test Contributions**:
- Integration test scenarios
- Load testing scripts
- Database version matrices
- Configuration validators

**Example Test**:
```go
func TestPostgreSQLSafetyTimeout(t *testing.T) {
    // Test that EXPLAIN respects timeout
    config := `
      SET LOCAL statement_timeout = '1ms';
      EXPLAIN (FORMAT JSON) 
      SELECT pg_sleep(10);
    `
    
    start := time.Now()
    _, err := db.Exec(config)
    elapsed := time.Since(start)
    
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "timeout")
    assert.Less(t, elapsed, 2*time.Second)
}
```

## Upstream Contribution Path

### Phase 1: Validate in Custom Processor
1. Implement feature in custom processor
2. Validate with production usage
3. Gather metrics and feedback
4. Document lessons learned

### Phase 2: Propose to OpenTelemetry
1. Open OTEP (Enhancement Proposal)
2. Present use case and data
3. Propose general solution
4. Build consensus

### Phase 3: Implement Upstream
1. Follow OTEL contribution guidelines
2. Implement with tests
3. Add documentation
4. Support through review

### Example: EXPLAIN Support in sqlqueryreceiver

```go
// Proposed enhancement to upstream
type ExplainConfig struct {
    Enabled  bool   `mapstructure:"enabled"`
    Format   string `mapstructure:"format"`   // json, xml, text
    Analyze  bool   `mapstructure:"analyze"`  // EXPLAIN ANALYZE
    Buffers  bool   `mapstructure:"buffers"`  // Include buffer stats
    Timeout  string `mapstructure:"timeout"`  // Statement timeout
}

type QueryConfig struct {
    SQL     string         `mapstructure:"sql"`
    Explain *ExplainConfig `mapstructure:"explain,omitempty"`
}
```

## Development Setup

### Prerequisites

```bash
# Go 1.21+
go version

# Docker for testing
docker --version

# Kind for Kubernetes testing
kind --version
```

### Local Development

```bash
# Clone repository
git clone https://github.com/youraccount/db-intelligence-mvp
cd db-intelligence-mvp

# Install dependencies
go mod download

# Run tests
make test

# Build collector
make build

# Run locally
./bin/otelcol --config=config/dev.yaml
```

### Testing Changes

```bash
# Unit tests
go test ./...

# Integration tests
make integration-test

# Load test
make load-test

# Database compatibility
make test-postgres-versions
```

## Code Standards

### Go Code

```go
// Package comment explains purpose
// Package planparser extracts attributes from database query plans
package planparser

import (
    "context"
    "fmt"
    
    "go.opentelemetry.io/collector/pdata/plog"
)

// ExtractorConfig configures the plan attribute extractor
type ExtractorConfig struct {
    // Timeout for each extraction operation
    TimeoutMS int `mapstructure:"timeout_ms"`
    
    // ErrorMode determines handling of extraction errors
    // Valid values: "ignore", "propagate"
    ErrorMode string `mapstructure:"error_mode"`
}

// Validate checks the configuration
func (cfg *ExtractorConfig) Validate() error {
    if cfg.TimeoutMS <= 0 {
        return fmt.Errorf("timeout_ms must be positive, got %d", cfg.TimeoutMS)
    }
    return nil
}
```

### Configuration YAML

```yaml
# Clear section comments
processors:
  # Plan attribute extraction with safety limits
  plan_attribute_extractor:
    # Timeout prevents runaway processing
    timeout_ms: 100
    
    # Error mode for production safety
    # Options: ignore (default), propagate
    error_mode: ignore
    
    # Database-specific extraction rules
    extractors:
      # PostgreSQL uses JSON path
      postgresql:
        format_detection: "$[0].Plan"
        attributes:
          # Cost is critical for sampling
          db.query.plan.cost: "$[0].Plan['Total Cost']"
```

### Documentation

```markdown
## Feature Name

### Overview
Brief description of what and why.

### Configuration
```yaml
# Complete working example
```

### How It Works
Technical explanation with examples.

### Limitations
Known issues and workarounds.

### See Also
- Related documentation
- Upstream issues
```

## Review Process

### PR Requirements
1. **Clear Title**: `feat:` `fix:` `docs:` `test:` prefix
2. **Description**: Problem, solution, testing done
3. **Tests**: Unit and/or integration tests
4. **Documentation**: Updates to relevant .md files
5. **Benchmarks**: For performance-critical changes

### Review Checklist
- [ ] Safe for production databases
- [ ] Handles errors gracefully
- [ ] Resource usage acceptable
- [ ] Documentation complete
- [ ] Tests pass
- [ ] No PII exposure risk

## Community

### Communication Channels
- GitHub Issues: Bugs and features
- Slack #database-observability: Questions
- Monthly Office Hours: Deep dives

### Code of Conduct
- Be respectful and inclusive
- Focus on what's best for users
- Assume positive intent
- Help others learn

### Recognition
Contributors will be recognized in:
- Release notes
- Documentation credits
- Conference talks
- Community spotlights

## Future Contribution Areas

### High-Priority Needs

1. **Cloud Database Support**
   - AWS RDS configurations
   - Azure SQL Database
   - Google Cloud SQL

2. **Performance Optimization**
   - Query plan caching
   - Parallel collection
   - Compression algorithms

3. **Security Enhancements**
   - Vault integration
   - mTLS support
   - Encryption at rest

4. **Observability**
   - Prometheus metrics
   - Distributed tracing
   - Health check endpoints

Thank you for contributing to make database observability better for everyone!