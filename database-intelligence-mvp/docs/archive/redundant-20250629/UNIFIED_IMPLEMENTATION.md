# Database Intelligence MVP - Unified Implementation Guide

## Overview

This document describes the unified implementation of the Database Intelligence MVP that consolidates all features from parallel development tracks into a cohesive, production-ready system.

## Architecture Evolution

### Previous State: Fragmented Implementation
- Multiple configuration files with conflicting settings
- Duplicate functionality in receivers and processors
- Inconsistent attribute naming between components
- Deployment configurations with different approaches

### Current State: Unified Architecture
- Single unified configuration with all features enabled
- Coordinated receiver-processor integration
- Standardized attribute mapping across components
- Consistent deployment patterns for all environments

## Key Components

### 1. Unified Collector Configuration (`collector.yaml`)

The unified configuration incorporates:

#### **All Receivers**
- PostgreSQL query receiver with enhanced metrics
- MySQL query receiver (optional)
- MongoDB receiver support (optional)
- File log receiver for database logs
- Prometheus receiver for internal metrics

#### **Complete Processor Pipeline**
```yaml
processors:
  - memory_limiter         # Resource protection
  - transform/circuit_breaker    # Database health monitoring
  - transform/sanitize_pii       # Security compliance
  - transform/attribute_mapper   # Receiver-processor compatibility
  - adaptivesampler             # Intelligent sampling
  - planattributeextractor      # Query plan analysis
  - transform/enrich_metadata   # Context enrichment
  - resource                    # Resource attributes
  - filter                      # Data filtering
  - batch                       # Optimized batching
```

#### **Multiple Pipelines**
- `logs/database`: Main database query monitoring
- `logs/application`: Database log ingestion
- `metrics/database`: Metric generation from logs
- `metrics/internal`: Collector self-monitoring

### 2. Attribute Mapping System (`attribute-mapping.yaml`)

Ensures compatibility between all components:

#### **Standard Attributes**
```yaml
# Query identification
query_id: "query_id"
query_hash: "query.hash"
query_text: "db.statement"

# Performance metrics
mean_duration_ms: "query.mean_duration_ms"
total_duration_ms: "query.total_duration_ms"
execution_count: "query.execution_count"
```

#### **Automatic Mapping**
- PostgreSQL: `postgresql.query.mean_time` → `query.mean_duration_ms`
- MySQL: `mysql.query.mean_time` → `query.mean_duration_ms`
- Processor compatibility: `query.mean_duration_ms` → `avg_duration_ms`

### 3. Enhanced Receiver Implementation

#### **Unified Adaptive Sampling**
```go
type UnifiedAdaptiveSamplerConfig struct {
    CoordinationMode string  // "receiver", "processor", "hybrid"
    PerformanceRules []PerformanceRule
    ResourceRules    []ResourceRule
    DatabaseOverrides map[string]DatabaseSamplingConfig
}
```

#### **Attribute Mapper Integration**
- Automatic attribute translation for processor compatibility
- Validation of required processor attributes
- Bidirectional mapping support

### 4. Unified Deployment Configurations

#### **Docker Compose**
- Primary and secondary instances for local HA
- Consistent environment variables
- Proper volume mounting for file storage
- Load balancer for high availability

#### **Kubernetes**
- StatefulSet with 3 replicas for production HA
- Horizontal Pod Autoscaler (3-10 replicas)
- Pod Disruption Budget for availability
- Network policies for security
- Persistent volume claims for state

## Configuration Management

### Environment Variables

All configurations support environment variable substitution:

```yaml
# Collection Settings
COLLECTION_INTERVAL_SECONDS: 60
QUERY_TIMEOUT_MS: 3000
MIN_QUERY_TIME_MS: 10
MAX_QUERIES_PER_COLLECTION: 100

# Sampling Configuration
ENABLE_ADAPTIVE_SAMPLER: true
SAMPLING_PERCENTAGE: 10
SLOW_QUERY_THRESHOLD_MS: 1000
BASE_SAMPLING_RATE: 0.1
MAX_SAMPLING_RATE: 1.0

# Resource Limits
MEMORY_LIMIT_PERCENTAGE: 75
MEMORY_SPIKE_LIMIT_PERCENTAGE: 20
BALLAST_SIZE_MIB: 256
```

### Feature Flags

Enable/disable features without code changes:

```yaml
ENABLE_ADAPTIVE_SAMPLER: true
ENABLE_PLAN_EXTRACTOR: true
ENABLE_FILE_LOG_RECEIVER: false
ENABLE_PII_SANITIZATION: true
```

## Deployment Patterns

### Development Environment
```bash
# Use Docker Compose
cd deploy/unified
docker-compose up -d
```

### Production Environment
```bash
# Use Kubernetes
kubectl apply -f deploy/unified/k8s-deployment.yaml
```

### High Availability Setup

The unified implementation supports true HA through:

1. **StatefulSet Deployment**: Maintains pod identity
2. **Anti-affinity Rules**: Distributes pods across nodes
3. **File Storage Coordination**: Each instance has separate storage
4. **Load Balancing**: Distributes requests across instances
5. **Health Checks**: Automatic failover on unhealthy instances

## Migration from Previous Versions

### Step 1: Update Configuration
```bash
# Backup existing configuration
cp config/collector.yaml config/collector.yaml.backup

# Use unified configuration
cp config/collector-unified.yaml config/collector.yaml
```

### Step 2: Update Environment Variables
```bash
# Add new required variables
export COLLECTION_INTERVAL_SECONDS=60
export QUERY_TIMEOUT_MS=3000
export ENABLE_ADAPTIVE_SAMPLER=true
```

### Step 3: Deploy Updated Version
```bash
# For Docker
docker-compose down
docker-compose -f deploy/unified/docker-compose.yaml up -d

# For Kubernetes
kubectl apply -f deploy/unified/k8s-deployment.yaml
```

## Monitoring and Observability

### Health Endpoints
- Health Check: `http://localhost:13133/health`
- Metrics: `http://localhost:8888/metrics`
- Prometheus: `http://localhost:8889/metrics`
- zPages: `http://localhost:55679/debug/tracez`
- pprof: `http://localhost:1777/debug/pprof/`

### Key Metrics to Monitor
```
# Query Performance
otelcol_postgresqlquery_queries_processed_total
otelcol_postgresqlquery_mean_query_duration_ms
otelcol_postgresqlquery_slow_queries_total

# Sampling Efficiency
otelcol_adaptivesampler_sampled_total
otelcol_adaptivesampler_dropped_total
otelcol_adaptivesampler_sampling_rate

# Resource Usage
otelcol_processor_memory_limiter_memory_used_bytes
otelcol_processor_memory_limiter_memory_limit_bytes

# Pipeline Health
otelcol_exporter_sent_logs_total
otelcol_exporter_send_failed_logs_total
```

## Performance Optimization

### Sampling Strategy
1. **Base Rate**: 10% for normal queries
2. **Slow Queries**: 50% for queries > 1s
3. **Critical Queries**: 100% for queries > 5s
4. **Resource-Heavy**: 100% for high temp space usage

### Resource Management
1. **Memory Ballast**: 256MB prevents GC thrashing
2. **Memory Limit**: 75% of available memory
3. **Spike Limit**: 20% buffer for traffic spikes
4. **Batch Size**: 1000 records for optimal throughput

## Security Considerations

### PII Sanitization
- Email addresses replaced with `***@***.***`
- SQL string literals replaced with `'***'`
- SSNs replaced with `***-**-****`
- Credit cards replaced with `****-****-****-****`
- Phone numbers replaced with `***-***-****`

### Network Security
- TLS enabled for all external connections
- Network policies restrict egress to databases
- No access to cloud metadata services
- Service accounts with minimal permissions

## Troubleshooting

### Common Issues

1. **Attribute Mapping Errors**
   - Check `transform/attribute_mapper` processor logs
   - Verify attribute names in `attribute-mapping.yaml`
   - Enable debug logging: `LOG_LEVEL=debug`

2. **High Memory Usage**
   - Adjust `MEMORY_LIMIT_PERCENTAGE`
   - Increase `BALLAST_SIZE_MIB`
   - Review sampling rates

3. **Missing Query Plans**
   - Verify `ENABLE_PLAN_EXTRACTOR=true`
   - Check PostgreSQL permissions for EXPLAIN
   - Review `plan_json` attribute in logs

## Future Enhancements

### Planned Features
1. **Multi-cluster Federation**: Cross-region monitoring
2. **ML-based Anomaly Detection**: Automatic query pattern analysis
3. **Custom Dashboards**: Database-specific visualizations
4. **Alert Rule Templates**: Pre-configured alerting

### Extension Points
1. **Custom Processors**: Add domain-specific logic
2. **Additional Receivers**: Support for more databases
3. **Export Formats**: Multiple backend support
4. **API Integration**: REST API for configuration

## Conclusion

The unified implementation consolidates all features into a production-ready system that:
- Eliminates configuration drift between environments
- Ensures consistent attribute naming across components
- Provides true high availability capabilities
- Maintains all advanced features from parallel development
- Simplifies deployment and operations

This unified approach provides a solid foundation for continued enhancement while maintaining stability and performance.