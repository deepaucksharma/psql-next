# Production Deployment Checklist

This checklist ensures a safe and successful deployment of the Database Intelligence OTEL Collector to production.

## Pre-Deployment Requirements

### ✅ Infrastructure Prerequisites

- [ ] **PostgreSQL 12+** with pg_stat_statements enabled
  ```sql
  SHOW shared_preload_libraries; -- Must include pg_stat_statements
  SELECT * FROM pg_extension WHERE extname = 'pg_stat_statements';
  ```

- [ ] **MySQL 5.7+** with Performance Schema enabled (if using MySQL)
  ```sql
  SHOW VARIABLES LIKE 'performance_schema'; -- Must be ON
  ```

- [ ] **Monitoring Database User** created with minimal privileges
  ```sql
  -- PostgreSQL
  CREATE USER db_monitor WITH PASSWORD 'secure_password';
  GRANT pg_monitor TO db_monitor;
  GRANT SELECT ON pg_stat_statements TO db_monitor;
  
  -- MySQL
  CREATE USER 'db_monitor'@'%' IDENTIFIED BY 'secure_password';
  GRANT PROCESS, REPLICATION CLIENT ON *.* TO 'db_monitor'@'%';
  GRANT SELECT ON performance_schema.* TO 'db_monitor'@'%';
  ```

- [ ] **Network Connectivity** verified
  - Database ports accessible from collector
  - OTLP endpoint (otlp.nr-data.net:4317) reachable
  - Internal metrics ports not exposed externally

- [ ] **Resource Allocation** confirmed
  - 512MB-1GB RAM available
  - 0.5-1 CPU core available
  - 100MB disk space for state files

### ✅ Build and Configuration

- [ ] **Fix Module Path Issues** (if building from source)
  ```bash
  sed -i 's|github.com/newrelic/database-intelligence-mvp|github.com/database-intelligence-mvp|g' otelcol-builder.yaml
  sed -i 's|github.com/database-intelligence/|github.com/database-intelligence-mvp/|g' ocb-config.yaml
  ```

- [ ] **Build Collector** (if not using pre-built image)
  ```bash
  make install-tools
  make build
  make validate-config
  ```

- [ ] **Environment Variables** configured
  ```bash
  # Required
  export POSTGRES_HOST=prod-db-host
  export POSTGRES_PORT=5432
  export POSTGRES_USER=db_monitor
  export POSTGRES_PASSWORD=<secure_password>
  export POSTGRES_DATABASE=production
  export NEW_RELIC_LICENSE_KEY=<your_license_key>
  
  # Optional but recommended
  export ENVIRONMENT=production
  export LOG_LEVEL=info
  ```

- [ ] **Configuration File** selected and validated
  - Use `collector.yaml` for full features
  - Validate: `./dist/otelcol-db-intelligence validate --config=config/collector.yaml`

### ✅ Security Review

- [ ] **Credentials** stored securely (not in code)
  - Using environment variables or secrets management
  - Database passwords are strong
  - New Relic license key is valid

- [ ] **Network Security** configured
  - TLS enabled for database connections (production)
  - Collector ports not exposed to internet
  - Firewall rules in place

- [ ] **PII Protection** enabled
  ```yaml
  processors:
    transform:
      metric_statements:
        - context: datapoint
          statements:
            - replace_pattern(attributes["query.text"], "\\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\\.[A-Z|a-z]{2,}\\b", "[EMAIL]")
  ```

## Deployment Steps

### Option A: Docker Deployment

- [ ] **Pull/Build Image**
  ```bash
  # Using official image
  docker pull otel/opentelemetry-collector-contrib:latest
  
  # Or build custom image
  make docker-build
  ```

- [ ] **Create Docker Compose** file with production settings
  ```yaml
  version: '3.8'
  services:
    collector:
      image: database-intelligence:latest
      restart: unless-stopped
      resources:
        limits:
          memory: 1G
          cpus: '1'
      # ... rest of config
  ```

- [ ] **Deploy Container**
  ```bash
  docker-compose -f docker-compose-production.yaml up -d
  ```

### Option B: Kubernetes Deployment

- [ ] **Create Namespace and Secrets**
  ```bash
  kubectl create namespace database-intelligence
  kubectl create secret generic db-credentials -n database-intelligence \
    --from-literal=username=$POSTGRES_USER \
    --from-literal=password=$POSTGRES_PASSWORD
  kubectl create secret generic newrelic-credentials -n database-intelligence \
    --from-literal=license-key=$NEW_RELIC_LICENSE_KEY
  ```

- [ ] **Apply Manifests**
  ```bash
  kubectl apply -f deploy/examples/kubernetes-production.yaml
  ```

- [ ] **Verify Deployment**
  ```bash
  kubectl get all -n database-intelligence
  kubectl logs -n database-intelligence deployment/database-intelligence-collector
  ```

## Post-Deployment Validation

### ✅ Health Checks

- [ ] **Collector Health Endpoint**
  ```bash
  curl http://<collector-host>:13133/
  # Expected: {"status":"Server available"}
  ```

- [ ] **Prometheus Metrics**
  ```bash
  curl http://<collector-host>:8888/metrics | grep -E "^otelcol_"
  # Should see collector metrics
  ```

- [ ] **Custom Processor Metrics**
  ```bash
  curl http://<collector-host>:8888/metrics | grep -E "(adaptive_sampler|circuit_breaker|verification)"
  # Should see processor-specific metrics
  ```

### ✅ Data Flow Validation

- [ ] **Database Metrics Collection**
  ```bash
  # Check receiver metrics
  curl http://<collector-host>:8888/metrics | grep receiver_accepted_metric_points
  # Should show increasing counts
  ```

- [ ] **New Relic Data Arrival** (wait 2-5 minutes)
  ```sql
  -- In New Relic Query Builder
  FROM Metric 
  SELECT count(*) 
  WHERE service.name = 'database-intelligence' 
  SINCE 10 minutes ago
  ```

- [ ] **Query Performance Metrics**
  ```sql
  -- In New Relic
  FROM Metric 
  SELECT average(db.query.exec_time.mean) 
  WHERE service.name = 'database-intelligence' 
  FACET query.text 
  SINCE 1 hour ago
  ```

### ✅ Performance Validation

- [ ] **Resource Usage** within limits
  ```bash
  # Docker
  docker stats database-intelligence-collector
  
  # Kubernetes
  kubectl top pod -n database-intelligence
  ```

- [ ] **Processing Latency** acceptable
  ```bash
  curl http://<collector-host>:8888/metrics | grep otelcol_processor_process_duration
  # Should be < 100ms for most processors
  ```

- [ ] **No Circuit Breaker Activations** (unless expected)
  ```bash
  curl http://<collector-host>:8888/metrics | grep circuit_breaker_state
  # Should mostly show state="closed"
  ```

## Monitoring Setup

- [ ] **Create Alerts** in New Relic
  - Collector health check failures
  - High error rates
  - Circuit breaker activations
  - Memory usage > 80%
  - Missing data

- [ ] **Create Dashboards**
  - Database performance overview
  - Query performance trends
  - Collector health metrics
  - Resource usage

- [ ] **Set up Log Aggregation**
  ```yaml
  service:
    telemetry:
      logs:
        output_paths: ["stdout", "/var/log/collector/collector.log"]
  ```

## Rollback Plan

- [ ] **Rollback Procedure** documented
  1. Stop collector: `docker-compose down` or `kubectl delete -f`
  2. Check for any database impact
  3. Restore previous monitoring solution if needed
  4. Investigate issues in test environment

- [ ] **Rollback Triggers** defined
  - Database performance degradation
  - Excessive resource usage
  - Data quality issues
  - Critical errors

## Production Handoff

- [ ] **Documentation** provided to ops team
  - Architecture diagram
  - Configuration reference
  - Troubleshooting guide
  - Runbook for common issues

- [ ] **Access Controls** configured
  - Who can modify configuration
  - Who can restart collector
  - Alert notification routing

- [ ] **Change Management** process defined
  - How to update configuration
  - How to upgrade collector
  - Testing requirements

## Final Checks

- [ ] All sensitive data is sanitized (PII protection working)
- [ ] Resource usage is within expected bounds
- [ ] No errors in collector logs
- [ ] Data appears correctly in New Relic
- [ ] Alerts are firing correctly (test one)
- [ ] Team is trained on troubleshooting
- [ ] Rollback plan is tested

## Sign-offs

- [ ] **Development Team**: Code and configuration ready
- [ ] **Security Team**: Security review passed
- [ ] **Operations Team**: Infrastructure ready
- [ ] **Database Team**: Database impact acceptable
- [ ] **Monitoring Team**: Dashboards and alerts configured

---

**Deployment Date**: _______________
**Deployed By**: _______________
**Version**: _______________
**Notes**: _______________