# Docker Setup for Database Intelligence Collector

This directory contains Docker configuration for running the Database Intelligence Collector with all its components.

## Quick Start

```bash
# Start all services
./scripts/docker-start.sh

# Or manually with docker-compose
docker-compose up -d
```

## Architecture

The Docker setup includes:

1. **PostgreSQL Database** (port 5432)
   - Pre-configured with pg_stat_statements
   - Sample tables and data
   - Test query generation

2. **MySQL Database** (port 3306)
   - Performance schema enabled
   - Sample tables and data
   - Test stored procedures

3. **OpenTelemetry Collector** (ports 4317, 4318, 8888)
   - Custom Database Intelligence processors
   - Configured pipelines for logs and metrics
   - Health checks and monitoring

4. **Prometheus** (port 9090)
   - Scrapes collector metrics
   - Database performance metrics
   - Processor health metrics

5. **Grafana** (port 3000)
   - Pre-configured datasources
   - Default credentials: admin/admin

## Configuration

### Environment Variables

Create a `.env` file in the project root:

```env
# New Relic (optional)
NEW_RELIC_LICENSE_KEY=your_license_key

# Database credentials (defaults provided)
POSTGRES_USER=postgres
POSTGRES_PASSWORD=postgres
POSTGRES_DB=testdb

MYSQL_USER=mysql
MYSQL_PASSWORD=mysql
MYSQL_DB=testdb

# Collector settings
LOG_LEVEL=info
ENVIRONMENT=development
```

### Processor Configuration

The collector is configured with all custom processors:

**Logs Pipeline:**
- Circuit Breaker - Protects against overload
- Plan Attribute Extractor - Extracts query plan hashes
- Verification - PII detection and sanitization
- Adaptive Sampler - Intelligent sampling

**Metrics Pipeline:**
- Query Correlator - Correlates queries across metrics
- NR Error Monitor - Detects integration errors
- Cost Control - Budget enforcement

## Testing

### Send Test Data

```bash
# Send OTLP logs
curl -X POST http://localhost:4318/v1/logs \
  -H "Content-Type: application/json" \
  -d @tests/e2e/testdata/sample-logs.json

# Send OTLP metrics
curl -X POST http://localhost:4318/v1/metrics \
  -H "Content-Type: application/json" \
  -d @tests/e2e/testdata/sample-metrics.json
```

### Run Test Generator

```bash
# Generate continuous test data
docker-compose --profile test up test-generator
```

### View Logs

```bash
# Collector logs
docker-compose logs -f otel-collector

# All services
docker-compose logs -f
```

## Monitoring

### Health Checks

- Collector health: http://localhost:13133
- Collector metrics: http://localhost:8888/metrics
- ZPages debug: http://localhost:55679

### Grafana Dashboards

1. Navigate to http://localhost:3000
2. Login with admin/admin
3. Import dashboards from `dashboards/` directory

### Prometheus Queries

Access Prometheus at http://localhost:9090

Example queries:
```promql
# Database query duration
db_query_duration{db_system="postgresql"}

# Processor health
otelcol_processor_accepted_spans{processor="circuitbreaker"}

# Cost tracking
dbintel_cost_bytes_ingested_total
```

## Troubleshooting

### Container Issues

```bash
# Check container status
docker-compose ps

# View container logs
docker-compose logs <service_name>

# Restart a service
docker-compose restart <service_name>
```

### Database Connection

```bash
# Test PostgreSQL connection
docker exec -it dbintel-postgres psql -U postgres -d testdb

# Test MySQL connection
docker exec -it dbintel-mysql mysql -u mysql -pmysql testdb
```

### Collector Issues

```bash
# Check collector config
docker exec -it dbintel-collector cat /etc/otel/config.yaml

# Test collector locally
docker exec -it dbintel-collector /usr/local/bin/otelcol validate --config=/etc/otel/config.yaml
```

## Development

### Rebuild After Changes

```bash
# Rebuild collector image
docker-compose build otel-collector

# Restart with new image
docker-compose up -d otel-collector
```

### Volume Mounts

For development, you can mount local directories:

```yaml
volumes:
  - ./config:/etc/otel
  - ./processors:/build/processors
```

## Cleanup

```bash
# Stop all services
docker-compose down

# Remove volumes (data)
docker-compose down -v

# Remove images
docker-compose down --rmi all
```