# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

### Build and Run
```bash
# Build the collector binary
make build

# Build for all platforms
make build-all

# Run with default config
make run

# Run in debug mode with verbose logging
make run-debug

# Quick development cycle (build + run)
make dev-run
```

### Testing
```bash
# Run unit tests
make test

# Run integration tests
make test-integration

# Run end-to-end tests
make test-e2e

# Run all tests with coverage
make test-coverage

# Run OTLP compliance tests
make test-otlp

# Quick test for development (unit tests only)
make quick-test

# Full test suite
make full-test

# Run a single test
go test -v -run TestName ./path/to/package
```

### Code Quality
```bash
# Format code
make fmt

# Run linters
make lint

# Security checks
make security

# Run all development checks (fmt, lint, security)
make dev

# Pre-commit checks
make pre-commit
```

### Docker Operations
```bash
# Build Docker image
make docker-build

# Start test environment
make docker-up

# View logs
make docker-logs

# Stop environment
make docker-down

# Clean Docker resources
make docker-clean
```

## Architecture Overview

### Project Structure
This is an OpenTelemetry Collector distribution specialized for database monitoring (PostgreSQL and MySQL) with New Relic integration.

### Two Operating Modes
1. **Config-Only Mode**: Uses standard OpenTelemetry components configured via YAML
   - Minimal resource usage (<5% CPU, <512MB memory)
   - No custom code required
   - Production-ready configurations in `configs/examples/`

2. **Enhanced Mode**: Includes 7 custom processors for advanced features
   - Located in `components/processors/`
   - Provides query intelligence, adaptive sampling, circuit breaking, cost control
   - Higher resource usage (<20% CPU, <2GB memory)

### Key Components Architecture

#### Custom Processors (`components/processors/`)
- **adaptivesampler**: Dynamic sampling based on system load
- **circuitbreaker**: 3-state FSM to protect databases from overload
- **planattributeextractor**: Extracts query plans using pg_querylens
- **verificationprocessor**: PII detection and data quality checks
- **costcontrol**: Enforces New Relic budget limits
- **nrerrormonitor**: Proactive error detection and alerting
- **querycorrelator**: Links queries to sessions and transactions

#### Custom Receivers (`components/receivers/`)
- **ashreceiver**: Active Session History monitoring
- **enhancedsqlreceiver**: Extended SQL metrics collection
- **kernelmetricsreceiver**: OS-level kernel metrics

#### Distributions (`distributions/`)
- **minimal**: Lightweight for resource-constrained environments
- **enterprise**: Full-featured with all custom components
- **production**: Optimized for production deployments

### Configuration Flow
1. Base configurations in `configs/examples/`
2. Environment overlays in `configs/overlays/` (dev, staging, prod)
3. Runtime configuration via environment variables
4. Validation using `make validate-config`

### Testing Architecture
- **Unit tests**: Component-level testing with mocks
- **Integration tests**: Cross-component interaction testing
- **E2E tests**: Full pipeline testing with real databases (`tests/e2e/`)
- **Performance tests**: Load and optimization testing
- Test framework located in `tests/e2e/framework/`

### Key Design Decisions
1. **Zero-Persistence**: All state maintained in memory
2. **Defense in Depth**: Multiple protection layers (memory limiter, circuit breaker, sampling)
3. **Graceful Degradation**: Features fail gracefully without affecting core functionality
4. **OpenTelemetry-First**: Leverages standard components wherever possible

### Integration Points
- **Databases**: PostgreSQL 11+ and MySQL 5.7+ via SQL receivers
- **New Relic**: OTLP export with API key authentication
- **Monitoring**: Prometheus metrics on :8888, health checks on :13133
- **Deployment**: Docker, Kubernetes (Helm), or binary deployment

### Performance Considerations
- Processing latency target: <5ms per metric
- Memory usage scales with number of unique queries
- Circuit breaker activates at 80% error rate
- Adaptive sampler adjusts based on CPU/memory usage

### Security Model
- Read-only database credentials required
- Environment variables for all secrets
- TLS for database connections
- PII detection and redaction in verification processor
- No persistent storage of sensitive data