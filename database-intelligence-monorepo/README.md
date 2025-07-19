# Database Intelligence MySQL - Monorepo

A modular OpenTelemetry-based MySQL monitoring system organized as a monorepo with independent, composable modules.

## Repository Structure

```
database-intelligence-monorepo/
├── modules/              # Independent monitoring modules
│   ├── core-metrics/     # Basic MySQL metrics collection
│   ├── sql-intelligence/ # Query analysis and optimization
│   ├── wait-profiler/    # Wait event profiling
│   ├── anomaly-detector/ # Anomaly detection
│   ├── business-impact/  # Business impact scoring
│   ├── replication-monitor/ # Replication monitoring
│   ├── performance-advisor/ # Performance recommendations
│   ├── resource-monitor/ # Resource usage tracking
│   ├── canary-tester/    # Synthetic monitoring
│   └── alert-manager/    # Alert management
├── shared/               # Shared resources
│   ├── interfaces/       # Common interfaces
│   ├── docker/          # Shared Docker configurations
│   └── scripts/         # Shared scripts
├── integration/         # Integration testing
└── Makefile            # Root orchestration
```

## Quick Start

### Run Individual Modules
```bash
# Run core metrics module
make run-core-metrics

# Run SQL intelligence module
make run-sql-intelligence

# Run multiple modules
make run-core run-intelligence
```

### Run All Modules
```bash
make run-all
```

### Test Modules
```bash
# Test individual module
make test-core-metrics

# Test all modules in parallel
make test
```

## Module Independence

Each module is designed to be:
- **Independently deployable**: Has its own docker-compose.yaml
- **Independently testable**: Has its own test suite
- **Optionally interconnected**: Can consume data from other modules
- **Self-contained**: Includes all necessary configurations

## Development Workflow

1. Choose a module to work on
2. Navigate to the module directory
3. Use module-specific Makefile commands
4. Test in isolation or with other modules
5. Integration test with full system

## Module Details

### Core Metrics
Basic MySQL metrics collection including connections, threads, and performance counters.

### SQL Intelligence
Query analysis, slow query detection, and optimization recommendations.

### Wait Profiler
Performance schema based wait event analysis.

### Anomaly Detector
Statistical anomaly detection across all metrics.

### Business Impact
Business value scoring for queries and operations.

### Replication Monitor
Master-slave replication monitoring and lag detection.

### Performance Advisor
Automated performance recommendations based on metrics.

### Resource Monitor
CPU, memory, disk, and network usage tracking.

### Canary Tester
Synthetic query testing and availability monitoring.

### Alert Manager
Centralized alert aggregation and routing.