# E2E Validation Framework

This framework provides comprehensive end-to-end testing for the Database Intelligence Monorepo modules. It validates that metrics are being collected, dashboards are rendering correctly, and alerts are properly configured.

## Overview

The framework consists of:

- **validation-framework.py** - Base framework and abstract classes for all validators
- **metric-validator.py** - Validates Prometheus metrics collection and availability
- **dashboard-validator.py** - Validates Grafana dashboard configuration and rendering
- **alert-validator.py** - Validates Prometheus alerting rules and Alertmanager configuration
- Module-specific configurations in `modules/<module-name>/e2e-config.yaml`

## Features

- **Modular Design**: Each validator focuses on a specific aspect of the monitoring stack
- **Module Support**: Can run tests for individual modules or all modules at once
- **Comprehensive Logging**: Detailed logs with different verbosity levels
- **Result Persistence**: Saves results in JSON and Markdown formats
- **Flexible Configuration**: Module-specific configurations via YAML files

## Installation

Install required dependencies:

```bash
pip install requests prometheus-client pyyaml
```

## Usage

### Running All Tests

Test all modules in the monorepo:

```bash
python validation-framework.py
```

### Running Tests for Specific Modules

Test specific modules only:

```bash
python validation-framework.py --modules anomaly-detector query-insights
```

### Adjusting Log Level

Control verbosity of output:

```bash
python validation-framework.py --log-level DEBUG
```

### Using Custom Base Path

If running from a different location:

```bash
python validation-framework.py --base-path /path/to/database-intelligence-monorepo
```

## Module Configuration

Each module can have an optional `e2e-config.yaml` file in its directory to customize validation settings:

```yaml
# modules/anomaly-detector/e2e-config.yaml
prometheus_url: http://localhost:9090
grafana_url: http://localhost:3000
alertmanager_url: http://localhost:9093

expected_metrics:
  anomaly_detection_enabled:
    type: gauge
    required: true
    min_value: 0
    max_value: 1
  anomalies_detected_total:
    type: counter
    required: true

expected_dashboards:
  anomaly-detection:
    title: "Anomaly Detection"
    required: true
    panels:
      - "Anomaly Timeline"
      - "Detection Status"
      - "Anomaly Types Distribution"

expected_alerts:
  anomaly_detected:
    name: "AnomalyDetected"
    severity: warning
    required: true
    expression: 'increase(anomalies_detected_total[5m]) > 0'
```

## Validators

### Metric Validator

Validates that expected metrics are being collected:

- **Prometheus Availability**: Checks if Prometheus endpoint is accessible
- **Metric Collection**: Verifies required metrics are present
- **Metric Values**: Validates values are within expected ranges
- **Metric Freshness**: Ensures metrics are being updated regularly
- **Custom Metrics**: Validates module-specific metrics

### Dashboard Validator

Validates Grafana dashboards:

- **Grafana Availability**: Checks if Grafana is accessible
- **Dashboard Existence**: Verifies required dashboards exist
- **Panel Configuration**: Validates panel setup and queries
- **Data Sources**: Ensures all panels have valid data sources
- **Query Validation**: Checks for properly formed queries
- **Annotations**: Verifies dashboards have proper annotations

### Alert Validator

Validates alerting configuration:

- **Alert Rules**: Verifies Prometheus alerting rules are configured
- **Alertmanager**: Checks Alertmanager availability and configuration
- **Routing**: Validates alert routing configuration
- **Active Alerts**: Monitors currently firing alerts
- **Alert History**: Reviews recent alert activity

## Output

The framework generates two types of output:

### Results JSON

Detailed test results in JSON format:

```json
{
  "results": [
    {
      "test_name": "prometheus_availability",
      "module": "anomaly-detector",
      "status": "passed",
      "message": "Prometheus endpoint is available",
      "duration": 0.123,
      "timestamp": "2024-01-15T10:30:00",
      "details": {}
    }
  ],
  "summary": {
    "total_tests": 15,
    "passed": 12,
    "failed": 2,
    "skipped": 1,
    "duration": 45.67,
    "modules_tested": 3
  }
}
```

### Summary Markdown

Human-readable summary in Markdown:

```markdown
# E2E Validation Summary

**Date**: 2024-01-15T10:30:00
**Duration**: 45.67 seconds

## Overall Results

- Total Tests: 15
- Passed: 12
- Failed: 2
- Skipped: 1

## Results by Module

### anomaly-detector
- Passed: 5
- Failed: 1
- Skipped: 0
```

## Extending the Framework

To add a new validator:

1. Create a new validator class inheriting from `BaseValidator`
2. Implement the `validate()` method
3. Use `_record_result()` to track test results
4. Register the validator in the main framework

Example:

```python
from validation_framework import BaseValidator, ValidationResult

class CustomValidator(BaseValidator):
    def validate(self) -> List[ValidationResult]:
        self.results = []
        
        # Run your validation tests
        start = time.time()
        
        # Test logic here
        if test_passed:
            self._record_result(
                "custom_test",
                "passed",
                "Custom test passed",
                time.time() - start
            )
        
        return self.results
```

## Best Practices

1. **Module Independence**: Each module should be testable independently
2. **Timeout Handling**: Use appropriate timeouts for HTTP requests
3. **Error Recovery**: Handle failures gracefully and continue testing
4. **Detailed Logging**: Log enough information for debugging failures
5. **Performance**: Avoid expensive operations that could slow down CI/CD

## Integration with CI/CD

The framework returns appropriate exit codes:
- `0` - All tests passed
- `1` - One or more tests failed

This makes it easy to integrate with CI/CD pipelines:

```yaml
# Example GitHub Actions workflow
- name: Run E2E Tests
  run: |
    cd shared/e2e
    python validation-framework.py --modules ${{ matrix.module }}
```

## Troubleshooting

### Common Issues

1. **Connection Refused**: Ensure Prometheus, Grafana, and Alertmanager are running
2. **Missing Metrics**: Check that exporters are configured and scraping is working
3. **Dashboard Not Found**: Verify dashboards are provisioned in Grafana
4. **No Alerts**: Ensure alert rules are loaded in Prometheus

### Debug Mode

Run with debug logging for more details:

```bash
python validation-framework.py --log-level DEBUG
```

### Logs Location

Logs are saved to: `shared/e2e/logs/validation_YYYYMMDD_HHMMSS.log`

## Contributing

When adding new modules:

1. Create an `e2e-config.yaml` in the module directory
2. Define expected metrics, dashboards, and alerts
3. Run the framework to validate your configuration
4. Update this README if adding new validation types