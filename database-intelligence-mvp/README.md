# New Relic Database Intelligence MVP

## What This Is

A production-ready OpenTelemetry Collector configuration that safely collects database execution plans and performance metrics, sending them to New Relic for analysis. Built on the principle of "Configure, Don't Build" - leveraging standard OTEL components with minimal custom code.

## What This Isn't (Yet)

- **Not** automatic APM-to-database correlation (requires manual correlation)
- **Not** real-time query analysis (batch collection only)
- **Not** multi-instance ready (single collector instance only)
- **Not** zero-configuration (requires database prerequisites)

## Quick Start

### 30-Second Overview

1. **Check Prerequisites**: Your database needs specific extensions enabled (see [PREREQUISITES.md](PREREQUISITES.md))
2. **Deploy Collector**: Single instance only - StatefulSet or DaemonSet
3. **Configure Safety**: Read-replica endpoints, read-only users
4. **Start Collecting**: See your first plan in New Relic within 60 seconds

### Critical Safety Warning

⚠️ **This collector MUST connect to read-replicas only**. Never point it at your primary database. All configurations include safety timeouts, but replica targeting is your first line of defense.

## Architecture Philosophy

We follow three core principles:

1. **Safety Over Features**: Every query has a timeout, every collection has a limit
2. **Honest Limitations**: We clearly document what doesn't work
3. **Incremental Value**: Start simple, enhance gradually

## What's Inside

- **Standard Receivers**: `sqlqueryreceiver` and `filelogreceiver` configured for safety
- **Custom Processors**: Minimal set for plan parsing and intelligent sampling
- **Persistent State**: File-backed deduplication (single instance only)

## Next Steps

1. Read [PREREQUISITES.md](PREREQUISITES.md) - Critical database setup required
2. Review [LIMITATIONS.md](LIMITATIONS.md) - Understand the boundaries
3. Follow [DEPLOYMENT.md](DEPLOYMENT.md) - Deploy safely and correctly