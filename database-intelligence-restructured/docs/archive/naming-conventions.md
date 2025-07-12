# Naming Conventions

This document defines the naming conventions used throughout the Database Intelligence project.

## Standard Name

The project uses **`database-intelligence`** as the standard name (with hyphen, all lowercase).

## Usage Guidelines

### Binary Names
- Collector binary: `database-intelligence-collector`
- Builder output: `database-intelligence-collector`

### Docker Images
- Image name: `database-intelligence:tag`
- Registry path: `ghcr.io/[org]/database-intelligence:tag`

### Kubernetes Resources
- Deployment: `database-intelligence`
- Service: `database-intelligence`
- ConfigMap: `database-intelligence-config`
- Secret: `database-intelligence-secrets`

### Service Names
- OpenTelemetry service.name: `database-intelligence-collector`
- Prometheus namespace: `database_intelligence` (underscore for Prometheus compatibility)

### Environment Variables
- Use underscores: `DATABASE_INTELLIGENCE_*`
- Example: `DATABASE_INTELLIGENCE_VERSION`

### Go Modules
- Module path: `github.com/[org]/database-intelligence`
- Package names: `databaseintelligence` (no hyphen in Go packages)

### File Names
- Config files: `database-intelligence-*.yaml`
- Scripts: `database-intelligence-*.sh`

## Deprecated Names

The following naming patterns are deprecated and should not be used:
- `db-otel`
- `db-intel`
- `database_intelligence` (except in Prometheus metrics)
- `databaseIntelligence` (except in Go code where required)
