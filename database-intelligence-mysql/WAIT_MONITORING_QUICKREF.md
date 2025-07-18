# MySQL Wait-Based Monitoring - Quick Reference

## 🚀 Quick Start

```bash
# Deploy everything
export NEW_RELIC_LICENSE_KEY="your-key"
./scripts/deploy-wait-monitoring.sh --with-workload

# Monitor in real-time
./scripts/monitor-waits.sh
```

## 📊 Key Metrics

| Metric | What It Means | Action Required |
|--------|---------------|-----------------|
| `wait_percentage > 90` | Query spending 90%+ time waiting | Check advisor recommendations |
| `mysql.blocking.active > 30` | Long-running blocks | Review locking strategy |
| `advisor.priority = P1` | Critical performance issue | Immediate action needed |
| `wait.category = io` | Storage bottleneck | Add indexes or upgrade storage |
| `plan.changed = true` | Execution plan regression | Review query performance |

## 🔍 Wait Categories

- **I/O Waits** (`wait/io/*`): Disk operations, missing indexes
- **Lock Waits** (`wait/lock/*`): Row/table locks, deadlocks
- **CPU Waits** (`wait/synch/*`): Mutexes, high CPU queries
- **Network Waits** (`wait/net/*`): Client communication delays

## 🎯 Advisory Types

1. **missing_index**: Query needs index for WHERE/JOIN
2. **lock_contention**: High lock waits detected
3. **inefficient_join**: Poor join strategy
4. **temp_table_to_disk**: Large sorts/groups
5. **lock_escalation_missing_index**: Table lock due to missing index

## 🛠️ Common Commands

```bash
# Check health
curl http://localhost:13133/  # Edge collector
curl http://localhost:13134/  # Gateway

# View metrics
curl http://localhost:9091/metrics | grep wait

# Generate test load
docker exec mysql-primary mysql -u root -prootpassword \
  wait_analysis_test -e "CALL generate_io_waits()"

# Check logs
docker-compose logs -f otel-collector-edge
docker-compose logs -f otel-gateway

# Validate setup
./scripts/validate-metrics.sh
```

## 📈 New Relic Queries

```sql
-- Top wait contributors
SELECT sum(mysql.query.wait_profile) as 'Total Wait', 
       latest(advisor.recommendation) as 'Fix'
FROM Metric 
WHERE query_hash IS NOT NULL 
FACET query_hash 
SINCE 1 hour ago

-- P1 advisories
SELECT count(*), latest(advisor.recommendation) 
FROM Metric 
WHERE advisor.priority = 'P1' 
FACET advisor.type

-- Blocking chains
SELECT max(mysql.blocking.active) as 'Duration',
       latest(lock_table) as 'Table'
FROM Metric 
WHERE metric.name = 'mysql.blocking.active'
FACET blocking_thread
```

## 🚨 Alert Thresholds

- **Critical Wait**: >90% wait percentage
- **Blocking Chain**: >60 seconds
- **I/O Saturation**: >50,000ms total wait
- **Missing Index Impact**: >10,000ms wait time
- **Plan Regression**: 5x slower execution

## 📁 File Structure

```
database-intelligence-mysql/
├── config/
│   ├── edge-collector-wait.yaml    # Wait analysis queries
│   └── gateway-advisory.yaml       # Advisory processing
├── dashboards/
│   └── newrelic/
│       ├── wait-analysis-dashboard.json
│       └── wait-based-alerts.yaml
├── mysql/init/
│   ├── 04-enable-wait-analysis.sql # Performance Schema setup
│   └── 05-create-test-workload.sql # Test data
├── scripts/
│   ├── deploy-wait-monitoring.sh   # Deployment
│   └── monitor-waits.sh           # Real-time monitoring
└── tests/e2e/
    ├── wait_analysis_test.go      # Wait validation
    └── pipeline_validation_test.go # E2E tests
```

## ⚡ Performance Tips

1. **Reduce overhead**: Increase `collection_interval` to 30s
2. **Control cardinality**: Limit unique queries to top 50
3. **Optimize batching**: Adjust `send_batch_size` (default: 2000)
4. **Memory limits**: Edge=384MB, Gateway=1GB

## 🔧 Troubleshooting

```bash
# No metrics?
docker-compose ps  # Check services
docker exec mysql-primary mysql -e "SELECT @@performance_schema"

# High memory?
docker stats otel-collector-edge

# Missing advisories?
curl http://localhost:9091/metrics | grep advisor_type
```

## 📞 Support

1. Collector logs: `docker-compose logs`
2. Validation: `./scripts/validate-metrics.sh`  
3. Documentation: `docs/WAIT_BASED_MONITORING_GUIDE.md`

---
**Remember**: If users aren't waiting, there's no performance problem!