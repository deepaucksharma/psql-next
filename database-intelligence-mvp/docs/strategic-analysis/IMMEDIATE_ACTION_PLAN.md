# Database Intelligence MVP: Immediate Action Plan

## 🚀 Quick Wins (Execute Today)

### 1. Fix Module Path Issues (30 minutes)
```bash
#!/bin/bash
# Run this script from project root
echo "Fixing module path inconsistencies..."

# Backup files first
cp otelcol-builder.yaml otelcol-builder.yaml.bak
cp ocb-config.yaml ocb-config.yaml.bak

# Fix module paths
sed -i 's|github.com/newrelic/database-intelligence-mvp|github.com/database-intelligence-mvp|g' otelcol-builder.yaml
sed -i 's|github.com/database-intelligence/|github.com/database-intelligence-mvp/|g' ocb-config.yaml

# Update processor go.mod files if needed
for dir in processors/*/; do
  if [ -f "$dir/go.mod" ]; then
    sed -i 's|github.com/newrelic/database-intelligence-mvp|github.com/database-intelligence-mvp|g' "$dir/go.mod"
  fi
done

echo "Module paths fixed. Ready to build!"
```

### 2. First Build Attempt (15 minutes)
```bash
# Install tools
make install-tools

# Attempt build
make build

# If successful, validate config
make validate-config
```

### 3. Remove/Disable Broken Components (15 minutes)
If build fails due to custom OTLP exporter:
```yaml
# In ocb-config.yaml, comment out:
# - gomod: github.com/database-intelligence-mvp/exporters/otlpexporter v0.1.0

# Use standard OTLP instead
```

## 📋 Week 1 Sprint Plan

### Day 1: Foundation
- [ ] Fix module paths (see script above)
- [ ] Get first successful build
- [ ] Run unit tests for processors
- [ ] Document any new issues found

### Day 2: Local Development Environment
- [ ] Deploy test databases with Docker
- [ ] Configure collector for local databases
- [ ] Verify basic metric collection
- [ ] Test with debug exporter

### Day 3: New Relic Integration Setup
- [ ] Create New Relic test account/workspace
- [ ] Configure OTLP exporter for New Relic
- [ ] Add required resource attributes
- [ ] Verify data arrival in New Relic

### Day 4: First End-to-End Test
- [ ] Deploy full stack locally
- [ ] Collect PostgreSQL metrics
- [ ] Verify in New Relic UI
- [ ] Create first dashboard

### Day 5: Documentation & Planning
- [ ] Document working configuration
- [ ] Create runbook for deployment
- [ ] Plan Phase 2 in detail
- [ ] Present findings to team

## 🔧 Critical Configuration Updates

### 1. Minimal Working Configuration
```yaml
# config/collector-quickstart.yaml
extensions:
  health_check:
    endpoint: 0.0.0.0:13133

receivers:
  postgresql:
    endpoint: ${POSTGRES_HOST}:${POSTGRES_PORT}
    username: ${POSTGRES_USER}
    password: ${POSTGRES_PASSWORD}
    databases:
      - ${POSTGRES_DATABASE}
    collection_interval: 60s
    tls:
      insecure: true

processors:
  memory_limiter:
    check_interval: 1s
    limit_percentage: 80
    spike_limit_percentage: 30
  
  batch:
    send_batch_size: 1000
    timeout: 10s
  
  resource:
    attributes:
      - key: service.name
        value: database-intelligence
        action: insert
      - key: deployment.environment  
        value: development
        action: insert

exporters:
  debug:
    verbosity: detailed
  
  otlp/newrelic:
    endpoint: https://otlp.nr-data.net:4318
    headers:
      api-key: ${NEW_RELIC_LICENSE_KEY}
    sending_queue:
      enabled: true
      num_consumers: 10
      queue_size: 1000

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [memory_limiter, resource, batch]
      exporters: [debug, otlp/newrelic]
  
  telemetry:
    logs:
      level: info
    metrics:
      level: detailed
      address: 0.0.0.0:8888
```

### 2. Docker Compose for Testing
```yaml
# deploy/docker/docker-compose-quickstart.yaml
version: '3.8'

services:
  postgres:
    image: postgres:15
    environment:
      POSTGRES_PASSWORD: postgres
      POSTGRES_DB: testdb
    ports:
      - "5432:5432"
    command: >
      postgres
      -c shared_preload_libraries=pg_stat_statements
      -c pg_stat_statements.track=all
  
  collector:
    build: ../..
    environment:
      POSTGRES_HOST: postgres
      POSTGRES_PORT: 5432
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
      POSTGRES_DATABASE: testdb
      NEW_RELIC_LICENSE_KEY: ${NEW_RELIC_LICENSE_KEY}
    volumes:
      - ../../config/collector-quickstart.yaml:/etc/otel/config.yaml
    command: ["--config", "/etc/otel/config.yaml"]
    depends_on:
      - postgres
    ports:
      - "13133:13133"  # Health check
      - "8888:8888"    # Metrics
```

## 🎯 Success Criteria for Week 1

1. **Build Success**: Collector builds without errors
2. **Local Testing**: Can collect metrics from local PostgreSQL
3. **New Relic Integration**: Data appears in New Relic
4. **Basic Dashboard**: One working dashboard created
5. **Documentation**: Clear steps for others to reproduce

## 🚨 Potential Blockers & Solutions

### If Build Fails
1. Check Go version (requires 1.21+)
2. Clear module cache: `go clean -modcache`
3. Verify all dependencies: `go mod tidy`
4. Use pre-built collector as fallback

### If New Relic Integration Fails
1. Verify license key is valid
2. Check endpoint URL (US vs EU datacenter)
3. Monitor NrIntegrationError events
4. Use debug exporter to verify data format

### If Metrics Don't Appear
1. Check pg_stat_statements is enabled
2. Verify database permissions
3. Look for errors in collector logs
4. Test with simpler sqlquery receiver

## 📊 Metrics to Track

- Build time
- Test coverage  
- Memory usage of collector
- Metrics per second collected
- Latency of metric export
- Error rate

## 🤝 Team Coordination

### Daily Standup Topics
1. Blockers encountered
2. Progress on action items
3. New discoveries about the codebase
4. Help needed

### End of Week 1 Deliverables
1. Working collector binary
2. Successful New Relic integration
3. Documentation updates
4. Phase 2 detailed plan
5. Demo to stakeholders

---

**Start Time**: ____________  
**Owner**: ____________  
**Last Updated**: 2025-06-30