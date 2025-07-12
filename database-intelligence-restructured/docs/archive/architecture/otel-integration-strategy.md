# OpenTelemetry Integration Strategy

## Quick Start

**Choose Your Mode:**
- **Config-Only**: Standard OTel components, YAML configuration only → [Jump to Config-Only](#configuration-only-mode)
- **Enhanced**: Custom components for advanced analytics → [Jump to Enhanced](#enhanced-mode)

Both modes are New Relic compatible via OTLP.

## Architecture Overview

```
Data Sources → OTel Collector → New Relic (OTLP)
```

| | Config-Only | Enhanced |
|---|---|---|
| **Setup** | YAML only | Custom binary |
| **Metrics** | Core DB stats | + Query plans, ASH |
| **Best For** | Standard monitoring | Enterprise analytics |

## Configuration-Only OTel Strategy

### Overview

The config-only approach leverages existing OpenTelemetry Collector components without requiring custom code. This mode provides comprehensive database monitoring through declarative configuration.

### Standard Receivers

<details>
<summary><b>PostgreSQL Receiver</b></summary>

```yaml
receivers:
  postgresql:
    endpoint: "${DB_ENDPOINT}"
    username: "${DB_USERNAME}"
    password: "${DB_PASSWORD}"
    collection_interval: 30s
```
[Full config example →](../../configs/examples/config-only-base.yaml)
</details>

<details>
<summary><b>MySQL Receiver</b></summary>

```yaml
receivers:
  mysql:
    endpoint: "${MYSQL_ENDPOINT}"
    collection_interval: 30s
```
[Full config example →](../../configs/examples/config-only-mysql.yaml)
</details>

<details>
<summary><b>Custom SQL Queries</b></summary>

```yaml
receivers:
  sqlquery:
    queries:
      - sql: "SELECT COUNT(*) as connections FROM pg_stat_activity"
        metrics:
          - metric_name: custom.connections
            value_column: connections
```
</details>

### Essential Processors

```yaml
processors:
  memory_limiter:
    limit_mib: 512
    
  batch:
    timeout: 10s
    
  resource:
    attributes:
      - key: service.name
        value: "${SERVICE_NAME}"
        action: insert
```

**Key processors**: `memory_limiter` → `resource` → `batch` → `cumulativetodelta`

### Export to New Relic

```yaml
exporters:
  otlp:
    endpoint: "https://otlp.nr-data.net:4318"
    headers:
      api-key: "${NEW_RELIC_LICENSE_KEY}"
```

**Complete pipeline**: receivers → processors → exporters

## Enhanced Mode

### Custom Receivers

**Enhanced SQL**: Query analytics, execution plans, wait events  
**ASH Receiver**: 1-second session sampling, blocking detection

```yaml
receivers:
  enhancedsql:
    features:
      query_stats: true
      execution_plans: true
      
  ash:
    sampling_interval: 1s
    features:
      blocking_chains: true
```

[Full component design →](custom-components-design.md)

### Custom Processor Pipeline

1. **Adaptive Sampler**: Dynamic sampling based on load
2. **Circuit Breaker**: Protects database from overhead
3. **Plan Extractor**: Query plan analysis
4. **Verification**: PII detection, cardinality limits
5. **Cost Control**: Budget management
6. **NR Error Monitor**: Integration health
7. **Query Correlator**: Transaction tracking

```yaml
processors: [adaptive_sampler, circuitbreaker, verification, costcontrol, batch]
```

[See complete enhanced configuration →](../../configs/examples/enhanced-mode-full.yaml)

## New Relic Integration

### Endpoints
- **US**: `https://otlp.nr-data.net:4318`
- **EU**: `https://otlp.eu01.nr-data.net:4318`

### Required Attributes
```yaml
service.name: "postgresql-prod-01"
db.system: "postgresql"
deployment.environment: "production"
```

### Validation
```sql
-- Check metrics
SELECT count(*) FROM Metric WHERE metric.name LIKE 'postgresql.%'

-- Check errors  
SELECT count(*) FROM NrIntegrationError WHERE category = 'MetricAPI'
```

[Full integration guide →](../new-relic-integration-guide.md)

## Metrics Collected

**Core Metrics** (Config-Only):
- Connections, transactions, cache hits
- Query counts and duration
- Database and table sizes
- Host CPU, memory, disk I/O

**Advanced Metrics** (Enhanced Mode):
- Query execution plans
- Active session history (ASH)
- Wait events and blocking chains
- Plan regression detection

[Full metrics documentation →](../metrics-collection-strategy.md)

## Quick Implementation

### Phase 1: Start Simple
1. Deploy standard OTel Collector
2. Use config-only YAML
3. Validate in New Relic

### Phase 2: Add Intelligence (Optional)
1. Deploy enhanced collector
2. Enable advanced features
3. Monitor performance impact

## Example Configurations

- [Config-Only PostgreSQL →](../../configs/examples/config-only-base.yaml)
- [Config-Only MySQL →](../../configs/examples/config-only-mysql.yaml)
- [Enhanced Mode Full →](../../configs/examples/enhanced-mode-full.yaml)

## Next Steps

1. **Deploy**: [Deployment Guide](../deployment-guide.md)
2. **Tune**: [Performance Tuning](../performance-tuning-guide.md)
3. **Monitor**: [New Relic Integration](../new-relic-integration-guide.md)

---

**Questions?** Check our [example configs](../../configs/examples/) or [detailed guides](../)