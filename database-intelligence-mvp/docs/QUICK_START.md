# Quick Start Guide

Get the Database Intelligence Collector running in minutes using our new Taskfile automation system.

## Prerequisites

- Docker and Docker Compose
- Go 1.21+ (for custom builds)
- Task (build automation tool)
- New Relic account with license key

## Install Task

```bash
# macOS
brew install go-task/tap/go-task

# Linux
sh -c "$(curl --location https://taskfile.dev/install.sh)" -- -d -b ~/.local/bin

# Windows (PowerShell)
iwr -useb https://taskfile.dev/install.ps1 | iex
```

## Quick Start Options

### Option 1: Fastest Start (Standard Mode)

```bash
# Clone repository
git clone https://github.com/database-intelligence-mvp/database-intelligence-mvp.git
cd database-intelligence-mvp

# Set up environment
cp .env.example .env
# Edit .env with your credentials

# Start everything
task quickstart
```

This will:
1. Install dependencies
2. Fix common setup issues
3. Build the collector
4. Start PostgreSQL and MySQL databases
5. Begin collecting metrics

### Option 2: Production-Like Setup

```bash
# Set up production config
cp .env.example .env.production
# Edit with production credentials

# Deploy with Helm
task deploy:helm ENV=production
```

### Option 3: Development with Hot Reload

```bash
# Start development environment
task dev:up

# In another terminal, start watch mode
task dev:watch
```

## Verify Installation

```bash
# Check collector health
task health-check

# View metrics
curl http://localhost:8888/metrics

# Check logs
task dev:logs
```

## Common Tasks

```bash
# Stop everything
task dev:down

# Reset and start fresh
task dev:reset

# Run tests
task test

# Build custom collector
task build MODE=experimental
```

## Environment Configuration

### Required Environment Variables

```bash
# Database connections
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_USER=monitoring_user
POSTGRES_PASSWORD=secure_password
POSTGRES_DB=postgres

# New Relic
NEW_RELIC_LICENSE_KEY=your_license_key_here
NEW_RELIC_OTLP_ENDPOINT=https://otlp.nr-data.net:4317

# Environment
ENVIRONMENT=development
```

### Using Different Environments

```bash
# Development
task dev:up

# Staging
task run CONFIG_ENV=staging

# Production
task run CONFIG_ENV=production
```

## Deployment Options

### Docker Compose

```bash
# Start all services
task dev:up

# Start specific profile
docker-compose --profile monitoring up -d
```

### Kubernetes with Helm

```bash
# Install
task deploy:helm

# Upgrade
task deploy:update

# Uninstall
helm uninstall db-intelligence
```

### Standalone Binary

```bash
# Build
task build

# Run
./dist/otelcol --config=configs/collector.yaml
```

## Monitoring in New Relic

Once running, your metrics will appear in New Relic:

1. Log into New Relic
2. Navigate to **Infrastructure > Third-party services**
3. Look for "OpenTelemetry"
4. Or use Query Builder:

```sql
SELECT * FROM Metric 
WHERE otel.library.name = 'otelcol/postgresqlreceiver'
SINCE 5 minutes ago
```

## Troubleshooting

### No metrics appearing?

```bash
# Check collector status
task health-check

# Validate configuration
task validate:config

# Check database connectivity
task test:connections
```

### Build failures?

```bash
# Fix common issues
task fix:all

# Clean and rebuild
task clean build
```

### Permission errors?

```bash
# Fix file permissions
task fix:permissions
```

## Next Steps

- Read the full [Taskfile Usage Guide](TASKFILE_USAGE.md)
- Configure [environment-specific settings](../configs/overlays/README.md)
- Review [architecture documentation](ARCHITECTURE.md)
- Set up [CI/CD pipeline](.github/workflows/ci.yml)

## Getting Help

```bash
# List all available tasks
task --list-all

# Get help for specific task
task help deploy:helm

# View task details
cat Taskfile.yml
```

For detailed documentation, see [TASKFILE_USAGE.md](TASKFILE_USAGE.md).