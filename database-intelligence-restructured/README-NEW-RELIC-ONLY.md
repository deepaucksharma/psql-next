# Database Intelligence - New Relic Only Configuration

This guide shows how to run the Database Intelligence system with **New Relic as the sole destination** for all telemetry data.

## 🎯 New Relic Only Architecture

```
┌─────────────────┬─────────────────┬─────────────────┐
│   DATABASES     │    COLLECTOR    │   NEW RELIC     │
├─────────────────┼─────────────────┼─────────────────┤
│ PostgreSQL:5432 │ OTLP:4317/4318  │ Metrics ✓       │
│ MySQL:3306      │ Health:13133    │ Traces ✓        │
│ Test Data ✓     │ Internal:8888   │ Logs ✓          │
│ PII Data ✓      │ ZPages:55679    │ Events ✓        │
└─────────────────┴─────────────────┴─────────────────┘

┌─────────────────────────────────────────────────────────┐
│              ALL PROCESSORS → NEW RELIC                │
├─────────────────┬─────────────────┬─────────────────────┤
│ AdaptiveSampler │ CircuitBreaker  │ CostControl         │
│ PlanExtractor   │ Verification    │ NRErrorMonitor     │
│ QueryCorrelator │ Resource        │ Transform           │
└─────────────────┴─────────────────┴─────────────────────┘
```

## 🚀 Quick Start

### 1. Setup Environment
```bash
# Initialize project
make -f Makefile.unified setup

# Edit .env file with your New Relic credentials
vi .env
```

### 2. Configure New Relic
Update the `.env` file with your New Relic details:

```bash
# New Relic Configuration (REQUIRED)
NEW_RELIC_API_KEY=your_actual_license_key_here
NEW_RELIC_ACCOUNT_ID=your_account_id_here

# Environment identifier
ENVIRONMENT=production
```

### 3. Run Everything
```bash
# Start complete system
make -f Makefile.unified docker-run

# Generate test load
make -f Makefile.unified test-load
```

## 📊 What Gets Sent to New Relic

### Metrics
- **Database Metrics**: PostgreSQL and MySQL performance data
- **Custom Metrics**: All 7 processor outputs
- **System Metrics**: Collector health and performance
- **Business Metrics**: Query patterns, execution plans, costs

### Traces
- **Query Traces**: Database query execution paths
- **Transaction Traces**: Multi-query transaction flows
- **Performance Traces**: Slow query analysis

### Logs
- **Query Logs**: Executed SQL statements (anonymized)
- **Error Logs**: Database and processor errors
- **Audit Logs**: PII detection and redaction events

### Events
- **Circuit Breaker Events**: When protection triggers
- **Cost Control Events**: Budget threshold alerts
- **PII Detection Events**: When sensitive data is found

## 🔧 Configuration Highlights

The system sends **everything** to New Relic:

```yaml
exporters:
  # Primary destination - New Relic
  otlp/newrelic:
    endpoint: https://otlp.nr-data.net:4317
    headers:
      api-key: ${NEW_RELIC_API_KEY}
    
  # Debug output for troubleshooting
  debug:
    verbosity: detailed
    
  # File backup for critical data
  file:
    path: ./telemetry-output.json

pipelines:
  metrics:
    exporters: [debug, otlp/newrelic, file]
  traces:
    exporters: [debug, otlp/newrelic]
  logs:
    exporters: [debug, otlp/newrelic]
```

## 📈 New Relic Dashboard Setup

### 1. Database Performance Dashboard
```sql
-- Query performance over time
SELECT average(duration) FROM Span 
WHERE service.name = 'database-intelligence-collector' 
AND span.kind = 'server' 
TIMESERIES AUTO

-- Top slow queries
SELECT query.text, average(duration) 
FROM Span 
WHERE db.operation IS NOT NULL 
GROUP BY query.text 
ORDER BY average(duration) DESC 
LIMIT 10
```

### 2. Processor Metrics Dashboard
```sql
-- Adaptive sampler efficiency
SELECT rate(sum(adaptivesampler.samples_dropped), 1 minute) 
FROM Metric 
TIMESERIES AUTO

-- Circuit breaker activations
SELECT count(*) FROM Metric 
WHERE metricName = 'circuitbreaker.state_changes' 
AND circuitbreaker.state = 'open' 
TIMESERIES AUTO

-- Cost control spending
SELECT latest(costcontrol.daily_spend_usd) 
FROM Metric 
TIMESERIES AUTO
```

### 3. Security & Compliance Dashboard
```sql
-- PII detection events
SELECT count(*) FROM Log 
WHERE message LIKE '%PII_DETECTED%' 
FACET pii.type 
TIMESERIES AUTO

-- Data quality issues
SELECT count(*) FROM Log 
WHERE log.level = 'ERROR' 
AND service.name = 'database-intelligence-collector' 
FACET error.type
```

## 🛡️ Security Features

### PII Protection
All sensitive data is detected and redacted:
- **Email addresses** → `REDACTED_EMAIL`
- **Phone numbers** → `REDACTED_PHONE`
- **SSN** → `REDACTED_SSN`
- **Credit cards** → `REDACTED_CC`

### Query Anonymization
SQL queries are anonymized before export:
- Parameter values replaced with placeholders
- PII in query text redacted
- Query structure preserved for analysis

## 🔍 Monitoring & Troubleshooting

### Health Checks
```bash
# Check collector health
curl http://localhost:13133/health

# View internal metrics
curl http://localhost:8888/metrics

# Check ZPages for OTEL internals
open http://localhost:55679
```

### Verify Data Flow
```bash
# Check if data is being sent to New Relic
make -f Makefile.unified verify-data

# View recent logs
make -f Makefile.unified logs
```

### Common Issues

1. **No data in New Relic**
   - Verify `NEW_RELIC_API_KEY` is correct
   - Check collector logs for authentication errors
   - Ensure network connectivity to `otlp.nr-data.net:4317`

2. **High data volume costs**
   - Cost Control processor automatically manages this
   - Check `costcontrol.daily_spend_usd` metric
   - Adjust budget in configuration

3. **Missing database metrics**
   - Verify database connectivity
   - Check credentials in `.env` file
   - Review PostgreSQL/MySQL receiver logs

## 🧪 Testing

### Comprehensive Test Suite
```bash
# Run all tests (validates New Relic export)
./tools/scripts/test/run-comprehensive-tests.sh

# Specific E2E tests
make -f Makefile.unified test-e2e

# Load testing
make -f Makefile.unified test-load
```

### Validate in New Relic
After running tests, check New Relic for:
- Database metrics arriving
- Processor metrics active
- PII redaction working
- Query plans extracted
- Cost tracking functional

## 📋 Production Checklist

- [ ] New Relic API key configured
- [ ] Database credentials set
- [ ] PII detection enabled
- [ ] Cost control budget set
- [ ] Circuit breaker thresholds configured
- [ ] Query anonymization active
- [ ] All processors enabled
- [ ] Health monitoring setup
- [ ] Alerting configured in New Relic
- [ ] Data retention policies set

## 🎯 Benefits

✅ **Unified Observability**: All telemetry in one place  
✅ **Cost Optimization**: Automated cost control and budgeting  
✅ **Security Compliance**: PII detection and data anonymization  
✅ **Performance Intelligence**: Query plan analysis and optimization  
✅ **Fault Tolerance**: Circuit breaker protection  
✅ **Real-time Monitoring**: Instant visibility into database performance  
✅ **Automated Insights**: AI-powered anomaly detection via New Relic  

This configuration provides a complete, production-ready database intelligence solution that sends all telemetry directly to New Relic for unified observability and analysis.