# Database Intelligence for PostgreSQL

> **Current Status**: PostgreSQL-only implementation with comprehensive metric collection in both standard (Config-Only) and enhanced (Custom) modes.

## 📚 Documentation Structure

### Primary Documentation
- **[CONSOLIDATED_DOCUMENTATION.md](CONSOLIDATED_DOCUMENTATION.md)** - Complete guide covering all aspects
- **[README.md](README.md)** - Original project overview
- **[QUICK_START.md](docs/QUICK_START.md)** - 5-minute setup guide

### Implementation Status
- **[IMPLEMENTATION_COMPLETE.md](IMPLEMENTATION_COMPLETE.md)** - Current implementation summary
- **[POSTGRESQL_ONLY_SUMMARY.md](POSTGRESQL_ONLY_SUMMARY.md)** - PostgreSQL focus changes
- **[TEST_REPORT.md](TEST_REPORT.md)** - Test execution results

### Deployment & Operations
- **[PARALLEL_DEPLOYMENT_GUIDE.md](deployments/PARALLEL_DEPLOYMENT_GUIDE.md)** - Run both modes
- **[TROUBLESHOOTING_METRICS.md](TROUBLESHOOTING_METRICS.md)** - Metric collection issues
- **[MIGRATION.md](MIGRATION.md)** - Migration strategies

### Development Resources
- **[CLAUDE.md](CLAUDE.md)** - AI assistant context
- **[e2e-validation-queries.md](e2e-validation-queries.md)** - 100+ validation queries
- **[ARCHIVED_DOCS_INDEX.md](ARCHIVED_DOCS_INDEX.md)** - Historical documentation

## 🚀 Quick Links

### Get Started
```bash
# 1. Set environment
export NEW_RELIC_LICENSE_KEY="your-key"
export NEW_RELIC_ACCOUNT_ID="your-account-id"

# 2. Deploy
./scripts/deploy-parallel-modes.sh

# 3. Verify
./scripts/verify-metrics.sh
```

### Key Features
- ✅ **35+ PostgreSQL metrics** in Config-Only mode
- ✅ **Active Session History (ASH)** in Custom mode
- ✅ **Parallel deployment** for comparison
- ✅ **Comprehensive testing** tools
- ✅ **Production ready** with Docker/K8s support

### Project Structure
```
database-intelligence-restructured/
├── components/          # Custom OTel components
├── configs/            # Sample configurations
├── dashboards/         # New Relic dashboards
├── deployments/        # Docker/K8s deployments
├── docs/              # Documentation
│   └── archive/       # Historical docs (100+ files)
├── scripts/           # Automation scripts
├── tests/             # Test suites
└── tools/             # Load generators
```

## 📊 Metrics Overview

### Config-Only Mode (35+ metrics)
Standard OpenTelemetry PostgreSQL receiver metrics including:
- Connection metrics (backends, max connections)
- Transaction metrics (commits, rollbacks, deadlocks)
- Performance metrics (buffer hits, disk reads)
- Replication metrics (WAL lag, delays)
- Table/Index metrics (size, scans, vacuum)

### Custom Mode (50+ metrics)
Everything in Config-Only PLUS:
- Active Session History (real-time monitoring)
- Wait event analysis
- Query plan extraction
- Blocked session detection
- Intelligent sampling and cost control

## 🛠️ Tools & Scripts

### Testing Tools
- **[postgres-test-generator](tools/postgres-test-generator/)** - Exercise all metrics
- **[load-generator](tools/load-generator/)** - Multiple load patterns

### Validation Scripts
- **[verify-metrics.sh](scripts/verify-metrics.sh)** - Check metric collection
- **[validate-metrics-e2e.sh](scripts/validate-metrics-e2e.sh)** - Generate NRQL queries
- **[deploy-parallel-modes.sh](scripts/deploy-parallel-modes.sh)** - Deploy both modes

### Dashboards
- **[postgresql-parallel-dashboard.json](dashboards/newrelic/postgresql-parallel-dashboard.json)** - 9-page comprehensive dashboard

## 📈 Performance Comparison

| Aspect | Config-Only | Custom Mode |
|--------|------------|-------------|
| Metrics | 35+ | 50+ |
| Memory | ~200MB | 500MB-1GB |
| CPU | 0.5 core | 1-2 cores |
| DPM | ~6,000 | ~30,000 |
| Features | Standard | ASH, Query Intelligence |

## 🔍 Troubleshooting

Common issues and solutions in [TROUBLESHOOTING_METRICS.md](TROUBLESHOOTING_METRICS.md):
- No metrics appearing
- Missing specific metrics
- Connection errors
- Performance issues

## 📖 Historical Context

The project evolved through several phases:
1. **Initial**: Dual PostgreSQL/MySQL support
2. **Issues Found**: 15+ module conflicts, memory leaks
3. **Fixes Applied**: Module consolidation, security fixes
4. **Current**: PostgreSQL-only with maximum metrics

See [ARCHIVED_DOCS_INDEX.md](ARCHIVED_DOCS_INDEX.md) for detailed history.

## 🎯 Next Steps

1. **Deploy** the parallel setup
2. **Generate** test data
3. **Verify** metrics in New Relic
4. **Compare** modes using the dashboard
5. **Choose** appropriate mode for production

## 📞 Support

- **Documentation**: This repository
- **Issues**: GitHub Issues
- **Community**: OpenTelemetry Slack #database-monitoring

---

*PostgreSQL Database Intelligence - Production Ready*
*Last Updated: [Current Date]*