# Quick Reference

## 🚀 Deployment Commands

### Standard Mode
```bash
./quickstart.sh all
```

### Experimental Mode
```bash
./quickstart.sh --experimental all
```

## 🔍 Diagnostic Commands

*   **Pre-flight checks**: `./scripts/preflight-check.sh [standard|experimental]`.
*   **Compare modes**: `./scripts/compare-modes.sh [--quick]`.
*   **Validate deployment**: `./tests/integration/validate-setup.sh [standard|experimental]`.
*   **View logs**: `./quickstart.sh logs` or `./quickstart.sh --experimental logs`.

## 📊 Monitoring Endpoints

*   **Standard Mode**: Health (`http://localhost:13133/`), Metrics (`http://localhost:8888/metrics`).
*   **Experimental Mode**: Health (`http://localhost:13134/`), Metrics (`http://localhost:8889/metrics`), Debug (`http://localhost:55680/debug/tracez`), Profiling (`http://localhost:6061/debug/pprof`), Grafana (`http://localhost:3001`).

## 🛑 Stop/Restart Commands

*   **Stop**: `./quickstart.sh stop` or `./quickstart.sh --experimental stop`.
*   **Restart**: `./quickstart.sh stop && ./quickstart.sh start` (or `--experimental` version).
*   **Emergency Stop (Docker)**: `docker stop db-intel-primary` or `db-intel-experimental`.
*   **Emergency Stop (Kubernetes)**: `kubectl scale deployment db-intelligence-collector --replicas=0 -n db-intelligence`.

## 📝 Configuration Files

*   **Environment Variables**: `.env` (copy from `env.example`).
*   **Key Files**: `config/collector.yaml` (Standard), `config/collector-experimental.yaml` (Experimental).

## 🔧 Common Tasks

*   **Update Database Connection**: Edit `.env` (`PG_REPLICA_DSN`, `MYSQL_READONLY_DSN`), then restart collector.
*   **Change Sampling Rate (Standard)**: Edit `config/collector.yaml` (`probabilistic_sampler -> sampling_percentage`), then restart.
*   **Enable/Disable ASH (Experimental)**: Edit `config/collector-experimental.yaml` (`ash_sampling -> enabled`), then restart.

## 🚨 Troubleshooting

*   **No Data Collected**: Check connectivity (`preflight-check.sh`, `psql`), check logs for errors (`./quickstart.sh logs | grep -i error`).
*   **High Memory Usage**: Check `docker stats`, reduce sampling, or increase limits.
*   **Circuit Breaker Opened (Experimental)**: Check metrics (`curl http://localhost:8889/metrics | grep circuitbreaker`), increase thresholds in `config/collector-experimental.yaml`.

## 📚 Key Documentation

*   **First Time?** → [START-HERE.md](START-HERE.md).
*   **Setup Guide** → [GETTING-STARTED.md](GETTING-STARTED.md).
*   **Choose Mode** → [DEPLOYMENT-OPTIONS.md](DEPLOYMENT-OPTIONS.md).
*   **Troubleshooting** → [TROUBLESHOOTING-GUIDE.md](TROUBLESHOOTING-GUIDE.md).
*   **Architecture** → [ARCHITECTURE.md](ARCHITECTURE.md).

## 🔗 Useful Queries

*   **Check `pg_stat_statements`**: `SELECT queryid, query, mean_exec_time, calls FROM pg_stat_statements ORDER BY mean_exec_time DESC LIMIT 10;`.
*   **Check Active Sessions**: `SELECT pid, wait_event_type, wait_event, query FROM pg_stat_activity WHERE state != 'idle';`.

## 🎯 Quick Decision Tree

*   **Need it now for production?** → Use Standard Mode.
*   **Need advanced features?** → Use Experimental Mode.

## 💡 Pro Tips

1.  Always use read replicas for database connections.
2.  Start with low sampling and increase if needed.
3.  Monitor the monitor (check collector metrics regularly).
4.  Test in dev first before production deployment.
5.  Keep credentials secure (use proper `.env` permissions).
