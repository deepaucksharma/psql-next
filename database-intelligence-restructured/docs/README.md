# Database Intelligence Documentation

Welcome to the comprehensive documentation for the Database Intelligence OpenTelemetry Collector project.

## Documentation Structure

### [Quick Start Guide](./01-quick-start/)

Get started quickly with installation, configuration, and basic usage.

### [End-to-End Testing Documentation](./02-e2e-testing/)

Comprehensive end-to-end testing documentation, strategies, and reports.

### [OHI to OpenTelemetry Migration](./03-ohi-migration/)

Complete guide for migrating from New Relic OHI to OpenTelemetry.

### [Implementation Analysis](./04-implementation/)

Technical analysis of the implementation, including OOTB vs custom components.

## Quick Links

- [Quick Start Guide](./01-quick-start/quick-start.md)
- [E2E Test Strategy](./02-e2e-testing/02-test-strategy.md)
- [OHI to OTEL Mapping](./03-ohi-migration/01-complete-mapping.md)
- [Implementation Analysis](./04-implementation/01-metric-source-analysis.md)

## Project Overview

This project implements a comprehensive database monitoring solution using OpenTelemetry collectors, 
with full feature parity for New Relic's On-Host Integration (OHI) for databases.

### Key Features

- ✅ PostgreSQL and MySQL monitoring
- ✅ Custom processors for advanced metrics
- ✅ Full OHI feature parity
- ✅ OpenTelemetry semantic conventions
- ✅ New Relic integration via OTLP
- ✅ Comprehensive E2E testing
- ✅ Production-ready configurations

### Architecture

The solution combines:
- **OOTB Components**: Standard OpenTelemetry receivers for basic metrics
- **Custom Components**: SQLQuery receivers for advanced metrics
- **Processors**: Transform, enrichment, and semantic mapping
- **Exporters**: OTLP export to New Relic

For detailed architecture information, see the [Implementation Analysis](./04-implementation/02-implementation-summary.md).
