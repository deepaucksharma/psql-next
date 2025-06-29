# Getting Started

Welcome to the Database Intelligence MVP! This guide will help you get up and running in minutes.

## Prerequisites

*   **Database Access**: PostgreSQL (`pg_stat_statements` extension) or MySQL (Performance Schema enabled), with read-only user credentials.
*   **System Requirements**: Docker, Docker Compose, 2GB free memory (minimum), New Relic account with license key.

## Quick Start (5 minutes)

### Option 1: Standard Deployment (Recommended)

Get production-ready monitoring with proven components:

```bash
# Clone the repository
git clone https://github.com/newrelic/database-intelligence-mvp
cd database-intelligence-mvp

# Run interactive setup
./quickstart.sh all
```

### Option 2: Experimental Deployment (Advanced)

For advanced features like ASH sampling and adaptive monitoring:

```bash
# Build and deploy experimental components
./quickstart.sh --experimental all
```

## What Happens During Setup

The `quickstart.sh` script guides you through:
1.  **Environment Configuration**: Prompts for PostgreSQL DSN and New Relic license key.
2.  **Connection Validation**: Verifies database connectivity and prerequisites.
3.  **Collector Startup**: Starts the collector, providing health and metrics endpoints.

## Verify Data Collection

After startup, verify data is flowing:
*   Check collector health: `curl http://localhost:13133/`.
*   View collection metrics: `curl http://localhost:8888/metrics | grep otelcol_receiver_accepted`.
*   Check logs for any issues: `./quickstart.sh logs`.

## View in New Relic

1.  Log into your New Relic account.
2.  Navigate to **APM & Services** → **Database**.
3.  Confirm database entities appear within 5 minutes.
4.  View query performance metrics and trends.

## Configuration Files

*   `.env`: Environment variables (credentials).
*   `config/collector.yaml`: Standard collector configuration.
*   `config/collector-experimental.yaml`: Experimental features configuration.

## Common Setup Issues

*   **Database Connection Failed**: Verify DSN format, network, user permissions, use read replica.
*   **No Data in New Relic**: Check license key, outbound HTTPS (443) allowance, collector logs.
*   **High Memory Usage**: Adjust sampling (`probabilistic_sampler`) for Standard Mode; 2GB+ expected for Experimental Mode.

## Next Steps

*   **Standard Mode Users**: Review metrics, set up alerts, create dashboards, fine-tune collection intervals.
*   **Experimental Mode Users**: Explore ASH data, monitor adaptive sampling, test circuit breaker thresholds, provide feedback.

## Useful Commands

*   **Start/Stop**: `./quickstart.sh start` / `./quickstart.sh stop`.
*   **Status**: `./quickstart.sh status`.
*   **Update Config**: `./quickstart.sh configure`.
*   **Run Safety Tests**: `./quickstart.sh test`.
*   **Build Experimental**: `./quickstart.sh --experimental build`.

## Getting Help

*   **Quick Help**: `./quickstart.sh` (no arguments).
*   **Troubleshooting**: See [TROUBLESHOOTING-GUIDE.md](TROUBLESHOOTING-GUIDE.md).
*   **GitHub Issues**: Report bugs or request features.
*   **GitHub Discussions**: Ask questions.

## What's Next?

*   **Production Deployment**: See [DEPLOYMENT.md](DEPLOYMENT.md).
*   **Advanced Features**: Explore [EXPERIMENTAL-FEATURES.md](EXPERIMENTAL-FEATURES.md).
*   **Architecture**: Understand the system in [ARCHITECTURE.md](ARCHITECTURE.md).
