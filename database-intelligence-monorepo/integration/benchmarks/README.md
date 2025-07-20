# Database Intelligence Performance Benchmarking Suite

A comprehensive benchmarking framework for measuring and analyzing the performance of the Database Intelligence monitoring system.

## Overview

This benchmarking suite provides tools to measure:
- Module startup time
- Metric collection latency
- Resource usage per module
- Query performance impact
- End-to-end pipeline latency

## Components

### 1. benchmark-framework.py
The core benchmarking framework that provides:
- Timing utilities with high precision
- Resource monitoring (CPU, memory)
- Statistical analysis
- Result persistence
- Comparison capabilities

### 2. module-benchmarks.py
Module-specific benchmark tests for:
- Connection monitoring module
- Query performance tracking module
- Lock monitoring module
- Replication monitoring module
- Resource usage tracking module
- Full metric collection pipeline

### 3. load-generator.py
Generates realistic database workloads including:
- Variable connection rates
- Read/write query patterns
- Slow query simulation
- Lock contention scenarios
- Long transaction testing
- Batch operations

### 4. report-generator.py
Creates comprehensive reports featuring:
- Performance comparisons
- Trend analysis over time
- Resource usage charts
- Regression detection
- HTML and Markdown output formats

## Quick Start

### Prerequisites

```bash
# Install required Python packages
pip install mysql-connector-python psutil matplotlib numpy
```

### Environment Setup

Set the following environment variables for database connection:

```bash
export MYSQL_HOST=localhost
export MYSQL_PORT=3306
export MYSQL_USER=root
export MYSQL_PASSWORD=your_password
export MYSQL_DATABASE=mysql
```

### Running Benchmarks

#### 1. Basic Module Benchmarks

```bash
# Run all module benchmarks
python module-benchmarks.py

# This will test each monitoring module and save results to:
# module_benchmark_results.json
```

#### 2. Load Testing

```bash
# Generate light load (good for development)
python load-generator.py --profile light

# Generate moderate load (typical production)
python load-generator.py --profile moderate

# Generate heavy load (stress testing)
python load-generator.py --profile heavy

# Custom load profile
python load-generator.py --profile moderate --duration 300 --connections 50 --qps 1000

# Available profiles:
# - light: 5 connections, 50 QPS, 60s
# - moderate: 20 connections, 200 QPS, 120s
# - heavy: 50 connections, 500 QPS, 180s
# - read_heavy: 95% read queries
# - write_heavy: 80% write queries
# - lock_contention: High lock contention testing
```

#### 3. Generate Reports

```bash
# Generate all reports (HTML, Markdown, charts)
python report-generator.py

# Generate specific format
python report-generator.py --format html

# Compare specific benchmark runs
python report-generator.py --compare baseline_results.json current_results.json
```

## Benchmark Workflow

### 1. Establish Baseline

```bash
# Run initial benchmarks
python module-benchmarks.py
mv module_benchmark_results.json baseline_module_results.json

# Generate load and measure performance
python load-generator.py --profile moderate
```

### 2. Make Changes

Implement your performance optimizations or changes to the monitoring system.

### 3. Run Comparison Benchmarks

```bash
# Run benchmarks again
python module-benchmarks.py

# Generate comparison report
python report-generator.py --compare baseline_module_results.json module_benchmark_results.json
```

### 4. Analyze Results

Open the generated `benchmark_report.html` to view:
- Performance comparison graphs
- Regression detection
- Resource usage trends
- Detailed metrics

## Understanding Results

### Key Metrics

1. **Duration Metrics**
   - `avg_duration`: Average time per operation
   - `p50/p95/p99`: Percentile latencies
   - `min/max`: Range of observed latencies

2. **Resource Metrics**
   - `cpu.avg`: Average CPU usage percentage
   - `memory.avg`: Average memory usage in MB

3. **Comparison Metrics**
   - `percent_change`: Performance change from baseline
   - `regression`: Flag for significant performance degradation (>10%)

### Interpreting Results

- **Green improvements**: Performance improved by >10%
- **Red regressions**: Performance degraded by >10%
- **Trend charts**: Show performance over multiple runs

## Advanced Usage

### Custom Benchmarks

Create custom benchmarks by extending the framework:

```python
from benchmark_framework import BenchmarkFramework

framework = BenchmarkFramework("Custom Benchmarks")

# Benchmark a function
def my_monitoring_function():
    # Your code here
    pass

framework.time_function(
    my_monitoring_function,
    name="Custom Monitor",
    category="custom",
    iterations=1000
)

# Benchmark with context manager
with framework.benchmark("Complex Operation", "custom", iterations=10) as result:
    for i in range(result.iterations):
        # Your complex operation
        pass

framework.print_summary()
framework.save_results("custom_results.json")
```

### Continuous Benchmarking

Set up automated benchmarking in CI/CD:

```bash
#!/bin/bash
# ci-benchmark.sh

# Run benchmarks
python module-benchmarks.py

# Compare with baseline
if [ -f "baseline_results.json" ]; then
    python report-generator.py --compare baseline_results.json module_benchmark_results.json
    
    # Check for regressions
    if grep -q '"regression": true' comparison_results.json; then
        echo "Performance regression detected!"
        exit 1
    fi
fi
```

### Load Profile Customization

Create custom load profiles:

```python
from load_generator import LoadProfile, LoadGenerator

custom_profile = LoadProfile(
    name="Custom Load",
    duration=300,  # 5 minutes
    connections=30,
    queries_per_second=250,
    read_ratio=0.6,  # 60% reads
    slow_query_ratio=0.2,  # 20% slow queries
    lock_contention_ratio=0.05,
    batch_operations=True,
    long_transactions=True
)

generator = LoadGenerator(db_config)
generator.generate_load(custom_profile)
```

## Troubleshooting

### Common Issues

1. **Connection Errors**
   - Verify database credentials
   - Check maximum connection limits
   - Ensure database is running

2. **Permission Errors**
   - Ensure user has necessary privileges
   - Grant access to information_schema
   - Check for SUPER privilege requirements

3. **Resource Limits**
   - Adjust connection pool size
   - Reduce load profile intensity
   - Monitor system resources during tests

### Debug Mode

Enable detailed logging:

```python
import logging
logging.basicConfig(level=logging.DEBUG)
```

## Best Practices

1. **Warm-up Runs**: Always include warm-up iterations to stabilize performance
2. **Multiple Runs**: Run benchmarks multiple times for statistical significance
3. **Isolation**: Run benchmarks on dedicated test environments
4. **Baseline Updates**: Regularly update baselines as system evolves
5. **Resource Monitoring**: Always monitor system resources during benchmarks

## Contributing

When adding new benchmarks:

1. Follow the existing pattern in `module-benchmarks.py`
2. Use meaningful benchmark names and categories
3. Include appropriate warm-up iterations
4. Document any special requirements
5. Update this README with new benchmark descriptions

## Future Enhancements

- [ ] Automated regression detection in CI/CD
- [ ] Cloud-based benchmark result storage
- [ ] Real-time benchmark dashboards
- [ ] Comparative analysis across versions
- [ ] Performance prediction models
- [ ] Integration with monitoring systems