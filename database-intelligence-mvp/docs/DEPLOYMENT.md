# Deployment Guide - Production Ready

✅ **PRODUCTION READY** - This guide provides instructions for deploying the Database Intelligence Collector using the stable single-instance model. All critical issues have been resolved as of June 2025.

## ✅ Production Status (June 2025)

- **✅ Single-Instance Deployment**: Reliable operation without Redis dependencies
- **✅ Zero External Dependencies**: Works with standard PostgreSQL pg_stat_statements
- **✅ Enhanced Security**: Comprehensive PII protection built-in
- **✅ Graceful Degradation**: All components work independently
- **✅ Production Configuration**: `config/collector-resilient.yaml` ready for use

## Prerequisites

### System Requirements
- **Task** (build automation tool) - Required
- Go 1.21+ (for building from source)
- Docker 20.10+ and Docker Compose v2+ (for container deployment)
- Kubernetes 1.24+ and Helm 3.0+ (for K8s deployment)
- PostgreSQL 12+ with pg_stat_statements
- MySQL 5.7+ with Performance Schema (optional)

### Install Task
```bash
# macOS
brew install go-task/tap/go-task

# Linux
sh -c "$(curl --location https://taskfile.dev/install.sh)" -- -d -b ~/.local/bin

# Windows
scoop install task
```

### ✅ Database Prerequisites (Simplified)

#### PostgreSQL Setup (Required)
```sql
-- Enable pg_stat_statements (standard extension)
ALTER SYSTEM SET shared_preload_libraries = 'pg_stat_statements';
-- Restart PostgreSQL

-- Create extension (built into PostgreSQL)
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

-- Create monitoring user with minimal privileges
CREATE USER monitoring_user WITH PASSWORD 'secure_password';
GRANT pg_monitor TO monitoring_user;
GRANT SELECT ON pg_stat_statements TO monitoring_user;
GRANT SELECT ON pg_stat_activity TO monitoring_user;

-- ✅ NO pg_querylens extension required (optional only)
-- ✅ NO additional PostgreSQL extensions needed
```

#### MySQL Setup (Optional)
```sql
-- Verify Performance Schema is enabled
SHOW VARIABLES LIKE 'performance_schema';

-- Create monitoring user
CREATE USER 'monitoring_user'@'%' IDENTIFIED BY 'secure_password';
GRANT PROCESS, REPLICATION CLIENT ON *.* TO 'monitoring_user'@'%';
GRANT SELECT ON performance_schema.* TO 'monitoring_user'@'%';
```

## ✅ Quick Start (Production Ready)

### Fastest Production Deployment
```bash
# Clone repository
git clone https://github.com/database-intelligence-mvp
cd database-intelligence-mvp

# Set up environment with production configuration
cp .env.example .env
# Edit .env with your credentials

# Start production-ready single instance
task quickstart CONFIG=resilient
```

This will:
1. Install dependencies
2. **✅ Use production-ready configuration** (`collector-resilient.yaml`)
3. Build the collector with **✅ in-memory state management**
4. Start **✅ single-instance collector** (no Redis needed)
5. Begin collecting metrics with **✅ enhanced PII protection**

## ✅ Production Deployment Options

### Option 1: Single-Instance Binary (Recommended)

#### Build from Source
```bash
# Build production-ready collector
task build

# ✅ Binary includes all fixed processors with in-memory state
# ✅ No Redis or external dependencies needed
# Binary will be in dist/otelcol
```

#### Run Production Binary
```bash
# Using production-ready configuration
task run CONFIG=resilient ENV_FILE=.env.production

# With environment variables (single instance)
POSTGRES_HOST=your-db-host \
POSTGRES_USER=monitoring_user \
NEW_RELIC_LICENSE_KEY=your_key_here \
ENVIRONMENT=production \
./dist/otelcol --config=config/collector-resilient.yaml
```

### Option 2: Docker Deployment (Single Instance)

#### Using Production Docker Compose
```bash
# ✅ Start production single-instance setup (no Redis)
task docker:prod

# Or manually with production config
docker-compose -f deploy/docker/docker-compose-ha.yaml up -d
# ✅ Note: This now deploys single instance despite the filename

# Start with different configurations
task docker:simple      # Basic monitoring only
task docker:resilient   # Full production features
```

#### Building Production Image
```bash
# Build production-optimized Docker image
task docker:build TARGET=production

# Build and push to registry
task docker:push REGISTRY=your-registry.com TAG=production
```

### Option 3: Kubernetes Single-Instance Deployment

#### Quick Deployment
```bash
# Deploy with default values
task deploy:helm

# Deploy to specific environment
task deploy:helm ENV=production

# Deploy with custom values
helm install db-intelligence ./deployments/helm/db-intelligence \
  -f deployments/helm/db-intelligence/values-production.yaml
```

#### Production Deployment
```bash
# Create namespace and secrets
kubectl create namespace db-intelligence
kubectl create secret generic database-credentials \
  --from-env-file=.env.production \
  -n db-intelligence

# Deploy with Helm
helm install db-intelligence ./deployments/helm/db-intelligence \
  -n db-intelligence \
  -f deployments/helm/db-intelligence/values-production.yaml \
  --set image.tag=v2.0.0
```

## Configuration Management

### Environment-Specific Configuration

We use a configuration overlay system for different environments:

```
configs/overlays/
├── base/           # Base configuration
├── dev/            # Development overrides
├── staging/        # Staging overrides
└── production/     # Production overrides
```

#### Using Overlays
```bash
# Development
task run CONFIG_ENV=dev

# Staging
task run CONFIG_ENV=staging

# Production
task run CONFIG_ENV=production
```

### Environment Variables

#### Development (.env.development)
```bash
ENVIRONMENT=development
LOG_LEVEL=debug
COLLECTION_INTERVAL_SECONDS=10
SAMPLING_PERCENTAGE=100
TLS_INSECURE_SKIP_VERIFY=true
```

#### Production (.env.production)
```bash
ENVIRONMENT=production
LOG_LEVEL=warn
COLLECTION_INTERVAL_SECONDS=300
SAMPLING_PERCENTAGE=25
TLS_INSECURE_SKIP_VERIFY=false
```

## Post-Deployment Validation

### Health Checks
```bash
# Check collector health
task health-check

# Check metrics endpoint
task metrics

# View logs
task dev:logs
```

### Verify Data Flow
```bash
# Check if metrics are being collected
task validate:metrics

# Test database connections
task test:connections

# Verify New Relic integration
task validate:newrelic
```

### NRQL Queries for Validation
```sql
-- Check collector is reporting
SELECT count(*) FROM Metric 
WHERE otel.library.name LIKE 'otelcol%' 
SINCE 5 minutes ago

-- Verify database metrics
SELECT latest(db_up), latest(db_connections_active) 
FROM Metric 
WHERE db_system IN ('postgresql', 'mysql') 
FACET db_name
```

## Production Best Practices

### Resource Allocation
- **Memory**: 512MB-1GB (1-2GB for experimental mode)
- **CPU**: 0.5-1 core (1-2 cores for experimental mode)
- **Storage**: 100MB for state files
- **Network**: Low latency to databases

### High Availability with Helm
```yaml
# values-production.yaml
replicaCount: 3
autoscaling:
  enabled: true
  minReplicas: 3
  maxReplicas: 10
  targetCPUUtilizationPercentage: 60

affinity:
  podAntiAffinity:
    requiredDuringSchedulingIgnoredDuringExecution:
      - topologyKey: kubernetes.io/hostname
```

### Security Configuration
```yaml
# Enable security features
securityContext:
  runAsNonRoot: true
  runAsUser: 1000
  fsGroup: 1000
  capabilities:
    drop: [ALL]
  readOnlyRootFilesystem: true

# Network policies
networkPolicy:
  enabled: true
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              name: production
```

## CI/CD Integration

### GitHub Actions
```yaml
- name: Deploy to Production
  run: |
    task ci:setup
    task validate
    task build
    task deploy:k8s ENV=production
```

### GitLab CI
```yaml
deploy:
  script:
    - task ci:deploy ENV=$CI_ENVIRONMENT_NAME
  environment:
    name: production
```

## Troubleshooting

### Common Issues

#### Build Failures
```bash
# Fix module path issues
task fix:module-paths

# Clean and rebuild
task clean build
```

#### No Metrics Appearing
```bash
# Check collector status
task health-check

# Validate configuration
task validate:config

# Check logs for errors
task dev:logs | grep ERROR
```

#### Database Connection Issues
```bash
# Test connections
task test:connections

# Check with specific database
task test:postgres
task test:mysql
```

### Debug Mode
```bash
# Run with debug logging
task run:debug

# Enable all debug endpoints
task dev:debug
```

## Migration from Old Infrastructure

### From Makefile to Taskfile
| Old Command | New Command |
|-------------|-------------|
| `make build` | `task build` |
| `make test` | `task test` |
| `make docker-build` | `task docker:build` |
| `make install-tools` | `task setup` |

### From Shell Scripts
| Old Script | New Task |
|------------|----------|
| `./scripts/build.sh` | `task build` |
| `./scripts/deploy.sh` | `task deploy:k8s` |
| `./scripts/test.sh` | `task test` |
| `./quickstart.sh` | `task quickstart` |

### From Raw Docker Commands
```bash
# Old way
docker build -t collector . && docker run -d collector

# New way
task docker:build docker:run
```

## Advanced Deployment Scenarios

### Multi-Region Deployment
```bash
# Deploy to multiple regions with Helm
for region in us-east-1 us-west-2 eu-west-1; do
  task deploy:helm ENV=production REGION=$region
done
```

### Blue-Green Deployment
```bash
# Deploy green version
task deploy:helm RELEASE=db-intel-green VERSION=v2.0.0

# Switch traffic
task deploy:switch-traffic FROM=blue TO=green

# Cleanup old version
task deploy:cleanup RELEASE=db-intel-blue
```

### Canary Deployment
```bash
# Deploy canary with 10% traffic
task deploy:canary VERSION=v2.0.0 WEIGHT=10

# Gradually increase traffic
task deploy:canary-promote WEIGHT=50
task deploy:canary-promote WEIGHT=100
```

## Performance Tuning

### Batch Processing
```yaml
# Optimize for throughput
processors:
  batch:
    timeout: 10s
    send_batch_size: 5000
    send_batch_max_size: 10000
```

### Memory Management
```yaml
# Configure memory limits
processors:
  memory_limiter:
    check_interval: 1s
    limit_percentage: 75
    spike_limit_percentage: 20
```

### Sampling Configuration
```yaml
# Adaptive sampling for high volume
processors:
  experimental:
    adaptiveSampler:
      default_sampling_rate: 0.1
      rules:
        - name: slow_queries
          condition: "db_query_duration > 5000"
          sampling_rate: 1.0
```

## Monitoring and Alerting

### New Relic Dashboards
```bash
# Import dashboard
task monitoring:import-dashboard

# Dashboard available at:
# monitoring/newrelic/dashboards/database-intelligence-overview.json
```

### Alert Configuration
```bash
# Apply alert policies
task monitoring:setup-alerts

# Key alerts:
# - Collector Down
# - High Memory Usage
# - Database Connection Failed
# - Slow Query Performance
```

## Production Checklist

### Pre-Deployment
- [ ] Task installed and configured
- [ ] Environment files created for each environment
- [ ] Database users created with correct permissions
- [ ] Network connectivity verified
- [ ] TLS certificates configured (production)

### Deployment
- [ ] Use appropriate deployment method (Binary/Docker/Helm)
- [ ] Configure resource limits
- [ ] Enable security features
- [ ] Set up monitoring

### Post-Deployment
- [ ] Verify health endpoints
- [ ] Check metrics in New Relic
- [ ] Configure alerts
- [ ] Document deployment

## Support

For detailed task information:
```bash
# List all available tasks
task --list-all

# Get help for specific task
task help deploy:helm

# View task details
task --summary deploy:k8s
```

For more information, see:
- [Taskfile Usage Guide](TASKFILE_USAGE.md)
- [Configuration Guide](CONFIGURATION.md)
- [Troubleshooting Guide](TROUBLESHOOTING.md)