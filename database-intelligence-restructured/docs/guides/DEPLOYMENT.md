# PostgreSQL Parallel Deployment Guide: Config-Only vs Custom Mode

This guide explains how to deploy and compare both Database Intelligence modes for PostgreSQL monitoring running in parallel.

## Overview

The parallel deployment allows you to:
- Run Config-Only mode (standard OTel components) and Custom mode (enhanced features) simultaneously for PostgreSQL
- Compare metrics, performance, and capabilities side-by-side
- Evaluate the value of enhanced features before full adoption
- Maintain backward compatibility while testing new features

## Architecture

```
     ┌─────────────────────┐
     │   PostgreSQL DB     │
     │   (Shared)          │
     └──────────┬──────────┘
                │
     ┌──────────┴──────────┐
     │                     │
┌────▼──────────────┐     ┌──────────▼────────────┐
│ Config-Only       │     │ Custom/Enhanced       │
│ Collector         │     │ Collector             │
│                   │     │                       │
│ • PostgreSQL recv │     │ • PostgreSQL recv     │
│ • SQL Query recv  │     │ • ASH receiver        │
│ • Host Metrics    │     │ • Enhanced SQL recv   │
│                   │     │ • Kernel Metrics      │
│ Standard          │     │ • Adaptive Sampling   │
│ Processors        │     │ • Circuit Breaker     │
│                   │     │ • Query Plans         │
│                   │     │ • Cost Control        │
└────────┬──────────┘     └──────────┬────────────┘
         │                           │
         └────────────┬──────────────┘
                      │
              ┌───────▼────────┐
              │  New Relic     │
              │  • PostgreSQL  │
              │    Dashboard   │
              │  • Comparison  │
              └────────────────┘
```

## Quick Start

### 1. Prerequisites

```bash
# Required environment variables
export NEW_RELIC_LICENSE_KEY="your-license-key"
export NEW_RELIC_ACCOUNT_ID="your-account-id"

# Optional
export NEW_RELIC_OTLP_ENDPOINT="https://otlp.nr-data.net:4317"
export DOCKER_REGISTRY="your-registry"  # If using private registry
```

### 2. Deploy Everything

```bash
# One command deployment
./scripts/deploy-parallel-modes.sh

# This will:
# 1. Build custom collector image
# 2. Start all services
# 3. Wait for health checks
# 4. Optionally deploy dashboards
```

### 3. Access Points

- **Config-Only Collector**: http://localhost:4318 (OTLP HTTP)
- **Custom Mode Collector**: http://localhost:5318 (OTLP HTTP)
- **PostgreSQL**: localhost:5432
- **New Relic Dashboard**: PostgreSQL dashboard showing both modes (deployed automatically)

## Mode Comparison

### Config-Only Mode

**What it includes:**
- Standard PostgreSQL receiver (35+ metrics)
- SQL Query receiver for custom PostgreSQL queries
- Host metrics (CPU, memory, disk, network)
- Basic processors (batch, memory limiter, attributes)

**Use cases:**
- Basic database monitoring
- Standard metric collection
- Simple deployments
- Lower resource usage

**Example metrics:**
```
postgresql.backends
postgresql.database.size
postgresql.commits
postgresql.deadlocks
postgresql.blocks_read
postgresql.wal.lag
system.cpu.utilization
```

### Custom/Enhanced Mode

**What it includes:**
- Everything from Config-Only mode, PLUS:
- ASH (Active Session History) receiver
- Enhanced SQL receiver with query stats
- Kernel metrics receiver
- Query plan extraction
- Query correlation
- Adaptive sampling
- Circuit breaker protection
- Cost control processor
- OHI transformation for compatibility

**Use cases:**
- Deep database performance analysis
- Query optimization
- Blocking and wait event analysis
- Advanced troubleshooting
- Intelligent metric processing

**Exclusive metrics:**
```
db.ash.active_sessions
db.ash.wait_events
db.ash.blocked_sessions
db.ash.long_running_queries
postgres.slow_queries.* (with plan analysis)
kernel.cpu.pressure
adaptive_sampling_rate
circuit_breaker_state
```

## New Relic Dashboard

### PostgreSQL Parallel Monitoring Dashboard
A comprehensive dashboard that monitors both deployment modes for PostgreSQL:

**Pages**:
1. **Executive Overview**: Deployment status, health scores, session counts
2. **Connection & Performance**: Connections, transactions, block I/O, row operations
3. **Wait Events & Blocking**: Wait event analysis, deadlocks, blocked sessions
4. **Query Intelligence**: Query performance, plans, execution trends
5. **Storage & Replication**: Database size, table stats, replication monitoring
6. **Enhanced Features**: ASH heatmaps, intelligent processing (Custom mode only)
7. **Mode Comparison**: Metric coverage, performance impact, intelligence value
8. **System Resources**: CPU, memory, and network usage by mode
9. **Alerting Recommendations**: PostgreSQL-specific alerts and conditions

**Key Features**:
- Real-time mode comparison
- Automatic mode detection via `deployment.mode` attribute
- Comprehensive metrics from both modes in one view
- Built-in alert recommendations
- Cost analysis and optimization metrics

## Configuration Details

### Config-Only Mode (`config-only-mode.yaml`)

```yaml
receivers:
  postgresql:
    collection_interval: 10s
    # All standard metrics enabled
    
  mysql:
    collection_interval: 10s
    # All standard metrics enabled
    
  sqlquery/postgresql:
    collection_interval: 30s
    queries:
      - sql: "SELECT state, COUNT(*) FROM pg_stat_activity..."
      
processors:
  attributes:
    actions:
      - key: deployment.mode
        value: config-only
        
exporters:
  otlp:
    endpoint: ${NEW_RELIC_OTLP_ENDPOINT}
```

### Custom Mode (`custom-mode.yaml`)

```yaml
receivers:
  ash:
    collection_interval: 1s
    sampling:
      base_rate: 1.0
      adaptive: true
      
  enhancedsql:
    queries:
      - name: query_stats
        sql: "SELECT * FROM pg_stat_statements..."
        
processors:
  adaptivesampler:
    evaluation_interval: 30s
    
  circuitbreaker:
    failure_threshold: 5
    
  planattributeextractor:
    extract_parameters: true
    
  ohitransform:
    # For backward compatibility
    
exporters:
  otlp:
    # Metrics to New Relic
  nri:
    # Events for OHI compatibility
```

## Monitoring & Troubleshooting

### View Logs

```bash
# Config-Only mode
docker logs -f db-intel-collector-config-only

# Custom mode
docker logs -f db-intel-collector-custom

# Databases
docker logs db-intel-postgres
docker logs db-intel-mysql
```

### Check Health

```bash
# All services status
docker compose -f deployments/docker/compose/docker-compose-parallel.yaml ps

# Collector health (custom mode)
curl http://localhost:13133/health
```

### Common Issues

1. **High Memory Usage (Custom Mode)**
   - Adjust `memory_limiter` processor settings
   - Reduce ASH sampling rate
   - Increase `cost_control` limits

2. **Missing Metrics**
   - Check collector logs for errors
   - Verify database permissions
   - Ensure receivers are properly configured

3. **Dashboard No Data**
   - Verify `deployment.mode` attribute is set
   - Check New Relic license key
   - Confirm OTLP endpoint connectivity

## Performance Comparison

| Metric | Config-Only | Custom Mode |
|--------|-------------|-------------|
| CPU Usage | ~5-10% | ~15-25% |
| Memory Usage | ~200MB | ~500MB-1GB |
| Metrics/sec | ~100-200 | ~500-1000 |
| Unique Metrics | ~150 | ~300+ |
| Query Analysis | Basic | Advanced |
| Session Monitoring | Connection count | Full ASH |
| Cost (DPM) | ~6,000 | ~30,000 |

## Migration Path

1. **Phase 1**: Run both modes in parallel
2. **Phase 2**: Compare dashboards and validate data
3. **Phase 3**: Gradually shift traffic to custom mode
4. **Phase 4**: Deprecate config-only mode

## Best Practices

1. **Start with Config-Only** for simple monitoring needs
2. **Enable Custom Mode** for:
   - Performance investigations
   - Query optimization projects
   - Blocking/deadlock issues
   - Capacity planning

3. **Resource Planning**:
   - Custom mode requires 2-3x more resources
   - Consider dedicated hosts for production
   - Monitor collector metrics closely

4. **Cost Optimization**:
   - Use adaptive sampling in custom mode
   - Enable cost control processor
   - Set appropriate retention policies

## Next Steps

1. Deploy the parallel setup
2. Review all three dashboards
3. Run load tests to see differences
4. Evaluate which mode fits your needs
5. Plan migration strategy if choosing custom mode

## Support

For issues or questions:
1. Check collector logs first
2. Review dashboard queries
3. Validate configuration syntax
4. Contact Database Intelligence team