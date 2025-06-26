# PostgreSQL Unified Collector Demo

This demo shows how the PostgreSQL Unified Collector works end-to-end with both New Relic Infrastructure (NRI) and OpenTelemetry (OTel) outputs.

## What's Included

1. **PostgreSQL 15** with pg_stat_statements enabled
2. **Workload Generator** - Creates realistic database activity
3. **Unified Collector** (Python simulation) - Demonstrates the collection logic
4. **OpenTelemetry Collector** - Receives and exports metrics
5. **Prometheus** - Stores metrics
6. **Grafana** - Visualizes metrics

## Running the Demo

1. Start all services:
```bash
cd demo
docker-compose up -d
```

2. Wait for services to be healthy (about 30 seconds):
```bash
docker-compose ps
```

3. View collector logs to see metrics being collected:
```bash
docker-compose logs -f collector
```

4. Access services:
- PostgreSQL: `localhost:5432` (user: postgres, password: postgres123)
- Prometheus: http://localhost:9090
- Grafana: http://localhost:3000 (user: admin, password: admin)

## What You'll See

### Collector Output (NRI Format)
```json
{
  "name": "com.newrelic.postgresql",
  "protocol_version": "4",
  "data": [{
    "entity": {
      "name": "postgres:5432",
      "type": "pg-instance",
      "metrics": [
        {
          "event_type": "PostgresSlowQueries",
          "query_id": "123456789",
          "query_text": "SELECT pg_sleep...",
          "avg_elapsed_time_ms": 1005.234,
          "execution_count": 10
        }
      ]
    }
  }]
}
```

### Prometheus Metrics
- `postgresql_query_duration_milliseconds` - Query execution time
- `postgresql_query_executions_total` - Query execution count
- `postgresql_blocking_sessions_total` - Number of blocking sessions

## Simulating Different Scenarios

### High Query Load
```bash
docker-compose exec workload bash -c "
  for i in {1..100}; do
    PGPASSWORD=postgres123 psql -h postgres -U postgres -d testdb -c 'SELECT * FROM test_table ORDER BY random() LIMIT 1000;' &
  done
"
```

### Blocking Sessions
```bash
# Terminal 1 - Start a long transaction
docker-compose exec postgres psql -U postgres -d testdb -c "
BEGIN;
UPDATE test_table SET data = 'locked' WHERE id = 1;
SELECT pg_sleep(60);
"

# Terminal 2 - Try to update same row
docker-compose exec postgres psql -U postgres -d testdb -c "
UPDATE test_table SET data = 'blocked' WHERE id = 1;
"
```

## Architecture

```
┌─────────────┐     ┌──────────────────┐     ┌─────────────┐
│  PostgreSQL │────▶│ Unified Collector│────▶│ NRI Output  │
│   Database  │     │    (Python Demo) │     └─────────────┘
└─────────────┘     └──────────────────┘            │
                             │                       ▼
                             │                ┌─────────────┐
                             └───────────────▶│ OTLP Output │
                                             └─────────────┘
                                                     │
                                                     ▼
                                            ┌─────────────────┐
                                            │ OTel Collector  │
                                            └─────────────────┘
                                                     │
                                                     ▼
                                            ┌─────────────────┐
                                            │   Prometheus    │
                                            └─────────────────┘
                                                     │
                                                     ▼
                                            ┌─────────────────┐
                                            │    Grafana      │
                                            └─────────────────┘
```

## Cleanup

```bash
docker-compose down -v
```

## Next Steps

This demo simulates the Rust-based unified collector. The actual implementation would:

1. Use the Rust binary instead of Python
2. Support all PostgreSQL extensions (pg_wait_sampling, pg_stat_monitor)
3. Include eBPF support for kernel-level metrics
4. Provide production-ready performance and reliability
5. Support all deployment patterns (K8s, systemd, etc.)