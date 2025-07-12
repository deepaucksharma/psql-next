# Documentation

Welcome to the Database Intelligence documentation. This directory contains all project documentation organized by purpose.

## 📚 Documentation Structure

```
docs/
├── guides/           # How-to guides and tutorials
│   ├── QUICK_START.md      # 5-minute getting started
│   ├── CONFIGURATION.md    # Configuration reference
│   ├── DEPLOYMENT.md       # Production deployment
│   └── TROUBLESHOOTING.md  # Problem solving
│
├── reference/        # Technical references
│   ├── ARCHITECTURE.md     # System design
│   ├── METRICS.md          # All metrics collected
│   ├── API.md              # Component APIs
│   └── POSTGRESQL_METRICS.md # PostgreSQL specifics
│
├── development/      # Developer documentation
│   ├── SETUP.md            # Development setup
│   ├── TESTING.md          # Testing guide
│   ├── TEST_REPORT.md      # Latest test results
│   ├── e2e-validation-queries.md # Validation queries
│   └── CLAUDE.md           # AI assistant context
│
└── archive/          # Historical documentation
    └── [100+ archived files for reference]
```

## 🚀 Quick Navigation

### For New Users
1. Start with [Quick Start Guide](guides/QUICK_START.md)
2. Configure using [Configuration Guide](guides/CONFIGURATION.md)
3. Deploy with [Deployment Guide](guides/DEPLOYMENT.md)
4. Fix issues using [Troubleshooting Guide](guides/TROUBLESHOOTING.md)

### For Developers
1. Set up with [Development Setup](development/SETUP.md)
2. Run tests with [Testing Guide](development/TESTING.md)
3. Check [API Reference](reference/API.md)
4. Review [Architecture](reference/ARCHITECTURE.md)

### For Operations
1. Understand [Metrics Reference](reference/METRICS.md)
2. Follow [Deployment Guide](guides/DEPLOYMENT.md)
3. Monitor with [PostgreSQL Metrics](reference/POSTGRESQL_METRICS.md)
4. Troubleshoot with [Troubleshooting Guide](guides/TROUBLESHOOTING.md)

## 📊 Current Implementation

- **Version**: 2.0 (PostgreSQL-Only)
- **Modes**: Config-Only (Standard OTel) and Custom (Enhanced)
- **Metrics**: 35+ PostgreSQL metrics in Config-Only, 50+ in Custom
- **Status**: Production Ready

## 🗂️ Archive

The `archive/` directory contains 100+ historical documentation files that provide context on:
- Project evolution
- Architecture decisions
- Implementation details
- Testing strategies

These files are preserved for reference but may not reflect the current implementation.

## 📝 Documentation Standards

- **Guides**: Task-oriented, how-to documentation
- **Reference**: Technical specifications and APIs
- **Development**: Code-focused documentation
- **Archive**: Historical context

## 🔍 Finding Information

### By Topic
- **Configuration**: See [CONFIGURATION.md](guides/CONFIGURATION.md)
- **Metrics**: See [METRICS.md](reference/METRICS.md)
- **Troubleshooting**: See [TROUBLESHOOTING.md](guides/TROUBLESHOOTING.md)
- **Architecture**: See [ARCHITECTURE.md](reference/ARCHITECTURE.md)

### By Task
- **Get Started**: [QUICK_START.md](guides/QUICK_START.md)
- **Deploy**: [DEPLOYMENT.md](guides/DEPLOYMENT.md)
- **Develop**: [SETUP.md](development/SETUP.md)
- **Test**: [TESTING.md](development/TESTING.md)

## 🤝 Contributing to Documentation

When adding documentation:
1. Place guides in `guides/`
2. Place technical specs in `reference/`
3. Place dev docs in `development/`
4. Archive old docs in `archive/`

Keep documentation:
- **Current**: Update when code changes
- **Clear**: Use examples and diagrams
- **Concise**: Get to the point
- **Consistent**: Follow existing patterns