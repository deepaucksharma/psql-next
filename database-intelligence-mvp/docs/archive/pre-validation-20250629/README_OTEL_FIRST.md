# Database Intelligence Collector - OTEL-First Implementation

A streamlined OpenTelemetry Collector configuration for database monitoring that maximizes standard OTEL components and adds custom processors only where needed.

## 🎯 Core Philosophy

**OTEL-First with Custom Components for Gaps**

- Use standard OpenTelemetry receivers, processors, and exporters wherever possible
- Build custom processors only for functionality OTEL doesn't provide
- Maintain clean separation between standard and custom components
- Focus on simplicity and maintainability

## 🏗️ Architecture Overview

```
┌─────────────────────┐     ┌─────────────────────┐
│   PostgreSQL DB     │────▶│ postgresql receiver │
└─────────────────────┘     └──────────┬──────────┘
                                       │
┌─────────────────────┐     ┌──────────▼──────────┐
│   Custom Queries    │────▶│ sqlquery receiver   │
└─────────────────────┘     └──────────┬──────────┘
                                       │
                            ┌──────────▼──────────┐
                            │ Standard Processors │
                            │ - memory_limiter    │
                            │ - batch             │
                            │ - resource          │
                            │ - transform         │
                            └──────────┬──────────┘
                                       │
                            ┌──────────▼──────────┐
                            │ Custom Processors   │
                            │ - adaptive_sampler  │
                            │ - circuit_breaker   │
                            └──────────┬──────────┘
                                       │
                            ┌──────────▼──────────┐
                            │   OTLP Exporter     │
                            │   (New Relic)       │
                            └─────────────────────┘
```

## 🚀 Quick Start

### Prerequisites

- Go 1.21+
- Docker and Docker Compose
- New Relic License Key

### 1. Clone and Setup

```bash
git clone https://github.com/database-intelligence-mvp
cd database-intelligence-mvp

# Copy environment template
cp env.example .env

# Edit .env and add your New Relic license key
```

### 2. Build the Collector

```bash
# Install tools
make install-tools

# Build the collector
make build
```

### 3. Run with Docker Compose

```bash
# Start all services
make docker-up

# Check logs
docker logs db-intelligence-collector

# Stop services
make docker-down
```

### 4. Run Standalone

```bash
# Set environment variables
export NEW_RELIC_LICENSE_KEY=your-key-here
export POSTGRES_HOST=localhost
export POSTGRES_PORT=5432

# Run collector
make run
```

## 📦 Components

### Standard OTEL Components Used

#### Receivers
- **postgresql**: Collects PostgreSQL server metrics
- **sqlquery**: Executes custom SQL queries for additional metrics

#### Processors
- **memory_limiter**: Prevents OOM conditions
- **batch**: Batches telemetry data for efficiency
- **resource**: Adds resource attributes
- **attributes**: Manages attribute operations
- **transform**: Data transformation and PII sanitization

#### Exporters
- **otlp**: Sends data to New Relic or any OTLP endpoint
- **prometheus**: Exposes metrics for Prometheus scraping
- **debug**: Development debugging

### Custom Processors (Gap Fillers)

#### 1. Adaptive Sampler (`adaptive_sampler`)
**Gap**: OTEL's probabilistic sampler can't adapt based on query performance

**Features**:
- Samples slow queries (>1s) at 100%
- Samples normal queries at configurable rate
- Reduces data volume while preserving important signals

**Configuration**:
```yaml
adaptive_sampler:
  rules:
    - name: "slow_queries"
      condition: "mean_exec_time > 1000"
      sampling_rate: 100
  default_sampling_rate: 10
```

#### 2. Circuit Breaker (`circuit_breaker`)
**Gap**: OTEL doesn't provide database-aware circuit breaking

**Features**:
- Protects database from monitoring overload
- Opens circuit when error rate exceeds threshold
- Automatic recovery with half-open state

**Configuration**:
```yaml
circuit_breaker:
  error_threshold_percent: 50
  volume_threshold_qps: 1000
  break_duration: 5m
```

## 📝 Configuration

### Minimal Production Config

```yaml
# config/collector-simplified.yaml
receivers:
  postgresql:
    endpoint: ${env:POSTGRES_HOST}:5432
    username: ${env:POSTGRES_USER}
    password: ${env:POSTGRES_PASSWORD}

processors:
  memory_limiter:
    limit_mib: 512
  batch:
    timeout: 10s
  adaptive_sampler:
    default_sampling_rate: 10
  circuit_breaker:
    error_threshold_percent: 50

exporters:
  otlp:
    endpoint: otlp.nr-data.net:4317
    headers:
      api-key: ${env:NEW_RELIC_LICENSE_KEY}

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [memory_limiter, adaptive_sampler, circuit_breaker, batch]
      exporters: [otlp]
```

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `POSTGRES_HOST` | PostgreSQL hostname | localhost |
| `POSTGRES_PORT` | PostgreSQL port | 5432 |
| `POSTGRES_USER` | PostgreSQL username | postgres |
| `POSTGRES_PASSWORD` | PostgreSQL password | postgres |
| `POSTGRES_DB` | Database name | postgres |
| `NEW_RELIC_LICENSE_KEY` | New Relic API key | (required) |
| `OTLP_ENDPOINT` | OTLP endpoint | otlp.nr-data.net:4317 |
| `ENVIRONMENT` | Deployment environment | production |
| `LOG_LEVEL` | Logging level | info |

## 🔧 Development

### Project Structure

```
database-intelligence-mvp/
├── main.go                    # Collector entry point
├── go.mod                     # Go module definition
├── Makefile                   # Build automation
├── ocb-config-simplified.yaml # OCB build config
├── config/
│   └── collector-simplified.yaml  # Main configuration
├── processors/
│   ├── adaptivesampler/      # Adaptive sampling processor
│   └── circuitbreaker/       # Circuit breaker processor
└── deploy/
    ├── docker-compose.yaml   # Local development stack
    └── Dockerfile            # Container build
```

### Building Custom Processors

Custom processors follow OTEL's processor interface:

```go
// processors/adaptivesampler/simple_processor.go
type simpleAdaptiveSampler struct {
    logger      *zap.Logger
    config      *Config
    nextMetrics consumer.Metrics
}

func (p *simpleAdaptiveSampler) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
    // Apply adaptive sampling logic
    // Forward to next consumer
    return p.nextMetrics.ConsumeMetrics(ctx, md)
}
```

### Running Tests

```bash
# Unit tests
make test

# Linting
make lint

# Format code
make fmt
```

## 📊 Metrics Collected

### Standard PostgreSQL Metrics
- Connection statistics
- Database size and growth
- Table and index statistics
- Replication lag
- Cache hit ratios

### Custom Query Metrics
- Query performance (calls, mean time)
- Active sessions by state
- Wait events
- Long-running queries

## 🚨 Monitoring & Alerting

### Health Check
- Endpoint: `http://localhost:13133/health`

### Prometheus Metrics
- Endpoint: `http://localhost:8889/metrics`
- Collector internal metrics
- Pipeline statistics

### Grafana Dashboards
- Access at: `http://localhost:3000`
- Default credentials: admin/admin

## 🔍 Troubleshooting

### Common Issues

1. **Collector won't start**
   - Check environment variables
   - Verify PostgreSQL connectivity
   - Review logs: `docker logs db-intelligence-collector`

2. **No data in New Relic**
   - Verify license key
   - Check OTLP endpoint connectivity
   - Enable debug exporter

3. **High memory usage**
   - Adjust `memory_limiter` settings
   - Reduce batch size
   - Increase sampling rate

### Debug Mode

```yaml
exporters:
  debug:
    verbosity: detailed

service:
  pipelines:
    metrics:
      exporters: [debug, otlp]
```

## 🤝 Contributing

1. Keep the OTEL-first philosophy
2. Only add custom components for clear gaps
3. Write tests for custom processors
4. Update documentation

## 📄 License

Apache License 2.0

## 🔗 Resources

- [OpenTelemetry Collector Docs](https://opentelemetry.io/docs/collector/)
- [OTEL Contrib Repository](https://github.com/open-telemetry/opentelemetry-collector-contrib)
- [New Relic OTLP Docs](https://docs.newrelic.com/docs/opentelemetry/)