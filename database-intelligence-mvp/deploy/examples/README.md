# Database Intelligence MVP - Deployment Examples

This directory contains simplified deployment examples that showcase the **OTEL-first architecture** for the Database Intelligence MVP.

## Quick Start

### Simple Docker Setup (Recommended for Development)
```bash
cd deploy/examples
export NEW_RELIC_LICENSE_KEY="your-license-key"
docker-compose -f docker-compose-simple.yaml up -d
```

### Production Docker Setup
```bash
cd deploy/examples
export NEW_RELIC_LICENSE_KEY="your-license-key"
docker-compose -f docker-compose-production.yaml up -d
```

### Kubernetes Minimal
```bash
kubectl apply -f kubernetes-minimal.yaml
```

### Kubernetes Production
```bash
kubectl apply -f kubernetes-production.yaml
```

## Files Overview

| File | Description | Use Case |
|------|-------------|----------|
| `docker-compose-simple.yaml` | Minimal setup with PostgreSQL | Development, Testing |
| `docker-compose-production.yaml` | Full production setup | Production with HA |
| `kubernetes-minimal.yaml` | Basic K8s deployment | Simple K8s environments |
| `kubernetes-production.yaml` | Enterprise K8s deployment | Production K8s with scaling |
| `deployment-guide.md` | Comprehensive deployment guide | Complete instructions |

## Architecture

### OTEL-First Approach
- **Standard OTEL Receivers**: PostgreSQL, MySQL, SQL Query
- **Custom Processors**: Only for specific gaps (adaptive sampling, circuit breaker)
- **Standard Exporters**: OTLP to New Relic, Prometheus for local metrics

### Key Features
- Health checks and monitoring endpoints
- PII sanitization and data protection
- Proper environment variable configuration
- Clear comments explaining each section
- Production-ready security and scaling

## Configuration Files

### Collector Configurations
- `configs/collector-simple.yaml` - Basic OTEL configuration
- `configs/collector-production.yaml` - Full production configuration

### Database Setup
- `init-scripts/postgres-simple-init.sql` - Basic PostgreSQL setup
- `init-scripts/postgres-monitoring-setup.sql` - Monitoring user setup

### Dependencies
Each example includes all necessary configuration files and initialization scripts.

## Getting Help

For detailed instructions, see `deployment-guide.md` in this directory.

For issues or questions, check the main project documentation.