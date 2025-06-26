# PostgreSQL Unified Collector - Migration Guide

## Table of Contents
1. [Migration Overview](#migration-overview)
2. [Pre-Migration Assessment](#pre-migration-assessment)
3. [Migration Strategies](#migration-strategies)
4. [Step-by-Step Migration](#step-by-step-migration)
5. [Validation and Testing](#validation-and-testing)
6. [Rollback Procedures](#rollback-procedures)
7. [Post-Migration](#post-migration)
8. [Troubleshooting](#troubleshooting)

## Migration Overview

The PostgreSQL Unified Collector is designed as a drop-in replacement for nri-postgresql, ensuring a smooth migration path with zero disruption to existing monitoring.

### Key Benefits of Migration
- **100% Backward Compatibility**: All existing dashboards and alerts continue to work
- **Enhanced Metrics**: Access to histograms, ASH, kernel metrics, and more
- **Dual Export**: Simultaneously send to New Relic and OpenTelemetry
- **Better Performance**: More efficient collection with adaptive sampling
- **Cloud-Native**: First-class Kubernetes and cloud provider support

### Migration Principles
1. **Zero Downtime**: No monitoring gaps during migration
2. **Gradual Rollout**: Test thoroughly before full deployment
3. **Easy Rollback**: Quick reversion if issues arise
4. **Validation First**: Verify metric parity before switching

## Pre-Migration Assessment

### 1. Current State Analysis

#### Inventory Your Environment
```bash
# List all PostgreSQL instances monitored
grep -r "nri-postgresql" /etc/newrelic-infra/integrations.d/

# Check current nri-postgresql version
/var/db/newrelic-infra/newrelic-integrations/bin/nri-postgresql --version

# Document current configuration
cat /etc/newrelic-infra/integrations.d/postgresql-config.yml
```

#### Assess Current Metrics
```sql
-- NRQL query to baseline current metrics
FROM PostgresSlowQueries, PostgresWaitEvents, PostgresBlockingSessions 
SELECT count(*) 
FACET eventType 
SINCE 24 hours ago
```

#### Identify Dependencies
- Custom dashboards using PostgreSQL metrics
- Alert policies based on PostgreSQL events
- Automated reports or integrations
- Third-party tools consuming metrics

### 2. Compatibility Check

#### PostgreSQL Version
```sql
-- Check PostgreSQL version
SELECT version();

-- Minimum supported: PostgreSQL 12
-- Optimal: PostgreSQL 14+
```

#### Required Extensions
```sql
-- Check installed extensions
SELECT name, installed_version 
FROM pg_available_extensions 
WHERE name IN (
  'pg_stat_statements',
  'pg_wait_sampling',
  'pg_stat_monitor'
);
```

#### Infrastructure Agent Version
```bash
# Check Infrastructure Agent version
newrelic-infra --version

# Minimum required: 1.20.0
# Recommended: Latest version
```

### 3. Risk Assessment

| Risk Level | Scenario | Mitigation |
|------------|----------|------------|
| **Low** | Single PostgreSQL instance, standard configuration | Direct replacement |
| **Medium** | Multiple instances, custom queries | Parallel testing recommended |
| **High** | Critical production, complex alerts | Phased migration with extended validation |

## Migration Strategies

### Strategy 1: Direct Replacement (Low Risk)

Best for: Development/staging environments, single instances

```bash
# Stop Infrastructure Agent
sudo systemctl stop newrelic-infra

# Backup current integration
sudo cp /var/db/newrelic-infra/newrelic-integrations/bin/nri-postgresql \
        /var/db/newrelic-infra/newrelic-integrations/bin/nri-postgresql.backup

# Replace with unified collector
sudo cp postgres-unified-collector /var/db/newrelic-infra/newrelic-integrations/bin/nri-postgresql

# Start Infrastructure Agent
sudo systemctl start newrelic-infra
```

### Strategy 2: Parallel Testing (Medium Risk)

Best for: Production environments with flexibility

```yaml
# Create parallel configuration
# /etc/newrelic-infra/integrations.d/postgresql-unified-test.yml
integrations:
  - name: nri-postgresql
    exec: /opt/postgres-unified-collector/bin/postgres-unified-collector
    env:
      POSTGRES_COLLECTOR_MODE: nri
      # Copy all existing environment variables
      HOSTNAME: ${HOSTNAME}
      PORT: ${PORT}
      USERNAME: ${USERNAME}
      PASSWORD: ${PASSWORD}
      # Add test suffix to differentiate metrics
      METRIC_PREFIX: "test_"
    interval: 60s
```

### Strategy 3: Canary Deployment (High Risk)

Best for: Critical production environments

```yaml
# Deploy to subset of instances
# Instance 1: Continue with nri-postgresql
# Instance 2: Switch to unified collector
# Monitor both for 24-48 hours
# Gradually increase unified collector coverage
```

### Strategy 4: Blue-Green Deployment (Kubernetes)

```yaml
# Deploy unified collector as new deployment
apiVersion: apps/v1
kind: Deployment
metadata:
  name: postgres-collector-unified
  labels:
    version: unified
spec:
  replicas: 1
  template:
    spec:
      containers:
      - name: collector
        image: newrelic/postgres-unified-collector:latest
        # Same configuration as existing
```

## Step-by-Step Migration

### Step 1: Preparation

#### 1.1 Download Unified Collector
```bash
# Download latest release
curl -LO https://github.com/newrelic/postgres-unified-collector/releases/latest/download/postgres-unified-collector_linux_amd64.tar.gz

# Extract
tar -xzf postgres-unified-collector_linux_amd64.tar.gz

# Verify binary
./postgres-unified-collector --version
```

#### 1.2 Test Connectivity
```bash
# Test with current configuration
./postgres-unified-collector test \
  --host=${POSTGRES_HOST} \
  --port=5432 \
  --username=${POSTGRES_USER} \
  --password=${POSTGRES_PASSWORD}
```

#### 1.3 Create Test Configuration
```toml
# test-config.toml
[collector]
mode = "nri"

[postgres]
host = "${POSTGRES_HOST}"
port = 5432
username = "${POSTGRES_USER}"
password = "${POSTGRES_PASSWORD}"
databases = ["postgres", "app_db"]

[ohi_compatibility]
preserve_metric_names = true
query_monitoring_count_threshold = 20
query_monitoring_response_time_threshold = 500
```

### Step 2: Validation Phase

#### 2.1 Dry Run Test
```bash
# Run collector in dry-run mode
./postgres-unified-collector \
  --config test-config.toml \
  --mode nri \
  --dry-run \
  --output metrics-sample.json

# Verify output format matches nri-postgresql
jq . metrics-sample.json
```

#### 2.2 Compare Metrics
```python
#!/usr/bin/env python3
# compare_metrics.py
import json
import sys

def compare_metrics(old_file, new_file):
    with open(old_file) as f:
        old_metrics = json.load(f)
    with open(new_file) as f:
        new_metrics = json.load(f)
    
    # Compare metric counts
    old_events = {}
    new_events = {}
    
    for entity in old_metrics.get('data', []):
        for metric in entity.get('metrics', []):
            event_type = metric.get('event_type', '')
            old_events[event_type] = old_events.get(event_type, 0) + 1
    
    for entity in new_metrics.get('data', []):
        for metric in entity.get('metrics', []):
            event_type = metric.get('event_type', '')
            new_events[event_type] = new_events.get(event_type, 0) + 1
    
    print("Metric Comparison:")
    print(f"{'Event Type':<30} {'Old':>10} {'New':>10} {'Diff':>10}")
    print("-" * 60)
    
    all_events = set(old_events.keys()) | set(new_events.keys())
    for event in sorted(all_events):
        old_count = old_events.get(event, 0)
        new_count = new_events.get(event, 0)
        diff = new_count - old_count
        print(f"{event:<30} {old_count:>10} {new_count:>10} {diff:>10}")

if __name__ == "__main__":
    compare_metrics(sys.argv[1], sys.argv[2])
```

#### 2.3 Parallel Validation
```bash
# Run both collectors for comparison
# Terminal 1: Original nri-postgresql
/var/db/newrelic-infra/newrelic-integrations/bin/nri-postgresql \
  --metrics --pretty > original-metrics.json

# Terminal 2: Unified collector
./postgres-unified-collector \
  --mode nri \
  --config test-config.toml \
  --once > unified-metrics.json

# Compare outputs
python3 compare_metrics.py original-metrics.json unified-metrics.json
```

### Step 3: Deployment

#### 3.1 Infrastructure Agent Integration
```bash
# Stop Infrastructure Agent
sudo systemctl stop newrelic-infra

# Backup original integration
sudo mv /var/db/newrelic-infra/newrelic-integrations/bin/nri-postgresql \
        /var/db/newrelic-infra/newrelic-integrations/bin/nri-postgresql.original

# Install unified collector
sudo cp postgres-unified-collector \
        /var/db/newrelic-infra/newrelic-integrations/bin/nri-postgresql
sudo chmod +x /var/db/newrelic-infra/newrelic-integrations/bin/nri-postgresql

# Start Infrastructure Agent
sudo systemctl start newrelic-infra

# Monitor logs
sudo journalctl -u newrelic-infra -f
```

#### 3.2 Kubernetes Migration
```bash
# Create new deployment with unified collector
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: postgres-collector-unified
  labels:
    app: postgres-collector
    version: unified
spec:
  replicas: 1
  selector:
    matchLabels:
      app: postgres-collector
      version: unified
  template:
    metadata:
      labels:
        app: postgres-collector
        version: unified
    spec:
      containers:
      - name: collector
        image: newrelic/postgres-unified-collector:latest
        env:
        # Copy environment from existing deployment
        - name: POSTGRES_HOST
          value: "postgres-service"
        # ... other environment variables
EOF

# Verify new deployment
kubectl logs -l app=postgres-collector,version=unified

# Scale down old deployment
kubectl scale deployment postgres-collector-old --replicas=0
```

### Step 4: Verification

#### 4.1 Verify Metrics Flow
```sql
-- Check metrics are arriving in New Relic
FROM PostgresSlowQueries 
SELECT count(*) 
WHERE collector.version = '1.0.0'  -- Unified collector version
SINCE 5 minutes ago

-- Compare with historical data
FROM PostgresSlowQueries 
SELECT count(*) 
FACET collector.version 
SINCE 1 hour ago
```

#### 4.2 Validate Dashboards
- Open each PostgreSQL dashboard
- Verify all widgets display data
- Check for any "No data" errors
- Compare values with historical trends

#### 4.3 Test Alerts
```bash
# Trigger test condition if possible
# Or temporarily lower thresholds to trigger alerts
# Verify alert notifications are received
```

## Validation and Testing

### Automated Validation Script
```bash
#!/bin/bash
# validate_migration.sh

echo "PostgreSQL Unified Collector Migration Validation"
echo "================================================"

# Check if collector is running
if pgrep -f "nri-postgresql" > /dev/null; then
    echo "✓ Collector is running"
else
    echo "✗ Collector is not running"
    exit 1
fi

# Check Infrastructure Agent
if systemctl is-active --quiet newrelic-infra; then
    echo "✓ Infrastructure Agent is active"
else
    echo "✗ Infrastructure Agent is not active"
    exit 1
fi

# Verify metrics in New Relic (requires API key)
if [ -n "$NEW_RELIC_API_KEY" ]; then
    METRICS=$(curl -s -X POST https://api.newrelic.com/graphql \
        -H "API-Key: $NEW_RELIC_API_KEY" \
        -H "Content-Type: application/json" \
        -d '{
            "query": "{ actor { account(id: YOUR_ACCOUNT_ID) { nrql(query: \"FROM PostgresSlowQueries SELECT count(*) SINCE 5 minutes ago\") { results } } } }"
        }')
    
    if echo "$METRICS" | grep -q "results"; then
        echo "✓ Metrics are flowing to New Relic"
    else
        echo "✗ No metrics found in New Relic"
    fi
fi

echo ""
echo "Manual checks required:"
echo "- [ ] Verify all dashboards are populated"
echo "- [ ] Check alert policies are functioning"
echo "- [ ] Compare metric values with baseline"
```

### Performance Validation
```bash
# Monitor resource usage
# Before migration
top -b -n 1 | grep nri-postgresql > before.txt

# After migration
top -b -n 1 | grep nri-postgresql > after.txt

# Compare CPU and memory usage
diff before.txt after.txt
```

## Rollback Procedures

### Immediate Rollback (< 5 minutes)
```bash
# Stop Infrastructure Agent
sudo systemctl stop newrelic-infra

# Restore original binary
sudo mv /var/db/newrelic-infra/newrelic-integrations/bin/nri-postgresql.original \
        /var/db/newrelic-infra/newrelic-integrations/bin/nri-postgresql

# Start Infrastructure Agent
sudo systemctl start newrelic-infra

# Verify rollback
sudo journalctl -u newrelic-infra -n 50
```

### Kubernetes Rollback
```bash
# Immediate rollback
kubectl rollout undo deployment postgres-collector

# Or scale up old deployment
kubectl scale deployment postgres-collector-old --replicas=1
kubectl scale deployment postgres-collector-unified --replicas=0
```

### Data Recovery
If metrics were lost during migration:
```sql
-- Query historical data from before migration
FROM PostgresSlowQueries 
SELECT * 
WHERE timestamp < '2024-01-20 10:00:00'  -- Before migration time
SINCE 48 hours ago
```

## Post-Migration

### 1. Enable Extended Features

Once migration is validated, enable new features:

```toml
# Update configuration
[features]
enable_extended_metrics = true
enable_ash = true
ash_sample_interval_ms = 1000
enable_plan_collection = true

[export]
# Enable dual export
[export.otlp]
enabled = true
endpoint = "https://otlp.nr-data.net:4317"
```

### 2. Update Documentation

Document the migration:
- Migration date and time
- Configuration changes
- Any custom modifications
- Contact information for support

### 3. Optimize Configuration

```toml
# Fine-tune based on your environment
[sampling]
enabled = true
base_sample_rate = 1.0

[[sampling.rules]]
name = "high_frequency_queries"
condition = "execution_count > 1000"
sample_rate = 0.1
```

### 4. Create New Dashboards

Leverage new metrics:
- Query latency histograms
- Active Session History timeline
- Wait event heat maps
- Plan regression tracking

## Troubleshooting

### Common Issues

#### Issue: No Metrics After Migration
```bash
# Check collector logs
sudo journalctl -u newrelic-infra | grep postgres

# Verify connectivity
./postgres-unified-collector test --config /etc/postgres-collector/config.toml

# Check permissions
psql -U monitoring -h localhost -c "SELECT 1"
```

#### Issue: Missing Specific Metrics
```sql
-- Verify extensions are enabled
SELECT * FROM pg_extension;

-- Check permissions
\du monitoring
```

#### Issue: Performance Degradation
```toml
# Reduce collection frequency
[collector]
collection_interval_secs = 120  # Increase from 60

# Enable sampling
[sampling]
base_sample_rate = 0.5  # Sample 50% of queries
```

#### Issue: Dashboard Errors
- Verify metric names haven't changed
- Check for new required attributes
- Update widget queries if needed

### Getting Help

1. **Check Logs**:
   ```bash
   # Infrastructure Agent logs
   sudo journalctl -u newrelic-infra -n 100
   
   # Collector debug mode
   RUST_LOG=debug postgres-unified-collector --config config.toml
   ```

2. **Generate Support Bundle**:
   ```bash
   postgres-unified-collector support-bundle \
     --config config.toml \
     --output support-bundle.tar.gz
   ```

3. **Community Support**:
   - GitHub Issues: https://github.com/newrelic/postgres-unified-collector/issues
   - New Relic Explorers Hub: https://discuss.newrelic.com

## Migration Checklist

### Pre-Migration
- [ ] Inventory all PostgreSQL instances
- [ ] Document current configuration
- [ ] Baseline current metrics
- [ ] Identify critical dashboards/alerts
- [ ] Download unified collector
- [ ] Test connectivity
- [ ] Create rollback plan

### During Migration
- [ ] Create backup of original integration
- [ ] Deploy unified collector
- [ ] Verify metrics flow
- [ ] Check all dashboards
- [ ] Test critical alerts
- [ ] Monitor performance
- [ ] Document any issues

### Post-Migration
- [ ] Enable extended features
- [ ] Update documentation
- [ ] Optimize configuration
- [ ] Create new dashboards
- [ ] Train team on new features
- [ ] Schedule review in 1 week

## Summary

The PostgreSQL Unified Collector provides a seamless migration path from nri-postgresql with significant benefits. By following this guide and using the validation tools provided, you can migrate with confidence while maintaining continuous monitoring coverage.

Remember: The unified collector is designed for zero-downtime migration. Take advantage of parallel testing and gradual rollout strategies to ensure a smooth transition.