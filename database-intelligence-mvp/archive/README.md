# Archived Configuration Files

This directory contains configuration files that were created during the migration from the custom architecture to the OTEL-first approach. These files are archived for historical reference but should not be used for new deployments.

## Active Configuration Files

The following configuration files remain active and should be used:

1. **config/collector.yaml** - Main production configuration with all features
2. **config/collector-simplified.yaml** - Simplified configuration for quick start
3. **config/collector-minimal.yaml** - Minimal configuration for testing

## Archived Files

The following files have been moved to this archive directory:

- **collector-experimental.yaml** - Early experimental configurations
- **collector-test.yaml** - Test configurations from development
- **collector-working.yaml** - Interim working configurations
- **collector-with-postgresql-receiver.yaml** - When testing PostgreSQL receiver
- **collector-unified.yaml** - Attempt at unified configuration
- **collector-newrelic-optimized.yaml** - New Relic specific optimizations
- **collector-ohi-compatible.yaml** - OHI compatibility attempts
- **collector-nr-test.yaml** - New Relic testing configurations
- **collector-postgresql.yaml** - PostgreSQL-only configurations
- **collector-ha.yaml** - High availability experiments
- **collector-dev.yaml** - Development configurations
- **collector-otel-metrics.yaml** - OTEL metrics testing
- **collector-otel-first.yaml** - Initial OTEL-first approach
- **collector-with-verification.yaml** - Verification processor testing
- **attribute-mapping.yaml** - Attribute mapping configurations

## Why These Were Archived

These configurations represent the evolution of the project from a custom implementation to an OTEL-first architecture. They contain:

- Experimental features that didn't make it to production
- Duplicate configurations with minor variations
- Test configurations for specific scenarios
- Interim solutions during migration

## Migration to Active Configurations

If you're looking for specific features from these archived configs:

1. **Standard OTEL receivers** → Use `collector.yaml`
2. **Minimal setup** → Use `collector-simplified.yaml`
3. **Custom processors** → See `collector.yaml` for proper integration
4. **New Relic export** → All active configs support this

## Note

These files are kept for reference only. Do not use them for production deployments as they may contain outdated or incorrect configurations.