# Development Setup

This guide covers setting up your development environment for Database Intelligence.

## Prerequisites

- Go 1.21 or later
- Docker and Docker Compose
- PostgreSQL 12+ (for testing)
- Git
- Make

## Clone Repository

```bash
git clone https://github.com/newrelic/database-intelligence
cd database-intelligence
```

## Environment Setup

### 1. Install Go Dependencies

```bash
# Install all dependencies
go mod download

# Verify installation
go mod verify
```

### 2. Install Development Tools

```bash
# Install golangci-lint for code quality
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin

# Install OpenTelemetry Collector Builder
go install go.opentelemetry.io/collector/cmd/builder@latest

# Install goreleaser for releases
go install github.com/goreleaser/goreleaser@latest
```

### 3. Set Up PostgreSQL

```bash
# Using Docker
docker run -d \
  --name postgres-dev \
  -e POSTGRES_PASSWORD=postgres \
  -p 5432:5432 \
  postgres:15

# Create test database
docker exec postgres-dev psql -U postgres -c "CREATE DATABASE testdb;"

# Enable required extensions
docker exec postgres-dev psql -U postgres -d testdb -c "CREATE EXTENSION IF NOT EXISTS pg_stat_statements;"
```

### 4. Configure Environment

Create a `.env` file:

```bash
# New Relic
export NEW_RELIC_LICENSE_KEY="your-dev-key"
export NEW_RELIC_ACCOUNT_ID="your-account-id"

# PostgreSQL
export POSTGRES_HOST="localhost"
export POSTGRES_PORT="5432"
export POSTGRES_USER="postgres"
export POSTGRES_PASSWORD="postgres"
export POSTGRES_DB="testdb"

# Development
export OTEL_SERVICE_NAME="db-intel-dev"
export DEPLOYMENT_MODE="development"
export LOG_LEVEL="debug"
```

## Building

### Build All Components

```bash
# Build everything
make build

# Build specific distribution
make build-enterprise
make build-production
make build-development
```

### Build Custom Collector

```bash
# Generate collector code
make generate

# Build collector binary
make collector

# Build Docker image
make docker-build
```

### Build Individual Components

```bash
# Build receivers
cd components/receivers/ash
go build -v ./...

# Build processors
cd components/processors/adaptivesampler
go build -v ./...

# Build tools
cd tools/load-generator
go build -o load-generator main.go
```

## Running Locally

### 1. Run Config-Only Mode

```bash
# Using standard OTel collector
docker run -d \
  --name otel-dev \
  -v $(pwd)/configs/config-only-mode.yaml:/config.yaml \
  --env-file .env \
  --network host \
  otel/opentelemetry-collector-contrib:latest \
  --config=/config.yaml
```

### 2. Run Custom Mode

```bash
# Build first
make build-enterprise

# Run
./bin/database-intelligence-enterprise \
  --config=configs/custom-mode.yaml
```

### 3. Run with Docker Compose

```bash
cd deployments/docker/compose
docker-compose -f docker-compose-parallel.yaml up
```

## Testing

### Unit Tests

```bash
# Run all tests
make test

# Run with coverage
make test-coverage

# Run specific package tests
go test -v ./components/receivers/ash/...

# Run with race detection
go test -race ./...
```

### Integration Tests

```bash
# Set up test environment
make test-setup

# Run integration tests
make test-integration

# Run E2E tests
make test-e2e
```

### Load Testing

```bash
# Build load generator
cd tools/load-generator
go build

# Run load test
./load-generator -pattern=mixed -qps=100
```

## Code Quality

### Linting

```bash
# Run linters
make lint

# Run specific linter
golangci-lint run ./components/...

# Auto-fix issues
golangci-lint run --fix
```

### Formatting

```bash
# Format code
make fmt

# Check formatting
make fmt-check
```

### Security Scanning

```bash
# Run security scan
make security-scan

# Check dependencies
go list -m all | nancy sleuth
```

## Debugging

### Enable Debug Logging

```yaml
# In your config file
service:
  telemetry:
    logs:
      level: debug
      development: true
      encoding: console
```

### Use Debug Exporter

```yaml
exporters:
  debug:
    verbosity: detailed
    sampling_initial: 10

service:
  pipelines:
    metrics:
      exporters: [debug, otlp]
```

### Remote Debugging

```bash
# Build with debug symbols
go build -gcflags="all=-N -l" -o collector ./distributions/enterprise

# Run with delve
dlv exec ./collector -- --config=configs/custom-mode.yaml
```

### Performance Profiling

```yaml
# Enable pprof extension
extensions:
  pprof:
    endpoint: 0.0.0.0:1777

service:
  extensions: [pprof]
```

Access profiles at `http://localhost:1777/debug/pprof/`

## Development Workflow

### 1. Create Feature Branch

```bash
git checkout -b feature/your-feature
```

### 2. Make Changes

Follow the coding standards:
- Write tests for new code
- Update documentation
- Add metrics where appropriate

### 3. Test Changes

```bash
# Run tests
make test

# Run linter
make lint

# Test locally
make run-dev
```

### 4. Commit Changes

```bash
# Stage changes
git add .

# Commit with conventional commit message
git commit -m "feat(ash): add query plan extraction"
```

### 5. Push and Create PR

```bash
git push origin feature/your-feature
```

## Project Structure

```
database-intelligence/
├── cmd/                    # Command line tools
├── components/            # Custom OTel components
│   ├── receivers/         # Data collection
│   ├── processors/        # Data processing
│   └── exporters/         # Data export
├── configs/               # Configuration examples
├── deployments/           # Deployment configurations
├── distributions/         # Collector distributions
├── docs/                  # Documentation
├── internal/              # Internal packages
├── pkg/                   # Public packages
├── scripts/               # Build and deploy scripts
├── tests/                 # Test suites
└── tools/                 # Development tools
```

## Common Tasks

### Add a New Metric

1. Define metric in receiver
2. Add to configuration
3. Update documentation
4. Add tests
5. Update dashboard

### Add a New Component

1. Create component directory
2. Implement factory
3. Add to distribution
4. Write tests
5. Document configuration

### Update Dependencies

```bash
# Update all dependencies
go get -u ./...

# Update specific dependency
go get -u github.com/open-telemetry/opentelemetry-collector-contrib

# Tidy modules
go mod tidy
```

## Troubleshooting Development

### Build Errors

```bash
# Clean build cache
go clean -cache
go clean -modcache

# Rebuild
make clean build
```

### Test Failures

```bash
# Run specific test with verbose output
go test -v -run TestName ./package/...

# Check test database
docker exec postgres-dev psql -U postgres -d testdb
```

### Module Issues

```bash
# Verify modules
go mod verify

# Download missing modules
go mod download

# Remove unused modules
go mod tidy
```

## IDE Setup

### VS Code

```json
// .vscode/settings.json
{
  "go.useLanguageServer": true,
  "go.lintTool": "golangci-lint",
  "go.lintFlags": ["--fast"],
  "go.testFlags": ["-v"],
  "go.testTimeout": "30s",
  "go.buildTags": "integration"
}
```

### GoLand

1. Set Go SDK to 1.21+
2. Enable Go Modules
3. Configure golangci-lint
4. Set test timeout to 30s

## Resources

- [OpenTelemetry Collector Docs](https://opentelemetry.io/docs/collector/)
- [Go Style Guide](https://google.github.io/styleguide/go/)
- [PostgreSQL Docs](https://www.postgresql.org/docs/)
- [New Relic Docs](https://docs.newrelic.com/)