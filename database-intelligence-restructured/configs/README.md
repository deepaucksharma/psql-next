# OpenTelemetry Collector Configuration Structure

This directory contains a consolidated and modular configuration structure for the Database Intelligence OpenTelemetry Collector. The configuration follows OpenTelemetry best practices with environment-specific overlays and extensive use of environment variables for flexibility.

## Directory Structure

```
configs/new/
├── base.yaml                 # Consolidated base configuration
├── overlays/                 # Environment-specific overlays
│   ├── dev.yaml             # Development environment overrides
│   ├── staging.yaml         # Staging environment overrides
│   └── prod.yaml            # Production environment overrides
├── .env.template            # Template for environment variables
└── README.md                # This file
```

## Configuration Architecture

### Base Configuration (`base.yaml`)

The base configuration contains:
- **Receivers**: PostgreSQL, MySQL, SQL Query, OTLP, and Enhanced SQL receivers
- **Processors**: Memory Limiter, Batch, Resource, and Attributes processors
- **Exporters**: OTLP/HTTP, Prometheus, Logging, and Debug exporters
- **Extensions**: Health Check, pprof, zPages, Memory Ballast, and File Storage
- **Service**: Basic pipeline definitions and telemetry configuration

All values use environment variables with sensible defaults, making the configuration portable across environments.

### Environment Overlays

Each overlay file modifies the base configuration for specific environments:

#### Development (`overlays/dev.yaml`)
- Verbose logging and debug outputs
- Shorter collection intervals (10s) for faster feedback
- Higher memory limits for flexibility
- File exporters for local inspection
- All debugging extensions enabled

#### Staging (`overlays/staging.yaml`)
- Production-like settings with additional monitoring
- Moderate collection intervals (30s)
- Sampling enabled for cost control
- Enhanced processors (adaptive sampling, circuit breaker, verification)
- TLS configuration for secure connections

#### Production (`overlays/prod.yaml`)
- Conservative collection intervals (60s)
- Strict resource limits and cost controls
- Full security with TLS requirements
- Multiple redundant export paths
- All protection features enabled (circuit breaker, cost control, PII detection)
- Minimal logging for performance

## Usage

### 1. Set Up Environment Variables

Copy the environment template and configure your values:

```bash
cp .env.template .env
# Edit .env with your actual values
```

### 2. Run with Environment-Specific Configuration

Use the `--config` flag to combine base and overlay configurations:

```bash
# Development
otelcol --config=base.yaml --config=overlays/dev.yaml

# Staging
otelcol --config=base.yaml --config=overlays/staging.yaml

# Production
otelcol --config=base.yaml --config=overlays/prod.yaml
```

### 3. Docker Compose Example

```yaml
version: '3.8'
services:
  otel-collector:
    image: otel/opentelemetry-collector-contrib:latest
    command: ["--config=/etc/otel/base.yaml", "--config=/etc/otel/overlays/${ENVIRONMENT}.yaml"]
    environment:
      - ENVIRONMENT=${ENVIRONMENT:-dev}
    env_file:
      - .env
    volumes:
      - ./base.yaml:/etc/otel/base.yaml
      - ./overlays:/etc/otel/overlays
    ports:
      - "4317:4317"   # OTLP gRPC
      - "4318:4318"   # OTLP HTTP
      - "8888:8888"   # Metrics
      - "8889:8889"   # Prometheus
      - "13133:13133" # Health check
```

## Key Features

### 1. Database Monitoring
- Native receivers for PostgreSQL and MySQL
- Custom SQL queries for advanced metrics
- Extension detection and feature discovery
- Connection pooling and performance optimization

### 2. Data Processing
- **Memory Protection**: Prevents OOM with configurable limits
- **Batching**: Optimizes network usage and compression
- **Sampling**: Intelligent sampling rules for cost control
- **Circuit Breaking**: Protects databases from overload
- **Cost Control**: Budget enforcement and monitoring
- **PII Detection**: Automatic redaction of sensitive data

### 3. Observability Exports
- **New Relic**: Primary destination via OTLP
- **Prometheus**: Local metrics scraping
- **File**: Data archival (dev only)
- **Logging**: Debugging and troubleshooting

### 4. Operational Features
- Health check endpoints
- Performance profiling (pprof)
- Live debugging (zPages)
- Persistent queue state
- Graceful degradation

## Environment Variables

The configuration uses environment variables extensively. Key categories include:

### Database Connections
- `POSTGRES_HOST`, `POSTGRES_PORT`, `POSTGRES_USER`, `POSTGRES_PASSWORD`
- `MYSQL_HOST`, `MYSQL_PORT`, `MYSQL_USER`, `MYSQL_PASSWORD`

### Collection Intervals
- `POSTGRES_COLLECTION_INTERVAL` (default: 60s)
- `MYSQL_COLLECTION_INTERVAL` (default: 60s)
- `POSTGRES_QUERY_INTERVAL` (default: 300s)

### Resource Limits
- `MEMORY_LIMIT_MIB` (default: 512)
- `MEMORY_SPIKE_LIMIT_MIB` (default: 128)
- `BATCH_SIZE` (default: 1024)

### Export Configuration
- `NEW_RELIC_LICENSE_KEY` (required for New Relic export)
- `OTLP_ENDPOINT` (default: https://otlp.nr-data.net)
- `PROMETHEUS_ENDPOINT` (default: 0.0.0.0:8889)

See `.env.template` for a complete list of available environment variables.

## Migration from Legacy Configuration

To migrate from the previous configuration structure:

1. **Identify Custom Settings**: Review your existing configuration for custom values
2. **Update Environment Variables**: Add custom values to your `.env` file
3. **Choose Overlay**: Select the appropriate environment overlay
4. **Test Configuration**: Validate with `otelcol validate --config=base.yaml --config=overlays/[env].yaml`
5. **Deploy**: Update your deployment scripts to use the new structure

## Best Practices

1. **Environment Variables**: Always use environment variables for sensitive data
2. **Overlay Selection**: Choose the appropriate overlay for your environment
3. **Resource Limits**: Set appropriate limits based on your infrastructure
4. **Monitoring**: Use health check and metrics endpoints
5. **Security**: Enable TLS in staging and production
6. **Cost Control**: Configure sampling and budgets appropriately

## Troubleshooting

### Common Issues

1. **Missing Environment Variables**
   - Check `.env` file exists and is properly formatted
   - Verify all required variables are set

2. **Connection Failures**
   - Verify database credentials and network connectivity
   - Check TLS configuration matches your database setup

3. **High Memory Usage**
   - Reduce batch sizes
   - Increase sampling rates
   - Lower collection intervals

4. **Export Failures**
   - Verify API keys and endpoints
   - Check network connectivity
   - Review retry configuration

### Debug Mode

Enable debug mode by using the development overlay or setting:
```bash
export LOG_LEVEL=debug
export DEBUG_VERBOSITY=detailed
```

## Support

For issues or questions:
1. Check collector logs for errors
2. Use health check endpoint: `http://localhost:13133/`
3. Review metrics at: `http://localhost:8888/metrics`
4. Access zPages for live debugging: `http://localhost:55679/`