# Database Intelligence Scripts

Organized collection of scripts for building, testing, deploying, and maintaining the Database Intelligence project.

## 📁 Directory Structure

```
scripts/
├── build/          # Build scripts
├── test/           # Test scripts  
├── deploy/         # Deployment scripts
├── dev/            # Development utilities
├── maintain/       # Maintenance scripts
└── utils/          # Shared utilities
```

## 🚀 Quick Start

### Building
```bash
# Build production distribution
./scripts/build/build.sh production

# Build all distributions
./scripts/build/build.sh all

# Build Docker image
./scripts/build/build.sh docker
```

### Testing
```bash
# Run all tests
./scripts/test/run-tests.sh

# Run specific test type
./scripts/test/run-tests.sh unit
./scripts/test/run-tests.sh integration postgresql
./scripts/test/run-tests.sh e2e
```

### Deployment
```bash
# Deploy with Docker
./scripts/deploy/docker.sh up

# Deploy to Kubernetes
./scripts/deploy/kubernetes.sh apply

# Check deployment status
./scripts/deploy/docker.sh status
```

### Development
```bash
# Fix module issues
./scripts/dev/fix-modules.sh

# Set up development environment
./scripts/dev/setup.sh

# Format and lint code
./scripts/dev/lint.sh
```

### Maintenance
```bash
# Clean up everything
./scripts/maintain/cleanup.sh

# Validate configurations
./scripts/maintain/validate.sh

# Update dependencies
./scripts/maintain/update-deps.sh
```

## 📋 Script Categories

### Build Scripts (`build/`)
- **build.sh** - Main build script supporting multiple modes and platforms
- See [build/README.md](build/README.md) for detailed documentation

### Test Scripts (`test/`)
- **run-tests.sh** - Unified test runner
- **unit.sh** - Unit test execution
- **integration.sh** - Integration test execution
- **config-validation.sh** - Configuration validation
- **performance.sh** - Performance testing

### Deployment Scripts (`deploy/`)
- **docker.sh** - Docker deployment management
- **kubernetes.sh** - Kubernetes deployment
- **start-services.sh** - Start all services
- **stop-services.sh** - Stop all services

### Development Scripts (`dev/`)
- **fix-modules.sh** - Fix Go module issues
- **setup.sh** - Development environment setup
- **lint.sh** - Code linting and formatting
- **watch.sh** - Watch mode for development

### Maintenance Scripts (`maintain/`)
- **cleanup.sh** - Clean build artifacts and temp files
- **validate.sh** - Validate project structure
- **update-deps.sh** - Update dependencies
- **check-security.sh** - Security scanning

### Utility Scripts (`utils/`)
- **common.sh** - Shared functions and utilities
- **colors.sh** - Color definitions
- **logging.sh** - Logging functions

## 🔧 Common Tasks

### Complete Build and Test
```bash
# Build and test everything
./scripts/build/build.sh production && ./scripts/test/run-tests.sh
```

### Development Workflow
```bash
# Fix modules, build, and test
./scripts/dev/fix-modules.sh
./scripts/build/build.sh production
./scripts/test/run-tests.sh unit
```

### Clean Development Environment
```bash
# Clean everything and rebuild
./scripts/maintain/cleanup.sh all
./scripts/build/build.sh clean
./scripts/build/build.sh production
```

### Deploy to Production
```bash
# Build, test, and deploy
./scripts/build/build.sh production
./scripts/test/run-tests.sh
./scripts/deploy/docker.sh up production
```

## 🌟 Best Practices

1. **Always source common.sh** for consistent utilities
2. **Use descriptive logging** with the provided functions
3. **Handle errors gracefully** with proper exit codes
4. **Support dry-run mode** where applicable
5. **Provide help/usage information**

## 🐛 Troubleshooting

### Script Not Found
```bash
# Make sure scripts are executable
chmod +x scripts/**/*.sh
```

### Module Issues
```bash
# Fix all module problems
./scripts/dev/fix-modules.sh all
```

### Build Failures
```bash
# Clean and rebuild
./scripts/build/build.sh clean
./scripts/build/build.sh production
```

### Test Failures
```bash
# Run tests with verbose output
VERBOSE=true ./scripts/test/run-tests.sh
```

## 📝 Contributing

When adding new scripts:
1. Place in appropriate category directory
2. Source `utils/common.sh` for utilities
3. Include usage/help information
4. Make executable with `chmod +x`
5. Update this README

## 🔗 Related Documentation

- [Project README](../README.md)
- [Development Guide](../docs/development/SETUP.md)
- [CI/CD Documentation](../.ci/README.md)

---

**Note**: These scripts have been consolidated from multiple locations to provide a single, organized location for all operational scripts.