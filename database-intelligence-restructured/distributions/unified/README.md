# Unified Distribution

A single, parameterized distribution of the Database Intelligence Collector that supports multiple profiles through build flags and runtime configuration.

## Profiles

### Minimal Profile
- Lightweight deployment with standard OpenTelemetry components only
- Suitable for resource-constrained environments
- ~50MB binary size

### Standard Profile (Default)
- Full-featured deployment with all custom database intelligence components
- Recommended for production environments
- ~120MB binary size

### Enterprise Profile
- Includes all standard features plus enterprise capabilities
- For large-scale deployments
- ~150MB binary size

## Building

### Build All Profiles
```bash
make build-unified
```

### Build Specific Profile
```bash
# Minimal
go build -tags minimal -o db-intel-minimal

# Standard (default)
go build -o db-intel-standard

# Enterprise
go build -tags enterprise -o db-intel-enterprise
```

## Running

### With Profile Flag
```bash
# Run with minimal profile
./database-intelligence-collector --profile=minimal --config=config.yaml

# Run with standard profile (default)
./database-intelligence-collector --config=config.yaml

# Run with enterprise profile
./database-intelligence-collector --profile=enterprise --config=config.yaml
```

### Show Version
```bash
./database-intelligence-collector --version
```

## Configuration

All profiles use the same configuration structure but support different components:

- **Minimal**: Use `configs/profiles/minimal.yaml`
- **Standard**: Use `configs/profiles/standard.yaml`
- **Enterprise**: Use `configs/profiles/enterprise.yaml`

## Migration from Legacy Distributions

If you're migrating from the old separate distributions:

1. **From `distributions/minimal/`** → Use `--profile=minimal`
2. **From `distributions/production/`** → Use `--profile=standard` (default)
3. **From `distributions/enterprise/`** → Use `--profile=enterprise`

## Benefits

- **Single Codebase**: One distribution to maintain instead of three
- **Reduced Duplication**: ~60% less code duplication
- **Flexible Deployment**: Choose profile at runtime
- **Simplified CI/CD**: One build pipeline for all profiles
- **Consistent Behavior**: Same binary with different feature sets