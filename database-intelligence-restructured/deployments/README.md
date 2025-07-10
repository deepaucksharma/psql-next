# Database Intelligence Deployments

This directory contains all deployment configurations for the Database Intelligence project.

## Directory Structure

```
deployments/
├── docker/           # Docker-related deployments
│   ├── compose/      # Docker Compose configurations
│   │   ├── docker-compose.yaml       # Default development setup
│   │   ├── docker-compose.prod.yaml  # Production deployment
│   │   ├── docker-compose.test.yaml  # Testing environment
│   │   ├── docker-compose-databases.yaml  # Database-only setup
│   │   └── docker-compose-ha.yaml    # High availability setup
│   ├── dockerfiles/  # Dockerfile definitions
│   │   ├── Dockerfile              # Main collector image
│   │   ├── Dockerfile.custom       # Custom build
│   │   ├── Dockerfile.loadgen      # Load generator
│   │   └── Dockerfile.test         # Test runner
│   └── init-scripts/ # Database initialization scripts
├── kubernetes/       # Kubernetes manifests
│   ├── base/         # Base Kustomize configurations
│   └── overlays/     # Environment-specific overlays
└── helm/             # Helm charts
    └── database-intelligence/  # Main Helm chart
```

## Quick Start

### Docker Compose

1. **Development Environment**:
   ```bash
   docker-compose -f deployments/docker/compose/docker-compose.yaml up
   ```

2. **Production Deployment**:
   ```bash
   docker-compose -f deployments/docker/compose/docker-compose.prod.yaml up -d
   ```

3. **Running Tests**:
   ```bash
   docker-compose -f deployments/docker/compose/docker-compose.test.yaml run tests
   ```

### Kubernetes

1. **Deploy with kubectl**:
   ```bash
   kubectl apply -k deployments/kubernetes/base/
   ```

2. **Deploy to specific environment**:
   ```bash
   # Development
   kubectl apply -k deployments/kubernetes/overlays/dev/
   
   # Production
   kubectl apply -k deployments/kubernetes/overlays/production/
   ```

### Helm

1. **Install with Helm**:
   ```bash
   helm install database-intelligence deployments/helm/database-intelligence/
   ```

2. **Upgrade deployment**:
   ```bash
   helm upgrade database-intelligence deployments/helm/database-intelligence/
   ```

## Environment Configuration

All deployments support configuration through environment variables. See `configs/templates/environment-template.env` for available options.

### Required Variables
- `DB_USERNAME` - Database username
- `DB_PASSWORD` - Database password
- `NEW_RELIC_LICENSE_KEY` - New Relic API key (if using New Relic export)

### Optional Variables
- `OTEL_LOG_LEVEL` - Logging level (default: info)
- `ENABLE_PROFILING` - Enable performance profiling (default: false)
- `METRIC_INTERVAL` - Metric collection interval (default: 10s)

## Docker Images

### Building Images

```bash
# Build main collector image
docker build -f deployments/docker/dockerfiles/Dockerfile -t database-intelligence:latest .

# Build with custom configuration
docker build -f deployments/docker/dockerfiles/Dockerfile.custom -t database-intelligence:custom .
```

### Multi-platform Builds

```bash
# Build for multiple platforms
docker buildx build --platform linux/amd64,linux/arm64 \
  -f deployments/docker/dockerfiles/Dockerfile \
  -t database-intelligence:latest .
```

## Production Considerations

1. **Resource Limits**: Set appropriate CPU and memory limits
2. **Persistence**: Mount volumes for data persistence
3. **Security**: Use secrets for sensitive configuration
4. **Monitoring**: Enable health checks and metrics export
5. **Scaling**: Use horizontal pod autoscaling for Kubernetes

## Troubleshooting

See [Operations Guide](../docs/operations/deployment.md) for detailed troubleshooting steps.
