# Configuration Reference

## Environment Variables

### Required
- `NEW_RELIC_API_KEY` - Your New Relic license key
- `NEW_RELIC_ACCOUNT_ID` - Your New Relic account ID

### MySQL Connection
- `MYSQL_PRIMARY_ENDPOINT` - Primary MySQL endpoint (default: localhost:3306)
- `MYSQL_REPLICA_ENDPOINT` - Replica MySQL endpoint (default: localhost:3307)
- `MYSQL_USER` - MySQL monitoring user (default: otel_monitor)
- `MYSQL_PASSWORD` - MySQL monitoring password

### Deployment Configuration
- `DEPLOYMENT_MODE` - Controls feature set
  - `minimal` - Basic MySQL metrics only
  - `standard` - Production recommended (balanced)
  - `advanced` - All features enabled (default)
  - `debug` - Debug mode with verbose output

### Feature Flags
- `ENABLE_SQL_INTELLIGENCE` - Enable comprehensive SQL analysis (default: true)
- `WAIT_PROFILE_ENABLED` - Enable wait profiling (default: true)
- `ML_FEATURES_ENABLED` - Enable ML feature generation (default: true)
- `BUSINESS_CONTEXT_ENABLED` - Enable business impact tracking (default: true)
- `ANOMALY_DETECTION_ENABLED` - Enable anomaly detection (default: true)
- `ADVISOR_ENGINE_ENABLED` - Enable performance advisory (default: true)

### Performance Tuning
- `MYSQL_COLLECTION_INTERVAL` - MySQL metrics interval (default: 5s)
- `SQL_INTELLIGENCE_INTERVAL` - Intelligence query interval (default: 5s)
- `BATCH_SIZE` - Metric batch size (default: 1000)
- `MEMORY_LIMIT_PERCENT` - Memory limit percentage (default: 80)
- `SCRAPE_INTERVAL` - Host metrics scrape interval (default: 10s)

### Resource Limits
- `GOMAXPROCS` - CPU cores for collector (default: 2)
- `GOMEMLIMIT` - Memory limit for collector (default: 1750MiB)

## Configuration Files

### Master Collector Configuration
**Location**: `config/collector/master.yaml`

Key sections:
- **Receivers**: MySQL, host metrics, SQL intelligence queries
- **Processors**: Batching, ML features, anomaly detection, business context
- **Exporters**: New Relic OTLP, Prometheus, debug
- **Pipelines**: critical_realtime, standard, wait_profile

### MySQL Configuration
**Location**: `config/mysql/`
- `primary.cnf` - Primary MySQL server configuration
- `replica.cnf` - Replica MySQL server configuration

Key settings:
- Performance Schema enabled
- Binary logging for replication
- InnoDB optimizations

## Deployment Modes Explained

### Minimal Mode
- Basic MySQL metrics
- Host metrics
- Simple pipeline
- Low resource usage
- Good for development

### Standard Mode
- All minimal features plus:
- Replica monitoring
- Optimized batching
- Resource attributes
- Good for production

### Advanced Mode (Default)
- All standard features plus:
- SQL intelligence queries
- Wait profile analysis
- ML feature generation
- Anomaly detection
- Business impact scoring
- Advisory engine
- Best for comprehensive monitoring

### Debug Mode
- All advanced features plus:
- Debug logging
- File output
- Performance profiling
- zpages enabled
- Good for troubleshooting

## Custom Configuration

To customize the collector configuration:

1. Copy the master config:
   ```bash
   cp config/collector/master.yaml config/collector/custom.yaml
   ```

2. Edit your custom config

3. Update docker-compose.yml or deployment script to use custom config

## Security Considerations

1. **Credentials**: Never commit credentials to version control
2. **Network**: Use TLS for production deployments
3. **Permissions**: MySQL user needs specific grants:
   - PROCESS
   - REPLICATION CLIENT
   - SELECT on performance_schema
   - SELECT on mysql
   - SELECT on sys