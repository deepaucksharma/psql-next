# Taskfile Usage Guide

This guide explains how to use the new Taskfile-based build and deployment system for the Database Intelligence Collector.

## Overview

We've replaced 30+ shell scripts and a complex Makefile with a unified Taskfile system that provides:
- Consistent command interface
- Better error handling
- Parallel execution
- Clear dependencies
- Environment-specific configurations

## Installation

First, install Task:

```bash
# macOS
brew install go-task/tap/go-task

# Linux
sh -c "$(curl --location https://taskfile.dev/install.sh)" -- -d -b ~/.local/bin

# Windows
scoop install task
```

## Quick Start

```bash
# First time setup
task quickstart

# This runs:
# 1. Install dependencies
# 2. Fix common issues
# 3. Build the collector
# 4. Start development environment
```

## Common Commands

### Development

```bash
# Start development environment
task dev:up

# Start with specific databases
task dev:postgres
task dev:mysql

# Watch for changes and rebuild
task dev:watch

# View logs
task dev:logs

# Clean up
task dev:down
```

### Building

```bash
# Build collector binary
task build

# Build with specific mode
task build MODE=experimental

# Build Docker image
task docker:build

# Build and push
task docker:push
```

### Testing

```bash
# Run all tests
task test

# Run specific test suites
task test:unit
task test:integration
task test:e2e

# Run with coverage
task test:coverage

# Run benchmarks
task test:bench
```

### Deployment

```bash
# Deploy to Kubernetes
task deploy:k8s ENV=production

# Deploy with Helm
task deploy:helm ENV=staging

# Update existing deployment
task deploy:update

# Rollback deployment
task deploy:rollback
```

### Validation

```bash
# Validate everything
task validate

# Validate specific components
task validate:config
task validate:helm
task validate:docker

# Fix common issues
task fix:all
```

## Environment-Specific Operations

### Development Environment

```bash
# Start with all tools
task dev:full

# This includes:
# - PostgreSQL & MySQL databases
# - Sample data generation
# - Collector with debug logging
# - New Relic integration

# Reset databases
task dev:reset-db
```

### Staging Environment

```bash
# Deploy to staging
task deploy:staging

# Run staging tests
task test:staging

# Validate staging config
task validate:config CONFIG_ENV=staging
```

### Production Environment

```bash
# Production deployment (requires confirmation)
task deploy:production

# Production health check
task prod:health-check

# Production rollback
task prod:rollback VERSION=v1.2.3
```

## Configuration Management

### Using Config Overlays

```bash
# Run with specific config overlay
task run CONFIG_ENV=production

# Validate overlay configuration
task validate:config CONFIG_ENV=staging

# Test config merge
task config:test ENV=production
```

### Environment Variables

```bash
# Set environment file
task run ENV_FILE=.env.production

# Override specific variables
task run POSTGRES_HOST=localhost NEW_RELIC_LICENSE_KEY=xxx
```

## Troubleshooting

### Common Issues

```bash
# Fix all common issues
task fix:all

# Fix specific issues
task fix:permissions      # File permission issues
task fix:module-paths    # Go module path issues
task fix:dependencies    # Dependency issues
```

### Debug Commands

```bash
# Run with debug logging
task run:debug

# Check collector health
task health-check

# View collector metrics
task metrics

# Test database connections
task test:connections
```

## Advanced Usage

### Parallel Execution

```bash
# Run multiple tasks in parallel
task build test:unit test:integration --parallel

# Deploy to multiple environments
task deploy:staging deploy:dev --parallel
```

### Task Dependencies

Tasks automatically handle dependencies:

```bash
# This automatically runs 'build' first
task docker:build

# This runs setup, build, then deploy
task deploy:k8s
```

### Custom Variables

```bash
# Override default values
task build BINARY_NAME=custom-collector

# Use custom config
task run CONFIG_FILE=/path/to/config.yaml

# Set multiple variables
task deploy:helm NAMESPACE=monitoring RELEASE_NAME=my-collector
```

### Conditional Execution

```bash
# Only run if changes detected
task build --watch

# Skip confirmation prompts
task deploy:production --force

# Dry run mode
task deploy:k8s --dry-run
```

## Task List

View all available tasks:

```bash
# List all tasks
task --list-all

# List tasks with descriptions
task --list

# Get help for specific task
task help build
```

### Key Task Categories

**Setup & Installation**
- `task setup` - Install all dependencies
- `task install-tools` - Install required Go tools

**Development**
- `task dev:up` - Start development environment
- `task dev:watch` - Watch mode with auto-rebuild
- `task dev:reset` - Reset development environment

**Building**
- `task build` - Build collector binary
- `task build:all` - Build all components
- `task docker:build` - Build Docker image

**Testing**
- `task test` - Run all tests
- `task test:unit` - Unit tests only
- `task test:integration` - Integration tests
- `task test:coverage` - Generate coverage report

**Deployment**
- `task deploy:k8s` - Deploy to Kubernetes
- `task deploy:helm` - Deploy using Helm
- `task deploy:docker` - Deploy with Docker Compose

**Validation**
- `task validate` - Run all validations
- `task validate:config` - Validate configurations
- `task validate:security` - Security checks

**Maintenance**
- `task clean` - Clean build artifacts
- `task update` - Update dependencies
- `task fix:all` - Fix common issues

## Best Practices

1. **Always run `task validate` before deployment**
2. **Use environment-specific configs for different stages**
3. **Run `task fix:all` if you encounter build issues**
4. **Use `--dry-run` flag for testing deployments**
5. **Keep your `.env` files secure and never commit them**

## CI/CD Integration

### GitHub Actions

```yaml
- name: Run tasks
  run: |
    task setup
    task validate
    task test
    task build
```

### GitLab CI

```yaml
script:
  - task ci:setup
  - task ci:test
  - task ci:build
  - task ci:deploy
```

## Migration from Old Scripts

| Old Command | New Task Command |
|-------------|------------------|
| `./scripts/build.sh` | `task build` |
| `./scripts/test.sh` | `task test` |
| `./scripts/deploy.sh` | `task deploy:k8s` |
| `make build-collector` | `task build` |
| `make docker-build` | `task docker:build` |
| `docker-compose up` | `task dev:up` |

## Getting Help

```bash
# Show task help
task help

# Show specific task details
task help build

# List all available tasks
task --list-all

# Check Task version
task --version
```

For more information, see the [Taskfile.yml](../Taskfile.yml) or run `task help`.