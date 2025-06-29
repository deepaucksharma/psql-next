# Deployment Checklist

Use this checklist to ensure a successful deployment of Database Intelligence MVP.

## Pre-Deployment

### Database Prerequisites

*   **PostgreSQL**: Version 10+, `pg_stat_statements` enabled, read-only user with SELECT on `pg_stat_statements`, connection uses read replica.
*   **MySQL** (if applicable): Version 5.7+, Performance Schema enabled, read-only user with SELECT on `performance_schema`.

### Infrastructure Requirements

*   **Standard Mode**: 512MB RAM, Docker/Kubernetes ready, outbound HTTPS (443) allowed.
*   **Experimental Mode**: 2GB RAM, Docker with build capability, single instance deployment planned.

### Credentials and Access

*   New Relic license key obtained.
*   Database connection strings prepared.
*   Network firewall rules configured.
*   SSL/TLS certificates (if required).

## Deployment Steps

### Standard Mode Deployment

1.  **Local/Docker**: Clone repo, `quickstart.sh configure`, `quickstart.sh validate`, `quickstart.sh start`, `curl http://localhost:13133/`.
2.  **Kubernetes**: Update `deploy/k8s/ha-deployment.yaml`, create secrets, `kubectl apply -f deploy/k8s/`, verify pods, check leader election.

### Experimental Mode Deployment

1.  **Build Phase**: Install Go 1.21+, `quickstart.sh --experimental build`, verify Docker image, run integration tests.
2.  **Deploy Phase**: Configure `collector-experimental.yaml`, `quickstart.sh --experimental start`, monitor resource usage, check circuit breaker status.

## Post-Deployment Validation

### Data Flow Verification

*   **Collector Health**: `curl http://localhost:13133/` (Standard) or `http://localhost:13134/` (Experimental).
*   **Metrics Collection**: `curl http://localhost:8888/metrics | grep otelcol_receiver_accepted`.
*   **New Relic Validation**: Log into New Relic, confirm database entities and query metrics.

### Mode-Specific Checks

*   **Standard Mode**: All 3 replicas healthy, leader election functioning, memory usage <512MB, sampling at 25%.
*   **Experimental Mode**: ASH samples collected, adaptive sampling adjusting rates, circuit breaker metrics available, memory usage <2GB.

## Monitoring Setup

### Alerts to Configure

*   **Standard Mode**: Collector down/unhealthy, no data received, high memory usage, database connection failures.
*   **Experimental Mode**: Circuit breaker opened, high memory usage, ASH buffer full, adaptive sampler stuck.

### Dashboards to Create

*   Query performance overview, slow query tracking, database health metrics, collector operational metrics.

## Security Review

*   Database credentials stored securely, network policies applied, read-only database access verified, no sensitive data in logs, PII sanitization working.

## Documentation

*   Deployment details documented, runbook created, configuration backed up, contact list updated.

## Rollback Plan

*   Previous configuration saved, rollback procedure documented, team knows how to disable collector, database impact plan ready.

## Sign-off

*   Operations team trained, monitoring configured, documentation complete, stakeholders notified.

## Quick Commands Reference

*   **Status**: `./quickstart.sh status` or `./quickstart.sh --experimental status`.
*   **Logs**: `./quickstart.sh logs` or `./quickstart.sh --experimental logs`.
*   **Stop**: `./quickstart.sh stop` or `./quickstart.sh --experimental stop`.
*   **Emergency Stop (Kubernetes)**: `kubectl scale deployment db-intelligence-collector --replicas=0 -n db-intelligence`.

## Need Help?

*   Review [TROUBLESHOOTING-GUIDE.md](TROUBLESHOOTING-GUIDE.md).
*   Check [DEPLOYMENT-OPTIONS.md](DEPLOYMENT-OPTIONS.md).
*   Open GitHub issue for bugs.
*   Use GitHub Discussions for questions.
