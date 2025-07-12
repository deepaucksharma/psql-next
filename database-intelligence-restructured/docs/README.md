# Database Intelligence with OpenTelemetry - Documentation

## 📖 Quick Navigation

| Document | Purpose | Implementation Status |
|----------|---------|---------------------|
| [📋 Overview](OVERVIEW.md) | Project architecture and capabilities | ✅ Current |
| [🚀 Quick Start](QUICK_START.md) | Get up and running in 5 minutes | ✅ Current |
| [⚙️ Configuration](CONFIGURATION.md) | Complete configuration reference | ✅ Current |
| [🏗️ Deployment](DEPLOYMENT.md) | Docker, K8s, and production setup | ✅ Current |
| [🧪 Testing](TESTING.md) | E2E tests and validation | ✅ Current |
| [🔧 Troubleshooting](TROUBLESHOOTING.md) | Common issues and solutions | ✅ Current |

## 🏛️ Architecture

This project implements **two distinct operational modes**:

### Config-Only Mode (Production Ready)
- Uses standard OpenTelemetry components
- No custom code required
- Resource usage: <5% CPU, <512MB memory
- **Status**: ✅ Fully implemented and tested

### Enhanced Mode (Development)
- Includes custom receivers and processors
- Advanced database intelligence features
- Resource usage: <20% CPU, <2GB memory  
- **Status**: ⚠️ Components implemented but not integrated into distributions

## 🚦 Current Implementation State

| Component | Status | Available In |
|-----------|--------|--------------|
| PostgreSQL Receiver | ✅ Production | All distributions |
| MySQL Receiver | ✅ Production | All distributions |
| SQL Query Receiver | ✅ Production | All distributions |
| Host Metrics Receiver | ✅ Production | All distributions |
| **ASH Receiver** | ⚠️ Source only | None |
| **Enhanced SQL Receiver** | ⚠️ Source only | None |
| **Plan Attribute Extractor** | ⚠️ Source only | None |
| **Adaptive Sampler** | ⚠️ Source only | None |
| **Circuit Breaker** | ⚠️ Source only | None |
| **Cost Control** | ⚠️ Source only | None |
| **Query Correlator** | ⚠️ Source only | None |
| **Verification Processor** | ⚠️ Source only | None |

## 📁 Documentation Structure

```
docs/
├── README.md                 # This file - navigation hub
├── OVERVIEW.md              # Architecture and capabilities
├── QUICK_START.md           # 5-minute setup guide
├── CONFIGURATION.md         # Complete config reference
├── DEPLOYMENT.md            # Production deployment
├── TESTING.md               # E2E testing guide
└── TROUBLESHOOTING.md       # Issue resolution
```

## 🗂️ Archived Documentation

Historical documentation has been consolidated and archived:
- `docs/archive/` - Previous iteration documentation
- All numbered directories (01-quick-start, 02-e2e-testing, etc.) have been consolidated
- Project status reports moved to `docs/archive/project-status/`

## 🎯 What to Read First

1. **New Users**: Start with [Quick Start](QUICK_START.md)
2. **Operators**: Review [Configuration](CONFIGURATION.md) and [Deployment](DEPLOYMENT.md)
3. **Developers**: See [Testing](TESTING.md) and component source code
4. **Troubleshooters**: Jump to [Troubleshooting](TROUBLESHOOTING.md)

## 🔗 External Resources

- [OpenTelemetry Collector Documentation](https://opentelemetry.io/docs/collector/)
- [New Relic OTLP Integration](https://docs.newrelic.com/docs/more-integrations/open-source-telemetry-integrations/opentelemetry/)
- [PostgreSQL Monitoring Guide](https://www.postgresql.org/docs/current/monitoring.html)
- [MySQL Performance Schema](https://dev.mysql.com/doc/refman/8.0/en/performance-schema.html)