#!/usr/bin/env python3
"""
Test helpers and utilities for E2E validation
"""

import json
import time
from typing import Any, Dict, List, Optional
from pathlib import Path

import yaml


class MockPrometheusServer:
    """Mock Prometheus server for testing validators"""
    
    def __init__(self, metrics: Dict[str, float]):
        self.metrics = metrics
        self.rules = []
        self.alerts = []
    
    def add_rule(self, name: str, expression: str, labels: Dict[str, str]):
        """Add an alerting rule"""
        self.rules.append({
            "name": name,
            "query": expression,
            "type": "alerting",
            "labels": labels,
            "state": "inactive"
        })
    
    def fire_alert(self, name: str, labels: Dict[str, str]):
        """Fire an alert"""
        self.alerts.append({
            "labels": {"alertname": name, **labels},
            "state": "firing",
            "activeAt": time.time()
        })
    
    def get_metric_families(self):
        """Return metrics in Prometheus format"""
        lines = []
        for metric, value in self.metrics.items():
            lines.append(f"# HELP {metric} Test metric")
            lines.append(f"# TYPE {metric} gauge")
            lines.append(f"{metric} {value}")
        return "\n".join(lines)


class MockGrafanaServer:
    """Mock Grafana server for testing validators"""
    
    def __init__(self):
        self.dashboards = []
        self.datasources = []
    
    def add_dashboard(self, uid: str, title: str, panels: List[Dict[str, Any]]):
        """Add a dashboard"""
        self.dashboards.append({
            "uid": uid,
            "title": title,
            "panels": panels
        })
    
    def add_datasource(self, name: str, type: str, uid: str):
        """Add a datasource"""
        self.datasources.append({
            "name": name,
            "type": type,
            "uid": uid
        })


def create_test_module_config(module_name: str, base_path: Path) -> Path:
    """Create a test module with E2E configuration"""
    module_path = base_path / "modules" / module_name
    module_path.mkdir(parents=True, exist_ok=True)
    
    config = {
        "prometheus_url": "http://localhost:9090",
        "grafana_url": "http://localhost:3000",
        "alertmanager_url": "http://localhost:9093",
        "expected_metrics": {
            f"{module_name}_test_metric": {
                "type": "gauge",
                "required": True,
                "min_value": 0,
                "max_value": 100
            }
        },
        "expected_dashboards": {
            f"{module_name}-dashboard": {
                "title": f"{module_name.title()} Dashboard",
                "required": True,
                "panels": ["Test Panel 1", "Test Panel 2"]
            }
        },
        "expected_alerts": {
            f"{module_name}_alert": {
                "name": f"{module_name.title()}Alert",
                "severity": "warning",
                "required": True,
                "expression": f"{module_name}_test_metric > 80"
            }
        }
    }
    
    config_path = module_path / "e2e-config.yaml"
    with open(config_path, 'w') as f:
        yaml.dump(config, f, default_flow_style=False)
    
    return module_path


def compare_validation_results(results1: List[Any], results2: List[Any]) -> Dict[str, Any]:
    """Compare two sets of validation results"""
    comparison = {
        "total_tests": (len(results1), len(results2)),
        "differences": []
    }
    
    # Create lookup maps
    r1_map = {(r.module, r.test_name): r for r in results1}
    r2_map = {(r.module, r.test_name): r for r in results2}
    
    # Find differences
    all_keys = set(r1_map.keys()) | set(r2_map.keys())
    
    for key in all_keys:
        r1 = r1_map.get(key)
        r2 = r2_map.get(key)
        
        if not r1:
            comparison["differences"].append({
                "test": key,
                "change": "added",
                "new_status": r2.status
            })
        elif not r2:
            comparison["differences"].append({
                "test": key,
                "change": "removed",
                "old_status": r1.status
            })
        elif r1.status != r2.status:
            comparison["differences"].append({
                "test": key,
                "change": "status_changed",
                "old_status": r1.status,
                "new_status": r2.status
            })
    
    return comparison


def generate_test_report(results: List[Any], output_path: Path) -> None:
    """Generate a detailed HTML test report"""
    html = """
<!DOCTYPE html>
<html>
<head>
    <title>E2E Validation Report</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .summary { background: #f0f0f0; padding: 15px; border-radius: 5px; }
        .passed { color: green; }
        .failed { color: red; }
        .skipped { color: orange; }
        table { border-collapse: collapse; width: 100%; margin-top: 20px; }
        th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }
        th { background-color: #4CAF50; color: white; }
        tr:nth-child(even) { background-color: #f2f2f2; }
    </style>
</head>
<body>
    <h1>E2E Validation Report</h1>
    <div class="summary">
        <h2>Summary</h2>
        <p>Total Tests: {total}</p>
        <p class="passed">Passed: {passed}</p>
        <p class="failed">Failed: {failed}</p>
        <p class="skipped">Skipped: {skipped}</p>
    </div>
    
    <h2>Test Results</h2>
    <table>
        <tr>
            <th>Module</th>
            <th>Test Name</th>
            <th>Status</th>
            <th>Message</th>
            <th>Duration (s)</th>
        </tr>
        {rows}
    </table>
</body>
</html>
    """
    
    # Calculate summary
    total = len(results)
    passed = sum(1 for r in results if r.status == "passed")
    failed = sum(1 for r in results if r.status == "failed")
    skipped = sum(1 for r in results if r.status == "skipped")
    
    # Generate table rows
    rows = []
    for result in results:
        status_class = result.status
        rows.append(f"""
        <tr>
            <td>{result.module}</td>
            <td>{result.test_name}</td>
            <td class="{status_class}">{result.status.upper()}</td>
            <td>{result.message}</td>
            <td>{result.duration:.3f}</td>
        </tr>
        """)
    
    # Fill template
    report = html.format(
        total=total,
        passed=passed,
        failed=failed,
        skipped=skipped,
        rows="\n".join(rows)
    )
    
    # Save report
    with open(output_path, 'w') as f:
        f.write(report)


def wait_for_metrics(prometheus_url: str, metric_name: str, timeout: int = 60) -> bool:
    """Wait for a metric to appear in Prometheus"""
    import requests
    
    start_time = time.time()
    while time.time() - start_time < timeout:
        try:
            response = requests.get(
                f"{prometheus_url}/api/v1/query",
                params={"query": metric_name},
                timeout=5
            )
            if response.status_code == 200:
                data = response.json()
                if data["status"] == "success" and data["data"]["result"]:
                    return True
        except Exception:
            pass
        
        time.sleep(2)
    
    return False


def validate_yaml_config(config_path: Path) -> List[str]:
    """Validate a module's E2E configuration file"""
    errors = []
    
    try:
        with open(config_path, 'r') as f:
            config = yaml.safe_load(f)
    except Exception as e:
        return [f"Failed to load config: {e}"]
    
    # Check required fields
    required_fields = ["expected_metrics", "expected_dashboards", "expected_alerts"]
    for field in required_fields:
        if field not in config:
            errors.append(f"Missing required field: {field}")
    
    # Validate metrics
    if "expected_metrics" in config:
        for metric_name, metric_config in config["expected_metrics"].items():
            if "type" not in metric_config:
                errors.append(f"Metric {metric_name} missing 'type' field")
            if metric_config.get("type") not in ["gauge", "counter", "histogram", "summary"]:
                errors.append(f"Metric {metric_name} has invalid type: {metric_config.get('type')}")
    
    # Validate dashboards
    if "expected_dashboards" in config:
        for dash_id, dash_config in config["expected_dashboards"].items():
            if "title" not in dash_config:
                errors.append(f"Dashboard {dash_id} missing 'title' field")
            if "panels" not in dash_config:
                errors.append(f"Dashboard {dash_id} missing 'panels' field")
    
    # Validate alerts
    if "expected_alerts" in config:
        for alert_id, alert_config in config["expected_alerts"].items():
            if "name" not in alert_config:
                errors.append(f"Alert {alert_id} missing 'name' field")
            if "expression" not in alert_config:
                errors.append(f"Alert {alert_id} missing 'expression' field")
    
    return errors


if __name__ == "__main__":
    # Example usage
    import sys
    
    if len(sys.argv) > 1 and sys.argv[1] == "validate":
        # Validate a config file
        if len(sys.argv) < 3:
            print("Usage: test_helpers.py validate <config_file>")
            sys.exit(1)
        
        config_path = Path(sys.argv[2])
        errors = validate_yaml_config(config_path)
        
        if errors:
            print(f"Configuration errors in {config_path}:")
            for error in errors:
                print(f"  - {error}")
            sys.exit(1)
        else:
            print(f"Configuration {config_path} is valid!")
    else:
        print("Test helpers loaded. Use 'validate' command to check config files.")