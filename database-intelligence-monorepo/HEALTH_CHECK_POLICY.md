# 🚫 Health Check Policy - Database Intelligence Monorepo

## ⚠️ CRITICAL POLICY: Health Checks Are Validation-Only

This document establishes the **official policy** for health check endpoints in the Database Intelligence MySQL Monorepo. **This policy must be followed by all contributors and maintainers.**

## 📋 Policy Summary

**Health check endpoints (port 13133) have been intentionally and permanently removed from all production code.**

### ❌ FORBIDDEN - Do NOT Add These Back:

1. **OpenTelemetry Configurations**:
   - ❌ `health_check:` extension in collector configs
   - ❌ `health_check` in service.extensions lists
   - ❌ Any reference to port 13133 in collector configs

2. **Docker Configurations**:
   - ❌ `healthcheck:` sections in docker-compose files
   - ❌ `HEALTHCHECK` instructions in Dockerfiles
   - ❌ Port 13133 mappings in Docker configurations

3. **Makefile Targets**:
   - ❌ `health:` targets in any Makefile
   - ❌ Health check commands in build/run targets
   - ❌ References to health endpoints in help text

4. **Production Documentation**:
   - ❌ Health check endpoints in production guides
   - ❌ Port 13133 in endpoint listings
   - ❌ Health checks as monitoring recommendations

## ✅ APPROVED - Use These Instead:

### For Production Monitoring:
- **Metrics Endpoints**: Use ports 8081-8088 for module metrics
- **Prometheus Scraping**: Direct metrics collection from `/metrics` endpoints
- **New Relic Monitoring**: OTLP export to New Relic for observability
- **Container Status**: Use Docker/Kubernetes native health mechanisms

### For Development/Validation:
- **Validation Script**: `./shared/validation/health-check-all.sh`
- **Module Validation**: Individual validation scripts in `shared/validation/`
- **Documentation**: `shared/validation/README-health-check.md`

## 🎯 Rationale

### Why Health Checks Were Removed:
1. **Production Simplicity**: Reduces attack surface and complexity
2. **Resource Efficiency**: Eliminates unnecessary endpoints and processes
3. **Clear Separation**: Distinguishes between validation tools and production features
4. **Monitoring Focus**: Emphasizes metrics-based observability over binary health status
5. **Container Native**: Leverages container orchestration health mechanisms

### Architecture Benefits:
- **Cleaner Production Images**: No health check overhead
- **Focused Monitoring**: Metrics-driven observability strategy
- **Better Resource Usage**: No dedicated health check processes
- **Improved Security**: Fewer exposed endpoints
- **Simplified Deployment**: No health check port coordination

## 📊 Production Monitoring Strategy

### Recommended Approach:
```yaml
# Instead of health checks, use metrics-based monitoring:
prometheus:
  scrape_configs:
    - job_name: 'database-intelligence'
      static_configs:
        - targets:
          - 'core-metrics:8081'
          - 'sql-intelligence:8082'
          - 'wait-profiler:8083'
          # ... all modules 8081-8088
      metrics_path: '/metrics'
```

### New Relic Integration:
```yaml
# Use OTLP export for observability:
exporters:
  otlphttp/newrelic:
    endpoint: https://otlp.nr-data.net:4318
    headers:
      api-key: ${NEW_RELIC_LICENSE_KEY}
```

## 🧪 Validation Guidelines

### Development Testing:
```bash
# Use validation scripts for testing
cd /path/to/database-intelligence-monorepo
./shared/validation/health-check-all.sh

# For specific module validation
./shared/validation/module-specific/validate-core-metrics.py
```

### CI/CD Integration:
```yaml
# Example CI pipeline step
- name: Validate Modules
  run: |
    cd database-intelligence-monorepo
    ./shared/validation/health-check-all.sh
    if [ $? -ne 0 ]; then
      echo "Module validation failed"
      exit 1
    fi
```

## 🚨 Enforcement

### Code Review Requirements:
- **Any PR adding health check endpoints MUST be rejected**
- **Documentation changes adding health check references MUST be rejected**
- **Docker configurations exposing port 13133 MUST be rejected**

### Automated Checks:
```bash
# Pre-commit hook example
#!/bin/bash
if grep -r "health_check:" modules/*/config/ &>/dev/null; then
  echo "ERROR: Health check extension found in collector config"
  echo "Health checks are forbidden in production code"
  exit 1
fi

if grep -r "healthcheck:" modules/*/docker-compose* &>/dev/null; then
  echo "ERROR: Docker healthcheck found"
  echo "Health checks are forbidden in Docker configs"
  exit 1
fi

if grep -r ":13133" modules/ &>/dev/null; then
  echo "ERROR: Health check port 13133 found"
  echo "Port 13133 is forbidden in production configs"
  exit 1
fi
```

## 📚 Reference Documentation

### Key Files to Consult:
- **Validation Tools**: `shared/validation/README-health-check.md`
- **Project Guidance**: `CLAUDE.md`
- **Module Examples**: Individual module README files
- **Deployment Guide**: `DEPLOYMENT-GUIDE.md`

### Warning Locations:
Health check prevention warnings have been added to:
- ✅ 51 OpenTelemetry collector configuration files
- ✅ 23 Docker files (compose + Dockerfiles)
- ✅ 11 Makefile files
- ✅ 16 Documentation files

## 🔍 Exception Process

### If Health Checks Are Absolutely Required:

1. **Create Issue**: Document the specific business requirement
2. **Architecture Review**: Evaluate alternatives (metrics, logs, traces)
3. **Security Assessment**: Analyze attack surface implications
4. **Validation-Only Scope**: Ensure it remains validation-only
5. **Approval Required**: Must be approved by project maintainers

### Alternative Solutions to Consider:
- **Liveness Probes**: Use container orchestration health mechanisms
- **Readiness Checks**: Based on metrics endpoint availability
- **Custom Metrics**: Application-specific health indicators
- **Log Analysis**: Pattern-based health detection
- **Distributed Tracing**: Request flow health validation

## 📞 Contact and Support

### For Questions:
- **Validation Issues**: Check `shared/validation/README-health-check.md`
- **Monitoring Setup**: Refer to `DEPLOYMENT-GUIDE.md`
- **Policy Clarification**: Review this document and related warnings

### Contributing:
- All contributions must comply with this health check policy
- PRs violating this policy will be automatically rejected
- Documentation updates must maintain validation-only messaging

---

## 🏆 Policy Compliance Checklist

Before submitting any changes, verify:

- [ ] No `health_check:` extensions in OpenTelemetry configs
- [ ] No `healthcheck:` sections in Docker files
- [ ] No port 13133 references in any configuration
- [ ] No `health:` targets in Makefiles
- [ ] No health check endpoints in production documentation
- [ ] Validation scripts used for testing instead of embedded health checks
- [ ] Metrics endpoints used for production monitoring
- [ ] All warnings and policy references maintained

**Remember: Health checks are validation tools, not production features.**

---

*This policy is enforced across the entire Database Intelligence MySQL Monorepo and must be respected by all contributors.*