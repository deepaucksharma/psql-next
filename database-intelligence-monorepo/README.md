# Database Intelligence MySQL - Monorepo

A modular OpenTelemetry-based MySQL monitoring system organized as a monorepo with independent, composable modules. Now includes all enhanced features from the master-enhanced configuration including cross-signal correlation, circuit breakers, and ML-based analysis.

## 🚀 Enhanced Features

- **Ultra-comprehensive SQL Intelligence**: 500-line query with CTEs for complete analysis
- **Cross-Signal Correlation**: Trace, log, and metric correlation with exemplars
- **Circuit Breaker Pattern**: Fault tolerance with fallback mechanisms
- **Persistent Queues**: File-based storage for reliability
- **Multi-Tenancy**: Schema-based routing and prioritization
- **ML-Based Analysis**: Anomaly detection and pattern recognition
- **Synthetic Monitoring**: Canary queries for baseline establishment
- **Business Impact Scoring**: Revenue and SLA impact assessment

## Repository Structure

```
database-intelligence-monorepo/
├── modules/                    # Independent monitoring modules
│   ├── core-metrics/          # Basic MySQL metrics (enhanced available)
│   ├── sql-intelligence/      # Query analysis (enhanced available)
│   ├── wait-profiler/         # Wait event profiling
│   ├── anomaly-detector/      # Statistical anomaly detection
│   ├── business-impact/       # Business impact scoring
│   ├── replication-monitor/   # Replication health (enhanced available)
│   ├── performance-advisor/   # Performance recommendations
│   ├── resource-monitor/      # System resource tracking
│   ├── canary-tester/         # Synthetic monitoring (enhanced available)
│   ├── alert-manager/         # Alert aggregation and routing
│   └── cross-signal-correlator/ # NEW: Trace/log/metric correlation
├── shared/                    # Shared resources
│   ├── interfaces/           # Common interfaces
│   ├── docker/              # Shared Docker configurations
│   └── scripts/             # Shared scripts
├── integration/             # Integration testing
│   ├── docker-compose.all.yaml      # Standard integration
│   └── docker-compose.enhanced.yaml # NEW: Enhanced integration
└── Makefile                # Root orchestration
```

## Quick Start

### Basic Usage

```bash
# Run individual modules
make run-core-metrics
make run-sql-intelligence

# Run module groups
make run-core         # Core metrics + resource monitor
make run-intelligence # SQL intelligence + wait profiler + anomaly detector
make run-business     # Business impact + performance advisor
```

### Enhanced Usage (Recommended)

```bash
# Run all modules with enhanced configurations
make run-enhanced

# Run specific module with enhanced config
make run-enhanced-core-metrics
make run-enhanced-sql-intelligence

# Run full stack with all features
make run-full-stack
```

### Testing

```bash
# Test all modules
make test

# Validate all configurations
make validate-configs

# Run integration tests
make integration-enhanced
```

## Module Capabilities

### Core Modules

#### 1. Core Metrics (Enhanced Available)
- All 40+ MySQL metrics
- Primary/replica monitoring
- Host resource metrics
- Delta conversion and entity synthesis

#### 2. SQL Intelligence (Enhanced Available)
- Ultra-comprehensive 500-line SQL query
- Real-time wait analysis
- ML-based anomaly scoring
- Business impact calculation
- Advanced advisory recommendations

#### 3. Wait Profiler
- Wait event categorization
- Mutex contention tracking
- I/O wait profiling
- Lock wait monitoring

#### 4. Replication Monitor (Enhanced Available)
- Master/slave status
- GTID tracking
- Binary log analysis
- Health scoring and alerts

### Intelligence Modules

#### 5. Anomaly Detector
- Z-score based detection
- Connection spike detection
- Query latency deviation
- Resource usage patterns

#### 6. Business Impact
- Revenue impact scoring
- SLA violation detection
- Critical table identification
- Customer operation tracking

#### 7. Performance Advisor
- Missing index recommendations
- Connection pool sizing
- Cache optimization
- Query pattern analysis

### Enhanced Modules

#### 8. Cross-Signal Correlator (NEW)
- Trace-to-metrics correlation
- Log parsing and conversion
- Exemplar generation
- Span metrics with histograms

#### 9. Canary Tester (Enhanced Available)
- Synthetic query execution
- Baseline deviation detection
- OLTP/OLAP workload simulation
- Health status classification

#### 10. Alert Manager
- Alert aggregation
- Webhook notifications
- Deduplication
- State management

## Configuration

### Environment Variables

```bash
# Core Configuration
export MYSQL_ENDPOINT=mysql:3306
export MYSQL_USER=root
export MYSQL_PASSWORD=password

# Enhanced Features
export ENABLE_SQL_INTELLIGENCE=true
export ML_FEATURES_ENABLED=true
export CIRCUIT_FAILURE_THRESHOLD=5
export ROLLOUT_PERCENTAGE=100

# Module Selection
export COLLECTOR_CONFIG=collector-enhanced.yaml  # Use enhanced configs
```

### Deployment Modes

1. **Minimal**: Basic monitoring only
2. **Standard**: Production recommended
3. **Advanced**: Deep insights with ML
4. **Enhanced**: All features enabled (NEW)

## Integration Patterns

### With New Relic

```yaml
# Set in environment
NEW_RELIC_LICENSE_KEY=your-key
NEW_RELIC_ACCOUNT_ID=your-account
OTLP_ENDPOINT=https://otlp.nr-data.net
```

### With Application Traces

```yaml
# Send traces to cross-signal-correlator
otlp:
  endpoint: cross-signal-correlator:4317
```

### With MySQL Logs

```yaml
# Mount slow query log
volumes:
  - /var/log/mysql:/var/log/mysql:ro
```

## Advanced Features

### Circuit Breaker Protection

```yaml
# Automatic failover on export failures
circuit_breaker:
  failure_threshold: 5
  recovery_timeout: 30s
  fallback_exporter: otlp/fallback
```

### Persistent Queues

```yaml
# File-based queue for reliability
sending_queue:
  enabled: true
  storage: file_storage/reliable
  queue_size: 50000
```

### Multi-Tenant Routing

```yaml
# Route by schema criticality
routing/tenant:
  from_attribute: db_schema
  table:
    - value: "orders|payments"
      exporters: [critical_tenant]
    - value: "analytics"
      exporters: [batch_tenant]
```

## Performance Optimization

### Resource Limits

Each module has optimized resource limits:
- Core Metrics: 2 CPU, 1.75GB RAM
- SQL Intelligence: 4 CPU, 3.5GB RAM
- Cross-Signal Correlator: 2 CPU, 2GB RAM
- Others: 1-2 CPU, 0.5-1GB RAM

### Batching Strategies

- Minimal: 1s timeout, 100 batch size
- Standard: 5s timeout, 1000 batch size
- Large: 30s timeout, 10000 batch size

## Troubleshooting

```bash
# Check module health
make health

# View module logs
make logs-sql-intelligence

# Validate configurations
make validate-configs

# Clean up resources
make docker-clean
```

## Development

```bash
# Setup development environment
make dev-setup

# Build all modules
make build

# Run specific tests
make quick-test-core-metrics

# Performance testing
make perf-test
```

## CI/CD

```bash
# CI build
make ci-build

# CI test
make ci-test

# CI integration
make ci-integration

# CI validation
make ci-validate
```

## Documentation

- [Implementation Status](IMPLEMENTATION_STATUS.md) - Detailed feature implementation
- [Module READMEs](modules/) - Individual module documentation
- [Integration Guide](integration/README.md) - Integration patterns
- [E2E Testing](shared/e2e/README.md) - End-to-end testing guide

## License

MIT License - See LICENSE file for details