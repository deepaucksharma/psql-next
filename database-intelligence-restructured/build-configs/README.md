# Build Configurations

This directory contains OpenTelemetry Collector Builder configurations for different distributions.

## Available Configurations

### 1. Minimal Build (`minimal.yaml`)
- Standard OpenTelemetry components only
- Smallest binary size
- No custom components

### 2. Enhanced Build (`enhanced.yaml`)
- All custom components included
- Full feature set
- Production-ready

### 3. Complete Build (`complete.yaml`)
- Legacy configuration (deprecated)
- Use `enhanced.yaml` instead

## Usage

### Building with Make

```bash
# Uses the appropriate config automatically
make build          # Uses enhanced.yaml
make build-minimal  # Uses minimal.yaml
```

### Direct Builder Usage

```bash
# Install builder
go install go.opentelemetry.io/collector/cmd/builder@v0.105.0

# Build minimal
builder --config=build-configs/minimal.yaml

# Build enhanced
builder --config=build-configs/enhanced.yaml
```

## Configuration Structure

Each builder configuration specifies:

1. **Distribution Settings**
   - Name and description
   - Output path
   - Version information

2. **Components**
   - Receivers
   - Processors
   - Exporters
   - Extensions
   - Connectors

3. **Replace Directives**
   - Local module paths
   - Version overrides

## Adding New Components

To add a new component:

1. Add the component to the appropriate section
2. Specify the module path and version
3. If it's a local component, add a replace directive

Example:
```yaml
receivers:
  - gomod: github.com/example/myreceiver v1.0.0
    # For local development:
    path: ./components/receivers/myreceiver
```

## Version Management

All OpenTelemetry components should use consistent versions:
- Core components: `v0.105.0`
- pdata: `v1.12.0`
- Contrib components: `v0.105.0`

## Troubleshooting

### Build Failures
- Check Go version (requires 1.22)
- Verify all local paths exist
- Ensure network access for remote modules

### Component Conflicts
- Check for version mismatches
- Verify replace directives are correct
- Review go.mod in distribution directory