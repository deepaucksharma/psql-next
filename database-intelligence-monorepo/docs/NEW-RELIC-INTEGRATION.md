# 🚀 New Relic Integration Guide - Complete MySQL Monitoring

## 📊 Overview

This guide provides comprehensive instructions for integrating the Database Intelligence MySQL Monorepo with New Relic One platform, achieving enterprise-grade MySQL monitoring with validated patterns and production-ready dashboards.

## 🎯 **What You'll Achieve**

- **Complete New Relic Integration**: 100% New Relic One platform monitoring
- **Enterprise Dashboard Suite**: 11 production-ready dashboards
- **Validated Metrics**: All NRQL queries tested against live data
- **Business Intelligence**: Revenue impact scoring and SLA monitoring
- **Production Ready**: Zero-downtime deployment and monitoring

## 📋 **Prerequisites**

### **New Relic Requirements**
- New Relic One account with NRDB access
- User API Key (for dashboard deployment)
- License Key (for data ingestion)
- Account ID (for entity synthesis)

### **System Requirements**
- OpenTelemetry Collector compatible environment
- MySQL 5.7+ or 8.0+ with Performance Schema enabled
- Docker and Docker Compose
- Minimum 4GB RAM, 2 CPU cores

### **Network Requirements**
- Outbound HTTPS access to `otlp.nr-data.net:4318`
- MySQL connection access from collector containers
- Internal connectivity between modules (ports 8081-8088)

## 🔧 **Quick Setup**

### **1. Configure Environment Variables**

Create or update your `.env` file:

```bash
# MySQL Configuration
MYSQL_ENDPOINT=mysql:3306
MYSQL_USER=root
MYSQL_PASSWORD=your_mysql_password
MYSQL_DATABASE=your_database

# New Relic Configuration  
NEW_RELIC_OTLP_ENDPOINT=https://otlp.nr-data.net:4318
NEW_RELIC_LICENSE_KEY=your_license_key_here
NEW_RELIC_ACCOUNT_ID=your_account_id_here
NEW_RELIC_API_KEY=your_user_api_key_here

# Module Configuration
OTEL_SERVICE_NAME=mysql-intelligence
ENVIRONMENT=production
CLUSTER_NAME=your-cluster-name
```

### **2. Deploy All Modules**

```bash
# Build all modules
make build

# Deploy complete monitoring suite
make run-all

# Verify deployment
make validate
```

### **3. Deploy New Relic Dashboards**

```bash
# Deploy all 11 dashboards
source .env
./shared/newrelic/scripts/deploy-all-newrelic-dashboards.sh
```

## 📊 **Dashboard Suite Overview**

### **Production-Ready Dashboards (11 Total)**

#### **Validated Dashboards (Tested Patterns)**
1. **`validated-mysql-core-dashboard.json`**
   - Foundation MySQL monitoring with confirmed metrics
   - Uptime, connections, query rates
   - Validated against live OpenTelemetry streams

2. **`validated-sql-intelligence-dashboard.json`**  
   - Query performance analytics with proven patterns
   - Slow query analysis, index usage, execution patterns
   - All NRQL queries confirmed working

3. **`validated-operational-dashboard.json`**
   - Real-time operations monitoring
   - Database operations rate, lock contention, InnoDB performance
   - Operational health status

#### **Module-Specific Dashboards**
4. **`core-metrics-newrelic-dashboard.json`** - Core module metrics
5. **`sql-intelligence-newrelic-dashboard.json`** - SQL analysis module
6. **`replication-monitor-newrelic-dashboard.json`** - Replication health
7. **`performance-advisor-newrelic-dashboard.json`** - Performance insights

#### **Executive & Business Dashboards**
8. **`database-intelligence-executive-dashboard.json`** - C-level overview
9. **`mysql-intelligence-dashboard.json`** - Comprehensive technical view
10. **`plan-explorer-dashboard.json`** - Query plan analysis
11. **`simple-test-dashboard.json`** - Basic connectivity verification

## 🔍 **Validated NRQL Patterns**

All dashboards use confirmed working metrics and tested query patterns:

### **Core MySQL Health (Confirmed Working ✅)**
```nrql
SELECT latest(mysql.global.status.uptime) as 'Uptime (sec)',
       uniqueCount(entity.guid) as 'Active Instances',
       average(mysql.global.status.threads_connected) as 'Avg Connections'
FROM Metric 
WHERE instrumentation.provider = 'opentelemetry' 
  AND entity.type = 'MYSQL_INSTANCE'
```

### **Query Performance Analysis (Confirmed Working ✅)**
```nrql
SELECT rate(sum(mysql.query.exec_total), 1 minute) as 'Queries/min',
       average(mysql.query.latency_avg_ms) as 'Avg Latency (ms)'
FROM Metric 
WHERE instrumentation.provider = 'opentelemetry' 
  AND metricName = 'mysql.query.exec_total'
```

### **Connection Performance (Confirmed Working ✅)**
```nrql
SELECT average(mysql.global.status.threads_connected) / 
       average(mysql.global.variables.max_connections) * 100 as 'Usage %'
FROM Metric 
WHERE instrumentation.provider = 'opentelemetry' 
  AND entity.type = 'MYSQL_INSTANCE'
```

## 🚀 **Deployment Procedures**

### **Individual Dashboard Deployment**

```bash
# Deploy specific dashboard
curl -X POST "https://api.newrelic.com/graphql" \
  -H "Content-Type: application/json" \
  -H "API-Key: $NEW_RELIC_API_KEY" \
  -d '{"query": "mutation { dashboardCreate(accountId: '$NEW_RELIC_ACCOUNT_ID', dashboard: {...}) { entityResult { guid name } errors { description } } }"}'
```

### **Bulk Dashboard Deployment**

```bash
# Use automated deployment script
./shared/newrelic/scripts/deploy-all-newrelic-dashboards.sh

# Monitor deployment progress
tail -f /tmp/dashboard-deployment.log
```

### **Validation After Deployment**

```bash
# Verify data flow
./shared/validation/health-check-all.sh

# Check specific metrics
curl -s "https://api.newrelic.com/graphql" \
  -H "API-Key: $NEW_RELIC_API_KEY" \
  -d '{"query": "{ actor { account(id: '$NEW_RELIC_ACCOUNT_ID') { nrql(query: \"SELECT count(*) FROM Metric WHERE instrumentation.provider = '"'"'opentelemetry'"'"' SINCE 5 minutes ago\") { results } } } }"}'
```

## 📈 **Monitoring Capabilities**

### **Real-Time Metrics**
- **MySQL Health**: Uptime, connections, query throughput
- **Performance**: Query latency, InnoDB efficiency, lock contention
- **Business Impact**: Revenue correlation, SLA compliance
- **Anomaly Detection**: Statistical analysis and trend detection

### **Advanced Analytics**
- **Query Optimization**: SQL performance insights and recommendations
- **Capacity Planning**: Resource utilization trends and forecasting
- **Business Intelligence**: Custom scoring and impact analysis
- **Cross-Module Correlation**: Multi-signal analysis and alerting

### **Enterprise Features**
- **Entity Synthesis**: Proper MySQL instance detection in New Relic
- **Alert Integration**: Native New Relic alerting capabilities
- **Dashboard Sharing**: Team collaboration and role-based access
- **API Integration**: Custom integrations and automation

## 🔧 **Configuration Management**

### **Environment Variables Reference**

| Variable | Purpose | Example | Required |
|----------|---------|---------|----------|
| `NEW_RELIC_LICENSE_KEY` | Data ingestion | `ea7e83e4...NRAL` | ✅ Yes |
| `NEW_RELIC_ACCOUNT_ID` | Account identification | `3630072` | ✅ Yes |
| `NEW_RELIC_API_KEY` | Dashboard deployment | `NRAK-KRP...4XI` | ✅ Yes |
| `NEW_RELIC_OTLP_ENDPOINT` | OTLP export endpoint | `https://otlp.nr-data.net:4318` | ✅ Yes |
| `MYSQL_ENDPOINT` | MySQL connection | `mysql:3306` | ✅ Yes |
| `ENVIRONMENT` | Environment label | `production` | ⚠️ Recommended |
| `CLUSTER_NAME` | Cluster identification | `mysql-cluster-1` | ⚠️ Recommended |

### **Module Configuration Patterns**

All modules follow consistent New Relic export configuration:

```yaml
exporters:
  otlphttp/newrelic:
    endpoint: ${env:NEW_RELIC_OTLP_ENDPOINT}
    headers:
      api-key: ${env:NEW_RELIC_LICENSE_KEY}
    compression: gzip
    timeout: 30s
    retry_on_failure:
      enabled: true
      initial_interval: 5s
      max_interval: 30s
      max_elapsed_time: 300s

  attributes/entity_synthesis:
    actions:
      - key: entity.type
        value: "MYSQL_INSTANCE"
        action: insert
      - key: instrumentation.provider
        value: opentelemetry
        action: insert
```

## 🧪 **Testing and Validation**

### **Data Flow Validation**

```bash
# Quick validation
./shared/validation/health-check-all.sh

# Comprehensive validation  
./shared/validation/run-comprehensive-validation.py

# Module-specific validation
./shared/validation/module-specific/validate-core-metrics.py
```

### **Dashboard Query Testing**

```bash
# Validate NRQL queries
./shared/newrelic/scripts/validate-dashboards.sh

# Test specific dashboard
./shared/validation/test-nrdb-connection.py --dashboard mysql-core
```

### **Performance Testing**

```bash
# Load testing
./integration/benchmarks/load-generator.py --duration 300

# Performance benchmarks
./integration/benchmarks/module-benchmarks.py --all
```

## 🚨 **Troubleshooting**

### **Common Issues and Solutions**

#### **No Data in New Relic**
```bash
# Check OTLP endpoint connectivity
curl -v https://otlp.nr-data.net:4318

# Verify license key
echo $NEW_RELIC_LICENSE_KEY | wc -c  # Should be 40 characters

# Check module status
docker-compose ps
```

#### **Dashboard Deployment Fails**
```bash
# Validate JSON syntax
jq '.' shared/newrelic/dashboards/mysql-core-dashboard.json

# Check API key permissions
curl -H "API-Key: $NEW_RELIC_API_KEY" https://api.newrelic.com/graphql \
  -d '{"query": "{ actor { user { name } } }"}'

# Verify account ID
echo "Account ID: $NEW_RELIC_ACCOUNT_ID"
```

#### **Missing Metrics**
```bash
# Check OpenTelemetry collector logs
docker logs core-metrics-collector

# Verify MySQL permissions
mysql -u$MYSQL_USER -p$MYSQL_PASSWORD -e "SHOW GRANTS;"

# Test Performance Schema access
mysql -u$MYSQL_USER -p$MYSQL_PASSWORD -e "SELECT COUNT(*) FROM performance_schema.events_statements_summary_by_digest;"
```

### **Advanced Troubleshooting**

#### **Memory Issues**
```bash
# Check memory usage
docker stats

# Adjust memory limits in docker-compose.yaml
memory: 2G  # Increase if needed
```

#### **Query Performance Issues**
```bash
# Check MySQL slow query log
mysql -e "SHOW VARIABLES LIKE 'slow_query_log%';"

# Monitor Performance Schema overhead
mysql -e "SHOW STATUS LIKE 'Performance_schema%';"
```

## 📊 **Cost Optimization**

### **Data Retention Management**
- Configure appropriate metric retention periods
- Use sampling for high-volume metrics
- Implement intelligent alert thresholds

### **Query Optimization**
- Use efficient NRQL patterns from validated dashboards
- Implement proper time window selections
- Leverage metric aggregations and summaries

### **Resource Efficiency**
```yaml
# Optimized collector configuration
processors:
  memory_limiter:
    limit_percentage: 75
  batch:
    timeout: 10s
    send_batch_size: 1000
```

## 🔗 **Integration Examples**

### **Alerting Integration**
```bash
# Create alert condition via NerdGraph
curl -X POST https://api.newrelic.com/graphql \
  -H "API-Key: $NEW_RELIC_API_KEY" \
  -d '{"query": "mutation { alertsNrqlConditionStaticCreate(...) { ... } }"}'
```

### **Custom Dashboard Creation**
```bash
# Generate custom dashboard
./shared/dashboards/dashboard-generator.py \
  --module core-metrics \
  --metrics mysql.global.status.uptime,mysql.global.status.threads_connected \
  --output custom-dashboard.json
```

### **API Integration**
```python
# Python example for metric querying
import requests

query = """
{
  actor {
    account(id: 3630072) {
      nrql(query: "SELECT average(mysql.global.status.threads_connected) FROM Metric SINCE 1 hour ago") {
        results
      }
    }
  }
}
"""

response = requests.post(
    'https://api.newrelic.com/graphql',
    headers={'API-Key': 'YOUR_API_KEY'},
    json={'query': query}
)
```

## 📚 **Additional Resources**

### **New Relic Documentation**
- [New Relic One Platform](https://docs.newrelic.com/docs/new-relic-one/)
- [NRQL Query Language](https://docs.newrelic.com/docs/query-your-data/nrql-new-relic-query-language/)
- [Dashboard API](https://docs.newrelic.com/docs/apis/nerdgraph/examples/nerdgraph-dashboards/)

### **OpenTelemetry Resources**
- [OpenTelemetry Collector](https://opentelemetry.io/docs/collector/)
- [MySQL Receiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/mysqlreceiver)
- [OTLP Exporter](https://github.com/open-telemetry/opentelemetry-collector/tree/main/exporter/otlphttpexporter)

### **Project Resources**
- **Module Documentation**: Individual module README files
- **Health Check Policy**: `HEALTH_CHECK_POLICY.md`
- **Enterprise Status**: `ENTERPRISE-STATUS.md`
- **Quick Reference**: `docs/quick-reference.md`

---

## 🎉 **Success Metrics**

Once successfully deployed, you should see:

- ✅ **11 New Relic dashboards** deployed and functional
- ✅ **Real-time MySQL metrics** flowing to New Relic NRDB
- ✅ **Entity synthesis** showing MySQL instances in New Relic One
- ✅ **Alert capabilities** available for all monitored metrics
- ✅ **Business intelligence** with custom scoring and correlation

**Your database intelligence platform is now enterprise-ready with New Relic One!** 🚀

---

*For additional support, refer to the validation scripts in `shared/validation/` or consult the enterprise status document.*