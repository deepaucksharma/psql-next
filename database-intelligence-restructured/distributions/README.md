# Distributions Directory

This directory contains the different distribution builds of the Database Intelligence Collector.

## Available Distributions

### 1. Minimal Distribution (`minimal/`)
- **Purpose**: Lightweight deployment with standard components only
- **Components**: Standard OTel receivers, processors, exporters
- **Use Case**: Resource-constrained environments
- **Size**: ~50MB
- **Build**: `make build-minimal`

### 2. Production Distribution (`production/`)
- **Purpose**: Full-featured production deployment
- **Components**: All standard + custom components
- **Use Case**: Production environments
- **Size**: ~120MB
- **Build**: `make build`

### 3. Enterprise Distribution (`enterprise/`)
- **Purpose**: Enterprise features with advanced monitoring
- **Components**: All components + enterprise features
- **Use Case**: Large-scale deployments
- **Size**: ~150MB
- **Build**: `make build-enterprise`

## Directory Structure

```
distributions/
├── minimal/
│   ├── main.go              # Minimal entry point
│   ├── go.mod               # Minimal dependencies
│   └── go.sum
├── production/
│   ├── main.go              # Production entry point
│   ├── components.go        # Standard components
│   ├── components_enhanced.go # Enhanced components
│   ├── go.mod
│   └── go.sum
└── enterprise/
    ├── main.go              # Enterprise entry point
    ├── components.go        # All components
    ├── go.mod
    └── go.sum
```

## Building Distributions

### Using Make (Recommended)

```bash
# Build all distributions
make build-all

# Build specific distribution
make build-minimal
make build
make build-enterprise
```

### Using OTel Builder

```bash
# Minimal distribution
builder --config=otelcol-builder-config-minimal.yaml

# Production distribution
builder --config=otelcol-builder-config-enhanced.yaml

# Enterprise distribution
builder --config=otelcol-builder-config-enterprise.yaml
```

### Direct Go Build

```bash
# Minimal
cd distributions/minimal && go build .

# Production
cd distributions/production && go build .

# Enterprise
cd distributions/enterprise && go build .
```

## Component Comparison

| Feature | Minimal | Production | Enterprise |
|---------|---------|------------|------------|
| PostgreSQL Receiver | ✅ | ✅ | ✅ |
| MySQL Receiver | ✅ | ✅ | ✅ |
| SQL Query Receiver | ✅ | ✅ | ✅ |
| OTLP Receiver | ✅ | ✅ | ✅ |
| ASH Receiver | ❌ | ✅ | ✅ |
| Enhanced SQL Receiver | ❌ | ✅ | ✅ |
| Kernel Metrics Receiver | ❌ | ✅ | ✅ |
| Memory Limiter | ✅ | ✅ | ✅ |
| Batch Processor | ✅ | ✅ | ✅ |
| Resource Processor | ✅ | ✅ | ✅ |
| Adaptive Sampler | ❌ | ✅ | ✅ |
| Circuit Breaker | ❌ | ✅ | ✅ |
| Plan Extractor | ❌ | ✅ | ✅ |
| Query Correlator | ❌ | ✅ | ✅ |
| Cost Control | ❌ | ✅ | ✅ |
| PII Detection | ❌ | ✅ | ✅ |
| OTLP Exporters | ✅ | ✅ | ✅ |
| NR Error Monitor | ❌ | ✅ | ✅ |
| Health Check | ✅ | ✅ | ✅ |
| pprof | ❌ | ✅ | ✅ |
| zPages | ❌ | ✅ | ✅ |

## Resource Requirements

### Minimal Distribution
- **CPU**: 0.5 cores
- **Memory**: 256MB - 512MB
- **Disk**: 10GB

### Production Distribution
- **CPU**: 2 cores
- **Memory**: 1GB - 2GB
- **Disk**: 50GB

### Enterprise Distribution
- **CPU**: 4 cores
- **Memory**: 2GB - 4GB
- **Disk**: 100GB

## Configuration

All distributions use the same configuration structure but support different components:

### Minimal Configuration
```yaml
# Use configs/modes/config-only.yaml
# Only standard components available
```

### Production/Enterprise Configuration
```yaml
# Use configs/modes/enhanced.yaml
# All custom components available
```

## Deployment Examples

### Docker

```bash
# Minimal
docker build -f deployments/docker/Dockerfile.standard -t db-intel:minimal .
docker run -d --name db-intel-minimal db-intel:minimal

# Production
docker build -f deployments/docker/Dockerfile.production -t db-intel:prod .
docker run -d --name db-intel-prod db-intel:prod

# Enterprise
docker build -f deployments/docker/Dockerfile.enterprise -t db-intel:enterprise .
docker run -d --name db-intel-enterprise db-intel:enterprise
```

### Binary

```bash
# Build
make build

# Run
./distributions/production/database-intelligence-collector \
  --config=configs/modes/enhanced.yaml
```

## Troubleshooting

### Build Failures
- Ensure Go 1.22 is installed
- Run `go work sync` to update dependencies
- Check `go.work` includes all required modules

### Missing Components
- Verify you're using the correct distribution
- Check component registration in `components.go`
- Ensure builder config includes all components

### Runtime Errors
- Verify configuration matches distribution capabilities
- Check logs for component initialization errors
- Ensure all required environment variables are set