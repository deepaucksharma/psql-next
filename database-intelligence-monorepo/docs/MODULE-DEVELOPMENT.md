# 🔧 Module Development Standards - Database Intelligence

## 📋 Overview

This document establishes development standards, patterns, and guidelines for creating and maintaining modules in the Database Intelligence MySQL Monorepo. All modules must follow these standards for consistency, maintainability, and production readiness.

## 🚫 **CRITICAL: Health Check Policy**

**⚠️ MANDATORY POLICY: Health checks are validation-only, NOT production features.**

### **DO NOT Add These to Any Module:**
- ❌ `health_check:` extensions in OpenTelemetry configs
- ❌ `healthcheck:` sections in Docker files  
- ❌ Port 13133 mappings or references
- ❌ `health:` targets in Makefiles
- ❌ Health check endpoints in production documentation

### **Use These Instead:**
- ✅ **Validation**: `shared/validation/health-check-all.sh`
- ✅ **Documentation**: `shared/validation/README-health-check.md`
- ✅ **Policy Reference**: `HEALTH_CHECK_POLICY.md`

## 🏗️ **Module Architecture Standards**

### **Required Module Structure**
```
modules/module-name/
├── Dockerfile                    # Container definition
├── Makefile                     # Build and run targets
├── README.md                    # Module-specific documentation
├── docker-compose.yaml          # Deployment configuration
├── config/                      # OpenTelemetry configurations
│   ├── collector.yaml           # Primary configuration
│   ├── collector-test.yaml      # Test configuration
│   └── collector-enhanced.yaml  # Enhanced features (optional)
├── src/                         # Source files (SQL, scripts, etc.)
├── tests/                       # Unit and integration tests
└── logs/                        # Log output directory
```

### **Required Files and Standards**

#### **1. Dockerfile Standards**
```dockerfile
FROM otel/opentelemetry-collector-contrib:latest

# WARNING: DO NOT ADD HEALTHCHECK instructions here!
# Health checks have been intentionally removed from production images.
# Use shared/validation/health-check-all.sh for validation purposes.

WORKDIR /etc/otel

# Copy configuration files
COPY config/ /etc/otel/
COPY src/ /opt/src/

# Create necessary directories
RUN mkdir -p /tmp/logs

# Expose ports for OpenTelemetry functionality only
EXPOSE 8081 4317 4318 8888 1777
# WARNING: DO NOT expose port 13133 (health check port)!

# Set default configuration
ENV COLLECTOR_CONFIG=collector.yaml

ENTRYPOINT ["/otelcol-contrib"]
CMD ["--config", "/etc/otel/collector.yaml"]
```

#### **2. Docker Compose Standards**
```yaml
version: '3.8'

services:
  otel-collector:
    # WARNING: DO NOT ADD healthcheck section here!
    # Health checks have been intentionally removed from production Docker configs.
    # Use shared/validation/health-check-all.sh for validation purposes.
    # See shared/validation/README-health-check.md for details.
    
    build: .
    container_name: module-name-collector
    env_file:
      - ../../shared/config/service-endpoints.env
    environment:
      MODULE_NAME: module-name
      MYSQL_ENDPOINT: ${MYSQL_ENDPOINT:-mysql-test:3306}
      MYSQL_USER: ${MYSQL_USER:-root}
      MYSQL_PASSWORD: ${MYSQL_PASSWORD:-test}
      EXPORT_PORT: 8081  # Module-specific port
      OTEL_SERVICE_NAME: module-name
      # New Relic Configuration
      NEW_RELIC_LICENSE_KEY: ${NEW_RELIC_LICENSE_KEY}
      NEW_RELIC_OTLP_ENDPOINT: ${NEW_RELIC_OTLP_ENDPOINT}
      NEW_RELIC_ACCOUNT_ID: ${NEW_RELIC_ACCOUNT_ID}
      NEW_RELIC_API_KEY: ${NEW_RELIC_API_KEY}
      ENVIRONMENT: production
      CLUSTER_NAME: database-intelligence-cluster
    ports:
      - "8081:8081"      # Module metrics (unique per module)
      - "4317:4317"      # OTLP gRPC
      - "4318:4318"      # OTLP HTTP
      - "55679:55679"    # ZPages
      - "6060:6060"      # pprof
      - "8888:8888"      # Internal telemetry
      # WARNING: DO NOT expose port 13133 (health check port)!
      # Health check ports have been intentionally removed.
    volumes:
      - ./config:/etc/otel:ro
      - ../../shared/config:/etc/shared:ro
      - ./logs:/tmp/logs
    command: ["--config", "/etc/otel/${COLLECTOR_CONFIG:-collector.yaml}"]
    networks:
      - db-intelligence
    deploy:
      resources:
        limits:
          memory: 2G
          cpus: '1.0'
        reservations:
          memory: 512M
          cpus: '0.5'

networks:
  db-intelligence:
    external: true
```

#### **3. Makefile Standards**
```makefile
# Module: module-name
# Purpose: Brief description of module functionality

MODULE_NAME = module-name
EXPORT_PORT = 8081

# WARNING: DO NOT ADD health check targets to this Makefile!
# Health checks have been intentionally removed from production code.
# Use shared/validation/health-check-all.sh for validation purposes.
# See shared/validation/README-health-check.md for details.

.PHONY: build run stop clean test logs
# WARNING: Do NOT add 'health' to .PHONY list!
# Health checks are validation-only, not production targets.

# Build module
build:
	@echo "Building $(MODULE_NAME) module..."
	@docker-compose build

# Run module
run:
	@echo "Starting $(MODULE_NAME) module..."
	@docker-compose up -d

# Stop module
stop:
	@echo "Stopping $(MODULE_NAME) module..."
	@docker-compose down

# View logs
logs:
	@docker-compose logs -f

# Clean module
clean:
	@echo "Cleaning $(MODULE_NAME) module..."
	@docker-compose down -v
	@docker system prune -f

# Test module
test:
	@echo "Testing $(MODULE_NAME) module..."
	# Health checks are available in shared/validation/ directory
	# Use: ../shared/validation/health-check-all.sh

# Help
help:
	@echo "Available targets for $(MODULE_NAME):"
	@echo "  build  - Build the module container"
	@echo "  run    - Start the module"
	@echo "  stop   - Stop the module"
	@echo "  logs   - View module logs"
	@echo "  clean  - Clean module and volumes"
	@echo "  test   - Run module tests"
	# Health checks are available in shared/validation/ directory
	# Use: ../shared/validation/health-check-all.sh
```

## 📊 **OpenTelemetry Configuration Standards**

### **Required Configuration Pattern**

```yaml
# Module Name - Configuration
# Brief description of module purpose

receivers:
  # Module-specific receivers here
  mysql:
    endpoint: ${env:MYSQL_ENDPOINT}
    username: ${env:MYSQL_USER}
    password: ${env:MYSQL_PASSWORD}
    collection_interval: 10s

  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318

processors:
  # WARNING: DO NOT ADD health_check extension here!
  # Health checks have been intentionally removed from production code.
  # Use shared/validation/health-check-all.sh for validation purposes.
  # See shared/validation/README-health-check.md for details.

  # Memory management - always first
  memory_limiter:
    check_interval: 5s
    limit_percentage: 80
    spike_limit_percentage: 30

  batch:
    timeout: 5s
    send_batch_size: 1000
    send_batch_max_size: 1500

  # Core attributes
  attributes:
    actions:
      - key: module
        value: module-name
        action: insert
      - key: mysql.endpoint
        value: ${env:MYSQL_ENDPOINT}
        action: insert
      - key: environment
        value: ${env:ENVIRONMENT}
        action: insert
      - key: cluster.name
        value: ${env:CLUSTER_NAME}
        action: insert

  resource:
    attributes:
      - key: service.name
        value: ${env:OTEL_SERVICE_NAME}
        action: upsert
      - key: service.version
        value: "2.0.0"
        action: upsert

  # New Relic specific attributes
  attributes/newrelic:
    actions:
      - key: instrumentation.provider
        value: opentelemetry
        action: insert
      - key: instrumentation.name
        value: mysql-module-name
        action: insert

  # Entity synthesis for New Relic One
  attributes/entity_synthesis:
    actions:
      - key: entity.type
        value: "MYSQL_INSTANCE"
        action: insert
      - key: entity.guid
        value: "${env:MYSQL_ENDPOINT}"
        action: insert

exporters:
  # Primary New Relic OTLP exporter
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

  # Prometheus exporter for federation
  prometheus:
    endpoint: 0.0.0.0:8081  # Module-specific port
    namespace: mysql
    resource_to_telemetry_conversion:
      enabled: true

  # Debug exporter
  debug:
    verbosity: basic

service:
  # WARNING: DO NOT ADD health_check to extensions list!
  # Health checks are validation-only, not production features.
  extensions: [zpages, pprof]  # NO health_check here!

  pipelines:
    metrics:
      receivers: [mysql, otlp]
      processors: [
        memory_limiter,
        batch,
        attributes,
        resource,
        attributes/newrelic,
        attributes/entity_synthesis
      ]
      exporters: [otlphttp/newrelic, prometheus, debug]

  telemetry:
    logs:
      level: info
      output_paths: ["/tmp/logs/collector.log"]
    metrics:
      level: detailed
      address: 0.0.0.0:8888

extensions:
  # WARNING: DO NOT ADD health_check extension here!
  # Health checks have been intentionally removed from production code.
  # Use shared/validation/health-check-all.sh for validation purposes.
  # See shared/validation/README-health-check.md for details.

  zpages:
    endpoint: 0.0.0.0:55679
    
  pprof:
    endpoint: 0.0.0.0:6060
```

## 📝 **README Template**

### **Required Module README Structure**

```markdown
# Module Name - Brief Description

## ⚠️ Health Check Policy

**IMPORTANT**: Health check endpoints (port 13133) have been intentionally removed from production code.

- **For validation**: Use `shared/validation/health-check-all.sh`
- **Documentation**: See `shared/validation/README-health-check.md`  
- **Do NOT**: Add health check endpoints back to production configs
- **Do NOT**: Expose port 13133 in Docker configurations
- **Do NOT**: Add health check targets to Makefiles

## 📊 Overview

Brief description of what this module does and its purpose in the overall database intelligence system.

## 🚀 Quick Start

```bash
# Build and run
make build
make run

# View logs
make logs

# Validate (instead of health checks)
../../shared/validation/health-check-all.sh
```

## 📈 Metrics

List of key metrics this module collects:

- `mysql.module.metric1` - Description
- `mysql.module.metric2` - Description

## 🔧 Configuration

Key configuration options and environment variables.

## 📊 Endpoints

- **Metrics**: http://localhost:8081/metrics
- **ZPages**: http://localhost:55679
- **pprof**: http://localhost:6060

## 🧪 Testing

```bash
# Run tests
make test

# Validation (replaces health checks)
../../shared/validation/module-specific/validate-module-name.py
```

## 🔗 Dependencies

List any module dependencies or federation requirements.
```

## 🧪 **Testing Standards**

### **Required Tests**

1. **Configuration Validation**
   ```bash
   # Test YAML syntax
   yq eval '.' config/collector.yaml > /dev/null
   ```

2. **Container Build**
   ```bash
   # Test Docker build
   docker build -t module-test .
   ```

3. **Metrics Validation**
   ```bash
   # Test metrics endpoint
   curl -f http://localhost:8081/metrics
   ```

4. **New Relic Integration**
   ```bash
   # Test OTLP export
   ../../shared/validation/test-nrdb-connection.py --module module-name
   ```

## 📊 **Port Allocation Standards**

### **Reserved Port Ranges**

| Port Range | Purpose | Assignment |
|------------|---------|------------|
| 8081-8088 | Module metrics | Assigned per module |
| 4317 | OTLP gRPC | Standard across all modules |
| 4318 | OTLP HTTP | Standard across all modules |
| 55679 | ZPages | Standard across all modules |
| 6060 | pprof | Standard across all modules |
| 8888 | Internal telemetry | Standard across all modules |
| ~~13133~~ | ~~Health checks~~ | **FORBIDDEN - Validation only** |

### **Module Port Assignments**

| Module | Port | Status |
|--------|------|--------|
| core-metrics | 8081 | ✅ Assigned |
| sql-intelligence | 8082 | ✅ Assigned |
| wait-profiler | 8083 | ✅ Assigned |
| anomaly-detector | 8084 | ✅ Assigned |
| business-impact | 8085 | ✅ Assigned |
| replication-monitor | 8086 | ✅ Assigned |
| performance-advisor | 8087 | ✅ Assigned |
| resource-monitor | 8088 | ✅ Assigned |

## 🔄 **Federation Patterns**

### **Module Dependencies**

Modules that consume data from other modules must use federation:

```yaml
receivers:
  prometheus/upstream:
    config:
      scrape_configs:
        - job_name: 'upstream-module'
          static_configs:
            - targets: ['${env:UPSTREAM_ENDPOINT:-upstream:8081}']
```

### **Data Flow Standards**

1. **Primary Data**: Collect directly from MySQL
2. **Enrichment**: Use federation for additional context
3. **Export**: Always export to New Relic + Prometheus
4. **Debug**: Include debug exporter for troubleshooting

## 🚀 **Deployment Patterns**

### **Standard Deployment Sequence**

1. **Core Infrastructure**: Deploy core-metrics first
2. **Intelligence Modules**: sql-intelligence, wait-profiler
3. **Analysis Modules**: anomaly-detector, business-impact
4. **Advisory Modules**: performance-advisor, replication-monitor
5. **Resource Monitoring**: resource-monitor
6. **Enterprise Features**: alert-manager, canary-tester, cross-signal-correlator

### **Validation After Deployment**

```bash
# Validate all modules
make validate

# Check specific module
../../shared/validation/module-specific/validate-module-name.py

# Verify New Relic integration
../../shared/newrelic/scripts/test-validation.sh
```

## 🔒 **Security Standards**

### **Required Security Measures**

1. **No Hardcoded Credentials**: Use environment variables only
2. **Secure Communications**: TLS for external connections
3. **Minimal Privileges**: MySQL user with minimal required permissions
4. **Container Security**: Non-root user in containers
5. **Network Isolation**: Use Docker networks for internal communication

### **Forbidden Practices**

- ❌ Hardcoded passwords or API keys
- ❌ Running containers as root
- ❌ Exposing unnecessary ports
- ❌ Including debug information in production
- ❌ Adding health check endpoints (validation-only)

## 📚 **Development Workflow**

### **Module Creation Checklist**

- [ ] Create module directory structure
- [ ] Implement Dockerfile with security warnings
- [ ] Create docker-compose.yaml with proper warnings
- [ ] Write Makefile following standards (no health targets)
- [ ] Implement OpenTelemetry configuration (no health_check)
- [ ] Write module README with health check policy
- [ ] Create tests and validation scripts
- [ ] Update port allocation documentation
- [ ] Add module to main documentation

### **Code Review Requirements**

- [ ] No health check endpoints or references
- [ ] Proper security practices implemented
- [ ] Standard port allocations used
- [ ] Documentation follows template
- [ ] Tests include validation scripts
- [ ] New Relic integration tested
- [ ] Warning comments present in all configs

---

## 🎯 **Remember: Health Checks Are Validation-Only**

Every module must comply with the health check policy:
- **Production**: No health check endpoints
- **Validation**: Use `shared/validation/` tools only
- **Documentation**: Reference validation tools, not health endpoints
- **Development**: No health targets in Makefiles

**For questions or exceptions, refer to `HEALTH_CHECK_POLICY.md`**

---

*This document is the authoritative source for module development standards. All modules must comply with these guidelines.*