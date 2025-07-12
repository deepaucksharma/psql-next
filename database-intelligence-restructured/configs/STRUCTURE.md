# Configuration Structure

This directory contains all OpenTelemetry Collector configurations organized by purpose and environment.

## Directory Organization

### `base/` - Component Definitions
Modular component configurations that can be composed:
- `receivers.yaml` - All receiver configurations  
- `processors.yaml` - All processor configurations
- `exporters.yaml` - All exporter configurations
- `extensions.yaml` - All extension configurations

### `modes/` - Operating Modes
Complete collector configurations for different operational modes:
- `config-only.yaml` - Uses only standard OpenTelemetry components
- `enhanced.yaml` - Includes custom Database Intelligence components

### `environments/` - Environment Overlays
Environment-specific overrides and settings:
- `development.yaml` - Development environment settings
- `staging.yaml` - Staging environment configuration  
- `production.yaml` - Production optimizations and settings

### `examples/` - Working Examples
Tested configurations for specific use cases:
- `config-only-base.yaml` - Basic config-only setup
- `config-only-mysql.yaml` - MySQL-specific configuration
- `config-only-working.yaml` - Proven working config-only setup
- `newrelic-alerts.yaml` - New Relic alerting configuration
- `runtime-collector-config.yaml` - Runtime configuration example

### `archive/` - Legacy Configurations
Historical configurations preserved for reference:
- Previous versions and deprecated configurations
- Test configurations from development phases

## Usage Patterns

### Development
```bash
# Use base configuration with development overlay
./collector --config=configs/modes/enhanced.yaml \
           --config=configs/environments/development.yaml
```

### Production (Recommended)
```bash
# Use config-only mode for production stability
./collector --config=configs/modes/config-only.yaml \
           --config=configs/environments/production.yaml
```

### Testing
```bash
# Use specific example configurations
./collector --config=configs/examples/config-only-working.yaml
```

## Configuration Hierarchy

1. **Base Mode**: `modes/config-only.yaml` OR `modes/enhanced.yaml`
2. **Environment Overlay**: `environments/{env}.yaml`
3. **Custom Overrides**: Additional configs as needed

This structure provides maximum flexibility while maintaining clear separation of concerns.