# Getting Started

## Prerequisites
- Docker and Docker Compose v2
- New Relic account with license key

## Quick Start

### 1. Clone and Setup
```bash
git clone <repository>
cd database-intelligence-mysql
cp .env.example .env
# Edit .env with your New Relic credentials
```

### 2. Deploy with Docker Compose
```bash
# Using setup script (recommended)
./deploy/setup.sh

# Or manually
docker compose up -d
```

### 3. Deploy with Quick Start Script
```bash
# Advanced mode (default - all features)
./deploy/deploy.sh

# With sample workload
./deploy/deploy.sh --with-workload

# Different deployment modes
DEPLOYMENT_MODE=minimal ./deploy/deploy.sh   # Basic metrics only
DEPLOYMENT_MODE=standard ./deploy/deploy.sh  # Production mode
DEPLOYMENT_MODE=debug ./deploy/deploy.sh     # Debug mode
```

## Verify Installation

### Check Connections
```bash
./operate/test-connection.sh
```

### Validate Metrics
```bash
./operate/validate-metrics.sh
```

### Run Diagnostics
```bash
./operate/diagnose.sh
```

## Generate Test Data
```bash
# Continuous workload
./operate/generate-workload.sh

# Full test suite
./operate/full-test.sh
```

## View Metrics

### Local Endpoints
- Health: http://localhost:13133/
- Metrics: http://localhost:8888/metrics
- Prometheus: http://localhost:8889/metrics
- Debug: http://localhost:55679/debug/pipelinez

### New Relic
1. Go to [New Relic One](https://one.newrelic.com)
2. Navigate to Metrics Explorer
3. Query: `FROM Metric SELECT * WHERE instrumentation.provider = 'opentelemetry'`

## Import Dashboards
1. Open New Relic Dashboards
2. Import JSON from `config/newrelic/dashboards.json`

## Next Steps
- Review [Configuration Guide](configuration.md)
- Check [Operational Procedures](operations.md)
- See [Troubleshooting](troubleshooting.md)