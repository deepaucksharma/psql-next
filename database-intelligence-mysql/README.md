# MySQL Intelligence Monitoring with OpenTelemetry

A comprehensive MySQL monitoring solution featuring intelligent wait analysis, ML-based anomaly detection, and business impact tracking. Built on OpenTelemetry standards with deep New Relic integration.

## 🚀 Features

- **40+ MySQL Metrics**: Complete coverage including Performance Schema integration
- **Intelligence Engine**: Advanced SQL analysis with wait profiling and pattern detection
- **Anomaly Detection**: ML-based detection with severity scoring and alerting
- **Business Impact**: Revenue impact estimation, SLA tracking, and cost analysis
- **Advisory System**: Actionable performance recommendations with priority scoring
- **Wait Analysis**: Detailed query wait profiling with categorization
- **Multiple Deployment Modes**: From minimal to comprehensive monitoring
- **Production Ready**: Health checks, resource limits, and error handling

## Quick Start

### Prerequisites
- Docker and Docker Compose V2
- New Relic account with License Key
- 4GB RAM minimum

### Deploy in 30 Seconds
```bash
# 1. Clone repository
git clone <repository-url>
cd database-intelligence-mysql

# 2. Configure credentials
cp .env.example .env
# Edit .env with your New Relic License Key

# 3. Deploy
./deploy/deploy.sh --with-workload
```

✅ **That's it!** Metrics will start flowing to New Relic immediately.

### Verify Deployment
```bash
./operate/diagnose.sh           # Check system health
./operate/test-connection.sh    # Test MySQL connections
./operate/validate-metrics.sh   # Validate metric flow
```

📚 **For detailed setup**: See [Getting Started](docs/getting-started.md)

## 🏗️ Architecture

```
┌─────────────────┐     ┌─────────────────────────┐     ┌─────────────────┐
│  MySQL Primary  │────▶│    OTel Collector       │────▶│   New Relic     │
└─────────────────┘     │                         │     └─────────────────┘
                        │ • MySQL Receiver        │
┌─────────────────┐     │ • SQL Intelligence      │     ┌─────────────────┐
│  MySQL Replica  │────▶│ • ML Processing         │────▶│   Prometheus    │
└─────────────────┘     │ • Anomaly Detection     │     └─────────────────┘
                        │ • Business Context      │
                        └─────────────────────────┘
```

## Metrics Collected

**40+ MySQL metrics** across 5 categories:

- **Connections**: threads, errors, max connections
- **Query Performance**: query counts, slow queries, execution times
- **InnoDB**: buffer pool, row operations, lock waits
- **Replication**: lag time, SQL delays
- **Table/Index I/O**: wait times and counts

📊 **Full metrics list**: See [Configuration Guide](docs/configuration.md)

## Configuration Options

### Deployment Modes
The master configuration supports 4 deployment modes:

- **Minimal**: Basic monitoring for development
- **Standard**: Production-ready with replication monitoring  
- **Advanced**: Deep insights with SQL intelligence queries
- **Debug**: Troubleshooting with verbose logging

🔧 **Configuration details**: See [Configuration Guide](docs/configuration.md#deployment-modes-explained)

### Environment Variables

| Variable | Description | Example |
|----------|-------------|---------|
| NEW_RELIC_LICENSE_KEY | New Relic License Key | `eu01xx...NRAL` |
| MYSQL_PRIMARY_ENDPOINT | Primary MySQL | `mysql-primary:3306` |
| DEPLOYMENT_MODE | Deployment mode | `minimal`, `standard`, `advanced` |

📋 **All variables**: See [Configuration Guide](docs/configuration.md#environment-variables)

## New Relic Integration

### Viewing Metrics

1. Log into your New Relic account
2. Navigate to **Explorer** → **Metrics**
3. Filter by `instrumentation.provider = 'opentelemetry'`
4. Look for metrics prefixed with `mysql.`

### Creating Dashboards

Import the provided dashboard:
1. Go to **Dashboards** → **Import dashboard**
2. Upload `config/newrelic/dashboards.json`

Key dashboards included:
- **Overview**: Health score, connections, query rate
- **Performance Analysis**: Wait profiles, buffer pool, locks
- **Intelligence & Advisory**: Anomalies, recommendations
- **Replication**: Lag monitoring and status

### Sample Queries

```sql
-- Query Performance
SELECT rate(sum(mysql.query.count), 1 minute) as 'QPS' 
FROM Metric 
WHERE instrumentation.provider = 'opentelemetry' 
TIMESERIES

-- Wait Analysis
SELECT sum(value) as 'Wait Time' 
FROM Metric 
WHERE metricName = 'mysql.query.wait_profile' 
FACET attributes['wait.category']

-- Anomalies
SELECT * FROM Metric 
WHERE attributes['anomaly.detected'] = true 
SINCE 30 minutes ago
```

## Troubleshooting

### No metrics in New Relic

1. Check collector logs:
```bash
docker compose logs -f otel-collector
```

2. Verify API key:
```bash
docker compose exec otel-collector env | grep NEW_RELIC
```

3. Test collector health:
```bash
curl http://localhost:13133/
```

### MySQL connection errors

1. Check MySQL is running:
```bash
docker compose ps
```

2. Verify monitoring user:
```bash
docker compose exec mysql-primary mysql -uotel_monitor -potelmonitorpass -e "SHOW GRANTS;"
```

### High memory usage

Adjust collector limits in `docker-compose.yml`:
```yaml
environment:
  GOMEMLIMIT: "1750MiB"
```

## 🔬 Advanced Features

### Intelligence Engine
When `DEPLOYMENT_MODE=advanced`, get:
- **Wait Profile Analysis**: Categorized query wait times
- **Anomaly Detection**: ML-based detection with severity scoring
- **Performance Advisory**: Actionable recommendations
- **Business Impact**: Revenue and SLA impact tracking
- **Historical Patterns**: Trend analysis and predictions

### Multi-Environment Support
```bash
# Production deployment
export DEPLOYMENT_MODE="standard"
export ENVIRONMENT="production"

# Development deployment  
export DEPLOYMENT_MODE="minimal"
export ENVIRONMENT="development"
```

## Performance & Security

**Performance**:
- Collector uses ~200MB RAM (adjustable via `MEMORY_LIMIT_PERCENT`)
- MySQL Performance Schema adds 5-10% overhead
- Batch sizes adjustable via `BATCH_SIZE` environment variable

**Security**:
- All credentials in environment variables
- MySQL user has minimal required privileges
- TLS supported for MySQL connections

## Support & Contributing

**Get Help**:
- 📚 [Getting Started](docs/getting-started.md)
- 🔧 [Configuration Guide](docs/configuration.md)
- 📝 [Operations Guide](docs/operations.md)
- 🤔 [Troubleshooting](docs/troubleshooting.md)
- 🌐 [New Relic Docs](https://docs.newrelic.com/)

**Contributing**:
1. Fork the repository
2. Create a feature branch
3. Run tests: `./operate/full-test.sh`
4. Submit a pull request

## License

MIT License - see LICENSE file for details