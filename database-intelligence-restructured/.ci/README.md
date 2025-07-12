# Continuous Integration and Build Configuration

This directory contains all CI/CD, build, and automation configurations for the Database Intelligence project.

## Directory Structure

```
.ci/
├── build/              # OpenTelemetry Collector Builder configurations
│   ├── minimal.yaml    # Minimal distribution build
│   ├── enhanced.yaml   # Full-featured build
│   └── archive/        # Legacy build configurations
├── workflows/          # CI/CD workflow definitions
│   ├── ci.yml         # Standard CI pipeline
│   ├── cd.yml         # Continuous deployment
│   ├── e2e-tests.yml  # End-to-end testing
│   └── release.yml    # Release automation
└── scripts/           # Build and deployment scripts
    ├── build.sh       # Unified build script
    └── deploy.sh      # Deployment automation
```

## Usage

### Building Distributions

```bash
# Build using OpenTelemetry Builder
builder --config=.ci/build/enhanced.yaml

# Build using unified script
.ci/scripts/build.sh production
```

### CI/CD Workflows

All workflows are located in `.ci/workflows/` and should be:
- Copied to `.github/workflows/` for GitHub Actions
- Adapted for other CI/CD platforms as needed

### Local Development

```bash
# Run build locally
.ci/scripts/build.sh

# Test deployment
.ci/scripts/deploy.sh staging
```

## Integration

This consolidation provides:
1. **Single Source**: All build and CI/CD logic in one place
2. **Consistency**: Unified approach across environments
3. **Maintainability**: Easier to update and manage
4. **Reusability**: Scripts work across different CI/CD platforms