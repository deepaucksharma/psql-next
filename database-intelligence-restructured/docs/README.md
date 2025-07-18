# Database Intelligence Documentation

## 📚 Documentation Structure

```
docs/
├── guides/           # Step-by-step instructions
├── reference/        # Technical specifications  
├── development/      # Developer resources
└── archive/          # Historical documentation
```

## 🚀 Start Here

### New Users
1. **[Quick Start Guide](guides/QUICK_START.md)** - Get running in 5 minutes
2. **[Configuration Guide](guides/CONFIGURATION.md)** - Customize your setup
3. **[Deployment Guide](guides/DEPLOYMENT.md)** - Production deployment

### Developers
1. **[Development Setup](development/SETUP.md)** - Set up your environment
2. **[Architecture Overview](reference/ARCHITECTURE.md)** - Understand the system
3. **[Testing Guide](development/TESTING.md)** - Run and write tests

### Operators
1. **[Metrics Reference](reference/METRICS.md)** - All available metrics
2. **[Troubleshooting Guide](guides/TROUBLESHOOTING.md)** - Solve common issues
3. **[API Reference](reference/API.md)** - Component interfaces

## 📖 Documentation by Topic

### Configuration & Deployment
- [Configuration Guide](guides/CONFIGURATION.md) - YAML configuration reference
- [Deployment Guide](guides/DEPLOYMENT.md) - Docker, Kubernetes, binary
- [Unified Deployment](guides/UNIFIED_DEPLOYMENT_GUIDE.md) - Multi-mode deployment

### Database-Specific Guides
- [PostgreSQL Metrics](reference/POSTGRESQL_METRICS.md) - PostgreSQL monitoring
- [MySQL Guide](guides/MYSQL_MAXIMUM_GUIDE.md) - MySQL configuration
- [MongoDB Guide](guides/MONGODB_MAXIMUM_GUIDE.md) - MongoDB setup
- [Config-Only Mode](guides/CONFIG_ONLY_MAXIMUM_GUIDE.md) - Standard OTel mode

### Development
- [Setup Guide](development/SETUP.md) - Development environment
- [Testing Guide](development/TESTING.md) - Testing strategies
- [E2E Queries](development/e2e-validation-queries.md) - Validation queries

### Reference
- [Architecture](reference/ARCHITECTURE.md) - System design
- [Metrics Reference](reference/METRICS.md) - Complete metrics list
- [API Reference](reference/API.md) - Component APIs

## 🏗️ Key Concepts

### Two Operating Modes
1. **Config-Only Mode** - Uses standard OpenTelemetry components
   - Works with official OTel Collector
   - Minimal resource usage
   - Quick setup

2. **Enhanced Mode** - Includes custom intelligence
   - Query plan analysis
   - Active Session History
   - Adaptive sampling
   - Advanced processors

### Component Types
- **Receivers** - Collect metrics from databases
- **Processors** - Transform and enrich data
- **Exporters** - Send data to backends

## 📝 Contributing to Docs

When adding documentation:
- **Guides** go in `guides/` - How-to instructions
- **Reference** goes in `reference/` - Technical specs
- **Development** goes in `development/` - Code-focused docs

Keep documentation:
- **Current** - Update when code changes
- **Clear** - Use examples and diagrams
- **Concise** - Get to the point quickly

## 🔍 Quick Search

| Looking for... | See... |
|---------------|--------|
| Getting started | [Quick Start](guides/QUICK_START.md) |
| Configuration options | [Configuration Guide](guides/CONFIGURATION.md) |
| Available metrics | [Metrics Reference](reference/METRICS.md) |
| Troubleshooting | [Troubleshooting Guide](guides/TROUBLESHOOTING.md) |
| System design | [Architecture](reference/ARCHITECTURE.md) |
| Development setup | [Setup Guide](development/SETUP.md) |
| Testing | [Testing Guide](development/TESTING.md) |
| API docs | [API Reference](reference/API.md) |

---

For project status and roadmap, see [PROJECT_STATUS.md](../PROJECT_STATUS.md).